package cistac

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type kandidatCiscenja struct {
	putanja     string
	naziv       string
	kategorija  string
	opis        string
	preporuceno bool
}

var preskocenePredmemorije = map[string]bool{
	"go-build":          true,
	"Homebrew":          true,
	"pip":               true,
	"Codex":             true,
	"goimports":         true,
	"golangci-lint":     true,
	"gopls":             true,
	"ms-playwright":     true,
	"ms-playwright-go":  true,
	"CloudKit":          true,
	"FamilyCircle":      true,
	"GeoServices":       true,
}

func trebaPreskocitiPredmemoriju(ime string) bool {
	if preskocenePredmemorije[ime] {
		return true
	}
	return strings.HasPrefix(ime, "com.apple.") || strings.HasPrefix(ime, ".")
}

func opisPredmemorije(ime string) string {
	if strings.HasSuffix(ime, ".ShipIt") || strings.Contains(strings.ToLower(ime), "updater") {
		return "Privremeni instalacijski paketi ažuriranja aplikacije."
	}
	return "Korisnička predmemorija aplikacije u ~/Library/Caches."
}

// SkenirajPredmemoriju pronalazi stavke u ~/Library/Caches i ~/Library/Logs veće od praga.
func SkenirajPredmemoriju(p Postavke) []StavkaCiscenja {
	var kandidati []kandidatCiscenja
	kandidati = append(kandidati, prikupiKandidateCaches(filepath.Join(p.KorijenKorisnika, "Library", "Caches"))...)
	kandidati = append(kandidati, prikupiKandidateLogova(filepath.Join(p.KorijenKorisnika, "Library", "Logs"))...)
	return izmjeriKandidate(kandidati, p.NajmanjaVelicinaPredmemorije)
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
			preporuceno: true,
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
	return izmjeriKandidate(kandidati, p.NajmanjaVelicinaRazvojnih)
}

func fiksniRazvojniKandidati(k string) []kandidatCiscenja {
	return []kandidatCiscenja{
		{filepath.Join(k, "Library", "Caches", "go-build"), "Go Build Cache", "dev_ai", "Međuspremnik prevedenih Go paketa (go clean -cache).", true},
		{filepath.Join(k, "go", "pkg", "mod", "cache"), "Go Module Cache", "dev_ai", "Arhive preuzetih Go modula (go clean -modcache).", false},
		{filepath.Join(k, "Library", "Caches", "Homebrew", "downloads"), "Homebrew Downloads", "dev_ai", "Preuzete instalacijske arhive u Homebrewu.", true},
		{filepath.Join(k, ".npm", "_cacache"), "NPM Package Cache", "dev_ai", "Međuspremnik preuzetih Node.js paketa.", true},
		{filepath.Join(k, "Library", "Caches", "pip"), "Python Pip Cache", "dev_ai", "Međuspremnik preuzetih Python paketa.", true},
		{filepath.Join(k, ".cargo", "registry", "cache"), "Rust Cargo Cache", "dev_ai", "Preuzete arhive Rust paketa.", true},
		{filepath.Join(k, "Library", "Caches", "goimports"), "Goimports Cache", "dev_ai", "Privremeni indeks za goimports alat.", true},
		{filepath.Join(k, "Library", "Caches", "golangci-lint"), "GolangCI-Lint Cache", "dev_ai", "Međuspremnik za statičku analizu Go koda.", true},
		{filepath.Join(k, "Library", "Caches", "gopls"), "Gopls Cache", "dev_ai", "Privremena predmemorija za Go Language Server.", true},
		{filepath.Join(k, "Library", "Caches", "Codex"), "Codex App Cache", "dev_ai", "Predmemorija za Codex AI aplikaciju.", true},
		{filepath.Join(k, ".cache", "codex-runtimes"), "Codex Runtimes", "dev_ai", "Privremena okruženja za Codex CLI.", false},
		{filepath.Join(k, "Library", "Developer", "CoreSimulator", "Caches"), "CoreSimulator Cache", "dev_ai", "Korisnička predmemorija za iOS simulator.", true},
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
			vel := IzracunajVelicinu(k.putanja)
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

// SkenirajSve izvršava cjeloviti pregled diska, predmemorije, razvojnih alata i ostataka aplikacija.
func SkenirajSve(p Postavke) IzvjestajSkeniranja {
	disk := DohvatiStanjeDiska(p.KorijenKorisnika)
	predmemorija := SkenirajPredmemoriju(p)
	razvojni := SkenirajRazvojneAlate(p)
	ostaci := SkenirajOstatke(p)

	var ukupno int64
	for _, s := range predmemorija {
		ukupno += s.VelicinaBajta
	}
	for _, s := range razvojni {
		ukupno += s.VelicinaBajta
	}
	for _, s := range ostaci {
		ukupno += s.VelicinaBajta
	}

	return IzvjestajSkeniranja{
		Disk:                disk,
		PredmemorijaSustava: predmemorija,
		RazvojniAlati:       razvojni,
		OstaciAplikacija:    ostaci,
		UkupnoZaOsloboditi:  ukupno,
	}
}
