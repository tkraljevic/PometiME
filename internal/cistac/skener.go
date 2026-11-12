package cistac

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// danaDoStarePredmemorije: predmemorija mijenjana u tom razdoblju koristi se i
// brzo bi se ponovno napunila, pa se ne preporučuje za čišćenje.
const danaDoStarePredmemorije = 30

var (
	pokrenuteAplikacijeFn = pokrenuteAplikacije
	uzorakIdentifikatora  = regexp.MustCompile(`bundleID="([^"]+)"`)
)

type kandidatCiscenja struct {
	putanja     string
	naziv       string
	kategorija  string
	opis        string
	preporuceno bool
}

var preskocenePredmemorije = map[string]bool{
	"go-build":                true,
	"Homebrew":                true,
	"pip":                     true,
	"Codex":                   true,
	"CocoaPods":               true,
	"Yarn":                    true,
	"goimports":               true,
	"golangci-lint":           true,
	"gopls":                   true,
	"ms-playwright":           true,
	"ms-playwright-go":        true,
	"CloudKit":                true,
	"FamilyCircle":            true,
	"GeoServices":             true,
	"PassKit":                 true,
	"GameKit":                 true,
	"Animoji":                 true,
	"SiriTTS":                 true,
	"AudioUnitCache":          true,
	"askpermissiond":          true,
	"antigravity":             true,
	"com.google.antigravity":  true,
	"io.tailscale.ipn.macsys": true,
}

var predmemorijePreglednika = map[string]bool{
	"Google":        true,
	"Firefox":       true,
	"Mozilla":       true,
	"BraveSoftware": true,
	"Arc":           true,
}

func trebaPreskocitiPredmemoriju(ime string) bool {
	if preskocenePredmemorije[ime] {
		return true
	}
	return strings.HasPrefix(ime, "com.apple.") || strings.HasPrefix(ime, ".")
}

func jePreporucenaPredmemorija(ime string) bool {
	return !predmemorijePreglednika[ime]
}

func opisPredmemorije(ime string) string {
	if strings.HasSuffix(ime, ".ShipIt") || strings.Contains(strings.ToLower(ime), "updater") {
		return "Privremeni instalacijski paketi ažuriranja aplikacije."
	}
	if predmemorijePreglednika[ime] {
		return "Predmemorija web-preglednika (preporučeno zatvoriti preglednik prije čišćenja)."
	}
	return "Korisnička predmemorija aplikacije u ~/Library/Caches."
}

// SkenirajPredmemoriju pronalazi stavke u ~/Library/Caches i ~/Library/Logs veće od praga.
func SkenirajPredmemoriju(p Postavke) []StavkaCiscenja {
	mapaCaches := filepath.Join(p.KorijenKorisnika, "Library", "Caches")
	var kandidati []kandidatCiscenja
	kandidati = append(kandidati, prikupiKandidateCaches(mapaCaches)...)
	kandidati = append(kandidati, prikupiKandidateLogova(filepath.Join(p.KorijenKorisnika, "Library", "Logs"))...)
	stavke := izmjeriKandidate(kandidati, p.NajmanjaVelicinaPredmemorije)
	return prilagodiPreporukePredmemorije(stavke, mapaCaches, pokrenuteAplikacijeFn())
}

// prilagodiPreporukePredmemorije ne preporučuje predmemoriju koju aplikacija upravo
// koristi ili koja se nedavno mijenjala: takva bi se brzo ponovno napunila.
func prilagodiPreporukePredmemorije(stavke []StavkaCiscenja, mapaCaches string, pokrenute map[string]bool) []StavkaCiscenja {
	for i := range stavke {
		st := &stavke[i]
		if !st.Preporuceno || !JeUnutarKorijena(st.Putanja, mapaCaches) {
			continue
		}
		switch {
		case jeUPokrenutojAplikaciji(st.Naziv, pokrenute):
			st.Preporuceno = false
			st.Opis += " Aplikacija je pokrenuta i koristi ovu predmemoriju."
		case st.DanaOdIzmjene < danaDoStarePredmemorije:
			st.Preporuceno = false
			st.Opis += " " + opisNedavneIzmjene(st.DanaOdIzmjene) + " Brzo bi se ponovno napunila."
		}
	}
	return stavke
}

func opisNedavneIzmjene(dana int) string {
	if dana <= 0 {
		return "Mijenjana je danas."
	}
	return fmt.Sprintf("Mijenjana je prije %s.", UzBrojHR(dana, "dan", "dana", "dana"))
}

