package web

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"pometime/internal/cistac"
)

const testniKljucSesije = "testni-kljuc-sesije"

// testnaZastita dopušta Host koji httptest.NewRequest postavlja sam (example.com).
var testnaZastita = Zastita{Kljuc: testniKljucSesije, Hostovi: []string{"example.com"}}

// prozorSucelja šalje zahtjeve kao sučelje PometiME: s ključem sesije i JSON tijelom.
type prozorSucelja struct{ pos *Posluzitelj }

func (s prozorSucelja) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Header.Set(ZaglavljeKljuca, testniKljucSesije)
	if r.Method == http.MethodPost {
		r.Header.Set("Content-Type", "application/json")
	}
	s.pos.ServeHTTP(w, r)
}

func postaviTestniPosluzitelj(t *testing.T) (http.Handler, string, string) {
	t.Helper()
	dom := t.TempDir()
	apps := t.TempDir()
	p := cistac.Postavke{
		KorijenKorisnika:             dom,
		KorijenAplikacija:            apps,
		NajmanjaVelicinaPredmemorije: 10,
		NajmanjaVelicinaRazvojnih:    10,
		NajmanjaVelicinaOstataka:     10,
	}
	return prozorSucelja{pos: NoviPosluzitelj(p, testnaZastita)}, dom, apps
}

func TestPosluziSuceljeI404(t *testing.T) {
	pos, _, _ := postaviTestniPosluzitelj(t)

	zah := httptest.NewRequest(http.MethodGet, "/", nil)
	odg := httptest.NewRecorder()
	pos.ServeHTTP(odg, zah)

	if odg.Code != http.StatusOK {
		t.Fatalf("očekivan status 200, dobiveno %d", odg.Code)
	}
	if !strings.Contains(odg.Body.String(), "PometiME") {
		t.Errorf("HTML odgovor ne sadrži naziv aplikacije PometiME")
	}

	zah404 := httptest.NewRequest(http.MethodGet, "/nepoznato", nil)
	odg404 := httptest.NewRecorder()
	pos.ServeHTTP(odg404, zah404)
	if odg404.Code != http.StatusNotFound {
		t.Fatalf("očekivan status 404, dobiveno %d", odg404.Code)
	}
}

