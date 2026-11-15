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

// lazniOdgovor je ono što lažni alat ispiše na jednu naredbu.
type lazniOdgovor struct {
	izlaz  string
	greska error
}

// postaviLazneAlate instalira lažne programe i bilježi svaki njihov poziv.
// Ključ odgovora je "program argumenti", kao u ljusci.
func postaviLazneAlate(t *testing.T, mapa string, programi []string, odgovori map[string]lazniOdgovor) *[]pozivAlata {
	t.Helper()
	mjesta := map[string][]string{}
	for _, ime := range programi {
		mjesta[ime] = []string{filepath.Join(mapa, "nema", ime), lazniProgram(t, filepath.Join(mapa, "bin", ime))}
	}
	pozivi := &[]pozivAlata{}
	var brava sync.Mutex // alati se provjeravaju usporedno
	t.Cleanup(PostaviAlateZaTest(mjesta, func(program string, argumenti ...string) ([]byte, error) {
		brava.Lock()
		*pozivi = append(*pozivi, pozivAlata{filepath.Base(program), strings.Join(argumenti, " ")})
		brava.Unlock()
		odg := odgovori[filepath.Base(program)+" "+strings.Join(argumenti, " ")]
		return []byte(odg.izlaz), odg.greska
	}))
	return pozivi
}

func opisAlataPoID(t *testing.T, id string) opisAlata {
	t.Helper()
	for _, opis := range popisAlata() {
		if opis.id == id {
			return opis
		}
	}
	t.Fatalf("alat %s nije u popisu", id)
	return opisAlata{}
}

func TestPronadiRazvojneAlate(t *testing.T) {
	mapa := t.TempDir()
	p := ZadanePostavke(filepath.Join(mapa, "dom"))
	gocache := filepath.Join(mapa, "go-build")
	modcache := filepath.Join(mapa, "go", "pkg", "mod")
	npmMapa := filepath.Join(mapa, "npm")
	store := filepath.Join(mapa, "pnpm-store")
	pip := filepath.Join(mapa, "pip")
	lint := filepath.Join(mapa, "golangci-lint")
	pods := filepath.Join(p.KorijenKorisnika, "Library", "Caches", "CocoaPods")
	napraviDatoteku(t, filepath.Join(gocache, "aa", "f-d"), 30000)
	napraviDatoteku(t, filepath.Join(modcache, "modul.zip"), 50000)
	napraviDatoteku(t, filepath.Join(npmMapa, "_cacache", "index"), 10000)
	napraviDatoteku(t, filepath.Join(npmMapa, "_npx", "abc", "package.json"), 7000)
	napraviDatoteku(t, filepath.Join(store, "paket.tgz"), 20000)
	napraviDatoteku(t, filepath.Join(pip, "http-v2", "a"), 4000)
	napraviDatoteku(t, filepath.Join(lint, "ab", "f"), 1000)
	napraviDatoteku(t, filepath.Join(pods, "Pods", "pod.zip"), 3000)

	postaviLazneAlate(t, mapa,
		[]string{"brew", "go", "npm", "pnpm", "yarn", "pip3", "uv", "pod", "swift", "golangci-lint"},
		map[string]lazniOdgovor{
			"brew cleanup --prune=all -n": {izlaz: "Would remove: x\n==> This operation would free approximately 1.5GB of disk space.\n"},
			"go env GOCACHE":              {izlaz: gocache + "\n"},
			"go env GOMODCACHE":           {izlaz: modcache + "\n"},
			"npm config get cache":        {izlaz: npmMapa + "\n"},
			"pnpm store path":             {izlaz: store + "\n"},
			// Yarn 2 i noviji ne zna "cache dir" izvan projekta, pa se ne nudi.
			"yarn cache dir":             {greska: errors.New("Usage Error")},
			"pip3 cache dir":             {izlaz: pip + "\n"},
			"uv cache dir":               {izlaz: "relativna/putanja\n"},
			"golangci-lint cache status": {izlaz: "Dir: " + lint + "\nSize: 1000B\n"},
		})

	alati := PronadiRazvojneAlate(p)
	ocekivani := map[string]int64{
		"brew":          3 << 29,
		"go_cache":      IzracunajVelicinu(gocache),
		"go_modcache":   IzracunajVelicinu(modcache),
		"npm":           IzracunajVelicinu(filepath.Join(npmMapa, "_cacache")),
		"npx":           IzracunajVelicinu(filepath.Join(npmMapa, "_npx")),
		"pnpm":          IzracunajVelicinu(store),
		"pip":           IzracunajVelicinu(pip),
		"cocoapods":     IzracunajVelicinu(pods),
		"swiftpm":       0,
		"golangci_lint": IzracunajVelicinu(lint),
	}
	var redoslijed []string
	for _, alat := range alati {
		redoslijed = append(redoslijed, alat.ID)
		velicina, ok := ocekivani[alat.ID]
		if !ok {
			t.Errorf("alat %s se ne smije nuditi: %+v", alat.ID, alat)
			continue
		}
		if alat.VelicinaBajta != velicina || alat.MozeOcistiti != (velicina > 0) {
			t.Errorf("%s: očekivano %d bajta, dobiveno %+v", alat.ID, velicina, alat)
		}
	}
	if got := strings.Join(redoslijed, ","); got != "brew,go_cache,go_modcache,npm,npx,pnpm,pip,cocoapods,swiftpm,golangci_lint" {
		t.Errorf("alati moraju doći redom iz popisa, dobiveno %s", got)
	}
	poID := map[string]AlatZaCiscenje{}
	for _, alat := range alati {
		poID[alat.ID] = alat
	}
	if a := poID["brew"]; !strings.Contains(a.Procjena, "1.50 GB") {
		t.Errorf("neočekivan Homebrew: %+v", a)
	}
	if a := poID["pnpm"]; !strings.Contains(a.Procjena, "nekorišteni") {
		t.Errorf("pnpm briše samo dio spremišta: %+v", a)
	}
	if a := poID["swiftpm"]; !strings.Contains(a.Procjena, "nema ničega") {
		t.Errorf("prazno spremište nema što osloboditi: %+v", a)
	}
	if a := poID["npx"]; a.Naredba != "npm cache npx rm --force" {
		t.Errorf("neočekivana naredba za npx: %+v", a)
	}
}

