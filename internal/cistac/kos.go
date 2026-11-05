package cistac

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const datotekaPovijestiKosa = ".pometime_povijest.json"

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
		VrijemeBrisanja: time.Now().Format("02.01.2006. 15:04:05"),
	}
	dodajUPovijestKosa(p, stavka)
	return nil
}

func dodajUPovijestKosa(p Postavke, nova StavkaKosa) {
	postojece := IzlistajKos(p)
	noviPopis := make([]StavkaKosa, 0, 1+len(postojece))
	noviPopis = append(noviPopis, nova)
	noviPopis = append(noviPopis, postojece...)
	spremiPovijestKosa(p, noviPopis)
}

func spremiPovijestKosa(p Postavke, stavke []StavkaKosa) {
	podaci, err := json.MarshalIndent(stavke, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(putanjaPovijesti(p), podaci, 0o600) // evidencija povrata ne smije prekinuti već izvršenu operaciju u Košu
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
			prisutne = append(prisutne, st)
		}
	}
	return prisutne
}

// VratiIzKosa vraća odabrane stavke iz ~/.Trash natrag na njihove izvorne putanje.
func VratiIzKosa(p Postavke, putanjeUKosu []string) RezultatCiscenja {
	trazene := skupPutanja(putanjeUKosu)
	povijest := IzlistajKos(p)
	var preostale []StavkaKosa
	var vraceno int
	var vracenoBajta int64
	var greske []string

	for _, st := range povijest {
		if !trazene[st.PutanjaUKosu] {
			preostale = append(preostale, st)
			continue
		}
		if err := vratiJednuStavkuIzKosa(p, st); err != nil {
			greske = append(greske, err.Error())
			preostale = append(preostale, st)
			continue
		}
		vraceno++
		vracenoBajta += st.VelicinaBajta
	}

	spremiPovijestKosa(p, preostale)
	return RezultatCiscenja{
		ObrisanoStavki:  vraceno,
		OslobodenoBajta: vracenoBajta,
		Poruka:          PorukaPovrata(vraceno, vracenoBajta),
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