func TestAPISkenirajIAplikacije(t *testing.T) {
	pos, dom, apps := postaviTestniPosluzitelj(t)

	cacheDat := filepath.Join(dom, "Library", "Caches", "TestApp", "podaci.bin")
	if err := os.MkdirAll(filepath.Dir(cacheDat), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheDat, make([]byte, 200), 0o600); err != nil {
		t.Fatal(err)
	}

	appBin := filepath.Join(apps, "TestApp.app", "Contents", "MacOS", "bin")
	if err := os.MkdirAll(filepath.Dir(appBin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(appBin, make([]byte, 500), 0o600); err != nil {
		t.Fatal(err)
	}

	zahSken := httptest.NewRequest(http.MethodGet, "/api/skeniraj", nil)
	odgSken := httptest.NewRecorder()
	pos.ServeHTTP(odgSken, zahSken)
	if odgSken.Code != http.StatusOK {
		t.Fatalf("očekivan status 200 za /api/skeniraj, dobiveno %d", odgSken.Code)
	}

	var izvj cistac.IzvjestajSkeniranja
	if err := json.Unmarshal(odgSken.Body.Bytes(), &izvj); err != nil {
		t.Fatal(err)
	}
	if len(izvj.PredmemorijaSustava) != 1 {
		t.Errorf("očekivana 1 stavka predmemorije, dobiveno %d", len(izvj.PredmemorijaSustava))
	}

	zahApp := httptest.NewRequest(http.MethodGet, "/api/aplikacije", nil)
	odgApp := httptest.NewRecorder()
	pos.ServeHTTP(odgApp, zahApp)
	if odgApp.Code != http.StatusOK {
		t.Fatalf("očekivan status 200 za /api/aplikacije, dobiveno %d", odgApp.Code)
	}
}

func TestAPIOcistiIDeinstaliraj(t *testing.T) {
	pos, dom, apps := postaviTestniPosluzitelj(t)

	// Neispravan JSON za /api/ocisti
	losOcisti := httptest.NewRequest(http.MethodPost, "/api/ocisti", bytes.NewBufferString("{neispravno"))
	odgLosOcisti := httptest.NewRecorder()
	pos.ServeHTTP(odgLosOcisti, losOcisti)
	if odgLosOcisti.Code != http.StatusBadRequest {
		t.Errorf("očekivan 400 za neispravan JSON u /api/ocisti, dobiveno %d", odgLosOcisti.Code)
	}

	// Ispravan /api/ocisti
	stavka := filepath.Join(dom, "Library", "Caches", "ZaBrisanje")
	if err := os.MkdirAll(stavka, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stavka, "f.bin"), make([]byte, 128), 0o600); err != nil {
		t.Fatal(err)
	}
	tijeloOcisti, _ := json.Marshal(zahtjevCiscenja{Putanje: []string{stavka}}) // poznata struktura bez mogućnosti greške serijalizacije
	dobarOcisti := httptest.NewRequest(http.MethodPost, "/api/ocisti", bytes.NewReader(tijeloOcisti))
	odgDobarOcisti := httptest.NewRecorder()
	pos.ServeHTTP(odgDobarOcisti, dobarOcisti)
	if odgDobarOcisti.Code != http.StatusOK {
		t.Errorf("očekivan 200 za /api/ocisti, dobiveno %d", odgDobarOcisti.Code)
	}

	// Neispravan JSON za /api/deinstaliraj
	losDeinst := httptest.NewRequest(http.MethodPost, "/api/deinstaliraj", bytes.NewBufferString("{neispravno"))
	odgLosDeinst := httptest.NewRecorder()
	pos.ServeHTTP(odgLosDeinst, losDeinst)
	if odgLosDeinst.Code != http.StatusBadRequest {
		t.Errorf("očekivan 400 za neispravan JSON u /api/deinstaliraj, dobiveno %d", odgLosDeinst.Code)
	}

	// Ispravan /api/deinstaliraj
	appPut := filepath.Join(apps, "Moja.app")
	if err := os.MkdirAll(appPut, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appPut, "bin"), make([]byte, 256), 0o600); err != nil {
		t.Fatal(err)
	}
	tijeloDeinst, _ := json.Marshal(zahtjevDeinstalacije{PutanjaAplikacije: appPut}) // poznata struktura bez mogućnosti greške serijalizacije
	dobarDeinst := httptest.NewRequest(http.MethodPost, "/api/deinstaliraj", bytes.NewReader(tijeloDeinst))
	odgDobarDeinst := httptest.NewRecorder()
	pos.ServeHTTP(odgDobarDeinst, dobarDeinst)
	if odgDobarDeinst.Code != http.StatusOK {
		t.Errorf("očekivan 200 za /api/deinstaliraj, dobiveno %d", odgDobarDeinst.Code)
	}

	// Provjera /api/kos i /api/vrati
	zahKos := httptest.NewRequest(http.MethodGet, "/api/kos", nil)
	odgKos := httptest.NewRecorder()
	pos.ServeHTTP(odgKos, zahKos)
	if odgKos.Code != http.StatusOK {
		t.Fatalf("očekivan 200 za /api/kos, dobiveno %d", odgKos.Code)
	}
	var stavkeKosa []cistac.StavkaKosa
	if err := json.Unmarshal(odgKos.Body.Bytes(), &stavkeKosa); err != nil || len(stavkeKosa) == 0 {
		t.Fatalf("očekivane stavke u Košu, dobiveno %v (%v)", len(stavkeKosa), err)
	}

	losVrati := httptest.NewRequest(http.MethodPost, "/api/vrati", bytes.NewBufferString("{neispravno"))
	odgLosVrati := httptest.NewRecorder()
	pos.ServeHTTP(odgLosVrati, losVrati)
	if odgLosVrati.Code != http.StatusBadRequest {
		t.Errorf("očekivan 400 za neispravan JSON u /api/vrati, dobiveno %d", odgLosVrati.Code)
	}

	tijeloVrati, _ := json.Marshal(zahtjevCiscenja{Putanje: []string{stavkeKosa[0].PutanjaUKosu}}) // poznata struktura bez mogućnosti greške serijalizacije
	dobarVrati := httptest.NewRequest(http.MethodPost, "/api/vrati", bytes.NewReader(tijeloVrati))
	odgDobarVrati := httptest.NewRecorder()
	pos.ServeHTTP(odgDobarVrati, dobarVrati)
	if odgDobarVrati.Code != http.StatusOK {
		t.Errorf("očekivan 200 za /api/vrati, dobiveno %d", odgDobarVrati.Code)
	}

	// Provjera /api/pokretaci i /api/optimiziraj
	zahPok := httptest.NewRequest(http.MethodGet, "/api/pokretaci", nil)
	odgPok := httptest.NewRecorder()
	pos.ServeHTTP(odgPok, zahPok)
	if odgPok.Code != http.StatusOK {
		t.Errorf("očekivan 200 za /api/pokretaci, dobiveno %d", odgPok.Code)
	}

	losOpt := httptest.NewRequest(http.MethodPost, "/api/optimiziraj", bytes.NewBufferString("{neispravno"))
	odgLosOpt := httptest.NewRecorder()
	pos.ServeHTTP(odgLosOpt, losOpt)
	if odgLosOpt.Code != http.StatusBadRequest {
		t.Errorf("očekivan 400 za neispravan JSON u /api/optimiziraj, dobiveno %d", odgLosOpt.Code)
	}

	nepoznataOpt, _ := json.Marshal(zahtjevOptimizacije{Akcija: "nepoznato"}) // poznata struktura bez mogućnosti greške serijalizacije
	odgNepoznataOpt := httptest.NewRecorder()
	pos.ServeHTTP(odgNepoznataOpt, httptest.NewRequest(http.MethodPost, "/api/optimiziraj", bytes.NewReader(nepoznataOpt)))
	if odgNepoznataOpt.Code != http.StatusBadRequest {
		t.Errorf("očekivan 400 za nepoznatu optimizaciju, dobiveno %d", odgNepoznataOpt.Code)
	}

	vratiNaredbu := cistac.PostaviPokretacNaredbiZaTest(func(name string, args ...string) error {
		return nil
	})
	defer vratiNaredbu()

	dnsOpt, _ := json.Marshal(zahtjevOptimizacije{Akcija: "dns"}) // poznata struktura bez mogućnosti greške serijalizacije
	odgDnsOpt := httptest.NewRecorder()
	pos.ServeHTTP(odgDnsOpt, httptest.NewRequest(http.MethodPost, "/api/optimiziraj", bytes.NewReader(dnsOpt)))
	if odgDnsOpt.Code != http.StatusOK {
		t.Errorf("očekivan 200 za optimizaciju dns, dobiveno %d", odgDnsOpt.Code)
	}

	// Provjera /api/snimke
	zahSnimke := httptest.NewRequest(http.MethodGet, "/api/snimke", nil)
	odgSnimke := httptest.NewRecorder()
	pos.ServeHTTP(odgSnimke, zahSnimke)
	if odgSnimke.Code != http.StatusOK {
		t.Errorf("očekivan 200 za /api/snimke, dobiveno %d", odgSnimke.Code)
	}
	var popisSnimaka []string
	if err := json.Unmarshal(odgSnimke.Body.Bytes(), &popisSnimaka); err != nil {
		t.Errorf("greška pri dekodiranju /api/snimke: %v", err)
	}

	// Provjera /api/aerials
	zahAer := httptest.NewRequest(http.MethodGet, "/api/aerials", nil)
	odgAer := httptest.NewRecorder()
	pos.ServeHTTP(odgAer, zahAer)
	if odgAer.Code != http.StatusOK {
		t.Errorf("očekivan 200 za /api/aerials, dobiveno %d", odgAer.Code)
	}
	var infoAer cistac.InformacijeZracnihPozadina
	if err := json.Unmarshal(odgAer.Body.Bytes(), &infoAer); err != nil {
		t.Errorf("greška pri dekodiranju /api/aerials: %v", err)
	}
	// Provjera /api/stablo
	odgStabloPrazno := httptest.NewRecorder()
	pos.ServeHTTP(odgStabloPrazno, httptest.NewRequest(http.MethodGet, "/api/stablo", nil))
	if odgStabloPrazno.Code != http.StatusBadRequest {
		t.Errorf("očekivan 400 za prazan /api/stablo, dobiveno %d", odgStabloPrazno.Code)
	}

	odgStabloErr := httptest.NewRecorder()
	pos.ServeHTTP(odgStabloErr, httptest.NewRequest(http.MethodGet, "/api/stablo?putanja=/etc/forbidden", nil))
	if odgStabloErr.Code != http.StatusBadRequest {
		t.Errorf("očekivan 400 za nedozvoljenu putanju /api/stablo, dobiveno %d", odgStabloErr.Code)
	}

	odgStabloOk := httptest.NewRecorder()
	pos.ServeHTTP(odgStabloOk, httptest.NewRequest(http.MethodGet, "/api/stablo?putanja="+dom, nil))
	if odgStabloOk.Code != http.StatusOK {
		t.Errorf("očekivan 200 za valjanu putanju /api/stablo, dobiveno %d", odgStabloOk.Code)
	}
}

