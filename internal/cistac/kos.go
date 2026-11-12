package cistac

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const datotekaPovijestiKosa = ".pometime_povijest.json"

// formatVremenaBrisanja je oblik u kojem povijest Koša pamti vrijeme premještanja.
const formatVremenaBrisanja = "02.01.2006. 15:04:05"

// bravaKosa štiti povijest Koša: čita se i sprema cijela, pa bi istodobni
// zahtjevi inače gazili zapise jedan drugome.
var bravaKosa sync.Mutex

func mapaKosa(p Postavke) string {
	return filepath.Join(filepath.Clean(p.KorijenKorisnika), ".Trash")
}

func putanjaPovijesti(p Postavke) string {
	return filepath.Join(mapaKosa(p), datotekaPovijestiKosa)
}

func odrediCiljUKosu(kos, osnovnoIme string) string {
	kandidat := filepath.Join(kos, osnovnoIme)
	if _, err := os.Lstat(kandidat); os.IsNotExist(err) {
		return kandidat
	}
	vremenskaOznaka := time.Now().Format("20060102_150405.000")
	return filepath.Join(kos, fmt.Sprintf("%s_%s", osnovnoIme, vremenskaOznaka))
}

// PremjestiUKos premješta zadanu putanju u korisnički ~/.Trash i bilježi je u povijest za povrat.
func PremjestiUKos(p Postavke, cistaPutanja string, velicina int64) error {
	kos := mapaKosa(p)
	if err := os.MkdirAll(kos, 0o700); err != nil {
		return fmt.Errorf("priprema mape Koša %s: %w", kos, err)
	}
	cilj := odrediCiljUKosu(kos, filepath.Base(cistaPutanja))
	if err := os.Rename(cistaPutanja, cilj); err != nil {
		return fmt.Errorf("premještanje u Koš %s: %w", cistaPutanja, err)
	}
	stavka := StavkaKosa{
		ID:              cilj,
		Naziv:           filepath.Base(cistaPutanja),
		IzvornaPutanja:  cistaPutanja,
		PutanjaUKosu:    cilj,
		VelicinaBajta:   velicina,
		VrijemeBrisanja: time.Now().Format(formatVremenaBrisanja),
	}
	dodajUPovijestKosa(p, stavka)
	return nil
}

func dodajUPovijestKosa(p Postavke, nova StavkaKosa) {
	bravaKosa.Lock()
	defer bravaKosa.Unlock()
	postojece := IzlistajKos(p)
	noviPopis := make([]StavkaKosa, 0, 1+len(postojece))
	noviPopis = append(noviPopis, nova)
	noviPopis = append(noviPopis, postojece...)
	spremiPovijestKosa(p, noviPopis)
}

// spremiPovijestKosa zapisuje privremenu datoteku i preimenuje je, pa čitatelj
// nikad ne vidi napola zapisanu povijest.
func spremiPovijestKosa(p Postavke, stavke []StavkaKosa) {
	podaci, err := json.MarshalIndent(stavke, "", "  ")
	if err != nil {
		return
	}
	privremena := putanjaPovijesti(p) + ".novo"
	if err := os.WriteFile(privremena, podaci, 0o600); err != nil {
		return
	}
	_ = os.Rename(privremena, putanjaPovijesti(p)) // evidencija povrata ne smije prekinuti već izvršenu operaciju u Košu
}

// IzlistajKos vraća sve stavke u ~/.Trash koje je PometiME premjestio i koje se još nalaze u Košu.
func IzlistajKos(p Postavke) []StavkaKosa {
	sadrzaj, err := os.ReadFile(putanjaPovijesti(p))
	if err != nil {
		return []StavkaKosa{}
	}
	var sve []StavkaKosa
	if err := json.Unmarshal(sadrzaj, &sve); err != nil {
		return []StavkaKosa{}
	}
	prisutne := make([]StavkaKosa, 0, len(sve))
	for _, st := range sve {
		if _, statErr := os.Lstat(st.PutanjaUKosu); statErr == nil {
			st.DanaUKosu = danaOd(st.VrijemeBrisanja, time.Now())
			prisutne = append(prisutne, st)
		}
	}
	return prisutne
}

// danaOd vraća broj punih dana od zapisanog vremena; nečitljivo vrijeme daje 0.
func danaOd(zapis string, sada time.Time) int {
	vrijeme, err := time.ParseInLocation(formatVremenaBrisanja, zapis, time.Local)
	if err != nil || vrijeme.After(sada) {
		return 0
	}
	return int(sada.Sub(vrijeme).Hours() / 24)
}

