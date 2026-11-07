package cistac

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestZadanePostavkeIModel(t *testing.T) {
	p := ZadanePostavke("/Users/test/")
	if p.KorijenKorisnika != "/Users/test" {
		t.Fatalf("očekivan čisti korijen /Users/test, dobiveno %s", p.KorijenKorisnika)
	}
	if p.KorijenAplikacija != "/Applications" {
		t.Fatalf("očekivan /Applications, dobiveno %s", p.KorijenAplikacija)
	}
	if p.NajmanjaVelicinaInstalacija != 1024*1024 {
		t.Fatalf("očekivana najmanja veličina instalacija 1MB, dobiveno %d", p.NajmanjaVelicinaInstalacija)
	}

	slucajevi := []struct {
		n        int
		ocekivan string
	}{
		{0, "0 stavki"},
		{1, "1 stavka"},
		{2, "2 stavke"},
		{4, "4 stavke"},
		{5, "5 stavki"},
		{11, "11 stavki"},
		{12, "12 stavki"},
		{14, "14 stavki"},
		{21, "21 stavka"},
		{23, "23 stavke"},
		{-1, "-1 stavka"},
	}
	for _, sl := range slucajevi {
		dobiveno := UzBrojHR(sl.n, "stavka", "stavke", "stavki")
		if dobiveno != sl.ocekivan {
			t.Errorf("UzBrojHR(%d) = %q, očekivano %q", sl.n, dobiveno, sl.ocekivan)
		}
	}
}

func TestPorukaRezultataIFormatiranje(t *testing.T) {
	prazno := PorukaRezultata(0, 0)
	if !strings.Contains(prazno, "Nijedna stavka") {
		t.Errorf("neočekivana poruka za 0: %s", prazno)
	}
	jedna := PorukaRezultata(1, 2048)
	if !strings.Contains(jedna, "1 stavka") || !strings.Contains(jedna, "2 KB") {
		t.Errorf("neočekivana poruka za 1: %s", jedna)
	}

	if FormatirajBajte(512) != "512 B" {
		t.Errorf("FormatirajBajte(512) = %s", FormatirajBajte(512))
	}
	if FormatirajBajte(2048) != "2 KB" {
		t.Errorf("FormatirajBajte(2048) = %s", FormatirajBajte(2048))
	}
	if FormatirajBajte(5*1024*1024) != "5.0 MB" {
		t.Errorf("FormatirajBajte(5MB) = %s", FormatirajBajte(5*1024*1024))
	}
	if FormatirajBajte(2*1024*1024*1024) != "2.00 GB" {
		t.Errorf("FormatirajBajte(2GB) = %s", FormatirajBajte(2*1024*1024*1024))
	}
}

func TestJeSigurnaPutanja(t *testing.T) {
	dom := t.TempDir()
	apps := t.TempDir()
	p := Postavke{
		KorijenKorisnika:  dom,
		KorijenAplikacija: apps,
	}

	sigurne := []string{
		filepath.Join(dom, "Library", "Caches", "com.nekialat"),
		filepath.Join(dom, "Library", "Logs", "nekialat.log"),
		filepath.Join(dom, ".claude", "projects", "moj-projekt", "sesija-1", "subagents"),
		filepath.Join(dom, "Downloads", "program.dmg"),
		filepath.Join(apps, "NekaApp.app"),
	}
	for _, s := range sigurne {
		if !JeSigurnaPutanja(p, s) {
			t.Errorf("putanja %s trebala bi biti sigurna", s)
		}
	}

	nesigurne := []string{
		"",
		"   ",
		"relativna/putanja",
		"/",
		dom,
		apps,
		filepath.Join(dom, "Library", "Caches"),
		filepath.Join(dom, "Downloads"),
		filepath.Join(dom, "Documents", "vazno.txt"),
		filepath.Join(dom, ".claude", "projects", "moj-projekt", "memory"),
		filepath.Join(dom, ".claude", "projects", "moj-projekt", "memory", "MEMORY.md"),
	}
	for _, ns := range nesigurne {
		if JeSigurnaPutanja(p, ns) {
			t.Errorf("putanja %s NE smije biti sigurna", ns)
		}
	}
	if JeUnutarKorijena("/a", "/") || JeUnutarKorijena("a", ".") {
		t.Errorf("korijen / ili . ne smije biti prihvaćen")
	}
}

