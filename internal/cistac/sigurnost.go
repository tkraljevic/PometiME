package cistac

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DozvoljeniKorijeni vraća popis mapa unutar kojih je dopušteno brisanje pod-stavki.
func DozvoljeniKorijeni(p Postavke) []string {
	k := filepath.Clean(p.KorijenKorisnika)
	app := filepath.Clean(p.KorijenAplikacija)
	return []string{
		filepath.Join(k, "Library", "Caches"),
		filepath.Join(k, "Library", "Logs"),
		filepath.Join(k, "Library", "Application Support"),
		filepath.Join(k, "Library", "Saved Application State"),
		filepath.Join(k, "Library", "WebKit"),
		filepath.Join(k, "Library", "HTTPStorages"),
		filepath.Join(k, "Library", "Preferences"),
		filepath.Join(k, "Library", "Containers"),
		filepath.Join(k, "Library", "Group Containers"),
		filepath.Join(k, "Library", "LaunchAgents"),
		filepath.Join(k, "Library", "Developer", "CoreSimulator"),
		filepath.Join(k, "Library", "Developer", "Xcode"),
		filepath.Join(k, "Library", "pnpm"),
		filepath.Join(k, ".npm"),
		filepath.Join(k, ".cache"),
		filepath.Join(k, ".gradle", "caches"),
		filepath.Join(k, ".m2", "repository"),
		filepath.Join(k, ".cargo", "registry", "cache"),
		filepath.Join(k, ".claude", "projects"),
		filepath.Join(k, ".android", "avd"),
		filepath.Join(k, "go", "pkg", "mod", "cache"),
		filepath.Join(k, "Downloads"),
		filepath.Join(k, "Applications"),
		app,
	}
}

// JeUnutarKorijena provjerava je li putanja izravno ili dublje dijete zadanog korijena.
func JeUnutarKorijena(cistaPutanja, korijen string) bool {
	cistiKorijen := filepath.Clean(korijen)
	if cistaPutanja == cistiKorijen || cistiKorijen == "/" || cistiKorijen == "." {
		return false
	}
	prefiks := cistiKorijen + string(filepath.Separator)
	return strings.HasPrefix(cistaPutanja, prefiks)
}

// jeZasticenaClaudeMemorija sprječava brisanje mape memory unutar Claude projekata.
func jeZasticenaClaudeMemorija(cistaPutanja string) bool {
	if !strings.Contains(cistaPutanja, ".claude") {
		return false
	}
	baza := filepath.Base(cistaPutanja)
	sadrziMem := strings.Contains(cistaPutanja, string(filepath.Separator)+"memory"+string(filepath.Separator))
	return baza == "memory" || sadrziMem
}

// JeSigurnaPutanja provjerava smije li se zadana putanja sigurno obrisati.
func JeSigurnaPutanja(p Postavke, putanja string) bool {
	if strings.TrimSpace(putanja) == "" || !filepath.IsAbs(putanja) {
		return false
	}
	cista := filepath.Clean(putanja)
	if jeZasticenaClaudeMemorija(cista) {
		return false
	}
	for _, korijen := range DozvoljeniKorijeni(p) {
		if JeUnutarKorijena(cista, korijen) {
			return true
		}
	}
	return false
}

// obrisiJednuStavku provjerava sigurnost, računa veličinu i uklanja putanju s diska.
func obrisiJednuStavku(p Postavke, putanja string) (int64, error) {
	if !JeSigurnaPutanja(p, putanja) {
		return 0, fmt.Errorf("putanja odbijena iz sigurnosnih razloga: %s", putanja)
	}
	cista := filepath.Clean(putanja)
	if _, err := os.Lstat(cista); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("provjera putanje %s: %w", cista, err)
	}
	velicina := IzracunajVelicinu(cista)
	if err := PremjestiUKos(p, cista, velicina); err != nil {
		return 0, err
	}
	return velicina, nil
}

// OcistiPutanje sigurno briše odabrane putanje i vraća izvještaj o oslobođenom prostoru.
func OcistiPutanje(p Postavke, putanje []string) RezultatCiscenja {
	var obrisano int
	var oslobodeno int64
	var greske []string

	for _, put := range putanje {
		bajta, err := obrisiJednuStavku(p, put)
		if err != nil {
			greske = append(greske, err.Error())
			continue
		}
		if bajta > 0 {
			obrisano++
			oslobodeno += bajta
		}
	}

	return RezultatCiscenja{
		ObrisanoStavki:  obrisano,
		OslobodenoBajta: oslobodeno,
		Poruka:          PorukaRezultata(obrisano, oslobodeno),
		Greske:          greske,
		Disk:            DohvatiStanjeDiska(p.KorijenKorisnika),
	}
}

// DeinstalirajAplikaciju provjerava je li riječ o .app paketu i premješta ga zajedno s podacima u Koš.
func DeinstalirajAplikaciju(p Postavke, putanjaApp string, povezane []string) RezultatCiscenja {
	cistaApp := filepath.Clean(putanjaApp)
	if !jeValjanaPutanjaAplikacije(p, cistaApp) {
		return RezultatCiscenja{
			Poruka: "Deinstalacija odbijena jer putanja nije valjana aplikacija.",
			Greske: []string{fmt.Sprintf("nevaljana putanja aplikacije: %s", putanjaApp)},
			Disk:   DohvatiStanjeDiska(p.KorijenKorisnika),
		}
	}
	svePutanje := make([]string, 0, 1+len(povezane))
	svePutanje = append(svePutanje, cistaApp)
	svePutanje = append(svePutanje, povezane...)
	return OcistiPutanje(p, svePutanje)
}

// UkloniPodatkeAplikacije premješta u Koš samo podatke aplikacije, bez paketa.
// Koristi se kad paket pripada rootu: korisnik ga premješta u Finderu, uz lozinku.
func UkloniPodatkeAplikacije(p Postavke, putanjaApp string, povezane []string) RezultatCiscenja {
	if !jeValjanaPutanjaAplikacije(p, filepath.Clean(putanjaApp)) {
		return RezultatCiscenja{
			Poruka: "Uklanjanje podataka odbijeno jer putanja nije valjana aplikacija.",
			Greske: []string{fmt.Sprintf("nevaljana putanja aplikacije: %s", putanjaApp)},
			Disk:   DohvatiStanjeDiska(p.KorijenKorisnika),
		}
	}
	return OcistiPutanje(p, povezane)
}

func jeValjanaPutanjaAplikacije(p Postavke, cistaApp string) bool {
	if !strings.HasSuffix(strings.ToLower(cistaApp), ".app") {
		return false
	}
	korisnickeApp := filepath.Join(filepath.Clean(p.KorijenKorisnika), "Applications")
	return JeUnutarKorijena(cistaApp, p.KorijenAplikacija) || JeUnutarKorijena(cistaApp, korisnickeApp)
}
