package web

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pometime/internal/cistac"
)

func postaviTestniPosluzitelj(t *testing.T) (*Posluzitelj, string, string) {
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
	return NoviPosluzitelj(p), dom, apps
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

func TestAPINadogradnja(t *testing.T) {
	pos, _, _ := postaviTestniPosluzitelj(t)

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/release-ok" {
			if err := json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "v1.1.0",
				"name":     "PometiME 1.1.0",
				"assets": []map[string]any{
					{
						"name":                 "PometiME-macOS.zip",
						"size":                 100,
						"browser_download_url": "http://" + r.Host + "/zip-ok",
					},
				},
			}); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			return
		}
		if r.URL.Path == "/zip-ok" {
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			f, err := zw.Create("PometiME.app/Contents/Info.plist")
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if _, err := f.Write([]byte("ok")); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_ = zw.Close() // zatvaranje zip arhive u testu
			_, _ = w.Write(buf.Bytes()) // slanje mock zip arhive
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockSrv.Close()

	// 1. GET /api/nadogradnja - uspješna provjera
	vratiNad := cistac.PostaviNadogradnjuZaTest(mockSrv.URL+"/release-ok", mockSrv.Client())
	defer vratiNad()

	zahNad := httptest.NewRequest(http.MethodGet, "/api/nadogradnja", nil)
	odgNad := httptest.NewRecorder()
	pos.ServeHTTP(odgNad, zahNad)
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

	// 3. POST /api/nadogradnja/primijeni - neispravan JSON
	zahNeispravan := httptest.NewRequest(http.MethodPost, "/api/nadogradnja/primijeni", bytes.NewReader([]byte("{neispravan-json")))
	odgNeispravan := httptest.NewRecorder()
	pos.ServeHTTP(odgNeispravan, zahNeispravan)
	if odgNeispravan.Code != http.StatusBadRequest {
		t.Errorf("očekivan status 400 za neispravan JSON, dobiveno %d", odgNeispravan.Code)
	}

	// 4. POST /api/nadogradnja/primijeni - prazna adresa
	zahPrazno, _ := json.Marshal(map[string]string{"download_url": ""}) // jednostavna mapa bez greške
	odgPrazno := httptest.NewRecorder()
	pos.ServeHTTP(odgPrazno, httptest.NewRequest(http.MethodPost, "/api/nadogradnja/primijeni", bytes.NewReader(zahPrazno)))
	if odgPrazno.Code != http.StatusInternalServerError {
		t.Errorf("očekivan status 500 za prazan download_url, dobiveno %d", odgPrazno.Code)
	}

	// 5. POST /api/nadogradnja/primijeni - uspješna nadogradnja
	privremenaMapa := t.TempDir()
	vratiPutanju := cistac.PostaviPutanjuAplikacijeZaTest(func() string {
		return filepath.Join(privremenaMapa, "PometiME.app")
	})
	defer vratiPutanju()

	zahOk, _ := json.Marshal(map[string]string{"download_url": mockSrv.URL + "/zip-ok"}) // json za uspješnu primjenu
	odgOk := httptest.NewRecorder()
	pos.ServeHTTP(odgOk, httptest.NewRequest(http.MethodPost, "/api/nadogradnja/primijeni", bytes.NewReader(zahOk)))
	if odgOk.Code != http.StatusOK {
		t.Errorf("očekivan status 200 za uspješnu primjenu nadogradnje, dobiveno %d (%s)", odgOk.Code, odgOk.Body.String())
	}
}
