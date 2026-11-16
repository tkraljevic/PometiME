package cistac

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
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

// opisAlata je jedan alat koji PometiME zna očistiti: kako ga pronaći,
// što briše i kojom naredbom.
type opisAlata struct {
	id      string
	program string   // ključ u mjestima programa
	naredba []string // argumenti naredbe za čišćenje
	naziv   string
	opis    string
	// mapa vraća mapu koju naredba čisti, da se izmjeri koliko zauzima.
	mapa func(p Postavke, program string) (string, bool)
	// samoDio znači da naredba briše samo nekorišteni dio mape.
	samoDio bool
	// procijeni zamjenjuje mjerenje mape kad alat sam zna procijeniti.
	procijeni func(program string, alat *AlatZaCiscenje)
}

// popisAlata vraća sve alate koje PometiME zna očistiti. Novi alat dodaje se
// samo ovdje. Tekstovi se prevode pri svakom pozivu, jer korisnik može
// promijeniti jezik dok program radi.
func popisAlata() []opisAlata {
	mapaNpma := mapaIzIspisa("config", "get", "cache")
	return []opisAlata{
		{id: "brew", program: "brew", naredba: []string{"cleanup", "--prune=all"},
			naziv:     T("Homebrew: stare verzije i preuzimanja"),
			opis:      T("Stare verzije paketa i preuzete arhive koje Homebrew više ne treba."),
			procijeni: procijeniHomebrew},
		{id: "go_cache", program: "go", naredba: []string{"clean", "-cache"},
			naziv: T("Go: međuspremnik izgradnje"),
			opis:  T("Prevedeni Go paketi i spremljeni rezultati testova. Go ih ponovno napravi, pa je sljedeća izgradnja sporija."),
			mapa:  mapaIzIspisa("env", "GOCACHE")},
		{id: "go_modcache", program: "go", naredba: []string{"clean", "-modcache"},
			naziv: T("Go moduli"),
			opis:  T("Preuzeti Go moduli. Projekti ih ponovno preuzmu pri sljedećoj izgradnji, za što treba internet."),
			mapa:  mapaIzIspisa("env", "GOMODCACHE")},
		{id: "npm", program: "npm", naredba: []string{"cache", "clean", "--force"},
			naziv: T("npm: preuzeti paketi"),
			opis:  T("Paketi koje je npm preuzeo. Pri sljedećoj instalaciji ponovno se preuzmu, za što treba internet."),
			mapa:  podmapa(mapaNpma, "_cacache")},
		{id: "npx", program: "npm", naredba: []string{"cache", "npx", "rm", "--force"},
			naziv: T("npx: preuzeti programi"),
			opis:  T("Programi pokrenuti naredbom npx, među njima često i MCP poslužitelji. Pri sljedećem pokretanju ponovno se preuzmu."),
			mapa:  podmapa(mapaNpma, "_npx")},
		{id: "pnpm", program: "pnpm", naredba: []string{"store", "prune"},
			naziv:   T("pnpm: nekorišteni paketi"),
			opis:    T("Briše samo pakete iz spremišta koje nijedan projekt više ne koristi."),
			mapa:    mapaIzIspisa("store", "path"),
			samoDio: true},
		{id: "yarn", program: "yarn", naredba: []string{"cache", "clean"},
			naziv: T("Yarn: preuzeti paketi"),
			opis:  T("Paketi koje je Yarn preuzeo. Pri sljedećoj instalaciji ponovno se preuzmu, za što treba internet."),
			mapa:  mapaIzIspisa("cache", "dir")},
		{id: "pip", program: "pip3", naredba: []string{"cache", "purge"},
			naziv: T("pip: preuzeti Python paketi"),
			opis:  T("Paketi koje je pip preuzeo. Instalirani paketi ostaju."),
			mapa:  mapaIzIspisa("cache", "dir")},
		{id: "uv", program: "uv", naredba: []string{"cache", "clean"},
			naziv: T("uv: preuzeti Python paketi"),
			opis:  T("Paketi koje je uv preuzeo. Pri sljedećoj instalaciji ponovno se preuzmu, za što treba internet."),
			mapa:  mapaIzIspisa("cache", "dir")},
		{id: "cocoapods", program: "pod", naredba: []string{"cache", "clean", "--all"},
			naziv: T("CocoaPods: preuzeti podovi"),
			opis:  T("Podovi koje je CocoaPods preuzeo. Projekti ih ponovno preuzmu pri sljedećem pod install."),
			mapa:  mapaUDomu("Library", "Caches", "CocoaPods")},
		{id: "swiftpm", program: "swift", naredba: []string{"package", "purge-cache"},
			naziv: T("Swift: preuzeti paketi"),
			opis:  T("Spremište Swift paketa. Projekti ih ponovno preuzmu pri sljedećoj izgradnji."),
			mapa:  mapaUDomu("Library", "Caches", "org.swift.swiftpm")},
		{id: "golangci_lint", program: "golangci-lint", naredba: []string{"cache", "clean"},
			naziv: T("golangci-lint: spremljene provjere"),
			opis:  T("Rezultati ranijih provjera koda. Sljedeća provjera bit će sporija."),
			mapa:  mapaIzIspisa("cache", "status")},
		{id: "dart_pub", program: "dart", naredba: []string{"pub", "cache", "clean", "--force"},
			naziv: T("Dart i Flutter: preuzeti paketi"),
			opis:  T("Paketi koje su preuzeli dart pub get i flutter pub get. Projekti ih ponovno preuzmu pri sljedećem pub get, za što treba internet."),
			mapa:  mapaUDomu(".pub-cache")},
		{id: "composer", program: "composer", naredba: []string{"clear-cache"},
			naziv: T("Composer: preuzeti PHP paketi"),
			opis:  T("Paketi koje je Composer preuzeo. Pri sljedećoj instalaciji ponovno se preuzmu, za što treba internet."),
			mapa:  mapaIzIspisa("config", "--global", "cache-dir")},
	}
}

