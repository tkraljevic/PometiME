package cistac

import (
	"fmt"
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
	napraviDatoteku(t, filepath.Join(dom, "Library", "Caches", "PremalaApp", "mali.db"), 0) // prazna datoteka ne zauzima nijedan blok
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
	if _, err := IzvrsiOptimizaciju(p, "simulators"); err != nil {
		t.Errorf("greška pri izvršavanju simulators optimizacije: %v", err)
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

	// Test greške pri izvršavanju dns, snapshots i simulators optimizacije
	pokreniNaredbu = func(name string, args ...string) error {
		return fmt.Errorf("simulirana greška")
	}
	if _, err := IzvrsiOptimizaciju(p, "dns"); err == nil {
		t.Errorf("očekivana greška pri simuliranom padu naredbe")
	}
	if _, err := IzvrsiOptimizaciju(p, "snapshots"); err == nil {
		t.Errorf("očekivana greška pri simuliranom padu snapshots naredbe")
	}
	if _, err := IzvrsiOptimizaciju(p, "simulators"); err == nil {
		t.Errorf("očekivana greška pri simuliranom padu simulators naredbe")
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
	if ocekivano := zauzeto(t, video1, video2, userCache); infoPuno.UkupnoBajta != ocekivano {
		t.Errorf("očekivano %d bajta (dva videa i korisnička predmemorija), dobiveno %d", ocekivano, infoPuno.UkupnoBajta)
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
	napraviDatoteku(t, maliIso, 0) // prazna datoteka ne zauzima nijedan blok, pa je ispod praga od 100
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

func TestSimulatoriIAvd(t *testing.T) {
	dom := t.TempDir()
	p := Postavke{
		KorijenKorisnika:          dom,
		NajmanjaVelicinaRazvojnih: 10,
	}

	// 1. Nepostojeće mape
	if k := prikupiSimulatorKandidate(filepath.Join(dom, "nepostoji")); k != nil {
		t.Errorf("očekivan nil za nepostojeću mapu simulatora")
	}
	if k := prikupiAvdKandidate(filepath.Join(dom, "nepostoji")); k != nil {
		t.Errorf("očekivan nil za nepostojeću mapu avd-a")
	}

	// 2. iOS simulatori
	simBaza := filepath.Join(dom, "Library", "Developer", "CoreSimulator", "Devices")
	sim1 := filepath.Join(simBaza, "UUID-1111")
	plist1 := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>name</key><string>iPad Air 13-inch (M3)</string>
<key>runtime</key><string>com.apple.CoreSimulator.SimRuntime.iOS-18-4</string>
</dict></plist>`
	napraviDatoteku(t, filepath.Join(sim1, "device.plist"), len(plist1))
	if err := os.WriteFile(filepath.Join(sim1, "device.plist"), []byte(plist1), 0o600); err != nil {
		t.Fatal(err)
	}
	napraviDatoteku(t, filepath.Join(sim1, "data", "app.bin"), 200)

	sim2 := filepath.Join(simBaza, "UUID-2222")
	plist2 := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>name</key><string>iPhone 16</string>
</dict></plist>`
	napraviDatoteku(t, filepath.Join(sim2, "device.plist"), len(plist2))
	if err := os.WriteFile(filepath.Join(sim2, "device.plist"), []byte(plist2), 0o600); err != nil {
		t.Fatal(err)
	}
	napraviDatoteku(t, filepath.Join(sim2, "data.bin"), 300)

	sim3 := filepath.Join(simBaza, "UUID-3333")
	napraviDatoteku(t, filepath.Join(sim3, "data.bin"), 400)

	napraviDatoteku(t, filepath.Join(simBaza, "device_set.plist"), 100)

	simKandidati := prikupiSimulatorKandidate(simBaza)
	if len(simKandidati) != 3 {
		t.Fatalf("očekivana 3 kandidata simulatora, dobiveno %d", len(simKandidati))
	}

	// 3. Android AVD-ovi
	avdBaza := filepath.Join(dom, ".android", "avd")
	avd1 := filepath.Join(avdBaza, "Pixel_7_API_34.avd")
	napraviDatoteku(t, filepath.Join(avd1, "userdata.img"), 500)
	napraviDatoteku(t, filepath.Join(avdBaza, "nije-avd", "test.txt"), 100)
	napraviDatoteku(t, filepath.Join(avdBaza, "Pixel_7_API_34.ini"), 50)

	avdKandidati := prikupiAvdKandidate(avdBaza)
	if len(avdKandidati) != 1 {
		t.Fatalf("očekivan 1 avd kandidat, dobiveno %d", len(avdKandidati))
	}
	if avdKandidati[0].naziv != "Android Emulator (AVD): Pixel_7_API_34" {
		t.Errorf("neočekivan naziv avd kandidata: %s", avdKandidati[0].naziv)
	}

	// 4. Provjera sigurnosti putanja
	if !JeSigurnaPutanja(p, sim1) {
		t.Errorf("putanja simulatora mora biti sigurna: %s", sim1)
	}
	if !JeSigurnaPutanja(p, avd1) {
		t.Errorf("putanja android avd mora biti sigurna: %s", avd1)
	}

	// 5. Test urediSimulatorRuntime
	if urediSimulatorRuntime("com.apple.CoreSimulator.SimRuntime.watchOS-10-0") != "watchOS 10.0" {
		t.Errorf("neočekivano formatiranje watchOS runtimea")
	}
	if urediSimulatorRuntime("tvOS") != "tvOS" {
		t.Errorf("neočekivano formatiranje tvOS bez verzije")
	}

	// 6. Integracija u SkenirajRazvojneAlate
	stavke := SkenirajRazvojneAlate(p)
	if len(stavke) != 4 {
		t.Fatalf("očekivane 4 stavke u SkenirajRazvojneAlate, dobiveno %d", len(stavke))
	}
}
