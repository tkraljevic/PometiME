package cistac

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// lazniProgram stvara izvršnu datoteku; sadržaj nije važan jer se pokretanje zamjenjuje.
func lazniProgram(t *testing.T, put string) string {
	t.Helper()
	napraviDatoteku(t, put, 1)
	if err := os.Chmod(put, 0o755); err != nil {
		t.Fatal(err)
	}
	return put
}

type pozivAlata struct {
	program   string
	argumenti string
}

// postaviLazneAlate pokreće lažne brew, go i pnpm; izlazi odgovaraju pravim alatima.
func postaviLazneAlate(t *testing.T, brewIzlaz string, brewGreska error) (string, string, *[]pozivAlata) {
	t.Helper()
	mapa := t.TempDir()
	modcache := filepath.Join(mapa, "go", "pkg", "mod")
	store := filepath.Join(mapa, "pnpm-store")
	napraviDatoteku(t, filepath.Join(modcache, "modul.zip"), 50000)
	napraviDatoteku(t, filepath.Join(store, "paket.tgz"), 20000)
	mjesta := map[string][]string{
		"brew": {filepath.Join(mapa, "nema", "brew"), lazniProgram(t, filepath.Join(mapa, "bin", "brew"))},
		"go":   {lazniProgram(t, filepath.Join(mapa, "bin", "go"))},
		"pnpm": {lazniProgram(t, filepath.Join(mapa, "bin", "pnpm"))},
	}
	pozivi := &[]pozivAlata{}
	var brava sync.Mutex // alati se provjeravaju usporedno
	t.Cleanup(PostaviAlateZaTest(mjesta, func(program string, argumenti ...string) ([]byte, error) {
		brava.Lock()
		*pozivi = append(*pozivi, pozivAlata{filepath.Base(program), strings.Join(argumenti, " ")})
		brava.Unlock()
		switch filepath.Base(program) + " " + strings.Join(argumenti, " ") {
		case "brew cleanup --prune=all -n":
			return []byte(brewIzlaz), brewGreska
		case "go env GOMODCACHE":
			return []byte(modcache + "\n"), nil
		case "pnpm store path":
			return []byte(store + "\n"), nil
		}
		return nil, nil
	}))
	return modcache, store, pozivi
}

func TestPronadiRazvojneAlate(t *testing.T) {
	modcache, store, _ := postaviLazneAlate(t, "Would remove: x\n==> This operation would free approximately 1.5GB of disk space.\n", nil)
	alati := PronadiRazvojneAlate(ZadanePostavke(t.TempDir()))
	if len(alati) != 3 {
		t.Fatalf("očekivana 3 alata, dobiveno %+v", alati)
	}
	if alati[0].ID != "brew" || alati[0].VelicinaBajta != 3<<29 || !alati[0].MozeOcistiti || !strings.Contains(alati[0].Procjena, "1.50 GB") {
		t.Errorf("neočekivan Homebrew: %+v", alati[0])
	}
	if alati[1].ID != "go_modcache" || alati[1].VelicinaBajta != IzracunajVelicinu(modcache) || !alati[1].MozeOcistiti {
		t.Errorf("neočekivani Go moduli: %+v", alati[1])
	}
	if alati[2].ID != "pnpm" || alati[2].VelicinaBajta != IzracunajVelicinu(store) || !strings.Contains(alati[2].Procjena, "nekorišteni") {
		t.Errorf("neočekivan pnpm: %+v", alati[2])
	}
}

func TestHomebrewBezProcjene(t *testing.T) {
	postaviLazneAlate(t, "", nil)
	if alat, _ := otkrijHomebrew(ZadanePostavke(t.TempDir())); alat.MozeOcistiti || !strings.Contains(alat.Procjena, "nema ničega") {
		t.Errorf("Homebrew bez ičega za ukloniti ne nudi čišćenje: %+v", alat)
	}
	postaviLazneAlate(t, "", errors.New("brew je zauzet"))
	if alat, _ := otkrijHomebrew(ZadanePostavke(t.TempDir())); !alat.MozeOcistiti || !strings.Contains(alat.Procjena, "nije dostupna") {
		t.Errorf("neuspjela procjena i dalje dopušta čišćenje: %+v", alat)
	}
	for izlaz, ocekivano := range map[string]int64{
		"would free approximately 12KB of":  12 << 10,
		"would free approximately 2.5MB of": 5 << 19,
		"would free approximately 7B of":    7,
		"nešto drugo":                       0,
	} {
		if v := procjenaHomebrewa(izlaz); v != ocekivano {
			t.Errorf("procjenaHomebrewa(%q) = %d, očekivano %d", izlaz, v, ocekivano)
		}
	}
}

