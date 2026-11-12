package cistac

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Pregled zauzeća pokazuje najveće mape i datoteke u korisničkoj mapi.
// Jedno mjerenje pamti veličine mapa do dubinaPamcenjaProstora razina ispod
// izmjerene mape, pa je ulazak u podmape trenutan.
const (
	dubinaPamcenjaProstora = 3
	najviseStavkiProstora  = 50
	trajanjeMjerenja       = 10 * time.Minute
	radnikaMjerenja        = 8
)

// StavkaProstora je jedna mapa ili datoteka u pregledu zauzeća.
type StavkaProstora struct {
	Naziv         string `json:"name"`
	Putanja       string `json:"path"`
	VelicinaBajta int64  `json:"size_bytes"`
	JeMapa        bool   `json:"is_dir"`
}

// IzvjestajProstora opisuje jednu mapu i njezin sadržaj, od najvećeg prema najmanjem.
type IzvjestajProstora struct {
	Putanja        string           `json:"path"`
	Naziv          string           `json:"name"`
	Nadredena      string           `json:"parent"`
	VelicinaBajta  int64            `json:"size_bytes"`
	Stavke         []StavkaProstora `json:"items"`
	SkrivenoStavki int              `json:"hidden_count"`
	SkrivenoBajta  int64            `json:"hidden_bytes"`
	Izmjereno      string           `json:"measured_at"`
}

// mjerenjeProstora je zadnje mjerenje: veličine mapa do zadane dubine ispod korijena.
type mjerenjeProstora struct {
	korijen  string
	velicine map[string]int64
	vrijeme  time.Time
}

var (
	bravaMjerenja    sync.Mutex
	zadnjeMjerenje   mjerenjeProstora
	sadaZaMjerenjeFn = time.Now
)

// IstraziProstor vraća sadržaj mape unutar korisničke mape poredan po veličini.
// Prazna putanja znači samu korisničku mapu; osvjezi traži novo mjerenje.
func IstraziProstor(p Postavke, putanja string, osvjezi bool) (*IzvjestajProstora, error) {
	dom := filepath.Clean(p.KorijenKorisnika)
	cista, err := odrediMapuProstora(dom, putanja)
	if err != nil {
		return nil, err
	}
	mjerenje := mjerenjeZa(cista, osvjezi)
	stavke, err := stavkeMape(cista, mjerenje.velicine)
	if err != nil {
		return nil, fmt.Errorf("mapu %s nije moguće pročitati: %w", cista, err)
	}
	izvj := &IzvjestajProstora{
		Putanja:       cista,
		Naziv:         filepath.Base(cista),
		VelicinaBajta: mjerenje.velicine[cista],
		Izmjereno:     mjerenje.vrijeme.Format("15:04"),
	}
	if cista != dom {
		izvj.Nadredena = filepath.Dir(cista)
	}
	izvj.Stavke, izvj.SkrivenoStavki, izvj.SkrivenoBajta = skratiPopis(stavke, najviseStavkiProstora)
	return izvj, nil
}

// odrediMapuProstora prihvaća samo postojeću mapu u korisničkoj mapi, bez poveznica.
func odrediMapuProstora(dom, putanja string) (string, error) {
	cista := dom
	if putanja != "" {
		cista = filepath.Clean(putanja)
	}
	if cista != dom && !JeUnutarKorijena(cista, dom) {
		return "", fmt.Errorf("mapa nije u vašoj korisničkoj mapi: %s", cista)
	}
	if info, err := os.Lstat(cista); err != nil || !info.IsDir() {
		return "", fmt.Errorf("mapa ne postoji ili nije mapa: %s", cista)
	}
	return cista, nil
}

// mjerenjeZa vraća mjerenje koje sadrži veličine podmapa zadane mape, a mjeri
// iznova kad takvog nema, kad je staro ili kad se traži osvježenje.
func mjerenjeZa(mapa string, osvjezi bool) mjerenjeProstora {
	bravaMjerenja.Lock()
	defer bravaMjerenja.Unlock()
	svjeze := sadaZaMjerenjeFn().Sub(zadnjeMjerenje.vrijeme) < trajanjeMjerenja
	if !osvjezi && svjeze && pokrivaPodmape(zadnjeMjerenje, mapa) {
		return zadnjeMjerenje
	}
	zadnjeMjerenje = mjerenjeProstora{korijen: mapa, velicine: izmjeriMape(mapa), vrijeme: sadaZaMjerenjeFn()}
	return zadnjeMjerenje
}

func pokrivaPodmape(m mjerenjeProstora, mapa string) bool {
	if m.korijen == "" || (mapa != m.korijen && !JeUnutarKorijena(mapa, m.korijen)) {
		return false
	}
	return dubinaIspod(m.korijen, mapa) < dubinaPamcenjaProstora
}

