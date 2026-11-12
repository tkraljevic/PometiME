package cistac

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// pripremiStavkuUKosu stvara mapu u ~/Library/Caches i premješta je u Koš.
func pripremiStavkuUKosu(t *testing.T, p Postavke, ime string, bajta int) string {
	t.Helper()
	izvor := filepath.Join(p.KorijenKorisnika, "Library", "Caches", ime)
	if err := os.MkdirAll(izvor, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(izvor, "podaci.bin"), make([]byte, bajta), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := PremjestiUKos(p, izvor, int64(bajta)); err != nil {
		t.Fatal(err)
	}
	for _, st := range IzlistajKos(p) {
		if st.IzvornaPutanja == izvor {
			return st.PutanjaUKosu
		}
	}
	t.Fatalf("stavka %s nije zapisana u povijest Koša", ime)
	return ""
}

func TestTrajnoIzbrisiIzKosa(t *testing.T) {
	p := ZadanePostavke(t.TempDir())
	prva := pripremiStavkuUKosu(t, p, "Prva", 1000)
	druga := pripremiStavkuUKosu(t, p, "Druga", 500)

	// Mapa samo za čitanje, kakvu ostavlja Go modcache.
	samoCitanje := filepath.Join(prva, "modcache", "paket")
	if err := os.MkdirAll(samoCitanje, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(samoCitanje, "a.go"), []byte("package a"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(samoCitanje, 0o555); err != nil {
		t.Fatal(err)
	}

	// Datoteka koju je korisnik sam stavio u Koš ne smije se dirati.
	tudja := filepath.Join(mapaKosa(p), "korisnikova.txt")
	if err := os.WriteFile(tudja, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	rez := TrajnoIzbrisiIzKosa(p, []string{prva, tudja})
	if rez.ObrisanoStavki != 1 || rez.OslobodenoBajta != 1000 || len(rez.Greske) != 0 {
		t.Fatalf("neočekivan rezultat trajnog brisanja: %+v", rez)
	}
	if !strings.Contains(rez.Poruka, "Trajno izbrisano 1 stavka") {
		t.Errorf("neočekivana poruka: %s", rez.Poruka)
	}
	if _, err := os.Lstat(prva); !os.IsNotExist(err) {
		t.Errorf("stavka s mapom samo za čitanje mora biti izbrisana: %v", err)
	}
	if _, err := os.Stat(tudja); err != nil {
		t.Errorf("datoteka koju PometiME nije premjestio ne smije se izbrisati: %v", err)
	}
	ostale := IzlistajKos(p)
	if len(ostale) != 1 || ostale[0].PutanjaUKosu != druga {
		t.Errorf("u povijesti mora ostati samo druga stavka, dobiveno %+v", ostale)
	}
	if prazno := TrajnoIzbrisiIzKosa(p, nil); prazno.Poruka != "Nijedna stavka nije trajno izbrisana." {
		t.Errorf("neočekivana poruka bez odabira: %s", prazno.Poruka)
	}
}

func TestTrajnoBrisanjePoveznicaBrišeSamoPoveznicu(t *testing.T) {
	p := ZadanePostavke(t.TempDir())
	odrediste := filepath.Join(p.KorijenKorisnika, "Documents", "vazno.txt")
	if err := os.MkdirAll(filepath.Dir(odrediste), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(odrediste, []byte("vazno"), 0o600); err != nil {
		t.Fatal(err)
	}
	poveznica := filepath.Join(p.KorijenKorisnika, "Library", "Caches", "poveznica")
	if err := os.MkdirAll(filepath.Dir(poveznica), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(odrediste, poveznica); err != nil {
		t.Fatal(err)
	}
	if err := PremjestiUKos(p, poveznica, 0); err != nil {
		t.Fatal(err)
	}
	rez := TrajnoIzbrisiIzKosa(p, []string{IzlistajKos(p)[0].PutanjaUKosu})
	if rez.ObrisanoStavki != 1 {
		t.Fatalf("poveznica mora biti izbrisana: %+v", rez)
	}
	if _, err := os.Stat(odrediste); err != nil {
		t.Errorf("odredište poveznice ne smije se izbrisati: %v", err)
	}
}

func TestTrajnoBrisanjeOdbijaIPrijavljujeGreske(t *testing.T) {
	p := ZadanePostavke(t.TempDir())
	ispravna := pripremiStavkuUKosu(t, p, "Ispravna", 10)

	// Podmetnuti zapis u povijesti pokazuje izvan Koša.
	izvan := filepath.Join(p.KorijenKorisnika, "Documents")
	if err := os.MkdirAll(izvan, 0o755); err != nil {
		t.Fatal(err)
	}
	povijest := append(IzlistajKos(p), StavkaKosa{Naziv: "Documents", PutanjaUKosu: izvan, VrijemeBrisanja: "nečitljivo"})
	podaci, _ := json.Marshal(povijest) // poznata struktura bez greške serijalizacije
	if err := os.WriteFile(putanjaPovijesti(p), podaci, 0o600); err != nil {
		t.Fatal(err)
	}
	rez := TrajnoIzbrisiIzKosa(p, []string{izvan})
	if rez.ObrisanoStavki != 0 || len(rez.Greske) != 1 || !strings.Contains(rez.Greske[0], "nije u Košu") {
		t.Fatalf("zapis izvan Koša mora biti odbijen: %+v", rez)
	}
	if _, err := os.Stat(izvan); err != nil {
		t.Errorf("mapa izvan Koša ne smije se izbrisati: %v", err)
	}

	// Koš samo za čitanje: brisanje ne uspijeva, a stavka ostaje u povijesti.
	if err := os.Chmod(mapaKosa(p), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(mapaKosa(p), 0o700) // TempDir mora se moći očistiti
	})
	rez = TrajnoIzbrisiIzKosa(p, []string{ispravna})
	if rez.ObrisanoStavki != 0 || len(rez.Greske) != 1 || !strings.Contains(rez.Greske[0], "trajno brisanje") {
		t.Fatalf("očekivana greška brisanja iz Koša samo za čitanje: %+v", rez)
	}
	if err := os.Chmod(mapaKosa(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if len(IzlistajKos(p)) != 2 {
		t.Errorf("neizbrisane stavke moraju ostati u povijesti: %+v", IzlistajKos(p))
	}

	if dopustiPisanjeMapi("x", nil, fmt.Errorf("nečitljivo")) != nil {
		t.Errorf("greška obilaska ne smije prekinuti brisanje")
	}
}

func TestDanaUKosu(t *testing.T) {
	sada := time.Date(2026, 11, 12, 10, 0, 0, 0, time.Local)
	slucajevi := map[string]int{
		"02.11.2026. 09:00:00": 10,
		"12.11.2026. 09:00:00": 0,
		"13.11.2026. 09:00:00": 0,
		"nečitljivo":           0,
	}
	for zapis, ocekivano := range slucajevi {
		if dobiveno := danaOd(zapis, sada); dobiveno != ocekivano {
			t.Errorf("danaOd(%q) = %d, očekivano %d", zapis, dobiveno, ocekivano)
		}
	}
	p := ZadanePostavke(t.TempDir())
	pripremiStavkuUKosu(t, p, "Nova", 10)
	if d := IzlistajKos(p)[0].DanaUKosu; d != 0 {
		t.Errorf("upravo premještena stavka mora imati 0 dana, dobiveno %d", d)
	}
}

// Istodobna čišćenja ne smiju izgubiti zapise u povijesti Koša.
func TestPovijestKosaIstodobniUpisi(t *testing.T) {
	p := ZadanePostavke(t.TempDir())
	const broj = 20
	var wg sync.WaitGroup
	for i := 0; i < broj; i++ {
		izvor := filepath.Join(p.KorijenKorisnika, "Library", "Caches", fmt.Sprintf("app%02d", i))
		if err := os.MkdirAll(izvor, 0o755); err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := PremjestiUKos(p, izvor, 1); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := len(IzlistajKos(p)); n != broj {
		t.Errorf("povijest Koša mora imati %d zapisa, ima %d", broj, n)
	}

	// Kad se povijest ne može zapisati, premještanje ipak uspijeva.
	if err := os.Chmod(mapaKosa(p), 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chmod(mapaKosa(p), 0o700) // TempDir mora se moći očistiti
	}()
	spremiPovijestKosa(p, nil)
	if n := len(IzlistajKos(p)); n != broj {
		t.Errorf("neuspjeli upis ne smije pokvariti postojeću povijest, ima %d zapisa", n)
	}
}