// obradiStavkeKosa primjenjuje radnju na tražene stavke iz povijesti i sprema
// povijest bez stavki koje su uspješno obrađene.
func obradiStavkeKosa(p Postavke, putanjeUKosu []string, radnja func(Postavke, StavkaKosa) error) (int, int64, []string) {
	bravaKosa.Lock()
	defer bravaKosa.Unlock()
	trazene := skupPutanja(putanjeUKosu)
	var preostale []StavkaKosa
	var obradeno int
	var obradenoBajta int64
	var greske []string

	for _, st := range IzlistajKos(p) {
		if !trazene[st.PutanjaUKosu] {
			preostale = append(preostale, st)
			continue
		}
		if err := radnja(p, st); err != nil {
			greske = append(greske, err.Error())
			preostale = append(preostale, st)
			continue
		}
		obradeno++
		obradenoBajta += st.VelicinaBajta
	}
	spremiPovijestKosa(p, preostale)
	return obradeno, obradenoBajta, greske
}

// VratiIzKosa vraća odabrane stavke iz ~/.Trash natrag na njihove izvorne putanje.
func VratiIzKosa(p Postavke, putanjeUKosu []string) RezultatCiscenja {
	vraceno, bajta, greske := obradiStavkeKosa(p, putanjeUKosu, vratiJednuStavkuIzKosa)
	return RezultatCiscenja{
		ObrisanoStavki:  vraceno,
		OslobodenoBajta: bajta,
		Poruka:          PorukaPovrata(vraceno, bajta),
		Greske:          greske,
		Disk:            DohvatiStanjeDiska(p.KorijenKorisnika),
	}
}

// TrajnoIzbrisiIzKosa trajno briše odabrane stavke i tek tada oslobađa prostor.
// Briše samo stavke iz povijesti PometiME-a, pa ne dira ništa drugo u Košu.
func TrajnoIzbrisiIzKosa(p Postavke, putanjeUKosu []string) RezultatCiscenja {
	izbrisano, bajta, greske := obradiStavkeKosa(p, putanjeUKosu, izbrisiJednuStavkuIzKosa)
	return RezultatCiscenja{
		ObrisanoStavki:  izbrisano,
		OslobodenoBajta: bajta,
		Poruka:          PorukaTrajnogBrisanja(izbrisano, bajta),
		Greske:          greske,
		Disk:            DohvatiStanjeDiska(p.KorijenKorisnika),
	}
}

func skupPutanja(putanje []string) map[string]bool {
	izlaz := make(map[string]bool, len(putanje))
	for _, put := range putanje {
		izlaz[filepath.Clean(put)] = true
	}
	return izlaz
}

func vratiJednuStavkuIzKosa(p Postavke, st StavkaKosa) error {
	if !JeUnutarKorijena(filepath.Clean(st.PutanjaUKosu), mapaKosa(p)) || !JeSigurnaPutanja(p, st.IzvornaPutanja) {
		return fmt.Errorf("nevaljana putanja za povrat: %s", st.IzvornaPutanja)
	}
	if err := os.MkdirAll(filepath.Dir(st.IzvornaPutanja), 0o755); err != nil {
		return fmt.Errorf("stvaranje izvorne mape za %s: %w", st.IzvornaPutanja, err)
	}
	if err := os.Rename(st.PutanjaUKosu, st.IzvornaPutanja); err != nil {
		return fmt.Errorf("povrat %s: %w", st.Naziv, err)
	}
	return nil
}

func izbrisiJednuStavkuIzKosa(p Postavke, st StavkaKosa) error {
	cista := filepath.Clean(st.PutanjaUKosu)
	if !JeUnutarKorijena(cista, mapaKosa(p)) {
		return fmt.Errorf("stavka %s nije u Košu, pa nije izbrisana", st.Naziv)
	}
	if err := ukloniTrajno(cista); err != nil {
		return fmt.Errorf("trajno brisanje %s: %w", st.Naziv, err)
	}
	return nil
}

// ukloniTrajno briše stavku i kad sadrži mape samo za čitanje, kakve ostavlja
// npr. Go modcache. Simboličke poveznice briše same, bez odredišta.
func ukloniTrajno(putanja string) error {
	if err := os.RemoveAll(putanja); err == nil {
		return nil
	}
	_ = filepath.WalkDir(putanja, dopustiPisanjeMapi) // mape samo za čitanje dobivaju dozvolu pisanja za vlasnika
	return os.RemoveAll(putanja)
}

func dopustiPisanjeMapi(put string, d fs.DirEntry, err error) error {
	if err != nil || !d.IsDir() {
		return nil
	}
	if info, errInfo := d.Info(); errInfo == nil {
		_ = os.Chmod(put, info.Mode().Perm()|0o700) // vlasnik smije čitati, pisati i ući u mapu
	}
	return nil
}
