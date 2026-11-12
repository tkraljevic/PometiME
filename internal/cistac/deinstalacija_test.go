package cistac

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func napraviAplikaciju(t *testing.T, mapa, ime, paketID string, binarniPlist bool) string {
	t.Helper()
	app := filepath.Join(mapa, ime+".app")
	plist := filepath.Join(app, "Contents", "Info.plist")
	napraviDatoteku(t, filepath.Join(app, "Contents", "MacOS", ime), 4096)
	sadrzaj := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict>` +
		`<key>CFBundleIdentifier</key><string>` + paketID + `</string>` +
		`<key>CFBundleShortVersionString</key><string>2.0</string></dict></plist>`
	if err := os.WriteFile(plist, []byte(sadrzaj), 0o644); err != nil {
		t.Fatal(err)
	}
	if binarniPlist {
		if izlaz, err := exec.Command("/usr/bin/plutil", "-convert", "binary1", plist).CombinedOutput(); err != nil {
			t.Fatalf("plutil: %v %s", err, izlaz)
		}
	}
	return app
}

func TestBinarniInfoPlist(t *testing.T) {
	app := napraviAplikaciju(t, t.TempDir(), "Word", "com.microsoft.Word", true)
	if meta := procitajInfoPlist(app); meta.paketID != "com.microsoft.Word" || meta.inacica != "2.0" {
		t.Errorf("binarni plist mora se pročitati: %+v", meta)
	}
	stari := pretvoriPlistFn
	t.Cleanup(func() { pretvoriPlistFn = stari })
	pretvoriPlistFn = func(string) ([]byte, error) { return nil, errors.New("plutil nije dostupan") }
	if meta := procitajInfoPlist(app); meta.paketID != "" || meta.naziv != "Word" {
		t.Errorf("bez plutila ostaje ime mape: %+v", meta)
	}
}

func TestPovezaniPodaciAplikacije(t *testing.T) {
	dom := t.TempDir()
	apps := t.TempDir()
	p := ZadanePostavke(dom)
	p.KorijenAplikacija = apps
	lib := filepath.Join(dom, "Library")
	appA := napraviAplikaciju(t, apps, "Alfa", "hr.primjer.alfa", false)
	napraviAplikaciju(t, apps, "Beta", "hr.primjer.beta", false)

	napraviDatoteku(t, filepath.Join(lib, "Containers", "hr.primjer.alfa", "Data", "x"), 4096)
	napraviDatoteku(t, filepath.Join(lib, "Group Containers", "TIM.alfa", "x"), 4096)
	napraviDatoteku(t, filepath.Join(lib, "Group Containers", "TIM.zajednicka", "x"), 4096)
	napraviDatoteku(t, filepath.Join(lib, "LaunchAgents", "hr.primjer.alfa.pomocnik.plist"), 10)
	pokretac := `<plist><dict><key>Program</key><string>` + appA + `/Contents/MacOS/azuriranje</string></dict></plist>`
	if err := os.WriteFile(filepath.Join(lib, "LaunchAgents", "azuriranje.plist"), []byte(pokretac), 0o644); err != nil {
		t.Fatal(err)
	}
	napraviDatoteku(t, filepath.Join(lib, "LaunchAgents", "tudji.plist"), 10)
	nedostupan := filepath.Join(lib, "LaunchAgents", "nedostupan.plist")
	napraviDatoteku(t, nedostupan, 10)
	if err := os.Chmod(nedostupan, 0o000); err != nil {
		t.Fatal(err)
	}

	stari := procitajGrupeFn
	t.Cleanup(func() { procitajGrupeFn = stari })
	procitajGrupeFn = func(put string) []string {
		if strings.HasSuffix(put, "Alfa.app") {
			return []string{"TIM.alfa", "TIM.zajednicka", "../izvan"}
		}
		return []string{"TIM.zajednicka"}
	}

	var alfa InstaliranaAplikacija
	for _, a := range SkenirajAplikacije(p) {
		if a.IdentifikatorPaketa == "hr.primjer.alfa" {
			alfa = a
		}
	}
	vrste := map[string]string{}
	for _, r := range alfa.PovezanePutanje {
		vrste[filepath.Base(r.Putanja)] = r.Vrsta
	}
	ocekivano := map[string]string{
		"hr.primjer.alfa":                "Containers",
		"TIM.alfa":                       "Group Containers",
		"hr.primjer.alfa.pomocnik.plist": "LaunchAgents",
		"azuriranje.plist":               "LaunchAgents",
	}
	for ime, vrsta := range ocekivano {
		if vrste[ime] != vrsta {
			t.Errorf("%s: očekivana vrsta %s, dobiveno %q (sve: %v)", ime, vrsta, vrste[ime], vrste)
		}
	}
	for _, ime := range []string{"TIM.zajednicka", "tudji.plist", "nedostupan.plist"} {
		if _, ima := vrste[ime]; ima {
			t.Errorf("%s ne smije biti među podacima aplikacije Alfa", ime)
		}
	}
	if alfa.TrebaAdmin {
		t.Errorf("aplikaciju u vlasništvu korisnika nije potrebno micati uz lozinku")
	}
}

func TestAplikacijaKojaTraziLozinku(t *testing.T) {
	dom := t.TempDir()
	apps := t.TempDir()
	p := ZadanePostavke(dom)
	p.KorijenAplikacija = apps
	app := napraviAplikaciju(t, apps, "Paket", "hr.primjer.paket", false)
	podaci := filepath.Join(dom, "Library", "Application Support", "hr.primjer.paket")
	napraviDatoteku(t, filepath.Join(podaci, "baza.db"), 4096)
	if err := os.Chmod(app, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(app, 0o755) // TempDir mora se moći očistiti
	})
	stari := procitajGrupeFn
	t.Cleanup(func() { procitajGrupeFn = stari })
	procitajGrupeFn = func(string) []string { return nil }

	aplikacije := SkenirajAplikacije(p)
	if len(aplikacije) != 1 || !aplikacije[0].TrebaAdmin {
		t.Fatalf("aplikacija bez prava pisanja mora tražiti lozinku: %+v", aplikacije)
	}
	if rez := UkloniPodatkeAplikacije(p, "/etc/nije.app", []string{podaci}); len(rez.Greske) != 1 {
		t.Errorf("nevaljana aplikacija mora biti odbijena: %+v", rez)
	}
	rez := UkloniPodatkeAplikacije(p, app, []string{podaci})
	if rez.ObrisanoStavki != 1 {
		t.Fatalf("podaci moraju u Koš: %+v", rez)
	}
	if _, err := os.Stat(app); err != nil {
		t.Errorf("paket aplikacije mora ostati na mjestu: %v", err)
	}

	t.Cleanup(PostaviPokretacNaredbiZaTest(func(string, ...string) error { return nil }))
	if err := PrikaziUFinderu(p, app); err != nil {
		t.Errorf("Finder smije pokazati aplikaciju: %v", err)
	}
}

func TestGrupeIzPotpisa(t *testing.T) {
	xml := `<plist><dict><key>com.apple.security.app-sandbox</key><true/>` +
		`<key>com.apple.security.application-groups</key><array><string>TIM.Office</string><string> group.hr.primjer </string></array></dict></plist>`
	if grupe := izvuciGrupe(xml); strings.Join(grupe, ",") != "TIM.Office,group.hr.primjer" {
		t.Errorf("neočekivane grupe: %v", grupe)
	}
	if grupe := izvuciGrupe("<plist><dict/></plist>"); grupe != nil {
		t.Errorf("bez grupa se vraća nil, dobiveno %v", grupe)
	}
	if grupe := procitajGrupeAplikacije(t.TempDir()); grupe != nil {
		t.Errorf("nepotpisana mapa nema grupa, dobiveno %v", grupe)
	}
	if grupe := procitajGrupeAplikacije("/System/Applications/Calculator.app"); grupe != nil {
		t.Logf("Kalkulator ima grupe %v", grupe)
	}
}

func TestAppleoveAplikacije(t *testing.T) {
	apps := t.TempDir()
	napraviAplikaciju(t, apps, "Safari", "com.apple.Safari", false)
	imovie := napraviAplikaciju(t, apps, "iMovie", "com.apple.iMovieApp", true)
	napraviDatoteku(t, filepath.Join(imovie, "Contents", "_MASReceipt", "receipt"), 10)
	p := ZadanePostavke(t.TempDir())
	p.KorijenAplikacija = apps
	stari := procitajGrupeFn
	t.Cleanup(func() { procitajGrupeFn = stari })
	procitajGrupeFn = func(string) []string { return nil }

	imena := []string{}
	for _, a := range SkenirajAplikacije(p) {
		imena = append(imena, a.IdentifikatorPaketa)
	}
	if strings.Join(imena, ",") != "com.apple.iMovieApp" {
		t.Errorf("na popisu smije biti samo Appleova aplikacija iz App Storea, dobiveno %v", imena)
	}
}