func dubinaIspod(korijen, mapa string) int {
	if mapa == korijen {
		return 0
	}
	rel, _ := filepath.Rel(korijen, mapa) // mapa je provjereno unutar korijena
	return strings.Count(rel, string(filepath.Separator)) + 1
}

// izmjeriMape vraća ukupnu veličinu korijena i svih mapa do dubinaPamcenjaProstora
// ispod njega. Podmape prvog reda mjere se usporedno; poveznice se ne prate.
func izmjeriMape(korijen string) map[string]int64 {
	velicine := map[string]int64{}
	ulazi, err := os.ReadDir(korijen)
	if err != nil {
		return velicine
	}
	var bravaZbroja sync.Mutex
	var wg sync.WaitGroup
	var datotekeKorijena int64
	mjesta := make(chan struct{}, radnikaMjerenja)
	for _, u := range ulazi {
		put := filepath.Join(korijen, u.Name())
		if !u.IsDir() {
			datotekeKorijena += velicinaUlazaPoImenu(put)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			mjesta <- struct{}{}
			djelomicno := izmjeriPodstablo(korijen, put)
			<-mjesta
			bravaZbroja.Lock()
			for mapa, bajta := range djelomicno {
				velicine[mapa] += bajta
			}
			bravaZbroja.Unlock()
		}()
	}
	wg.Wait()
	velicine[korijen] += datotekeKorijena
	return velicine
}

// izmjeriPodstablo zbraja datoteke jedne podmape korijena i dodaje ih korijenu
// te svakoj nadređenoj mapi do dubinaPamcenjaProstora.
func izmjeriPodstablo(korijen, podmapa string) map[string]int64 {
	velicine := map[string]int64{podmapa: 0}
	zbroji := func(put string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil // nedostupne mape i poveznice se preskaču
		}
		bajta := velicinaUlaza(d, nil)
		velicine[korijen] += bajta
		rel, _ := filepath.Rel(korijen, put) // put je uvijek unutar korijena
		dijelovi := strings.Split(rel, string(filepath.Separator))
		mapa := korijen
		for i := 0; i < len(dijelovi)-1 && i < dubinaPamcenjaProstora; i++ {
			mapa = filepath.Join(mapa, dijelovi[i])
			velicine[mapa] += bajta
		}
		return nil
	}
	_ = filepath.WalkDir(podmapa, zbroji) // greške obilaska obrađuje zbroji: preskače nedostupno
	return velicine
}

func velicinaUlazaPoImenu(put string) int64 {
	info, err := os.Lstat(put)
	if err != nil || !info.Mode().IsRegular() {
		return 0
	}
	return velicinaDatoteke(info)
}

// stavkeMape vraća sve stavke mape s veličinama: mape iz mjerenja, datoteke izravno.
func stavkeMape(mapa string, velicine map[string]int64) ([]StavkaProstora, error) {
	ulazi, err := os.ReadDir(mapa)
	if err != nil {
		return nil, err
	}
	stavke := make([]StavkaProstora, 0, len(ulazi))
	for _, u := range ulazi {
		put := filepath.Join(mapa, u.Name())
		st := StavkaProstora{Naziv: u.Name(), Putanja: put, JeMapa: u.IsDir()}
		if st.JeMapa {
			st.VelicinaBajta = velicine[put]
		} else {
			st.VelicinaBajta = velicinaUlazaPoImenu(put)
		}
		stavke = append(stavke, st)
	}
	sort.SliceStable(stavke, func(i, j int) bool { return stavke[i].VelicinaBajta > stavke[j].VelicinaBajta })
	return stavke, nil
}

// skratiPopis zadržava najveće stavke, a ostale samo broji i zbraja.
func skratiPopis(stavke []StavkaProstora, najvise int) ([]StavkaProstora, int, int64) {
	if len(stavke) <= najvise {
		return stavke, 0, 0
	}
	var skrivenoBajta int64
	for _, st := range stavke[najvise:] {
		skrivenoBajta += st.VelicinaBajta
	}
	return stavke[:najvise], len(stavke) - najvise, skrivenoBajta
}

// PrikaziUFinderu otvara Finder s označenom stavkom iz korisničke mape.
// Brisanje ostaje u Finderu: pregled zauzeća ništa ne mijenja na disku.
func PrikaziUFinderu(p Postavke, putanja string) error {
	dom := filepath.Clean(p.KorijenKorisnika)
	cista := filepath.Clean(putanja)
	if cista != dom && !JeUnutarKorijena(cista, dom) {
		return fmt.Errorf("stavka nije u vašoj korisničkoj mapi: %s", cista)
	}
	if _, err := os.Lstat(cista); err != nil {
		return fmt.Errorf("stavka više ne postoji: %s", cista)
	}
	if err := pokreniNaredbu("open", "-R", cista); err != nil {
		return fmt.Errorf("Finder nije otvoren: %w", err)
	}
	return nil
}