func TestDiskIIzracunajVelicinu(t *testing.T) {
	priv := t.TempDir()
	st := DohvatiStanjeDiska(priv)
	if st.UkupnoBajta <= 0 || st.PostotakZauzet <= 0 {
		t.Errorf("neočekivano stanje diska: %+v", st)
	}
	stPrazno := DohvatiStanjeDiska("")
	if stPrazno.TockaMontaze != "/" {
		t.Errorf("očekivana točka montaže /, dobiveno %s", stPrazno.TockaMontaze)
	}
	stNepostojece := DohvatiStanjeDiska("/nepostojeca/putanja/xyz123")
	if stNepostojece.UkupnoBajta != 0 {
		t.Errorf("očekivano 0 za nepostojeću putanju, dobiveno %d", stNepostojece.UkupnoBajta)
	}
	nulaStanje := izracunajStanjeIzStatfs("/", 0, 0, 4096)
	if nulaStanje.PostotakZauzet != 0 {
		t.Errorf("očekivano 0%% za 0 blokova")
	}

	podmapa := filepath.Join(priv, "pod")
	if err := os.MkdirAll(podmapa, 0o755); err != nil {
		t.Fatal(err)
	}
	dat := filepath.Join(podmapa, "datoteka.bin")
	if err := os.WriteFile(dat, make([]byte, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	if vel := IzracunajVelicinu(priv); vel != 4096 {
		t.Errorf("očekivano 4096 bajta, dobiveno %d", vel)
	}
	if vel := velicinaUlaza(nil, os.ErrNotExist); vel != 0 {
		t.Errorf("očekivano 0 za grešku ulaza, dobiveno %d", vel)
	}
}

func napraviDatoteku(t *testing.T, putanja string, velicina int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(putanja), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(putanja, make([]byte, velicina), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSkenirajSveIOcisti(t *testing.T) {
	dom := t.TempDir()
	apps := t.TempDir()
	p := Postavke{
		KorijenKorisnika:             dom,
		KorijenAplikacija:            apps,
		NajmanjaVelicinaPredmemorije: 100,
		NajmanjaVelicinaRazvojnih:    100,
		NajmanjaVelicinaOstataka:     100,
		NajmanjaVelicinaKopija:       100,
		NajmanjaVelicinaInstalacija:  100,
	}

	// 1. Predmemorija i logovi
	napraviDatoteku(t, filepath.Join(dom, "Library", "Caches", "com.test.ShipIt", "pkg.bin"), 500)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Caches", "ObičnaApp", "cache.db"), 300)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Caches", "Google", "chrome.db"), 350)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Caches", "PremalaApp", "mali.db"), 10)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Caches", "com.apple.Safari", "sys.db"), 500)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Caches", "go-build", "go.o"), 600)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Logs", ".skriveni"), 500)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Logs", "TestLog", "app.log"), 400)

	// 2. Claude sesije (s memory mapom koja se mora preskočiti)
	napraviDatoteku(t, filepath.Join(dom, ".claude", "projects", "obicna-datoteka.txt"), 50)
	claudeProj := filepath.Join(dom, ".claude", "projects", "moj-proj")
	napraviDatoteku(t, filepath.Join(claudeProj, "memory", "MEMORY.md"), 800)
	napraviDatoteku(t, filepath.Join(claudeProj, "datoteka.jsonl"), 800)
	napraviDatoteku(t, filepath.Join(claudeProj, "sesija-1", "subagents", "agent.jsonl"), 700)

	// 3. Aplikacije i ostaci
	plistTekst := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>org.primjer.mojaapp</string>