func TestZastitaAPIja(t *testing.T) {
	pos := NoviPosluzitelj(cistac.Postavke{KorijenKorisnika: t.TempDir()}, testnaZastita)
	posalji := func(metoda, putanja, host, kljuc, vrsta, tijelo string) *httptest.ResponseRecorder {
		zah := httptest.NewRequest(metoda, putanja, strings.NewReader(tijelo))
		zah.Host = host
		if kljuc != "" {
			zah.Header.Set(ZaglavljeKljuca, kljuc)
		}
		if vrsta != "" {
			zah.Header.Set("Content-Type", vrsta)
		}
		odg := httptest.NewRecorder()
		pos.ServeHTTP(odg, zah)
		return odg
	}
	slucajevi := []struct {
		naziv, metoda, putanja, host, kljuc, vrsta, tijelo string
		status                                             int
	}{
		{"sučelje", "GET", "/", "example.com", "", "", "", http.StatusOK},
		{"sučelje preko tuđeg imena", "GET", "/", "napadac.example:19845", "", "", "", http.StatusForbidden},
		{"API preko tuđeg imena", "GET", "/api/kos", "napadac.example", testniKljucSesije, "", "", http.StatusForbidden},
		{"API bez ključa", "GET", "/api/kos", "example.com", "", "", "", http.StatusForbidden},
		{"API s krivim ključem", "GET", "/api/kos", "example.com", "krivi", "", "", http.StatusForbidden},
		{"API s ključem", "GET", "/api/kos", "EXAMPLE.com", testniKljucSesije, "", "", http.StatusOK},
		{"čišćenje kao običan tekst", "POST", "/api/ocisti", "example.com", testniKljucSesije, "text/plain;charset=UTF-8", `{"paths":[]}`, http.StatusUnsupportedMediaType},
		{"čišćenje bez vrste sadržaja", "POST", "/api/ocisti", "example.com", testniKljucSesije, "", `{"paths":[]}`, http.StatusUnsupportedMediaType},
		{"čišćenje sa stranice bez ključa", "POST", "/api/ocisti", "example.com", "", "application/json", `{"paths":[]}`, http.StatusForbidden},
		{"čišćenje kao JSON", "POST", "/api/ocisti", "example.com", testniKljucSesije, "application/json; charset=utf-8", `{"paths":[]}`, http.StatusOK},
		{"preveliko tijelo", "POST", "/api/ocisti", "example.com", testniKljucSesije, "application/json", `{"paths":["` + strings.Repeat("a", najveceTijeloZahtjeva) + `"]}`, http.StatusBadRequest},
	}
	for _, s := range slucajevi {
		if odg := posalji(s.metoda, s.putanja, s.host, s.kljuc, s.vrsta, s.tijelo); odg.Code != s.status {
			t.Errorf("%s: očekivan status %d, dobiveno %d (%s)", s.naziv, s.status, odg.Code, odg.Body.String())
		}
	}

	bezKljuca := NoviPosluzitelj(cistac.Postavke{KorijenKorisnika: t.TempDir()}, Zastita{Hostovi: []string{"example.com"}})
	zah := httptest.NewRequest(http.MethodGet, "/api/kos", nil)
	odg := httptest.NewRecorder()
	bezKljuca.ServeHTTP(odg, zah)
	if odg.Code != http.StatusForbidden {
		t.Errorf("poslužitelj bez ključa mora odbiti API, dobiveno %d", odg.Code)
	}
}