// Svaki alat u popisu mora se moći pronaći, izmjeriti i prikazati.
func TestPopisAlataJeCjelovit(t *testing.T) {
	mjesta := mjestaProgramaFn("/Users/primjer")
	vidjeni := map[string]bool{}
	for _, opis := range popisAlata() {
		if vidjeni[opis.id] {
			t.Errorf("dva alata imaju isti id %s", opis.id)
		}
		vidjeni[opis.id] = true
		if len(mjesta[opis.program]) == 0 {
			t.Errorf("%s: nije zapisano gdje se traži program %s", opis.id, opis.program)
		}
		for _, put := range mjesta[opis.program] {
			if !filepath.IsAbs(put) || strings.HasPrefix(put, "/usr/bin/") {
				t.Errorf("%s: program se ne smije tražiti na %s", opis.id, put)
			}
		}
		if len(opis.naredba) == 0 || opis.naziv == "" || opis.opis == "" {
			t.Errorf("%s: nedostaje naredba ili tekst", opis.id)
		}
		if (opis.mapa == nil) == (opis.procijeni == nil) {
			t.Errorf("%s: alat treba ili mapu ili vlastitu procjenu", opis.id)
		}
	}
}

func TestHomebrewBezProcjene(t *testing.T) {
	brew := opisAlataPoID(t, "brew")
	p := ZadanePostavke(t.TempDir())
	postaviLazneAlate(t, t.TempDir(), []string{"brew"}, nil)
	if alat, _ := otkrijAlat(p, brew); alat.MozeOcistiti || !strings.Contains(alat.Procjena, "nema ničega") {
		t.Errorf("Homebrew bez ičega za ukloniti ne nudi čišćenje: %+v", alat)
	}
	postaviLazneAlate(t, t.TempDir(), []string{"brew"}, map[string]lazniOdgovor{
		"brew cleanup --prune=all -n": {greska: errors.New("brew je zauzet")},
	})
	if alat, _ := otkrijAlat(p, brew); !alat.MozeOcistiti || !strings.Contains(alat.Procjena, "nije dostupna") {
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

	// Alat postoji, ali vrati relativnu putanju: takvoj mapi se ne vjeruje.
	postaviLazneAlate(t, mapa, []string{"go"}, map[string]lazniOdgovor{"go env GOMODCACHE": {izlaz: "relativna/putanja\n"}})
	if _, ok := otkrijAlat(ZadanePostavke(mapa), opisAlataPoID(t, "go_modcache")); ok {
		t.Errorf("relativna putanja ne smije se prihvatiti kao mapa alata")
	}

	// npm koji ne zna reći gdje mu je spremište ne nudi ni npm ni npx.
	postaviLazneAlate(t, mapa, []string{"npm"}, map[string]lazniOdgovor{"npm config get cache": {greska: errors.New("env: node: No such file or directory")}})
	for _, id := range []string{"npm", "npx"} {
		if _, ok := otkrijAlat(ZadanePostavke(mapa), opisAlataPoID(t, id)); ok {
			t.Errorf("%s se ne smije nuditi kad npm ne radi", id)
		}
	}
}

func TestPutanjaIzIspisa(t *testing.T) {
	for izlaz, ocekivano := range map[string]string{
		"/Users/primjer/Library/Caches/pip\n":                             "/Users/primjer/Library/Caches/pip",
		"Dir: /Users/primjer/Library/Caches/golangci-lint\nSize: 18MiB\n": "/Users/primjer/Library/Caches/golangci-lint",
		"upozorenje: stara verzija\n/Users/primjer/.npm\n":                "/Users/primjer/.npm",
		"relativna/putanja\n":                                             "",
		"":                                                                "",
	} {
		put, ok := putanjaIzIspisa(izlaz)
		if put != ocekivano || ok != (ocekivano != "") {
			t.Errorf("putanjaIzIspisa(%q) = %q, %v; očekivano %q", izlaz, put, ok, ocekivano)
		}
	}
}

func TestOcistiRazvojniAlat(t *testing.T) {
	mapa := t.TempDir()
	pozivi := postaviLazneAlate(t, mapa, []string{"go", "npm"}, nil)
	p := ZadanePostavke(t.TempDir())
	for id, ocekivana := range map[string]string{
		"go_cache":    "go clean -cache",
		"go_modcache": "go clean -modcache",
		"npx":         "npm cache npx rm --force",
	} {
		poruka, err := OcistiRazvojniAlat(p, id)
		if err != nil || !strings.Contains(poruka, ocekivana+" je završila") {
			t.Fatalf("%s: očekivano uspješno čišćenje, dobiveno %q (%v)", id, poruka, err)
		}
		zadnji := (*pozivi)[len(*pozivi)-1]
		if zadnji.program+" "+zadnji.argumenti != ocekivana {
			t.Errorf("%s: pokrenuta je kriva naredba: %+v", id, zadnji)
		}
	}
	if _, err := OcistiRazvojniAlat(p, "rm -rf"); err == nil {
		t.Errorf("nepoznat alat mora biti odbijen")
	}

	t.Cleanup(PostaviAlateZaTest(map[string][]string{}, nil))
	if _, err := OcistiRazvojniAlat(p, "brew"); err == nil || !strings.Contains(err.Error(), "nije pronađen") {
		t.Errorf("očekivana greška za program koji ne postoji, dobiveno %v", err)
	}

	postaviLazneAlate(t, mapa, []string{"pnpm"}, map[string]lazniOdgovor{
		"pnpm store prune": {greska: errors.New("spremište je zaključano")},
	})
	if _, err := OcistiRazvojniAlat(p, "pnpm"); err == nil || !strings.Contains(err.Error(), "spremište je zaključano") || strings.Contains(err.Error(), "ipak") {
		t.Errorf("greška alata mora doći do korisnika, dobiveno %v", err)
	}
}

// go clean -cache javi grešku kad drugi program u isto vrijeme piše u mapu,
// a ipak obriše gotovo sve. Korisnik mora vidjeti i grešku i oslobođeni prostor.
func TestDjelomicnoUspjelaNaredba(t *testing.T) {
	postaviLazneAlate(t, t.TempDir(), []string{"go"}, map[string]lazniOdgovor{
		"go clean -cache": {greska: errors.New("go: unlinkat /x/1d: directory not empty")},
	})
	slobodno := int64(50 << 30)
	stari := slobodnoBajtaFn
	slobodnoBajtaFn = func(string) int64 { s := slobodno; slobodno += 128 << 30; return s }
	t.Cleanup(func() { slobodnoBajtaFn = stari })

	_, err := OcistiRazvojniAlat(ZadanePostavke(t.TempDir()), "go_cache")
	if err == nil || !strings.Contains(err.Error(), "directory not empty") || !strings.Contains(err.Error(), "ipak ima 128.00 GB više") {
		t.Errorf("očekivana greška s oslobođenim prostorom, dobiveno %v", err)
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

// npm i Yarn su skripte koje traže node pokraj sebe. Aplikacija iz Findera ima
// samo /usr/bin u PATH-u, pa mapa alata mora biti dodana.
func TestAlatNalaziProgramePokrajSebe(t *testing.T) {
	mapa := t.TempDir()
	pomocni := filepath.Join(mapa, "pomocni-node")
	if err := os.WriteFile(pomocni, []byte("#!/bin/sh\necho radi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	alat := filepath.Join(mapa, "alat")
	if err := os.WriteFile(alat, []byte("#!/usr/bin/env pomocni-node\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "/usr/bin:/bin")
	if izlaz, err := pokreniAlat(alat); err != nil || string(izlaz) != "radi\n" {
		t.Errorf("alat mora naći program iz svoje mape, dobiveno %q (%v)", izlaz, err)
	}
}
