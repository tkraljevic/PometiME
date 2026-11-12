package cistac

import (
	"fmt"
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
	if izvjDat.BrojStavki != 1 || izvjDat.VelicinaBajta != zauzeto(t, datPutanja) {
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

// Ispravci iz pregleda koda: stablo mora pokazati najveće stavke i stvarne veličine.
func TestStabloNajveceStavkeIStvarneVelicine(t *testing.T) {
	dom := t.TempDir()
	p := ZadanePostavke(dom)
	mapa := filepath.Join(dom, "Library", "Caches", "app")
	sve := []string{}
	for i := 0; i < 49; i++ {
		sve = append(sve, filepath.Join(mapa, fmt.Sprintf("a%02d", i)))
		napraviDatoteku(t, sve[i], 10)
	}
	velika := filepath.Join(mapa, "zzz_velika")
	duboka := filepath.Join(mapa, "pod", "jos", "dublje", "f")
	napraviDatoteku(t, velika, 50000)
	napraviDatoteku(t, duboka, 30000)

	izvj, err := PregledajStablo(p, mapa)
	if err != nil {
		t.Fatal(err)
	}
	if ocekivano := zauzeto(t, append(sve, velika, duboka)...); izvj.VelicinaBajta != ocekivano {
		t.Errorf("ukupna veličina mora biti stvarna (%d), dobiveno %d", ocekivano, izvj.VelicinaBajta)
	}
	if len(izvj.Stavke) != najviseCvorovaStabla || izvj.Stavke[0].Naziv != "zzz_velika" {
		t.Errorf("najveća stavka mora biti prva od %d prikazanih, dobiveno %d stavki, prva %s", najviseCvorovaStabla, len(izvj.Stavke), izvj.Stavke[0].Naziv)
	}
	if d := zauzeto(t, duboka); izvj.Stavke[1].Naziv != "pod" || izvj.Stavke[1].VelicinaBajta != d || izvj.Stavke[1].Djeca[0].VelicinaBajta != d {
		t.Errorf("mapa i njezina podmapa na zadnjoj razini moraju imati stvarnu veličinu: %+v", izvj.Stavke[1])
	}
	if jePutanjaDozvoljenaZaPregled(p, dom+"-drugi/tajno") || jePutanjaDozvoljenaZaPregled(p, "/ApplicationsDrugo") {
		t.Errorf("susjedna mapa s istim početkom imena ne smije biti dopuštena")
	}
	if !jePutanjaDozvoljenaZaPregled(p, "/Applications") || !jePutanjaDozvoljenaZaPregled(p, dom) {
		t.Errorf("sami korijeni moraju biti dopušteni")
	}
}