func TestPolitikaSadrzaja(t *testing.T) {
	odg := httptest.NewRecorder()
	NoviPosluzitelj(cistac.Postavke{}, testnaZastita).ServeHTTP(odg, httptest.NewRequest(http.MethodGet, "/", nil))
	politika := odg.Header().Get("Content-Security-Policy")
	if !strings.Contains(politika, "script-src 'sha256-") || strings.Contains(politika, "script-src 'unsafe-inline'") {
		t.Errorf("sučelje mora dopuštati samo ugrađenu skriptu, dobiveno %q", politika)
	}
	if strings.Count(string(ugradenoSucelje), "<script") != strings.Count(politika, "'sha256-") {
		t.Errorf("svaka skripta u sučelju mora imati sažetak u politici: %q", politika)
	}
	if p := izracunajPolitikuSadrzaja([]byte("<p>bez skripte</p>")); !strings.Contains(p, "script-src 'none'") {
		t.Errorf("stranica bez skripte ne smije dopustiti nijednu skriptu: %q", p)
	}
}

func TestAPINadogradnja(t *testing.T) {
	pos, _, _ := postaviTestniPosluzitelj(t)
	tajni := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, ed25519.SeedSize))
	javni, _ := tajni.Public().(ed25519.PublicKey) // ed25519.PrivateKey uvijek vraća ed25519.PublicKey

	var arhiva bytes.Buffer
	zw := zip.NewWriter(&arhiva)
	f, err := zw.Create("PometiME.app/Contents/Info.plist")
	if err != nil {
		t.Fatal(err)
	}
	plist := `<plist><dict><key>CFBundleIdentifier</key><string>hr.kraljevic.pometime</string>` +
		`<key>CFBundleShortVersionString</key><string>9.9.9</string></dict></plist>`
	if _, err := f.Write([]byte(plist)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	potpis := base64.StdEncoding.EncodeToString(ed25519.Sign(tajni, arhiva.Bytes()))

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/release-ok", "/release-bez-potpisa":
			assets := []map[string]any{{"name": "PometiME-macOS.zip", "size": arhiva.Len(), "browser_download_url": "http://" + r.Host + "/dl/a.zip"}}
			if r.URL.Path == "/release-ok" {
				assets = append(assets, map[string]any{"name": "PometiME-macOS.zip.sig", "browser_download_url": "http://" + r.Host + "/dl/a.zip.sig"})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v9.9.9", "assets": assets}) // kodiranje lažnog odgovora GitHuba
		case "/dl/a.zip":
			_, _ = w.Write(arhiva.Bytes()) // slanje potpisane arhive
		case "/dl/a.zip.sig":
			_, _ = w.Write([]byte(potpis)) // slanje potpisa
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer mockSrv.Close()
	defer cistac.PostaviProvjeruIzdanjaZaTest(mockSrv.URL+"/dl/", javni)()

	// 1. GET /api/nadogradnja - uspješna provjera
	vratiNad := cistac.PostaviNadogradnjuZaTest(mockSrv.URL+"/release-ok", mockSrv.Client())
	defer vratiNad()
	odgNad := httptest.NewRecorder()
	pos.ServeHTTP(odgNad, httptest.NewRequest(http.MethodGet, "/api/nadogradnja", nil))
	if odgNad.Code != http.StatusOK {
		t.Errorf("očekivan status 200 za /api/nadogradnja, dobiveno %d", odgNad.Code)
	}

	// 2. GET /api/nadogradnja - neuspješno povezivanje
	vratiErr := cistac.PostaviNadogradnjuZaTest(mockSrv.URL+"/error", mockSrv.Client())
	odgNadErr := httptest.NewRecorder()
	pos.ServeHTTP(odgNadErr, httptest.NewRequest(http.MethodGet, "/api/nadogradnja", nil))
	if odgNadErr.Code != http.StatusBadGateway {
		t.Errorf("očekivan status 502 za /api/nadogradnja, dobiveno %d", odgNadErr.Code)
	}
	vratiErr()

	privremenaMapa := t.TempDir()
	defer cistac.PostaviPutanjuAplikacijeZaTest(func() string {
		return filepath.Join(privremenaMapa, "PometiME.app")
	})()

	// 3. Izdanje bez potpisa se ne instalira, a adresa iz zahtjeva se ne koristi.
	vratiBez := cistac.PostaviNadogradnjuZaTest(mockSrv.URL+"/release-bez-potpisa", mockSrv.Client())
	tijelo := `{"download_url":"` + mockSrv.URL + `/dl/a.zip"}`
	odgBez := httptest.NewRecorder()
	pos.ServeHTTP(odgBez, httptest.NewRequest(http.MethodPost, "/api/nadogradnja/primijeni", strings.NewReader(tijelo)))
	if odgBez.Code != http.StatusInternalServerError {
		t.Errorf("očekivan status 500 za izdanje bez potpisa, dobiveno %d", odgBez.Code)
	}
	if _, err := os.Stat(filepath.Join(privremenaMapa, "PometiME.app")); err == nil {
		t.Errorf("izdanje bez potpisa ne smije se instalirati")
	}
	vratiBez()

	// 4. Potpisano izdanje se instalira.
	odgOk := httptest.NewRecorder()
	pos.ServeHTTP(odgOk, httptest.NewRequest(http.MethodPost, "/api/nadogradnja/primijeni", strings.NewReader("{}")))
	if odgOk.Code != http.StatusOK {
		t.Errorf("očekivan status 200 za uspješnu primjenu nadogradnje, dobiveno %d (%s)", odgOk.Code, odgOk.Body.String())
	}
}

func TestAPITrajnoBrisanjeIzKosa(t *testing.T) {
	pos, dom, _ := postaviTestniPosluzitelj(t)
	stavka := filepath.Join(dom, "Library", "Caches", "Stara")
	if err := os.MkdirAll(stavka, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stavka, "f.bin"), make([]byte, 64), 0o600); err != nil {
		t.Fatal(err)
	}
	tijelo, _ := json.Marshal(zahtjevCiscenja{Putanje: []string{stavka}}) // poznata struktura bez greške serijalizacije
	pos.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/ocisti", bytes.NewReader(tijelo)))
	stavke := cistac.IzlistajKos(cistac.Postavke{KorijenKorisnika: dom})
	if len(stavke) != 1 {
		t.Fatalf("očekivana jedna stavka u Košu, dobiveno %d", len(stavke))
	}

	los := httptest.NewRecorder()
	pos.ServeHTTP(los, httptest.NewRequest(http.MethodPost, "/api/kos/izbrisi", strings.NewReader("{neispravno")))
	if los.Code != http.StatusBadRequest {
		t.Errorf("očekivan 400 za neispravan JSON, dobiveno %d", los.Code)
	}

	tijelo, _ = json.Marshal(zahtjevCiscenja{Putanje: []string{stavke[0].PutanjaUKosu}}) // poznata struktura bez greške serijalizacije
	odg := httptest.NewRecorder()
	pos.ServeHTTP(odg, httptest.NewRequest(http.MethodPost, "/api/kos/izbrisi", bytes.NewReader(tijelo)))
	var rez cistac.RezultatCiscenja
	if err := json.Unmarshal(odg.Body.Bytes(), &rez); err != nil || odg.Code != http.StatusOK || rez.ObrisanoStavki != 1 {
		t.Fatalf("očekivano trajno brisanje jedne stavke, dobiveno %d %s (%v)", odg.Code, odg.Body.String(), err)
	}
	if _, err := os.Lstat(stavke[0].PutanjaUKosu); !os.IsNotExist(err) {
		t.Errorf("stavka mora biti trajno izbrisana iz Koša: %v", err)
	}
}

