package cistac

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ostari postavlja vrijeme izmjene datoteke na zadani broj dana unatrag.
func ostari(t *testing.T, put string, dana int) {
	t.Helper()
	vrijeme := time.Now().Add(-time.Duration(dana) * 24 * time.Hour)
	if err := os.Chtimes(put, vrijeme, vrijeme); err != nil {
		t.Fatal(err)
	}
}

func TestSkenirajPredmemorijuStarostIPokrenuteAplikacije(t *testing.T) {
	dom := t.TempDir()
	p := ZadanePostavke(dom)
	p.NajmanjaVelicinaPredmemorije = 1
	caches := filepath.Join(dom, "Library", "Caches")
	datoteke := map[string]int{
		filepath.Join(caches, "com.stara.app", "a.db"):          40,
		filepath.Join(caches, "com.nova.app", "a.db"):           3,
		filepath.Join(caches, "com.pokrenuta.app", "a.db"):      90,
		filepath.Join(caches, "com.pokrenuta.app.ShipIt", "u"):  90,
		filepath.Join(dom, "Library", "Logs", "Svjez", "a.log"): 0,
	}
	for put, dana := range datoteke {
		napraviDatoteku(t, put, 5000)
		ostari(t, put, dana)
	}
	stari := pokrenuteAplikacijeFn
	t.Cleanup(func() { pokrenuteAplikacijeFn = stari })
	pokrenuteAplikacijeFn = func() map[string]bool { return map[string]bool{"com.pokrenuta.app": true} }

	preporuka := map[string]bool{}
	opisi := map[string]string{}
	for _, st := range SkenirajPredmemoriju(p) {
		preporuka[st.Naziv] = st.Preporuceno
		opisi[st.Naziv] = st.Opis
	}
	ocekivano := map[string]bool{
		"com.stara.app":            true,
		"com.nova.app":             false,
		"com.pokrenuta.app":        false,
		"com.pokrenuta.app.ShipIt": false,
		"Logovi: Svjez":            true,
	}
	for naziv, ocek := range ocekivano {
		if preporuka[naziv] != ocek {
			t.Errorf("%s: preporučeno %v, očekivano %v (%s)", naziv, preporuka[naziv], ocek, opisi[naziv])
		}
	}
	if !strings.Contains(opisi["com.nova.app"], "Mijenjana je prije 3 dana.") {
		t.Errorf("opis mora reći zašto nije preporučeno: %s", opisi["com.nova.app"])
	}
	if !strings.Contains(opisi["com.pokrenuta.app"], "Aplikacija je pokrenuta") {
		t.Errorf("opis mora reći da je aplikacija pokrenuta: %s", opisi["com.pokrenuta.app"])
	}
}

func TestOpisIzmjeneIDani(t *testing.T) {
	if o := opisNedavneIzmjene(0); o != "Mijenjana je danas." {
		t.Errorf("neočekivan opis: %s", o)
	}
	if o := opisNedavneIzmjene(1); o != "Mijenjana je prije 1 dan." {
		t.Errorf("neočekivan opis: %s", o)
	}
	if d := danaOdVremena(time.Time{}); d != -1 {
		t.Errorf("mapa bez datoteka nema vrijeme izmjene, dobiveno %d", d)
	}
	mapa := t.TempDir()
	napraviDatoteku(t, filepath.Join(mapa, "stara"), 10)
	napraviDatoteku(t, filepath.Join(mapa, "pod", "novija"), 10)
	ostari(t, filepath.Join(mapa, "stara"), 50)
	ostari(t, filepath.Join(mapa, "pod", "novija"), 5)
	if _, zadnja := izmjeriMapu(mapa); danaOdVremena(zadnja) != 5 {
		t.Errorf("zadnja izmjena mora biti najnovija datoteka (5 dana), dobiveno %d", danaOdVremena(zadnja))
	}
}

func TestPokrenuteAplikacije(t *testing.T) {
	stara := pokreniIzlazNaredbe
	t.Cleanup(func() { pokreniIzlazNaredbe = stara })
	pokreniIzlazNaredbe = func(string, ...string) ([]byte, error) {
		return []byte(`"Safari" ASN:0x0-0x1: bundleID="com.apple.Safari" bundlepath="/Applications/Safari.app"` + "\n" +
			`"Slack" ASN:0x0-0x2: bundleID="com.tinyspeck.slackmacgap"`), nil
	}
	pokrenute := pokrenuteAplikacije()
	if len(pokrenute) != 2 || !pokrenute["com.apple.safari"] || !pokrenute["com.tinyspeck.slackmacgap"] {
		t.Errorf("neočekivane pokrenute aplikacije: %v", pokrenute)
	}
	pokreniIzlazNaredbe = func(string, ...string) ([]byte, error) { return nil, errors.New("lsappinfo ne postoji") }
	if len(pokrenuteAplikacije()) != 0 {
		t.Errorf("bez lsappinfo nema pokrenutih aplikacija")
	}
}