<key>CFBundleDisplayName</key><string>Moja App</string>
<key>CFBundleShortVersionString</key><string>2.4.1</string>
</dict></plist>`
	appPut := filepath.Join(apps, "MojaApp.app")
	napraviDatoteku(t, filepath.Join(appPut, "Contents", "Info.plist"), len(plistTekst))
	if err := os.WriteFile(filepath.Join(appPut, "Contents", "Info.plist"), []byte(plistTekst), 0o600); err != nil {
		t.Fatal(err)
	}
	napraviDatoteku(t, filepath.Join(appPut, "Contents", "MacOS", "bin"), 1000)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Application Support", "org.primjer.mojaapp", "data.db"), 450)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Application Support", "com.apple.siri", "sys.db"), 450)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Application Support", "google", "g.db"), 450)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Application Support", "com.google.Chrome", "c.db"), 450)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Application Support", "StaraObrisanaApp", "old.db"), 900)

	// 4. iOS Backupi i Mail preuzimanja
	napraviDatoteku(t, filepath.Join(dom, "Library", "Application Support", "MobileSync", "Backup", "00008101-001234567890ABCD", "Manifest.db"), 600)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Application Support", "MobileSync", "Backup", ".skriveni", "s.db"), 600)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Containers", "com.apple.mail", "Data", "Library", "Mail Downloads", "izvjestaj.pdf"), 750)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Containers", "com.apple.mail", "Data", "Library", "Mail Downloads", ".skriveni.pdf"), 750)

	// 5. Stari instaleri u Downloads
	installerPath := filepath.Join(dom, "Downloads", "program.dmg")
	napraviDatoteku(t, installerPath, 850)
	stariDatum := time.Now().Add(-10 * 24 * time.Hour)
	_ = os.Chtimes(installerPath, stariDatum, stariDatum) // simulacija starosti datoteke

	// Skeniranje nad nepostojećim mapama
	_ = SkenirajSve(Postavke{KorijenKorisnika: filepath.Join(dom, "nepostoji"), KorijenAplikacija: filepath.Join(apps, "nepostoji")})

	izvj := SkenirajSve(p)
	if len(izvj.PredmemorijaSustava) != 4 {
		t.Fatalf("očekivane 4 stavke predmemorije/logova, dobiveno %d", len(izvj.PredmemorijaSustava))
	}
	if len(izvj.RazvojniAlati) != 2 {
		t.Fatalf("očekivane 2 razvojne stavke (go-build i claude subagents), dobiveno %d", len(izvj.RazvojniAlati))
	}
	if len(izvj.OstaciAplikacija) != 1 {
		t.Fatalf("očekivan 1 ostatak aplikacije, dobiveno %d", len(izvj.OstaciAplikacija))
	}
	if len(izvj.KopijeIPrivici) != 2 {
		t.Fatalf("očekivane 2 stavke kopija i privitaka, dobiveno %d", len(izvj.KopijeIPrivici))
	}
	if len(izvj.InstalacijskeDatoteke) != 1 {
		t.Fatalf("očekivana 1 stavka instalacijskih datoteka, dobiveno %d", len(izvj.InstalacijskeDatoteke))
	}
	if skratiIdentifikator("kratko") != "kratko" {
		t.Errorf("očekivan nepromijenjen kratki identifikator")
	}

	// Test čišćenja u Koš i povrata iz Koša
	prvaPutanja := izvj.PredmemorijaSustava[0].Putanja
	rez := OcistiPutanje(p, []string{
		prvaPutanja,
		filepath.Join(dom, "Library", "Caches", "nepostojeca-stavka"),
		"/zabranjena/putanja",
	})
	if rez.ObrisanoStavki != 1 || len(rez.Greske) != 1 {
		t.Fatalf("neočekivan rezultat čišćenja: %+v", rez)
	}
	uKosu := IzlistajKos(p)
	if len(uKosu) != 1 {
		t.Fatalf("očekivana 1 stavka u Košu, dobiveno %d", len(uKosu))
	}
	povratRez := VratiIzKosa(p, []string{uKosu[0].PutanjaUKosu})
	if povratRez.ObrisanoStavki != 1 {
		t.Fatalf("očekivana 1 vraćena stavka, dobiveno %+v", povratRez)
	}
	if _, err := os.Stat(prvaPutanja); err != nil {
		t.Errorf("izvorna putanja %s nije vraćena iz Koša: %v", prvaPutanja, err)
	}
}

func TestSkenirajIDeinstalirajAplikacije(t *testing.T) {
	dom := t.TempDir()
	apps := t.TempDir()
	p := Postavke{KorijenKorisnika: dom, KorijenAplikacija: apps}

	// Aplikacija s CFBundleName, Apple aplikacija, aplikacija bez plista i obična datoteka
	if err := os.WriteFile(filepath.Join(apps, "README.txt"), []byte("info"), 0o600); err != nil {
		t.Fatal(err)
	}

	appleApp := filepath.Join(apps, "Safari.app")
	napraviDatoteku(t, filepath.Join(appleApp, "Contents", "Info.plist"), 10)
	_ = os.WriteFile(filepath.Join(appleApp, "Contents", "Info.plist"), []byte(`<key>CFBundleIdentifier</key><string>com.apple.Safari</string>`), 0o600) // testna datoteka u TempDir

	bezPlistApp := filepath.Join(apps, "DrugaApp.app")
	napraviDatoteku(t, filepath.Join(bezPlistApp, "Contents", "MacOS", "bin"), 300)
	napraviDatoteku(t, filepath.Join(dom, "Library", "Caches", "drugaapp", "c.bin"), 200)

	imePlistApp := filepath.Join(apps, "Podmapa", "Treca.app")
	napraviDatoteku(t, filepath.Join(imePlistApp, "Contents", "Info.plist"), 10)
	_ = os.WriteFile(filepath.Join(imePlistApp, "Contents", "Info.plist"), []byte(`<key>CFBundleName</key><string>Treca App</string>`), 0o600) // testna datoteka u TempDir
	napraviDatoteku(t, filepath.Join(apps, "Podmapa", "obicna.txt"), 10)

	popis := SkenirajAplikacije(p)
	if len(popis) != 2 {
		t.Fatalf("očekivane 2 ne-Apple aplikacije, dobiveno %d", len(popis))
	}

	// Odbijanje nevaljane deinstalacije
	losRez := DeinstalirajAplikaciju(p, filepath.Join(apps, "NijeApp.txt"), nil)
	if len(losRez.Greske) == 0 {
		t.Errorf("očekivana greška za nevaljanu putanju aplikacije")
	}

	// Uspješna deinstalacija u Koš i test kolizije naziva u Košu
	dobarRez := DeinstalirajAplikaciju(p, bezPlistApp, []string{
		filepath.Join(dom, "Library", "Caches", "drugaapp"),
	})
	if dobarRez.ObrisanoStavki != 2 {
		t.Errorf("očekivane 2 obrisane stavke pri deinstalaciji, dobiveno %d", dobarRez.ObrisanoStavki)
	}

	// Druga stavka istog imena za test vremenske oznake u Košu i praznog povrata
	napraviDatoteku(t, filepath.Join(dom, "Library", "Caches", "drugaapp", "c2.bin"), 150)
	_ = OcistiPutanje(p, []string{filepath.Join(dom, "Library", "Caches", "drugaapp")})
	prazanPovrat := VratiIzKosa(p, []string{"/nepostojeca/u/kosu"})
	if !strings.Contains(prazanPovrat.Poruka, "Nijedna stavka") {
		t.Errorf("neočekivana poruka za prazan povrat: %s", prazanPovrat.Poruka)
	}
}

func TestSkenirajPokretaceIOptimizaciju(t *testing.T) {
	dom := t.TempDir()
	p := Postavke{KorijenKorisnika: dom, KorijenAplikacija: t.TempDir()}

	// Nepostojeća mapa
	if res := SkenirajPokretace(Postavke{KorijenKorisnika: filepath.Join(dom, "nepostoji")}); len(res) != 0 {
		t.Errorf("očekivano 0 pokretača za nepostojeću mapu")
	}

	mapaAg := filepath.Join(dom, "Library", "LaunchAgents")
	postojeciBin := filepath.Join(dom, "bin", "moj-alat")
	napraviDatoteku(t, postojeciBin, 100)

	// 1. Aktivan s Program ključem
	aktivanPlist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>Program</key><string>%s</string>
</dict></plist>`, postojeciBin)
	napraviDatoteku(t, filepath.Join(mapaAg, "hr.aktivan.plist"), len(aktivanPlist))
	_ = os.WriteFile(filepath.Join(mapaAg, "hr.aktivan.plist"), []byte(aktivanPlist), 0o600) // testni plist pokretača

	// 2. Siroče s ProgramArguments ključem koji cilja nepostojeći program
	sirocePlist := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>ProgramArguments</key><array><string>/Applications/Obrisana.app/Contents/MacOS/bin</string></array>
