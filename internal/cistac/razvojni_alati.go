package cistac

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AlatZaCiscenje je razvojni alat koji sam zna očistiti svoje spremište.
// Njegova naredba briše trajno, bez Koša, pa sučelje traži potvrdu.
type AlatZaCiscenje struct {
	ID            string `json:"id"`
	Naziv         string `json:"name"`
	Opis          string `json:"description"`
	Naredba       string `json:"command"`
	VelicinaBajta int64  `json:"size_bytes"`
	Procjena      string `json:"estimate"`
	MozeOcistiti  bool   `json:"can_clean"`
}

// naredbaAlata opisuje program i argumente kojima alat čisti svoje spremište.
type naredbaAlata struct {
	program   string
	argumenti []string
}

const rokNaredbeAlata = 10 * time.Minute

var (
	// Aplikacija pokrenuta iz Findera nema PATH ljuske, pa se programi traže na poznatim mjestima.
	mjestaProgramaFn = func(dom string) map[string][]string {
		return map[string][]string{
			"brew": {"/opt/homebrew/bin/brew", "/usr/local/bin/brew"},
			"go":   {"/opt/homebrew/bin/go", "/usr/local/go/bin/go", "/usr/local/bin/go"},
			"pnpm": {filepath.Join(dom, "Library", "pnpm", "pnpm"), "/opt/homebrew/bin/pnpm", "/usr/local/bin/pnpm"},
		}
	}
	pokreniAlatFn = pokreniAlat
	naredbeAlata  = map[string]naredbaAlata{
		"brew":        {"brew", []string{"cleanup", "--prune=all"}},
		"go_modcache": {"go", []string{"clean", "-modcache"}},
		"pnpm":        {"pnpm", []string{"store", "prune"}},
	}
	uzorakBrewProcjene = regexp.MustCompile(`free approximately ([0-9.]+)([KMGT]?B)`)
)

// PostaviAlateZaTest privremeno mijenja mjesta programa i pokretanje naredbi.
func PostaviAlateZaTest(mjesta map[string][]string, pokreni func(string, ...string) ([]byte, error)) func() {
	staraMjesta, staroPokretanje := mjestaProgramaFn, pokreniAlatFn
	mjestaProgramaFn = func(string) map[string][]string { return mjesta }
	pokreniAlatFn = pokreni
	return func() {
		mjestaProgramaFn, pokreniAlatFn = staraMjesta, staroPokretanje
	}
}

func pokreniAlat(program string, argumenti ...string) ([]byte, error) {
	ctx, otkazi := context.WithTimeout(context.Background(), rokNaredbeAlata)
	defer otkazi()
	naredba := exec.CommandContext(ctx, program, argumenti...)
	naredba.Env = append(os.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ENV_HINTS=1")
	return naredba.Output()
}

func pronadiProgram(p Postavke, ime string) string {
	for _, put := range mjestaProgramaFn(p.KorijenKorisnika)[ime] {
		if info, err := os.Stat(put); err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0 {
			return put
		}
	}
	return ""
}

// PronadiRazvojneAlate vraća instalirane alate s procjenom prostora koji mogu osloboditi.
// Alati se provjeravaju usporedno, jer samo pokusno čišćenje Homebrewa traje desetak sekundi.
func PronadiRazvojneAlate(p Postavke) []AlatZaCiscenje {
	otkrivanja := []func(Postavke) (AlatZaCiscenje, bool){otkrijHomebrew, otkrijGoModule, otkrijPnpm}
	pronadeni := make([]*AlatZaCiscenje, len(otkrivanja))
	var wg sync.WaitGroup
	for i, otkrij := range otkrivanja {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if alat, ok := otkrij(p); ok {
				pronadeni[i] = &alat
			}
		}()
	}
	wg.Wait()
	alati := []AlatZaCiscenje{}
	for _, alat := range pronadeni {
		if alat != nil {
			alati = append(alati, *alat)
		}
	}
	return alati
}

func otkrijHomebrew(p Postavke) (AlatZaCiscenje, bool) {
	program := pronadiProgram(p, "brew")
	if program == "" {
		return AlatZaCiscenje{}, false
	}
	alat := AlatZaCiscenje{
		ID:      "brew",
		Naziv:   "Homebrew: stare verzije i preuzimanja",
		Opis:    "Stare verzije paketa i preuzete arhive koje Homebrew više ne treba.",
		Naredba: "brew cleanup --prune=all",
	}
	izlaz, err := pokreniAlatFn(program, "cleanup", "--prune=all", "-n")
	if err != nil {
		alat.Procjena = "Procjena nije dostupna, ali naredbu možete pokrenuti."
		alat.MozeOcistiti = true
		return alat, true
	}
	alat.VelicinaBajta = procjenaHomebrewa(string(izlaz))
	alat.MozeOcistiti = alat.VelicinaBajta > 0
	alat.Procjena = "Trenutačno nema ničega za ukloniti."
	if alat.MozeOcistiti {
		alat.Procjena = "Oslobodit će oko " + FormatirajBajte(alat.VelicinaBajta) + "."
	}
	return alat, true
}

