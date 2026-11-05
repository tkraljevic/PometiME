package web

import (
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
}