</dict></plist>`
	napraviDatoteku(t, filepath.Join(mapaAg, "hr.siroce.plist"), len(sirocePlist))
	_ = os.WriteFile(filepath.Join(mapaAg, "hr.siroce.plist"), []byte(sirocePlist), 0o600) // testni plist siročeta

	// 3. Pokretač bez program ključa (fallback)
	prazanPlist := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>Label</key><string>hr.prazan</string>
</dict></plist>`
	napraviDatoteku(t, filepath.Join(mapaAg, "hr.prazan.plist"), len(prazanPlist))
	_ = os.WriteFile(filepath.Join(mapaAg, "hr.prazan.plist"), []byte(prazanPlist), 0o600) // testni prazni plist

	// Datoteka koja nije plist
	napraviDatoteku(t, filepath.Join(mapaAg, "README.txt"), 10)

	pokretaci := SkenirajPokretace(p)
	if len(pokretaci) != 3 {
		t.Fatalf("očekivana 3 pokretača, dobiveno %d", len(pokretaci))
	}
	// Prvi mora biti siroče zbog sortiranja
	if pokretaci[0].Status != "siroce" {
		t.Errorf("očekivano da siroče bude prvo na popisu, dobiveno %s", pokretaci[0].Status)
	}

	// Test nečitljivog pokretača
	necitljiv := obradiJedanPokretac(filepath.Join(dom, "nepostojeci.plist"), "nepostojeci.plist", true)
	if necitljiv.Status != "nepoznato" {
		t.Errorf("očekivan status nepoznato za nepostojeću datoteku, dobiveno %s", necitljiv.Status)
	}

	// Test optimizacije
	vratiPokretac := PostaviPokretacNaredbiZaTest(func(name string, args ...string) error {
		return nil
	})
	defer vratiPokretac()

	if _, err := IzvrsiOptimizaciju(p, "dns"); err != nil {
		t.Errorf("greška pri izvršavanju dns optimizacije: %v", err)
	}
	if _, err := IzvrsiOptimizaciju(p, "quicklook"); err != nil {
		t.Errorf("greška pri izvršavanju quicklook optimizacije: %v", err)
	}
	if _, err := IzvrsiOptimizaciju(p, "snapshots"); err != nil {
		t.Errorf("greška pri izvršavanju snapshots optimizacije: %v", err)
	}
	if _, err := IzvrsiOptimizaciju(p, "otvori_aerials"); err != nil {
		t.Errorf("greška pri otvori_aerials optimizaciji: %v", err)
	}
	if _, err := IzvrsiOptimizaciju(p, "nepoznato"); err == nil {
		t.Errorf("očekivana greška za nepoznatu optimizaciju")
	}

	// Test PopisLokalnihSnimaka
	stariIzlaz := pokreniIzlazNaredbe
	defer func() { pokreniIzlazNaredbe = stariIzlaz }()
	pokreniIzlazNaredbe = func(name string, args ...string) ([]byte, error) {
		return []byte("Snapshots for volume group '/':\ncom.apple.TimeMachine.2026-11-05-143021.local\ncom.apple.TimeMachine.2026-11-06-184512.local\n"), nil
	}
	snimke := PopisLokalnihSnimaka()
	if len(snimke) != 2 {
		t.Fatalf("očekivane 2 snimke, dobiveno %d", len(snimke))
	}

	// Test greške pri dohvatu snimaka
	pokreniIzlazNaredbe = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("simulirani pad naredbe")
	}
	if sn := PopisLokalnihSnimaka(); sn != nil {
		t.Errorf("očekivan nil pri grešci naredbe, dobiveno %v", sn)
	}

	// Test greške pri izvršavanju dns i snapshots optimizacije
	pokreniNaredbu = func(name string, args ...string) error {
		return fmt.Errorf("simulirana greška")
	}
	if _, err := IzvrsiOptimizaciju(p, "dns"); err == nil {
		t.Errorf("očekivana greška pri simuliranom padu naredbe")
	}
	if _, err := IzvrsiOptimizaciju(p, "snapshots"); err == nil {
		t.Errorf("očekivana greška pri simuliranom padu snapshots naredbe")
	}
}

