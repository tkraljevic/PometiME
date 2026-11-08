package cistac

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPregledajStablo(t *testing.T) {
	tempDir := t.TempDir()
	p := ZadanePostavke(tempDir)

	// 1. Nedozvoljena putanja
	if _, err := PregledajStablo(p, "/etc/shadow"); err == nil {
		t.Errorf("očekivana greška za nedozvoljenu putanju")
	}
	if _, err := PregledajStablo(p, ""); err == nil {
		t.Errorf("očekivana greška za praznu putanju")
	}

	// 2. Putanja koja ne postoji
	if _, err := PregledajStablo(p, filepath.Join(tempDir, "nepostoji")); err == nil {
		t.Errorf("očekivana greška za nepostojeću putanju")
	}

	// 3. Pojedinačna datoteka
	datPutanja := filepath.Join(tempDir, "test.dmg")
	if err := os.WriteFile(datPutanja, []byte("sadrzaj-instalera-123"), 0o644); err != nil {
		t.Fatalf("greška pri pisanju datoteke: %v", err)
	}

	izvjDat, err := PregledajStablo(p, datPutanja)
	if err != nil {
		t.Fatalf("neočekivana greška pri pregledu datoteke: %v", err)
	}
	if izvjDat.BrojStavki != 1 || izvjDat.VelicinaBajta != int64(len("sadrzaj-instalera-123")) {
		t.Errorf("neispravni podaci za datoteku: %+v", izvjDat)
	}

	// 4. Mapa s podmapama i datotekama
	mapaPutanja := filepath.Join(tempDir, "Caches", "MojaAplikacija")
	podmapa := filepath.Join(mapaPutanja, "Podaci")
	if err := os.MkdirAll(podmapa, 0o755); err != nil {
		t.Fatalf("greška pri kreiranju podmape: %v", err)
	}
	_ = os.WriteFile(filepath.Join(podmapa, "file1.dat"), make([]byte, 100), 0o644) // priprema testne datoteke u podmapi
	_ = os.WriteFile(filepath.Join(mapaPutanja, ".skriveno"), make([]byte, 50), 0o644) // testna skrivena datoteka koja se preskače
	_ = os.WriteFile(filepath.Join(mapaPutanja, "file2.dat"), make([]byte, 200), 0o644) // priprema testne datoteke u mapi

	izvjMap, err := PregledajStablo(p, mapaPutanja)
	if err != nil {
		t.Fatalf("neočekivana greška pri pregledu mape: %v", err)
	}
	if izvjMap.BrojStavki == 0 || izvjMap.VelicinaBajta < 300 {
		t.Errorf("neispravni podaci za mapu: %+v", izvjMap)
	}

	// 5. Dozvoljene sistemske putanje
	if !jePutanjaDozvoljenaZaPregled(p, "/Applications/MojProgram.app") {
		t.Errorf("očekivano dozvoljena putanja za /Applications")
	}
	if !jePutanjaDozvoljenaZaPregled(p, "/Library/Application Support/com.apple.idleassetsd/Customer") {
		t.Errorf("očekivano dozvoljena putanja za idleassetsd")
	}

	// 6. Test citajDjecuMape s greškom i limitom
	if djecaErr := citajDjecuMape(filepath.Join(tempDir, "ne-postoji-mapa"), 1, 2); djecaErr != nil {
		t.Errorf("očekivan nil za nepostojeću mapu u citajDjecuMape")
	}

	mnogoMapa := filepath.Join(tempDir, "MnogoDatoteka")
	if err := os.MkdirAll(mnogoMapa, 0o755); err != nil {
		t.Fatalf("greška pri stvaranju mape mnogoMapa: %v", err)
	}
	for i := 0; i < 45; i++ {
		_ = os.WriteFile(filepath.Join(mnogoMapa, filepath.Base(mnogoMapa)+string(rune('a'+i))), []byte("x"), 0o644) // testne datoteke za limit
	}
	djecaMnogo := citajDjecuMape(mnogoMapa, 1, 2)
	if len(djecaMnogo) != 40 {
		t.Errorf("očekivano točno 40 stavki u citajDjecuMape, dobiveno %d", len(djecaMnogo))
	}

	// 7. PregledajStabloAplikacije
	app := InstaliranaAplikacija{
		Naziv:              "MojProgram",
		PutanjaAplikacije:  filepath.Join(tempDir, "MojProgram.app"),
		VelicinaAplikacije: 5000,
		UkupnaVelicina:     8000,
		PovezanePutanje: []PovezanaPutanja{
			{
				Vrsta:         "Podrška",
				Putanja:       filepath.Join(tempDir, "Podrska"),
				VelicinaBajta: 3000,
			},
		},
	}
	izvjApp := PregledajStabloAplikacije(app)
	if izvjApp.BrojStavki != 2 || izvjApp.VelicinaBajta != 8000 {
		t.Errorf("neispravno stablo aplikacije: %+v", izvjApp)
	}
}