func TestBezInstaliranihAlata(t *testing.T) {
	mapa := t.TempDir()
	napraviDatoteku(t, filepath.Join(mapa, "nije-izvrsna"), 1)
	t.Cleanup(PostaviAlateZaTest(map[string][]string{
		"brew": {filepath.Join(mapa, "nije-izvrsna")},
		"go":   {mapa},
	}, func(string, ...string) ([]byte, error) { return []byte("relativna/putanja"), nil }))
	if alati := PronadiRazvojneAlate(ZadanePostavke(mapa)); len(alati) != 0 {
		t.Errorf("bez izvršnih programa nema alata, dobiveno %+v", alati)
	}
	if _, ok := mapaIzAlata(ZadanePostavke(mapa), "pnpm", "store", "path"); ok {
		t.Errorf("program koji ne postoji ne daje mapu")
	}

	// Alat postoji, ali vrati relativnu putanju: takvoj mapi se ne vjeruje.
	t.Cleanup(PostaviAlateZaTest(map[string][]string{"go": {lazniProgram(t, filepath.Join(mapa, "bin", "go"))}},
		func(string, ...string) ([]byte, error) { return []byte("relativna/putanja\n"), nil }))
	if _, ok := mapaIzAlata(ZadanePostavke(mapa), "go", "env", "GOMODCACHE"); ok {
		t.Errorf("relativna putanja ne smije se prihvatiti kao mapa alata")
	}
}

func TestOcistiRazvojniAlat(t *testing.T) {
	_, _, pozivi := postaviLazneAlate(t, "", nil)
	p := ZadanePostavke(t.TempDir())
	poruka, err := OcistiRazvojniAlat(p, "go_modcache")
	if err != nil || !strings.Contains(poruka, "go clean -modcache je završila") {
		t.Fatalf("očekivano uspješno čišćenje, dobiveno %q (%v)", poruka, err)
	}
	if zadnji := (*pozivi)[len(*pozivi)-1]; zadnji.program != "go" || zadnji.argumenti != "clean -modcache" {
		t.Errorf("pokrenuta je kriva naredba: %+v", zadnji)
	}
	if _, err := OcistiRazvojniAlat(p, "rm -rf"); err == nil {
		t.Errorf("nepoznat alat mora biti odbijen")
	}

	t.Cleanup(PostaviAlateZaTest(map[string][]string{}, nil))
	if _, err := OcistiRazvojniAlat(p, "brew"); err == nil || !strings.Contains(err.Error(), "nije pronađen") {
		t.Errorf("očekivana greška za program koji ne postoji, dobiveno %v", err)
	}

	mapa := t.TempDir()
	t.Cleanup(PostaviAlateZaTest(map[string][]string{"pnpm": {lazniProgram(t, filepath.Join(mapa, "pnpm"))}},
		func(string, ...string) ([]byte, error) { return nil, errors.New("spremište je zaključano") }))
	if _, err := OcistiRazvojniAlat(p, "pnpm"); err == nil || !strings.Contains(err.Error(), "spremište je zaključano") {
		t.Errorf("greška alata mora doći do korisnika, dobiveno %v", err)
	}
}

func TestPokretanjeIGreskeNaredbe(t *testing.T) {
	if izlaz, err := pokreniAlat("/bin/echo", "radi"); err != nil || string(izlaz) != "radi\n" {
		t.Errorf("očekivan izlaz naredbe, dobiveno %q (%v)", izlaz, err)
	}
	_, err := exec.Command("/bin/sh", "-c", "echo prvi redak >&2; echo 'Error: nema mjesta na disku' >&2; exit 2").Output()
	if opis := opisGreskeNaredbe(err); opis != "Error: nema mjesta na disku" {
		t.Errorf("očekivan zadnji redak greške, dobiveno %q", opis)
	}
	_, err = exec.Command("/bin/sh", "-c", "exit 1").Output()
	if opis := opisGreskeNaredbe(err); !strings.Contains(opis, "exit status 1") {
		t.Errorf("bez ispisa vrijedi poruka o izlaznom kodu, dobiveno %q", opis)
	}
}
