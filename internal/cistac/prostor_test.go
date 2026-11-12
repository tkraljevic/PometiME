package cistac

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// novoMjerenje briše zapamćeno mjerenje, da testovi ne ovise jedan o drugome.
func novoMjerenje(t *testing.T) {
	t.Helper()
	bravaMjerenja.Lock()
	zadnjeMjerenje = mjerenjeProstora{}
	bravaMjerenja.Unlock()
}

func TestIstraziProstor(t *testing.T) {
	novoMjerenje(t)
	dom := t.TempDir()
	p := ZadanePostavke(dom)
	film := filepath.Join(dom, "Movies", "film.mov")
	duboko := filepath.Join(dom, "Library", "Caches", "app", "duboko", "jos", "x.bin")
	staro := filepath.Join(dom, ".Trash", "staro.zip")
	biljeska := filepath.Join(dom, "biljeska.txt")
	napraviDatoteku(t, film, 50000)
	napraviDatoteku(t, duboko, 30000)
	napraviDatoteku(t, staro, 20000)
	napraviDatoteku(t, biljeska, 1000)
	if err := os.MkdirAll(filepath.Join(dom, "Prazno"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dom, "Movies"), filepath.Join(dom, "Poveznica")); err != nil {
		t.Fatal(err)
	}

	izvj, err := IstraziProstor(p, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if izvj.VelicinaBajta != zauzeto(t, film, duboko, staro, biljeska) || izvj.Nadredena != "" || izvj.Putanja != dom {
		t.Fatalf("neočekivan korijen: %+v", izvj)
	}
	redoslijed := []string{}
	for _, st := range izvj.Stavke {
		redoslijed = append(redoslijed, st.Naziv)
	}
	if got := strings.Join(redoslijed[:4], " "); got != "Movies Library .Trash biljeska.txt" || izvj.Stavke[0].VelicinaBajta != zauzeto(t, film) {
		t.Errorf("stavke moraju biti poredane po veličini, dobiveno %s", strings.Join(redoslijed, " "))
	}
	for _, st := range izvj.Stavke {
		if st.Naziv == "Poveznica" && (st.JeMapa || st.VelicinaBajta != 0) {
			t.Errorf("poveznica se ne smije pratiti: %+v", st)
		}
	}

	// Ista mapa ponovno: odgovor iz zapamćenog mjerenja.
	if ponovno, err := IstraziProstor(p, "", false); err != nil || ponovno.VelicinaBajta != izvj.VelicinaBajta {
		t.Errorf("ponovni upit korijena mora doći iz mjerenja: %+v (%v)", ponovno, err)
	}

	// Podmapa unutar zapamćenog mjerenja: veličine bez novog mjerenja.
	novo := filepath.Join(dom, "Library", "Caches", "app", "novo.bin")
	napraviDatoteku(t, novo, 999)
	caches, err := IstraziProstor(p, filepath.Join(dom, "Library", "Caches"), false)
	if err != nil {
		t.Fatal(err)
	}
	if d := zauzeto(t, duboko); caches.VelicinaBajta != d || caches.Nadredena != filepath.Join(dom, "Library") || caches.Stavke[0].VelicinaBajta != d {
		t.Errorf("podmapa mora doći iz zapamćenog mjerenja: %+v", caches)
	}

	// Osvježavanje mjeri iznova i vidi novu datoteku.
	osvjezeno, err := IstraziProstor(p, filepath.Join(dom, "Library", "Caches"), true)
	if err != nil || osvjezeno.VelicinaBajta != zauzeto(t, duboko, novo) {
		t.Errorf("osvježeno mjerenje mora vidjeti novu datoteku: %+v (%v)", osvjezeno, err)
	}

	// Dublje od zapamćenih razina mjeri se ta mapa.
	dubokaMapa, err := IstraziProstor(p, filepath.Join(dom, "Library", "Caches", "app", "duboko", "jos"), false)
	if err != nil || dubokaMapa.VelicinaBajta != zauzeto(t, duboko) {
		t.Errorf("duboka mapa mora imati točnu veličinu: %+v (%v)", dubokaMapa, err)
	}
}

func TestIstraziProstorOdbija(t *testing.T) {
	novoMjerenje(t)
	dom := t.TempDir()
	p := ZadanePostavke(dom)
	napraviDatoteku(t, filepath.Join(dom, "datoteka.txt"), 1)
	if err := os.Mkdir(dom+"-drugi", 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dom + "-drugi") // mapa izvan TempDir-a
	})
	nedostupna := filepath.Join(dom, "Nedostupno")
	if err := os.Mkdir(nedostupna, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(nedostupna, 0o755) // TempDir mora se moći očistiti
	})
	for _, put := range []string{"/etc", dom + "-drugi", filepath.Join(dom, "datoteka.txt"), filepath.Join(dom, "nema")} {
		if _, err := IstraziProstor(p, put, false); err == nil {
			t.Errorf("očekivano odbijanje za %s", put)
		}
	}
	if _, err := IstraziProstor(p, nedostupna, false); err == nil || !strings.Contains(err.Error(), "nije moguće pročitati") {
		t.Errorf("nedostupna mapa mora vratiti razumljivu grešku, dobiveno %v", err)
	}
	if izvj, err := IstraziProstor(p, "", false); err != nil || izvj.VelicinaBajta != zauzeto(t, filepath.Join(dom, "datoteka.txt")) {
		t.Errorf("nedostupna podmapa se preskače, ostatak se mjeri: %+v (%v)", izvj, err)
	}
	if len(izmjeriMape(filepath.Join(dom, "nema"))) != 0 {
		t.Errorf("nepostojeća mapa nema veličina")
	}
}

