package cistac

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZadanePostavkeIModel(t *testing.T) {
	p := ZadanePostavke("/Users/test/")
	if p.KorijenKorisnika != "/Users/test" {
		t.Fatalf("očekivan čisti korijen /Users/test, dobiveno %s", p.KorijenKorisnika)
	}
	if p.KorijenAplikacija != "/Applications" {
		t.Fatalf("očekivan /Applications, dobiveno %s", p.KorijenAplikacija)
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