// procjenaHomebrewa čita redak "would free approximately 1.2GB" iz pokusnog čišćenja.
func procjenaHomebrewa(izlaz string) int64 {
	m := uzorakBrewProcjene.FindStringSubmatch(izlaz)
	if m == nil {
		return 0
	}
	broj, _ := strconv.ParseFloat(m[1], 64) // regex propušta samo znamenke i točku
	jedinice := map[string]float64{"B": 1, "KB": 1 << 10, "MB": 1 << 20, "GB": 1 << 30, "TB": 1 << 40}
	return int64(broj * jedinice[m[2]])
}

func otkrijGoModule(p Postavke) (AlatZaCiscenje, bool) {
	mapa, ok := mapaIzAlata(p, "go", "env", "GOMODCACHE")
	if !ok {
		return AlatZaCiscenje{}, false
	}
	velicina := IzracunajVelicinu(mapa)
	return AlatZaCiscenje{
		ID:            "go_modcache",
		Naziv:         "Go moduli",
		Opis:          "Preuzeti Go moduli. Projekti ih ponovno preuzmu pri sljedećoj izgradnji, za što treba internet.",
		Naredba:       "go clean -modcache",
		VelicinaBajta: velicina,
		Procjena:      "Oslobodit će oko " + FormatirajBajte(velicina) + ".",
		MozeOcistiti:  velicina > 0,
	}, true
}

func otkrijPnpm(p Postavke) (AlatZaCiscenje, bool) {
	mapa, ok := mapaIzAlata(p, "pnpm", "store", "path")
	if !ok {
		return AlatZaCiscenje{}, false
	}
	velicina := IzracunajVelicinu(mapa)
	return AlatZaCiscenje{
		ID:            "pnpm",
		Naziv:         "pnpm: nekorišteni paketi",
		Opis:          "Briše samo pakete iz spremišta koje nijedan projekt više ne koristi.",
		Naredba:       "pnpm store prune",
		VelicinaBajta: velicina,
		Procjena:      "Spremište zauzima " + FormatirajBajte(velicina) + ", a oslobodit će se samo nekorišteni dio.",
		MozeOcistiti:  velicina > 0,
	}, true
}

// mapaIzAlata pita alat gdje mu je spremište i vraća putanju ako je apsolutna.
func mapaIzAlata(p Postavke, ime string, argumenti ...string) (string, bool) {
	program := pronadiProgram(p, ime)
	if program == "" {
		return "", false
	}
	izlaz, err := pokreniAlatFn(program, argumenti...)
	mapa := strings.TrimSpace(string(izlaz))
	if err != nil || !filepath.IsAbs(mapa) {
		return "", false
	}
	return mapa, true
}

// OcistiRazvojniAlat pokreće naredbu alata i javlja koliko je slobodnog prostora dobiveno.
func OcistiRazvojniAlat(p Postavke, id string) (string, error) {
	naredba, ok := naredbeAlata[id]
	if !ok {
		return "", fmt.Errorf("nepoznat alat: %s", id)
	}
	program := pronadiProgram(p, naredba.program)
	if program == "" {
		return "", fmt.Errorf("program %s nije pronađen na ovom Macu", naredba.program)
	}
	opis := naredba.program + " " + strings.Join(naredba.argumenti, " ")
	prije := DohvatiStanjeDiska(p.KorijenKorisnika).SlobodnoBajta
	if _, err := pokreniAlatFn(program, naredba.argumenti...); err != nil {
		return "", fmt.Errorf("naredba %s nije uspjela: %s", opis, opisGreskeNaredbe(err))
	}
	dobiveno := max(DohvatiStanjeDiska(p.KorijenKorisnika).SlobodnoBajta-prije, 0)
	return fmt.Sprintf("Naredba %s je završila. Slobodnog prostora ima %s više.", opis, FormatirajBajte(dobiveno)), nil
}

// opisGreskeNaredbe dodaje zadnji redak koji je alat ispisao, jer on obično kaže zašto.
func opisGreskeNaredbe(err error) string {
	var greskaIzlaza *exec.ExitError
	if errors.As(err, &greskaIzlaza) {
		retci := strings.Split(strings.TrimSpace(string(greskaIzlaza.Stderr)), "\n")
		if zadnji := strings.TrimSpace(retci[len(retci)-1]); zadnji != "" {
			return zadnji
		}
	}
	return err.Error()
}