// jeUPokrenutojAplikaciji prepoznaje mapu nazvanu po identifikatoru pokrenute
// aplikacije, i njezine podmape poput com.primjer.app.ShipIt.
func jeUPokrenutojAplikaciji(ime string, pokrenute map[string]bool) bool {
	malo := strings.ToLower(ime)
	for id := range pokrenute {
		if malo == id || strings.HasPrefix(malo, id+".") {
			return true
		}
	}
	return false
}

// pokrenuteAplikacije čita identifikatore iz lsappinfo, koji ne traži dozvolu za automatizaciju.
func pokrenuteAplikacije() map[string]bool {
	izlaz, err := pokreniIzlazNaredbe("/usr/bin/lsappinfo", "list")
	if err != nil {
		return map[string]bool{}
	}
	return izdvojiIdentifikatore(string(izlaz))
}

func izdvojiIdentifikatore(tekst string) map[string]bool {
	identifikatori := map[string]bool{}
	for _, m := range uzorakIdentifikatora.FindAllStringSubmatch(tekst, -1) {
		identifikatori[strings.ToLower(m[1])] = true
	}
	return identifikatori
}

// danaOdVremena vraća pune dane od zadanog vremena; nulto vrijeme znači da datoteka nema.
func danaOdVremena(vrijeme time.Time) int {
	if vrijeme.IsZero() {
		return -1
	}
	return int(time.Since(vrijeme).Hours() / 24)
}

func prikupiKandidateCaches(mapaCaches string) []kandidatCiscenja {
	ulazi, err := os.ReadDir(mapaCaches)
	if err != nil {
		return nil
	}
	var izlaz []kandidatCiscenja
	for _, u := range ulazi {
		ime := u.Name()
		if trebaPreskocitiPredmemoriju(ime) {
			continue
		}
		izlaz = append(izlaz, kandidatCiscenja{
			putanja:     filepath.Join(mapaCaches, ime),
			naziv:       ime,
			kategorija:  "system_caches",
			opis:        opisPredmemorije(ime),
			preporuceno: jePreporucenaPredmemorija(ime),
		})
	}
	return izlaz
}

func prikupiKandidateLogova(mapaLogova string) []kandidatCiscenja {
	ulazi, err := os.ReadDir(mapaLogova)
	if err != nil {
		return nil
	}
	var izlaz []kandidatCiscenja
	for _, u := range ulazi {
		ime := u.Name()
		if strings.HasPrefix(ime, ".") {
			continue
		}
		izlaz = append(izlaz, kandidatCiscenja{
			putanja:     filepath.Join(mapaLogova, ime),
			naziv:       "Logovi: " + ime,
			kategorija:  "system_caches",
			opis:        "Zapisnici i dijagnostički izvještaji u ~/Library/Logs.",
			preporuceno: true,
		})
	}
	return izlaz
}

// SkenirajRazvojneAlate pronalazi predmemorije razvojnih i AI alata.
func SkenirajRazvojneAlate(p Postavke) []StavkaCiscenja {
	kandidati := fiksniRazvojniKandidati(p.KorijenKorisnika)
	kandidati = append(kandidati, prikupiClaudeKandidate(filepath.Join(p.KorijenKorisnika, ".claude", "projects"))...)
	kandidati = append(kandidati, prikupiSimulatorKandidate(filepath.Join(p.KorijenKorisnika, "Library", "Developer", "CoreSimulator", "Devices"))...)
	kandidati = append(kandidati, prikupiAvdKandidate(filepath.Join(p.KorijenKorisnika, ".android", "avd"))...)
	return izmjeriKandidate(kandidati, p.NajmanjaVelicinaRazvojnih)
}

