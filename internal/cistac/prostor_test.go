package cistac

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func zapisiDatoteku(t *testing.T, put string, bajta int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(put), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(put, make([]byte, bajta), 0o600); err != nil {
		t.Fatal(err)
	}
}

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
	zapisiDatoteku(t, filepath.Join(dom, "Movies", "film.mov"), 5000)
	zapisiDatoteku(t, filepath.Join(dom, "Library", "Caches", "app", "duboko", "jos", "x.bin"), 3000)
	zapisiDatoteku(t, filepath.Join(dom, ".Trash", "staro.zip"), 2000)
	zapisiDatoteku(t, filepath.Join(dom, "biljeska.txt"), 100)
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
	if izvj.VelicinaBajta != 10100 || izvj.Nadredena != "" || izvj.Putanja != dom {
		t.Fatalf("neočekivan korijen: %+v", izvj)
	}
	redoslijed := []string{}
	for _, st := range izvj.Stavke {
		redoslijed = append(redoslijed, fmt.Sprintf("%s=%d", st.Naziv, st.VelicinaBajta))
	}
	if got := strings.Join(redoslijed[:4], " "); got != "Movies=5000 Library=3000 .Trash=2000 biljeska.txt=100" {
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
	zapisiDatoteku(t, filepath.Join(dom, "Library", "Caches", "app", "novo.bin"), 999)
	caches, err := IstraziProstor(p, filepath.Join(dom, "Library", "Caches"), false)
	if err != nil {
		t.Fatal(err)
	}
	if caches.VelicinaBajta != 3000 || caches.Nadredena != filepath.Join(dom, "Library") || caches.Stavke[0].VelicinaBajta != 3000 {
		t.Errorf("podmapa mora doći iz zapamćenog mjerenja: %+v", caches)
	}

	// Osvježavanje mjeri iznova i vidi novu datoteku.
	osvjezeno, err := IstraziProstor(p, filepath.Join(dom, "Library", "Caches"), true)
	if err != nil || osvjezeno.VelicinaBajta != 3999 {
		t.Errorf("osvježeno mjerenje mora vidjeti novu datoteku: %+v (%v)", osvjezeno, err)
	}

	// Dublje od zapamćenih razina mjeri se ta mapa.
	duboko, err := IstraziProstor(p, filepath.Join(dom, "Library", "Caches", "app", "duboko", "jos"), false)
	if err != nil || duboko.VelicinaBajta != 3000 {
		t.Errorf("duboka mapa mora imati točnu veličinu: %+v (%v)", duboko, err)
	}
}

func TestIstraziProstorOdbija(t *testing.T) {
	novoMjerenje(t)
	dom := t.TempDir()
	p := ZadanePostavke(dom)
	zapisiDatoteku(t, filepath.Join(dom, "datoteka.txt"), 1)
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
	if izvj, err := IstraziProstor(p, "", false); err != nil || izvj.VelicinaBajta != 1 {
		t.Errorf("nedostupna podmapa se preskače, ostatak se mjeri: %+v (%v)", izvj, err)
	}
	if len(izmjeriMape(filepath.Join(dom, "nema"))) != 0 {
		t.Errorf("nepostojeća mapa nema veličina")
	}
}

func TestSkracivanjeIZastarjeloMjerenje(t *testing.T) {
	novoMjerenje(t)
	dom := t.TempDir()
	for i := 0; i < najviseStavkiProstora+5; i++ {
		zapisiDatoteku(t, filepath.Join(dom, fmt.Sprintf("f%02d", i)), i+1)
	}
	p := ZadanePostavke(dom)
	izvj, err := IstraziProstor(p, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(izvj.Stavke) != najviseStavkiProstora || izvj.SkrivenoStavki != 5 || izvj.SkrivenoBajta != 1+2+3+4+5 {
		t.Errorf("neočekivano skraćivanje: %d stavki, skriveno %d (%d B)", len(izvj.Stavke), izvj.SkrivenoStavki, izvj.SkrivenoBajta)
	}

	staro := sadaZaMjerenjeFn
	t.Cleanup(func() { sadaZaMjerenjeFn = staro })
	zapisiDatoteku(t, filepath.Join(dom, "nova"), 1000)
	sadaZaMjerenjeFn = func() time.Time { return time.Now().Add(trajanjeMjerenja + time.Minute) }
	izvj, err = IstraziProstor(p, "", false)
	if err != nil || izvj.Stavke[0].Naziv != "nova" {
		t.Errorf("zastarjelo mjerenje mora se ponoviti: %+v (%v)", izvj, err)
	}
}

func TestPrikaziUFinderu(t *testing.T) {
	dom := t.TempDir()
	p := ZadanePostavke(dom)
	zapisiDatoteku(t, filepath.Join(dom, "Movies", "film.mov"), 1)
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