func TestAPIProstorIFinder(t *testing.T) {
	pos, dom, _ := postaviTestniPosluzitelj(t)
	if err := os.MkdirAll(filepath.Join(dom, "Movies"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dom, "Movies", "film.mov"), make([]byte, 300), 0o600); err != nil {
		t.Fatal(err)
	}

	odg := httptest.NewRecorder()
	pos.ServeHTTP(odg, httptest.NewRequest(http.MethodGet, "/api/prostor?osvjezi=1", nil))
	var izvj cistac.IzvjestajProstora
	if err := json.Unmarshal(odg.Body.Bytes(), &izvj); err != nil || odg.Code != http.StatusOK || izvj.VelicinaBajta < 300 {
		t.Fatalf("očekivano barem 300 B zauzeća, dobiveno %d %s (%v)", odg.Code, odg.Body.String(), err)
	}
	odgIzvan := httptest.NewRecorder()
	pos.ServeHTTP(odgIzvan, httptest.NewRequest(http.MethodGet, "/api/prostor?putanja=/etc", nil))
	if odgIzvan.Code != http.StatusBadRequest {
		t.Errorf("mapa izvan korisničke mape mora biti odbijena, dobiveno %d", odgIzvan.Code)
	}

	defer cistac.PostaviPokretacNaredbiZaTest(func(string, ...string) error { return nil })()
	slucajevi := []struct {
		tijelo string
		status int
	}{
		{`{"path":"` + filepath.Join(dom, "Movies") + `"}`, http.StatusOK},
		{`{"path":"/etc/hosts"}`, http.StatusBadRequest},
		{`{neispravno`, http.StatusBadRequest},
	}
	for _, s := range slucajevi {
		odgF := httptest.NewRecorder()
		pos.ServeHTTP(odgF, httptest.NewRequest(http.MethodPost, "/api/prikazi-u-finderu", strings.NewReader(s.tijelo)))
		if odgF.Code != s.status {
			t.Errorf("%s: očekivan status %d, dobiveno %d", s.tijelo, s.status, odgF.Code)
		}
	}
}