func TestSkracivanjeIZastarjeloMjerenje(t *testing.T) {
	novoMjerenje(t)
	dom := t.TempDir()
	najmanje := []string{}
	for i := 0; i < najviseStavkiProstora+5; i++ {
		put := filepath.Join(dom, fmt.Sprintf("f%02d", i))
		napraviDatoteku(t, put, (i+1)*4096) // svaka datoteka zauzima drugi broj blokova
		if i < 5 {
			najmanje = append(najmanje, put)
		}
	}
	p := ZadanePostavke(dom)
	izvj, err := IstraziProstor(p, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(izvj.Stavke) != najviseStavkiProstora || izvj.SkrivenoStavki != 5 || izvj.SkrivenoBajta != zauzeto(t, najmanje...) {
		t.Errorf("neočekivano skraćivanje: %d stavki, skriveno %d (%d B)", len(izvj.Stavke), izvj.SkrivenoStavki, izvj.SkrivenoBajta)
	}

	staro := sadaZaMjerenjeFn
	t.Cleanup(func() { sadaZaMjerenjeFn = staro })
	napraviDatoteku(t, filepath.Join(dom, "nova"), 1<<20)
	sadaZaMjerenjeFn = func() time.Time { return time.Now().Add(trajanjeMjerenja + time.Minute) }
	izvj, err = IstraziProstor(p, "", false)
	if err != nil || izvj.Stavke[0].Naziv != "nova" {
		t.Errorf("zastarjelo mjerenje mora se ponoviti: %+v (%v)", izvj, err)
	}
}

func TestPrikaziUFinderu(t *testing.T) {
	dom := t.TempDir()
	p := ZadanePostavke(dom)
	napraviDatoteku(t, filepath.Join(dom, "Movies", "film.mov"), 1)
	var pozvano []string
	t.Cleanup(PostaviPokretacNaredbiZaTest(func(name string, args ...string) error {
		pozvano = append([]string{name}, args...)
		if strings.HasSuffix(args[len(args)-1], "Movies") {
			return fmt.Errorf("Finder nije dostupan")
		}
		return nil
	}))
	if err := PrikaziUFinderu(p, filepath.Join(dom, "Movies", "film.mov")); err != nil {
		t.Fatal(err)
	}
	if strings.Join(pozvano[:2], " ") != "open -R" {
		t.Errorf("očekivan open -R, dobiveno %v", pozvano)
	}
	for _, put := range []string{"/etc/hosts", filepath.Join(dom, "nema"), filepath.Join(dom, "Movies")} {
		if err := PrikaziUFinderu(p, put); err == nil {
			t.Errorf("očekivana greška za %s", put)
		}
	}
}