func TestZracnePozadine(t *testing.T) {
	dom := t.TempDir()
	sysLib := t.TempDir()
	stariSysLib := korijenSustavskeKnjiznice
	korijenSustavskeKnjiznice = sysLib
	defer func() { korijenSustavskeKnjiznice = stariSysLib }()

	p := Postavke{KorijenKorisnika: dom, KorijenAplikacija: t.TempDir()}

	// 1. Prazno stanje
	infoPrazno := SkenirajZracnePozadine(p)
	if infoPrazno.BrojVidea != 0 || infoPrazno.UkupnoBajta != 0 {
		t.Errorf("očekivano 0 videa i 0 bajta, dobiveno %d (%d)", infoPrazno.BrojVidea, infoPrazno.UkupnoBajta)
	}

	porukaPrazno, err := OcistiZracnePozadine(p)
	if err != nil || !strings.Contains(porukaPrazno, "Nema preuzetih") {
		t.Errorf("očekivana poruka za prazno stanje, dobiveno %s (%v)", porukaPrazno, err)
	}

	// 2. Kreiranje datoteka u Customer mapi i korisničkoj mapi
	mapaCustomer := filepath.Join(sysLib, "Application Support", "com.apple.idleassetsd", "Customer", "4KSDR240FPS")
	video1 := filepath.Join(mapaCustomer, "video1.mov")
	video2 := filepath.Join(mapaCustomer, "video2.MOV")
	bundleTxt := filepath.Join(sysLib, "Application Support", "com.apple.idleassetsd", "Customer", "TVIdleScreenStrings.bundle", "strings.plist")
	napraviDatoteku(t, video1, 1000)
	napraviDatoteku(t, video2, 2000)
	napraviDatoteku(t, bundleTxt, 500)

	userCache := filepath.Join(dom, "Library", "Containers", "com.apple.wallpaper.agent", "Data", "Library", "Caches", "com.apple.wallpaper.caches", "cached.bin")
	napraviDatoteku(t, userCache, 300)

	infoPuno := SkenirajZracnePozadine(p)
	if infoPuno.BrojVidea != 2 {
		t.Errorf("očekivana 2 videa, dobiveno %d", infoPuno.BrojVidea)
	}
	if infoPuno.UkupnoBajta != 3300 {
		t.Errorf("očekivano 3300 bajta (3000 videa + 300 user cache), dobiveno %d", infoPuno.UkupnoBajta)
	}

	// 3. Test čišćenja
	porukaOcisceno, err := IzvrsiOptimizaciju(p, "aerials")
	if err != nil || !strings.Contains(porukaOcisceno, "uspješno očišćeni") {
		t.Errorf("očekivan uspjeh čišćenja aerials, dobiveno %s (%v)", porukaOcisceno, err)
	}

	// 4. Test greške pri administratorskom brisanju
	napraviDatoteku(t, video1, 1000)
	staraNaredba := pokreniNaredbu
	defer func() { pokreniNaredbu = staraNaredba }()

	// Simuliraj da os.Remove baci grešku tako da uđe u AppleScript granu
	_ = os.Chmod(filepath.Dir(video1), 0o555) // simulacija nedostatka prava pisanja u testu
	defer func() {
		_ = os.Chmod(filepath.Dir(video1), 0o755) // vraćanje prava nakon testa
	}()

	pokreniNaredbu = func(name string, args ...string) error {
		return fmt.Errorf("odbijena administratorska dozvola")
	}
	if _, err := OcistiZracnePozadine(p); err == nil {
		t.Errorf("očekivana greška pri odbijanju administratorske dozvole")
	}
}

func TestUsporediInacice(t *testing.T) {
	slucajevi := []struct {
		trenutna  string
		najnovija string
		ocekivano bool
	}{
		{"1.0.0", "1.0.1", true},
		{"1.0.0", "1.1.0", true},
		{"1.0.0", "2.0.0", true},
		{"1.0.0", "1.0.0", false},
		{"1.0.1", "1.0.0", false},
		{"v1.0.0", "v1.0.1", true},
		{"v1.2.0", "1.1.9", false},
		{"abc", "def", false},
	}

	for _, s := range slucajevi {
		dobiveno := UsporediInacice(s.trenutna, s.najnovija)
		if dobiveno != s.ocekivano {
			t.Errorf("UsporediInacice(%q, %q) = %v; očekivano %v", s.trenutna, s.najnovija, dobiveno, s.ocekivano)
		}
	}
}

