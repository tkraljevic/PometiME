// Package cistac sadrži logiku za pregled stabla datoteka i mapa stavki za čišćenje.
package cistac

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// najviseCvorovaStabla je broj najvećih stavki koje stablo prikazuje po mapi.
const najviseCvorovaStabla = 40

// CvorStabla predstavlja datoteku ili mapu unutar stabla stavke.
type CvorStabla struct {
	Naziv         string       `json:"name"`
	Putanja       string       `json:"path"`
	VelicinaBajta int64        `json:"size_bytes"`
	JeDirektorij  bool         `json:"is_dir"`
	Djeca         []CvorStabla `json:"children,omitempty"`
}

// IzvjestajStabla sadrži informacije o korijenu i stablu podstavki.
type IzvjestajStabla struct {
	Putanja       string       `json:"path"`
	Naziv         string       `json:"name"`
	VelicinaBajta int64        `json:"size_bytes"`
	BrojStavki    int          `json:"item_count"`
	Stavke        []CvorStabla `json:"items"`
}

// PregledajStablo stvara stablo datoteka i mapa za zadanu putanju do dubine 2.
func PregledajStablo(p Postavke, putanja string) (*IzvjestajStabla, error) {
	cistaPutanja := filepath.Clean(putanja)
	if !jePutanjaDozvoljenaZaPregled(p, cistaPutanja) {
		return nil, fmt.Errorf("putanja nije dopuštena za pregled: %s", cistaPutanja)
	}

	st, err := os.Stat(cistaPutanja)
	if err != nil {
		return nil, fmt.Errorf("čitanje putanje: %w", err)
	}

	if !st.IsDir() {
		return stvoriIzvjestajZaDatoteku(cistaPutanja, st), nil
	}

	return stvoriIzvjestajZaMapu(cistaPutanja), nil
}

// jePutanjaDozvoljenaZaPregled traži da putanja bude sama mapa ili unutar nje:
// /Users/ana ne smije propustiti /Users/ana-drugi.
func jePutanjaDozvoljenaZaPregled(p Postavke, putanja string) bool {
	korijeni := []string{filepath.Clean(p.KorijenKorisnika), "/Applications", "/Library/Application Support/com.apple.idleassetsd"}
	for _, korijen := range korijeni {
		if putanja == korijen || JeUnutarKorijena(putanja, korijen) {
			return true
		}
	}
	return false
}

func stvoriIzvjestajZaDatoteku(putanja string, st os.FileInfo) *IzvjestajStabla {
	return &IzvjestajStabla{
		Putanja:       putanja,
		Naziv:         filepath.Base(putanja),
		VelicinaBajta: velicinaDatoteke(st),
		BrojStavki:    1,
		Stavke: []CvorStabla{
			{
				Naziv:         filepath.Base(putanja),
				Putanja:       putanja,
				VelicinaBajta: velicinaDatoteke(st),
				JeDirektorij:  false,
			},
		},
	}
}

func stvoriIzvjestajZaMapu(putanja string) *IzvjestajStabla {
	djeca := citajDjecuMape(putanja, 1, 2)
	return &IzvjestajStabla{
		Putanja:       putanja,
		Naziv:         filepath.Base(putanja),
		VelicinaBajta: IzracunajVelicinu(putanja),
		BrojStavki:    len(djeca),
		Stavke:        djeca,
	}
}

func citajDjecuMape(mapa string, dubina, maxDubina int) []CvorStabla {
	entries, err := os.ReadDir(mapa)
	if err != nil {
		return nil
	}

	var cvorovi []CvorStabla
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue // preskaču se skrivene sistemske datoteke
		}
		cvorovi = append(cvorovi, obradiJedanUlaz(mapa, e, dubina, maxDubina))
	}
	// Prvo se sortira, pa reže: inače bi ostalo prvih 40 po abecedi, a ne najvećih 40.
	sortirajCvorovePoVelicini(cvorovi)
	if len(cvorovi) > najviseCvorovaStabla {
		cvorovi = cvorovi[:najviseCvorovaStabla]
	}
	return cvorovi
}

func obradiJedanUlaz(mapa string, e os.DirEntry, dubina, maxDubina int) CvorStabla {
	punPutanja := filepath.Join(mapa, e.Name())
	jeDir := e.IsDir()
	var velicina int64
	var djeca []CvorStabla

	switch {
	case jeDir && dubina < maxDubina:
		djeca = citajDjecuMape(punPutanja, dubina+1, maxDubina)
		velicina = IzracunajVelicinu(punPutanja)
	case jeDir:
		velicina = IzracunajVelicinu(punPutanja) // mapa na zadnjoj razini prikazuje stvarnu veličinu, ne 0 B
	default:
		velicina = velicinaUlaza(e, nil)
	}

	return CvorStabla{
		Naziv:         e.Name(),
		Putanja:       punPutanja,
		VelicinaBajta: velicina,
		JeDirektorij:  jeDir,
		Djeca:         djeca,
	}
}

func sortirajCvorovePoVelicini(cvorovi []CvorStabla) {
	sort.Slice(cvorovi, func(i, j int) bool {
		return cvorovi[i].VelicinaBajta > cvorovi[j].VelicinaBajta
	})
}

// PregledajStabloAplikacije stvara stablo za aplikaciju i njezine pripadajuće putanje.
func PregledajStabloAplikacije(app InstaliranaAplikacija) *IzvjestajStabla {
	var stavke []CvorStabla
	stavke = append(stavke, CvorStabla{
		Naziv:         filepath.Base(app.PutanjaAplikacije),
		Putanja:       app.PutanjaAplikacije,
		VelicinaBajta: app.VelicinaAplikacije,
		JeDirektorij:  true,
	})

	for _, rel := range app.PovezanePutanje {
		stavke = append(stavke, CvorStabla{
			Naziv:         fmt.Sprintf("[%s] %s", rel.Vrsta, filepath.Base(rel.Putanja)),
			Putanja:       rel.Putanja,
			VelicinaBajta: rel.VelicinaBajta,
			JeDirektorij:  true,
		})
	}
	sortirajCvorovePoVelicini(stavke)

	return &IzvjestajStabla{
		Putanja:       app.PutanjaAplikacije,
		Naziv:         app.Naziv,
		VelicinaBajta: app.UkupnaVelicina,
		BrojStavki:    len(stavke),
		Stavke:        stavke,
	}
}