func TestAPIRazvojniAlati(t *testing.T) {
	pos, dom, _ := postaviTestniPosluzitelj(t)
	brew := filepath.Join(dom, "bin", "brew")
	if err := os.MkdirAll(filepath.Dir(brew), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brew, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var pokrenuto []string
	var brava sync.Mutex // alati se provjeravaju usporedno
	defer cistac.PostaviAlateZaTest(map[string][]string{"brew": {brew}}, func(_ string, argumenti ...string) ([]byte, error) {
		brava.Lock()
		defer brava.Unlock()
		pokrenuto = append(pokrenuto, strings.Join(argumenti, " "))
		return []byte("==> This operation would free approximately 10MB of disk space."), nil
	})()

	odg := httptest.NewRecorder()
	pos.ServeHTTP(odg, httptest.NewRequest(http.MethodGet, "/api/razvojni-alati", nil))
	var alati []cistac.AlatZaCiscenje
	if err := json.Unmarshal(odg.Body.Bytes(), &alati); err != nil || len(alati) != 1 || alati[0].ID != "brew" {
		t.Fatalf("očekivan Homebrew, dobiveno %s (%v)", odg.Body.String(), err)
	}

	slucajevi := []struct {
		tijelo string
		status int
	}{
		{`{"id":"brew"}`, http.StatusOK},
		{`{"id":"rm -rf ~"}`, http.StatusBadRequest},
		{`{neispravno`, http.StatusBadRequest},
	}
	for _, s := range slucajevi {
		o := httptest.NewRecorder()
		pos.ServeHTTP(o, httptest.NewRequest(http.MethodPost, "/api/razvojni-alati/ocisti", strings.NewReader(s.tijelo)))
		if o.Code != s.status {
			t.Errorf("%s: očekivan status %d, dobiveno %d (%s)", s.tijelo, s.status, o.Code, o.Body.String())
		}
	}
	if pokrenuto[len(pokrenuto)-1] != "cleanup --prune=all" {
		t.Errorf("pokrenuta je kriva naredba: %v", pokrenuto)
	}
}

func TestAPIOtvoriPostavkeDozvole(t *testing.T) {
	pos, _, _ := postaviTestniPosluzitelj(t)
	greska := error(nil)
	defer cistac.PostaviPokretacNaredbiZaTest(func(string, ...string) error { return greska })()
	for _, s := range []struct {
		greska error
		status int
	}{{nil, http.StatusOK}, {os.ErrNotExist, http.StatusInternalServerError}} {
		greska = s.greska
		odg := httptest.NewRecorder()
		pos.ServeHTTP(odg, httptest.NewRequest(http.MethodPost, "/api/otvori-postavke-dozvole", strings.NewReader("{}")))
		if odg.Code != s.status {
			t.Errorf("očekivan status %d, dobiveno %d", s.status, odg.Code)
		}
	}
}