func TestProvjeriNadogradnju(t *testing.T) {
	posluzitelj := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/neispravan-json" {
			_, _ = w.Write([]byte("{neispravan"))
			return
		}

		izdanje := githubRelease{
			TagName: "v1.2.0",
			Name:    "PometiME 1.2.0",
			Body:    "Ispravci i poboljšanja",
			HTMLURL: "https://github.com/tkraljevic/PometiME/releases/tag/v1.2.0",
			Assets: []githubAsset{
				{
					Name:               "PometiME-macOS.zip",
					Size:               7000000,
					BrowserDownloadURL: "https://github.com/tkraljevic/PometiME/releases/download/v1.2.0/PometiME-macOS.zip",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(izdanje) // kodiranje mock odgovora
	}))
	defer posluzitelj.Close()

	vrati := PostaviNadogradnjuZaTest(posluzitelj.URL, posluzitelj.Client())
	defer vrati()

	// 1. Uspješna provjera
	info, err := ProvjeriNadogradnju()
	if err != nil {
		t.Fatalf("očekivana uspješna provjera, dobiveno: %v", err)
	}
	if !info.ImaNadogradnju {
		t.Errorf("očekivano info.ImaNadogradnju == true")
	}
	if info.NajnovijaInacica != "1.2.0" {
		t.Errorf("očekivana najnovija inačica 1.2.0, dobiveno %s", info.NajnovijaInacica)
	}
	if info.VelicinaBajta != 7000000 {
		t.Errorf("očekivana veličina 7000000, dobiveno %d", info.VelicinaBajta)
	}

	// 2. Greška poslužitelja
	_ = PostaviNadogradnjuZaTest(posluzitelj.URL+"/error", posluzitelj.Client())
	if _, err := ProvjeriNadogradnju(); err == nil {
		t.Errorf("očekivana greška za status 500")
	}

	// 3. Neispravan JSON
	_ = PostaviNadogradnjuZaTest(posluzitelj.URL+"/neispravan-json", posluzitelj.Client())
	if _, err := ProvjeriNadogradnju(); err == nil {
		t.Errorf("očekivana greška za neispravan JSON")
	}

	// 4. Greška u URL-u / mreži
	_ = PostaviNadogradnjuZaTest("http://127.0.0.1:0/nepostojece", posluzitelj.Client())
	if _, err := ProvjeriNadogradnju(); err == nil {
		t.Errorf("očekivana greška za nepostojeći host")
	}
}