func fiksniRazvojniKandidati(k string) []kandidatCiscenja {
	return []kandidatCiscenja{
		{filepath.Join(k, "Library", "Developer", "Xcode", "DerivedData"), "Xcode DerivedData", "dev_ai", "Međuspremnik prevedenih Xcode projekata i indeksa.", true},
		{filepath.Join(k, "Library", "Developer", "Xcode", "Archives"), "Xcode Archives", "dev_ai", "Stare arhive izdanja iz Xcodea.", false},
		{filepath.Join(k, "Library", "Developer", "Xcode", "iOS DeviceSupport"), "Xcode iOS DeviceSupport", "dev_ai", "Simboli za otklanjanje grešaka starih iOS verzija.", false},
		{filepath.Join(k, "Library", "Developer", "CoreSimulator", "Caches"), "CoreSimulator Cache", "dev_ai", "Korisnička predmemorija za iOS simulator.", true},
		{filepath.Join(k, "Library", "Caches", "CocoaPods"), "CocoaPods Cache", "dev_ai", "Međuspremnik preuzetih iOS/macOS CocoaPods paketa.", true},
		{filepath.Join(k, "Library", "Caches", "go-build"), "Go Build Cache", "dev_ai", "Međuspremnik prevedenih Go paketa (go clean -cache).", false},
		{filepath.Join(k, "go", "pkg", "mod", "cache"), "Go Module Cache", "dev_ai", "Arhive preuzetih Go modula (go clean -modcache).", false},
		{filepath.Join(k, "Library", "Caches", "Homebrew", "downloads"), "Homebrew Downloads", "dev_ai", "Preuzete instalacijske arhive u Homebrewu.", true},
		{filepath.Join(k, ".npm", "_cacache"), "NPM Package Cache", "dev_ai", "Međuspremnik preuzetih Node.js paketa.", true},
		{filepath.Join(k, "Library", "Caches", "Yarn"), "Yarn Cache", "dev_ai", "Međuspremnik preuzetih Yarn paketa.", true},
		{filepath.Join(k, "Library", "pnpm", "store"), "PNPM Store", "dev_ai", "Globalno spremište PNPM paketa.", false},
		{filepath.Join(k, "Library", "Caches", "pip"), "Python Pip Cache", "dev_ai", "Međuspremnik preuzetih Python paketa.", true},
		{filepath.Join(k, ".cargo", "registry", "cache"), "Rust Cargo Cache", "dev_ai", "Preuzete arhive Rust paketa.", true},
		{filepath.Join(k, ".gradle", "caches"), "Gradle Cache", "dev_ai", "Međuspremnik preuzetih Java/Android Gradle paketa.", false},
		{filepath.Join(k, ".m2", "repository"), "Maven Repository", "dev_ai", "Lokalno spremište Java Maven ovisnosti.", false},
		{filepath.Join(k, "Library", "Caches", "goimports"), "Goimports Cache", "dev_ai", "Privremeni indeks za goimports alat.", true},
		{filepath.Join(k, "Library", "Caches", "golangci-lint"), "GolangCI-Lint Cache", "dev_ai", "Međuspremnik za statičku analizu Go koda.", true},
		{filepath.Join(k, "Library", "Caches", "gopls"), "Gopls Cache", "dev_ai", "Privremena predmemorija za Go Language Server.", true},
		{filepath.Join(k, "Library", "Caches", "Codex"), "Codex App Cache", "dev_ai", "Predmemorija za Codex AI aplikaciju.", true},
		{filepath.Join(k, ".cache", "codex-runtimes"), "Codex Runtimes", "dev_ai", "Privremena okruženja za Codex CLI.", false},
	}
}

func prikupiClaudeKandidate(mapaProjekata string) []kandidatCiscenja {
	projekti, err := os.ReadDir(mapaProjekata)
	if err != nil {
		return nil
	}
	var izlaz []kandidatCiscenja
	for _, pr := range projekti {
		if !pr.IsDir() {
			continue
		}
		izlaz = append(izlaz, claudeKandidatiProjekta(filepath.Join(mapaProjekata, pr.Name()), pr.Name())...)
	}
	return izlaz
}

func claudeKandidatiProjekta(putanjaProjekta, nazivProjekta string) []kandidatCiscenja {
	stavke, err := os.ReadDir(putanjaProjekta)
	if err != nil {
		return nil
	}
	var izlaz []kandidatCiscenja
	for _, st := range stavke {
		ime := st.Name()
		if ime == "memory" || !st.IsDir() {
			continue
		}
		izlaz = append(izlaz, claudePodmapeSesije(filepath.Join(putanjaProjekta, ime), nazivProjekta)...)
	}
	return izlaz
}

func claudePodmapeSesije(putanjaSesije, nazivProjekta string) []kandidatCiscenja {
	var izlaz []kandidatCiscenja
	for _, pod := range []string{"subagents", "tool-results"} {
		p := filepath.Join(putanjaSesije, pod)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			izlaz = append(izlaz, kandidatCiscenja{
				putanja:     p,
				naziv:       "Claude " + pod + " (" + nazivProjekta + ")",
				kategorija:  "dev_ai",
				opis:        "Privremeni zapisi pod-agenata i alata (mapa memory ostaje sačuvana).",
				preporuceno: true,
			})
		}
	}
	return izlaz
}