const rokNaredbeAlata = 10 * time.Minute

var (
	// Aplikacija pokrenuta iz Findera nema PATH ljuske, pa se programi traže na poznatim mjestima.
	// Za Python i Swift uzimaju se pravi programi, a ne /usr/bin: ondje su samo omotači
	// koji bez Xcodeovih alata otvore prozor za njihovu instalaciju.
	mjestaProgramaFn = func(dom string) map[string][]string {
		return map[string][]string{
			"brew":          {"/opt/homebrew/bin/brew", "/usr/local/bin/brew"},
			"go":            {"/opt/homebrew/bin/go", "/usr/local/go/bin/go", "/usr/local/bin/go"},
			"npm":           {"/opt/homebrew/bin/npm", "/usr/local/bin/npm", filepath.Join(dom, ".volta", "bin", "npm")},
			"pnpm":          {filepath.Join(dom, "Library", "pnpm", "pnpm"), "/opt/homebrew/bin/pnpm", "/usr/local/bin/pnpm"},
			"yarn":          {"/opt/homebrew/bin/yarn", "/usr/local/bin/yarn", filepath.Join(dom, ".yarn", "bin", "yarn")},
			"pip3":          {"/opt/homebrew/bin/pip3", "/usr/local/bin/pip3", "/Library/Frameworks/Python.framework/Versions/Current/bin/pip3", "/Library/Developer/CommandLineTools/usr/bin/pip3", "/Applications/Xcode.app/Contents/Developer/usr/bin/pip3"},
			"uv":            {filepath.Join(dom, ".local", "bin", "uv"), filepath.Join(dom, ".cargo", "bin", "uv"), "/opt/homebrew/bin/uv", "/usr/local/bin/uv"},
			"pod":           {"/opt/homebrew/bin/pod", "/usr/local/bin/pod"},
			"swift":         {"/Library/Developer/CommandLineTools/usr/bin/swift", "/Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/bin/swift"},
			"golangci-lint": {"/opt/homebrew/bin/golangci-lint", "/usr/local/bin/golangci-lint", filepath.Join(dom, "go", "bin", "golangci-lint")},
			"composer":      {"/opt/homebrew/bin/composer", "/usr/local/bin/composer"},
			"dart":          {"/opt/homebrew/bin/dart", "/usr/local/bin/dart", filepath.Join(dom, "flutter", "bin", "dart"), filepath.Join(dom, "development", "flutter", "bin", "dart"), filepath.Join(dom, "fvm", "default", "bin", "dart")},
			"rustup":        {filepath.Join(dom, ".cargo", "bin", "rustup"), "/opt/homebrew/opt/rustup/bin/rustup", "/opt/homebrew/bin/rustup", "/usr/local/bin/rustup"},
		}
	}
	pokreniAlatFn      = pokreniAlat
	slobodnoBajtaFn    = func(put string) int64 { return DohvatiStanjeDiska(put).SlobodnoBajta }
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

// pokreniAlat pokreće alat iz mape /, da ga ne zbune postavke nekog projekta.
func pokreniAlat(program string, argumenti ...string) ([]byte, error) {
	ctx, otkazi := context.WithTimeout(context.Background(), rokNaredbeAlata)
	defer otkazi()
	naredba := exec.CommandContext(ctx, program, argumenti...)
	naredba.Dir = "/"
	naredba.Env = okruzenjeAlata(program)
	return naredba.Output()
}

// okruzenjeAlata dodaje u PATH mapu samog alata i Homebrew. npm, Yarn i Composer
// su skripte koje traže node ili php, a aplikacija iz Findera ima samo /usr/bin.
// Ostale varijable sprečavaju da alat nešto preuzima ili nudi nadogradnju.
func okruzenjeAlata(program string) []string {
	put := strings.Join([]string{filepath.Dir(program), "/opt/homebrew/bin", "/usr/local/bin", os.Getenv("PATH")}, string(os.PathListSeparator))
	return append(os.Environ(),
		"PATH="+put,
		"HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ENV_HINTS=1",
		"COREPACK_ENABLE_NETWORK=0",
		"npm_config_update_notifier=false",
		"PIP_DISABLE_PIP_VERSION_CHECK=1",
	)
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
	opisi := popisAlata()
	pronadeni := make([][]AlatZaCiscenje, len(opisi)+1)
	var wg sync.WaitGroup
	for i, opis := range opisi {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if alat, ok := otkrijAlat(p, opis); ok {
				pronadeni[i] = []AlatZaCiscenje{alat}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		pronadeni[len(opisi)] = otkrijRustToolchainove(p)
	}()
	wg.Wait()
	alati := []AlatZaCiscenje{}
	for _, dio := range pronadeni {
		alati = append(alati, dio...)
	}
	return alati
}

// otkrijAlat provjerava je li alat instaliran i koliko zauzima ono što briše.
// Alat koji ne zna reći gdje mu je spremište ne nudi se, jer vjerojatno ne radi.
func otkrijAlat(p Postavke, opis opisAlata) (AlatZaCiscenje, bool) {
	program := pronadiProgram(p, opis.program)
	if program == "" {
		return AlatZaCiscenje{}, false
	}
	alat := AlatZaCiscenje{
		ID:      opis.id,
		Naziv:   opis.naziv,
		Opis:    opis.opis,
		Naredba: opis.program + " " + strings.Join(opis.naredba, " "),
	}
	if opis.procijeni != nil {
		opis.procijeni(program, &alat)
		return alat, true
	}
	mapa, ok := opis.mapa(p, program)
	if !ok {
		return AlatZaCiscenje{}, false
	}
	alat.VelicinaBajta = IzracunajVelicinu(mapa)
	alat.MozeOcistiti = alat.VelicinaBajta > 0
	switch {
	case !alat.MozeOcistiti:
		alat.Procjena = T("Trenutačno nema ničega za ukloniti.")
	case opis.samoDio:
		alat.Procjena = fmt.Sprintf(T("Spremište zauzima %s, a oslobodit će se samo nekorišteni dio."), FormatirajBajte(alat.VelicinaBajta))
	default:
		alat.Procjena = fmt.Sprintf(T("Oslobodit će oko %s."), FormatirajBajte(alat.VelicinaBajta))
	}
	return alat, true
}

// procijeniHomebrew pokreće pokusno čišćenje, jer Homebrew briše po više mapa.
func procijeniHomebrew(program string, alat *AlatZaCiscenje) {
	izlaz, err := pokreniAlatFn(program, "cleanup", "--prune=all", "-n")
	if err != nil {
		alat.Procjena = T("Procjena nije dostupna, ali naredbu možete pokrenuti.")
		alat.MozeOcistiti = true
		return
	}
	alat.VelicinaBajta = procjenaHomebrewa(string(izlaz))
	alat.MozeOcistiti = alat.VelicinaBajta > 0
	alat.Procjena = T("Trenutačno nema ničega za ukloniti.")
	if alat.MozeOcistiti {
		alat.Procjena = fmt.Sprintf(T("Oslobodit će oko %s."), FormatirajBajte(alat.VelicinaBajta))
	}
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

// mapaIzIspisa pita alat gdje mu je spremište.
func mapaIzIspisa(argumenti ...string) func(Postavke, string) (string, bool) {
	return func(_ Postavke, program string) (string, bool) {
		izlaz, err := pokreniAlatFn(program, argumenti...)
		if err != nil {
			return "", false
		}
		return putanjaIzIspisa(string(izlaz))
	}
}

// putanjaIzIspisa uzima prvi redak koji je apsolutna putanja. golangci-lint
// ispred putanje piše "Dir:", ostali alati ispisuju samo putanju.
func putanjaIzIspisa(izlaz string) (string, bool) {
	for _, redak := range strings.Split(izlaz, "\n") {
		redak = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(redak), "Dir:"))
		if filepath.IsAbs(redak) {
			return filepath.Clean(redak), true
		}
	}
	return "", false
}

// podmapa vraća mapu unutar spremišta koje alat javlja.
func podmapa(mapa func(Postavke, string) (string, bool), ime string) func(Postavke, string) (string, bool) {
	return func(p Postavke, program string) (string, bool) {
		korijen, ok := mapa(p, program)
		if !ok {
			return "", false
		}
		return filepath.Join(korijen, ime), true
	}
}

// mapaUDomu je stalna mapa u korisničkoj mapi, za alate koji je ne znaju ispisati.
func mapaUDomu(dijelovi ...string) func(Postavke, string) (string, bool) {
	return func(p Postavke, _ string) (string, bool) {
		return filepath.Join(append([]string{p.KorijenKorisnika}, dijelovi...)...), true
	}
}

// OcistiRazvojniAlat pokreće naredbu alata i javlja koliko je slobodnog prostora dobiveno.
func OcistiRazvojniAlat(p Postavke, id string) (string, error) {
	program, ime, argumenti, err := naredbaAlata(p, id)
	if err != nil {
		return "", err
	}
	opis := ime + " " + strings.Join(argumenti, " ")
	prije := slobodnoBajtaFn(p.KorijenKorisnika)
	_, err = pokreniAlatFn(program, argumenti...)
	dobiveno := max(slobodnoBajtaFn(p.KorijenKorisnika)-prije, 0)
	if err != nil {
		greska := strings.TrimRight(opisGreskeNaredbe(err), ".")
		// go clean -cache javi grešku ako drugi program u isto vrijeme piše u mapu,
		// a ipak obriše gotovo sve. Korisnik treba znati i to.
		if dobiveno > 0 {
			return "", fmt.Errorf(T("naredba %s nije uspjela: %s. Slobodnog prostora ipak ima %s više."), opis, greska, FormatirajBajte(dobiveno))
		}
		return "", fmt.Errorf(T("naredba %s nije uspjela: %s"), opis, greska)
	}
	return fmt.Sprintf(T("Naredba %s je završila. Slobodnog prostora ima %s više."), opis, FormatirajBajte(dobiveno)), nil
}

// naredbaAlata pronalazi program i argumente za alat koji je korisnik odabrao.
// Rust toolchain uklanja se samo ako je instaliran i nije zadani.
func naredbaAlata(p Postavke, id string) (program, ime string, argumenti []string, err error) {
	if tc, jeToolchain := strings.CutPrefix(id, prefiksToolchaina); jeToolchain {
		ime, argumenti = "rustup", []string{"toolchain", "uninstall", tc}
	} else {
		opisi := popisAlata()
		i := slices.IndexFunc(opisi, func(o opisAlata) bool { return o.id == id })
		if i < 0 {
			return "", "", nil, fmt.Errorf(T("nepoznat alat: %s"), id)
		}
		ime, argumenti = opisi[i].program, opisi[i].naredba
	}
	program = pronadiProgram(p, ime)
	if program == "" {
		return "", "", nil, fmt.Errorf(T("program %s nije pronađen na ovom Macu"), ime)
	}
	if ime == "rustup" && !jeSuvisanToolchain(program, argumenti[2]) {
		return "", "", nil, fmt.Errorf(T("toolchain %s nije instaliran ili je zadani"), argumenti[2])
	}
	return program, ime, argumenti, nil
}

// Rust toolchainovi nisu jedan alat: svaki toolchain osim zadanog dobije svoj red.
const prefiksToolchaina = "rustup:"

var uzorakImenaToolchaina = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type toolchainRusta struct {
	ime    string
	zadani bool // zadani ili aktivni toolchain nikad se ne nudi za uklanjanje
}

// toolchainoviRusta čita "rustup toolchain list", npr. "stable-aarch64-apple-darwin (active, default)".
func toolchainoviRusta(program string) ([]toolchainRusta, bool) {
	izlaz, err := pokreniAlatFn(program, "toolchain", "list")
	if err != nil {
		return nil, false
	}
	var popis []toolchainRusta
	for _, redak := range strings.Split(string(izlaz), "\n") {
		polja := strings.Fields(redak)
		if len(polja) == 0 || !uzorakImenaToolchaina.MatchString(polja[0]) || strings.HasPrefix(redak, "no ") {
			continue
		}
		oznake := strings.Join(polja[1:], " ")
		popis = append(popis, toolchainRusta{polja[0], strings.Contains(oznake, "default") || strings.Contains(oznake, "active")})
	}
	return popis, true
}

func jeSuvisanToolchain(program, ime string) bool {
	popis, ok := toolchainoviRusta(program)
	return ok && slices.ContainsFunc(popis, func(tc toolchainRusta) bool { return tc.ime == ime && !tc.zadani })
}

// otkrijRustToolchainove nudi uklanjanje svakog toolchaina osim zadanog i
// kaže kad je to ista verzija kao zadani, jer je onda sigurno višak.
func otkrijRustToolchainove(p Postavke) []AlatZaCiscenje {
	program := pronadiProgram(p, "rustup")
	if program == "" {
		return nil
	}
	popis, ok := toolchainoviRusta(program)
	dom, okDom := mapaIzIspisa("show", "home")(p, program)
	if !ok || !okDom {
		return nil
	}
	zadana := zadanaVerzija(dom, popis)
	var alati []AlatZaCiscenje
	for _, tc := range popis {
		if !tc.zadani {
			alati = append(alati, redToolchaina(dom, tc.ime, zadana))
		}
	}
	return alati
}

func zadanaVerzija(dom string, popis []toolchainRusta) string {
	for _, tc := range popis {
		if tc.zadani {
			return verzijaToolchaina(dom, tc.ime)
		}
	}
	return ""
}

// verzijaToolchaina vraća npr. "rustc 1.99.1 (58cf8f9c3 2026-10-13)" ili prazan tekst.
func verzijaToolchaina(dom, ime string) string {
	izlaz, err := pokreniAlatFn(filepath.Join(dom, "toolchains", ime, "bin", "rustc"), "--version")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(izlaz))
}

func redToolchaina(dom, ime, zadana string) AlatZaCiscenje {
	velicina := IzracunajVelicinu(filepath.Join(dom, "toolchains", ime))
	return AlatZaCiscenje{
		ID:            prefiksToolchaina + ime,
		Naziv:         fmt.Sprintf(T("Rust toolchain %s"), ime),
		Opis:          opisToolchaina(verzijaToolchaina(dom, ime), zadana),
		Naredba:       "rustup toolchain uninstall " + ime,
		VelicinaBajta: velicina,
		Procjena:      fmt.Sprintf(T("Oslobodit će oko %s."), FormatirajBajte(velicina)),
		MozeOcistiti:  velicina > 0,
	}
}

func opisToolchaina(verzija, zadana string) string {
	if verzija == "" || verzija != zadana {
		return T("Rust toolchain koji nije zadani. Projekt koji ga treba ponovno će ga preuzeti.")
	}
	broj := verzija
	if polja := strings.Fields(verzija); len(polja) > 1 {
		broj = polja[1]
	}
	return fmt.Sprintf(T("Ista verzija kao zadani toolchain (%s), pa je višak."), broj)
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