func stvoriTestniZipPaket(t *testing.T, sadrzajApp bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	if sadrzajApp {
		fDir, err := zw.Create("PometiME.app/")
		if err != nil {
			t.Fatal(err)
		}
		_ = fDir

		f1, err := zw.Create("PometiME.app/Contents/Info.plist")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f1.Write([]byte("<plist></plist>")); err != nil {
			t.Fatal(err)
		}

		f2, err := zw.Create("PometiME.app/Contents/MacOS/PometiME")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f2.Write([]byte("dummy-bin")); err != nil {
			t.Fatal(err)
		}

		fSkip, err := zw.Create("../opasno.txt")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fSkip.Write([]byte("skip")); err != nil {
			t.Fatal(err)
		}

		fAbs, err := zw.Create("/apsolutno.txt")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fAbs.Write([]byte("abs")); err != nil {
			t.Fatal(err)
		}
	} else {
		f, err := zw.Create("obican.txt")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte("tekst")); err != nil {
			t.Fatal(err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPrimijeniNadogradnju(t *testing.T) {
	stariKlijent := klijentNadogradnje
	defer func() {
		klijentNadogradnje = stariKlijent
	}()

	zipIspravan := stvoriTestniZipPaket(t, true)
	zipBezAppa := stvoriTestniZipPaket(t, false)

	posluzitelj := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipIspravan)
		case "/bez-appa.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipBezAppa)
		case "/neispravan.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write([]byte("nije zip datoteka"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer posluzitelj.Close()

	klijentNadogradnje = posluzitelj.Client()

	privremenaMapa := t.TempDir()
	odredisniApp := filepath.Join(privremenaMapa, "PometiME.app")
	vratiPutanju := PostaviPutanjuAplikacijeZaTest(func() string {
		return odredisniApp
	})
	defer vratiPutanju()

	// 1. Prazna poveznica
	if err := PrimijeniNadogradnju(""); err == nil {
		t.Errorf("očekivana greška za prazan downloadURL")
	}

	// 2. 404 poslužitelj
	if err := PrimijeniNadogradnju(posluzitelj.URL + "/nepostojece.zip"); err == nil {
		t.Errorf("očekivana greška za status 404")
	}

	// 3. Neispravan zip
	if err := PrimijeniNadogradnju(posluzitelj.URL + "/neispravan.zip"); err == nil {
		t.Errorf("očekivana greška za oštećenu arhivu")
	}

	// 4. Zip bez PometiME.app
	if err := PrimijeniNadogradnju(posluzitelj.URL + "/bez-appa.zip"); err == nil {
		t.Errorf("očekivana greška za arhivu bez PometiME.app paketa")
	}

	// 5. Uspješna instalacija (ciljna aplikacija još ne postoji)
	if err := PrimijeniNadogradnju(posluzitelj.URL + "/ok.zip"); err != nil {
		t.Fatalf("očekivana uspješna instalacija, dobiveno: %v", err)
	}
	if _, err := os.Stat(filepath.Join(odredisniApp, "Contents", "Info.plist")); err != nil {
		t.Errorf("instalirani paket ne sadrži Info.plist: %v", err)
	}

	// 6. Uspješna nadogradnja (postojeća aplikacija već postoji na disku)
	if err := PrimijeniNadogradnju(posluzitelj.URL + "/ok.zip"); err != nil {
		t.Fatalf("očekivana uspješna nadogradnja preko postojećeg paketa, dobiveno: %v", err)
	}

	// 7. Provjera pomoćne funkcije odrediPutanjuAplikacije
	putanja := odrediPutanjuAplikacije()
	if putanja == "" {
		t.Errorf("odrediPutanjuAplikacije vratila prazan niz")
	}

	p1 := izracunajPutanjuAppa("/Applications/PometiME.app/Contents/MacOS/PometiME")
	if p1 != "/Applications/PometiME.app" {
		t.Errorf("očekivano /Applications/PometiME.app, dobiveno %s", p1)
	}

	p2 := izracunajPutanjuAppa("/usr/local/bin/pometime")
	if p2 != "/Applications/PometiME.app" {
		t.Errorf("očekivano /Applications/PometiME.app, dobiveno %s", p2)
	}

	// 8. Provjera pronadjiZipAsset s praznim ili nepodržanim formatima
	urlAsset, vel := pronadjiZipAsset([]githubAsset{{Name: "paket.tar.gz", Size: 100}})
	if urlAsset != "" || vel != 0 {
		t.Errorf("očekivan prazan rezultat za nepodržani format, dobiveno %s, %d", urlAsset, vel)
	}

	// 9. Provjera pronadjiAppPaket u nepostojećoj mapi
	if appPaket := pronadjiAppPaket(filepath.Join(privremenaMapa, "nepostoji")); appPaket != "" {
		t.Errorf("očekivan prazan paket za nepostojeću mapu, dobiveno %s", appPaket)
	}

	// 10. Greška pri stvaranju privremene mape
	stariStvoriDir := stvoriPrivremenuMapuFn
	defer func() { stvoriPrivremenuMapuFn = stariStvoriDir }()
	stvoriPrivremenuMapuFn = func() (string, error) {
		return "", fmt.Errorf("greška privremene mape")
	}
	if err := PrimijeniNadogradnju("https://example.com/test.zip"); err == nil {
		t.Errorf("očekivana greška pri neuspjehu stvaranja privremene mape")
	}
	stvoriPrivremenuMapuFn = stariStvoriDir

	// 11. Greška pri dohvaćanju izvršne putanje
	stariExec := executablePutanjaFn
	defer func() { executablePutanjaFn = stariExec }()
	executablePutanjaFn = func() (string, error) {
		return "", fmt.Errorf("greška putanje")
	}
	if p := odrediPutanjuAplikacije(); p != "/Applications/PometiME.app" {
		t.Errorf("očekivan fallback na /Applications/PometiME.app, dobiveno %s", p)
	}
	executablePutanjaFn = stariExec

	// 12. Greška preuzimanja u nevažeću lokalnu datoteku i mrežna greška
	if err := preuzmiDatoteku(posluzitelj.URL+"/ok.zip", filepath.Join(privremenaMapa, "nepostoji", "nemoguce.zip")); err == nil {
		t.Errorf("očekivana greška za nemoguću lokalnu datoteku")
	}
	if err := preuzmiDatoteku("http://127.0.0.1:0/err", filepath.Join(privremenaMapa, "dummy.zip")); err == nil {
		t.Errorf("očekivana mrežna greška za preuzmiDatoteku")
	}

	// 13. Greška raspakiravanja u blokiranu datoteku umjesto mape
	blokiraniCilj := filepath.Join(privremenaMapa, "blokirano")
	if err := os.WriteFile(blokiraniCilj, []byte("sadrzaj"), 0o600); err != nil {
		t.Fatal(err)
	}
	zipPutanja := filepath.Join(privremenaMapa, "privremeni.zip")
	if err := os.WriteFile(zipPutanja, zipIspravan, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := raspakirajZip(zipPutanja, blokiraniCilj); err == nil {
		t.Errorf("očekivana greška pri raspakiravanju u datoteku umjesto mape")
	}

	// 14. Greška pri zamjeni na nevažeću lokaciju
	_ = PostaviPutanjuAplikacijeZaTest(func() string {
		return "/dev/null/nemoguce/PometiME.app"
	})
	if err := PrimijeniNadogradnju(posluzitelj.URL + "/ok.zip"); err == nil {
		t.Errorf("očekivana greška pri instalaciji na nevažeću lokaciju")
	}
	if err := zamijeniAplikaciju(filepath.Join(privremenaMapa, "novi.app"), "/dev/null/nemoguce/PometiME.app"); err == nil {
		t.Errorf("očekivana greška pri nemogućoj zamjeni aplikacije")
	}

	// 15. Rollback u zamijeniAplikaciju
	postojeciAppZaRollback := filepath.Join(privremenaMapa, "ZaRollback.app")
	if err := os.MkdirAll(postojeciAppZaRollback, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := zamijeniAplikaciju(filepath.Join(privremenaMapa, "nepostojeci-novi.app"), postojeciAppZaRollback); err == nil {
		t.Errorf("očekivana greška pri zamjeni s nepostojećim novim appom")
	}
	if _, err := os.Stat(postojeciAppZaRollback); err != nil {
		t.Errorf("očekivano da rollback vrati aplikaciju na izvorno mjesto: %v", err)
	}

	// 16. Greška pri otvaranju unosa s nepodržanom kompresijom
	var bufBad bytes.Buffer
	zwBad := zip.NewWriter(&bufBad)
	wBad, err := zwBad.Create("nepodrzan.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wBad.Write([]byte("podaci")); err != nil {
		t.Fatal(err)
	}
	if err := zwBad.Close(); err != nil {
		t.Fatal(err)
	}

	zipSirovo := bufBad.Bytes()
	// Zamijeni metodu kompresije u centralnom direktoriju (PK\x01\x02) na nepoznatu metodu 999
	for i := 0; i < len(zipSirovo)-4; i++ {
		if zipSirovo[i] == 0x50 && zipSirovo[i+1] == 0x4b && zipSirovo[i+2] == 0x01 && zipSirovo[i+3] == 0x02 {
			zipSirovo[i+10] = 0x99
			zipSirovo[i+11] = 0x09
		}
	}

	rBad, err := zip.NewReader(bytes.NewReader(zipSirovo), int64(len(zipSirovo)))
	if err != nil {
		t.Fatal(err)
	}
	if err := raspakirajJedanUnos(rBad.File[0], t.TempDir()); err == nil {
		t.Errorf("očekivana greška za nepodržanu zip kompresiju")
	}
}

func TestSkenirajInstalacije(t *testing.T) {
	if !jeInstalacijskaDatoteka("program.dmg") || !jeInstalacijskaDatoteka("alat.pkg") ||
		!jeInstalacijskaDatoteka("sustav.iso") || !jeInstalacijskaDatoteka("xcode.xip") {
		t.Errorf("očekivano prepoznavanje instalacijskih formata")
	}
	if jeInstalacijskaDatoteka("dokument.pdf") || jeInstalacijskaDatoteka("slika.png") {
		t.Errorf("neočekivano prepoznavanje neinstalacijskih formata")
	}

	opisDanas, recDanas := opisInstalacije("app.dmg", 0)
	if recDanas || !strings.Contains(opisDanas, "danas") {
		t.Errorf("neočekivan opis za današnju datoteku: %s, %v", opisDanas, recDanas)
	}
	opisJucer, recJucer := opisInstalacije("app.pkg", 2)
	if recJucer || !strings.Contains(opisJucer, "prije 2 dana") {
		t.Errorf("neočekivan opis za datoteku staru 2 dana: %s, %v", opisJucer, recJucer)
	}
	opisStara, recStara := opisInstalacije("app.iso", 14)
	if !recStara || !strings.Contains(opisStara, "Preporučeno za uklanjanje") {
		t.Errorf("neočekivan opis za staru datoteku: %s, %v", opisStara, recStara)
	}

	dom := t.TempDir()
	p := Postavke{
		KorijenKorisnika:            dom,
		NajmanjaVelicinaInstalacija: 100,
	}

	if res := SkenirajInstalacije(p); res != nil {
		t.Errorf("očekivan nil za nepostojeću Downloads mapu")
	}

	mapaDown := filepath.Join(dom, "Downloads")
	stariDmg := filepath.Join(mapaDown, "stari.dmg")
	noviPkg := filepath.Join(mapaDown, "novi.pkg")
	maliIso := filepath.Join(mapaDown, "mali.iso")
	skriveni := filepath.Join(mapaDown, ".skriveni.dmg")
	podmapa := filepath.Join(mapaDown, "Podmapa.dmg")

	napraviDatoteku(t, stariDmg, 500)
	napraviDatoteku(t, noviPkg, 600)
	napraviDatoteku(t, maliIso, 10) // premalo za prag od 100
	napraviDatoteku(t, skriveni, 500)
	if err := os.MkdirAll(podmapa, 0o755); err != nil {
		t.Fatal(err)
	}

	stariDatum := time.Now().Add(-14 * 24 * time.Hour)
	_ = os.Chtimes(stariDmg, stariDatum, stariDatum) // postavljanje starog datuma

	rez := SkenirajInstalacije(p)
	if len(rez) != 2 {
		t.Fatalf("očekivane 2 instalacijske stavke, dobiveno %d", len(rez))
	}

	var pronasaoStari, pronasaoNovi bool
	for _, st := range rez {
		if st.Putanja == stariDmg {
			pronasaoStari = true
			if !st.Preporuceno {
				t.Errorf("stari .dmg trebao bi biti označen kao preporučen")
			}
		}
		if st.Putanja == noviPkg {
			pronasaoNovi = true
			if st.Preporuceno {
				t.Errorf("novi .pkg NE bi trebao biti označen kao preporučen")
			}
		}
	}
	if !pronasaoStari || !pronasaoNovi {
		t.Errorf("nedostaje očekivana stavka među rezultatima: %+v", rez)
	}

	// 4. Test obradiUlazInstalacije kada je datoteka uklonjena prije dohvata Info
	brisiDmg := filepath.Join(mapaDown, "brisi.dmg")
	napraviDatoteku(t, brisiDmg, 100)
	ulazi, err := os.ReadDir(mapaDown)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(brisiDmg) // simulacija uklanjanja datoteke u letu
	for _, u := range ulazi {
		if u.Name() == "brisi.dmg" {
			if _, ok := obradiUlazInstalacije(mapaDown, u); ok {
				t.Errorf("očekivano false za uklonjenu datoteku")
			}
		}
	}
}