func izmjeriKandidate(kandidati []kandidatCiscenja, najmanjaVelicina int64) []StavkaCiscenja {
	var mu sync.Mutex
	var stavke []StavkaCiscenja
	var wg sync.WaitGroup

	for _, kan := range kandidati {
		wg.Add(1)
		go func(k kandidatCiscenja) {
			defer wg.Done()
			vel, zadnjaIzmjena := izmjeriMapu(k.putanja)
			if vel < najmanjaVelicina {
				return
			}
			mu.Lock()
			stavke = append(stavke, StavkaCiscenja{
				ID:            k.putanja,
				Naziv:         k.naziv,
				Kategorija:    k.kategorija,
				Putanja:       k.putanja,
				VelicinaBajta: vel,
				Opis:          k.opis,
				Preporuceno:   k.preporuceno,
				DanaOdIzmjene: danaOdVremena(zadnjaIzmjena),
			})
			mu.Unlock()
		}(kan)
	}
	wg.Wait()

	sort.Slice(stavke, func(i, j int) bool {
		return stavke[i].VelicinaBajta > stavke[j].VelicinaBajta
	})
	return stavke
}

// SkenirajKopijeIPrivike pronalazi iOS sigurnosne kopije i privremena preuzimanja Maila.
func SkenirajKopijeIPrivike(p Postavke) []StavkaCiscenja {
	var kandidati []kandidatCiscenja
	kandidati = append(kandidati, prikupiKandidateIOSBackupova(filepath.Join(p.KorijenKorisnika, "Library", "Application Support", "MobileSync", "Backup"))...)
	kandidati = append(kandidati, prikupiKandidateMailDownloads(filepath.Join(p.KorijenKorisnika, "Library", "Containers", "com.apple.mail", "Data", "Library", "Mail Downloads"))...)
	return izmjeriKandidate(kandidati, p.NajmanjaVelicinaKopija)
}

func prikupiKandidateIOSBackupova(mapaBackup string) []kandidatCiscenja {
	ulazi, err := os.ReadDir(mapaBackup)
	if err != nil {
		return nil
	}
	var izlaz []kandidatCiscenja
	for _, u := range ulazi {
		if !u.IsDir() || strings.HasPrefix(u.Name(), ".") {
			continue
		}
		izlaz = append(izlaz, kandidatCiscenja{
			putanja:     filepath.Join(mapaBackup, u.Name()),
			naziv:       "iOS Sigurnosna Kopija: " + skratiIdentifikator(u.Name()),
			kategorija:  "backups_attachments",
			opis:        "Lokalna sigurnosna kopija iPhone ili iPad uređaja.",
			preporuceno: false,
		})
	}
	return izlaz
}

func skratiIdentifikator(id string) string {
	if len(id) > 16 {
		return id[:12] + "…"
	}
	return id
}

func prikupiKandidateMailDownloads(mapaMail string) []kandidatCiscenja {
	ulazi, err := os.ReadDir(mapaMail)
	if err != nil {
		return nil
	}
	var izlaz []kandidatCiscenja
	for _, u := range ulazi {
		if strings.HasPrefix(u.Name(), ".") {
			continue
		}
		izlaz = append(izlaz, kandidatCiscenja{
			putanja:     filepath.Join(mapaMail, u.Name()),
			naziv:       "Mail Privitak: " + u.Name(),
			kategorija:  "backups_attachments",
			opis:        "Lokalna kopija privitaka preuzetih unutar aplikacije Apple Mail.",
			preporuceno: true,
		})
	}
	return izlaz
}

// SkenirajSve izvršava cjeloviti pregled diska, predmemorije, razvojnih alata, ostataka i instalacija.
func SkenirajSve(p Postavke) IzvjestajSkeniranja {
	disk := DohvatiStanjeDiska(p.KorijenKorisnika)
	predmemorija := SkenirajPredmemoriju(p)
	razvojni := SkenirajRazvojneAlate(p)
	ostaci := SkenirajOstatke(p)
	kopije := SkenirajKopijeIPrivike(p)
	instalacije := SkenirajInstalacije(p)

	ukupno := zbrojiStavke(predmemorija, razvojni, ostaci, kopije, instalacije)

	return IzvjestajSkeniranja{
		Disk:                  disk,
		PredmemorijaSustava:   predmemorija,
		RazvojniAlati:         razvojni,
		OstaciAplikacija:      ostaci,
		KopijeIPrivici:        kopije,
		InstalacijskeDatoteke: instalacije,
		UkupnoZaOsloboditi:    ukupno,
		NedostupneMape:        ProvjeriDozvole(p),
	}
}

func zbrojiStavke(liste ...[]StavkaCiscenja) int64 {
	var ukupno int64
	for _, lista := range liste {
		for _, s := range lista {
			ukupno += s.VelicinaBajta
		}
	}
	return ukupno
}
