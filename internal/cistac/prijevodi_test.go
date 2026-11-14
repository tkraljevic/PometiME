package cistac

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// naEngleskom prebacuje jezik na engleski do kraja testa.
func naEngleskom(t *testing.T) {
	t.Helper()
	if err := PostaviJezik(JezikEngleski); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = PostaviJezik(JezikHrvatski) // vraća zadani jezik za ostale testove
	})
}

func TestTIPostaviJezik(t *testing.T) {
	if T("Mijenjana je danas.") != "Mijenjana je danas." || Jezik() != JezikHrvatski {
		t.Fatalf("zadani jezik mora biti hrvatski")
	}
	naEngleskom(t)
	if prijevod := T("Mijenjana je danas."); prijevod != "Changed today." {
		t.Errorf("očekivan engleski prijevod, dobiveno %q", prijevod)
	}
	if T("Tekst bez prijevoda") != "Tekst bez prijevoda" {
		t.Errorf("tekst bez prijevoda ostaje izvorni")
	}
	if err := PostaviJezik("fr"); err == nil || !strings.Contains(err.Error(), "unsupported language") {
		t.Errorf("očekivano odbijanje nepodržanog jezika, dobiveno %v", err)
	}
}

func TestUzBrojNaEngleskom(t *testing.T) {
	naEngleskom(t)
	slucajevi := map[int]string{1: "1 item", 3: "3 items", 11: "11 items"}
	for n, ocekivano := range slucajevi {
		if dobiveno := UzBrojHR(n, "stavka", "stavke", "stavki"); dobiveno != ocekivano {
			t.Errorf("UzBrojHR(%d) = %q, očekivano %q", n, dobiveno, ocekivano)
		}
	}
	if dobiveno := UzBrojHR(2, "mapa", "mape", "mapa"); dobiveno != "2 mape" {
		t.Errorf("riječ bez engleskog oblika ostaje hrvatska, dobiveno %q", dobiveno)
	}
	if poruka := PorukaTrajnogBrisanja(2, 2048); poruka != "Permanently deleted 2 items. Freed 2 KB of disk space." {
		t.Errorf("neočekivana engleska poruka: %q", poruka)
	}
}

func TestUcitajISpremiJezik(t *testing.T) {
	dom := t.TempDir()
	for sustav, ocekivano := range map[string]string{"": "hr", "hr-HR": "hr", "HR": "hr", "en-US": "en", "de-DE": "en"} {
		if dobiveno := UcitajJezik(dom, sustav); dobiveno != ocekivano {
			t.Errorf("UcitajJezik(%q) = %q, očekivano %q", sustav, dobiveno, ocekivano)
		}
	}
	t.Cleanup(func() {
		_ = PostaviJezik(JezikHrvatski) // SpremiJezik mijenja i trenutni jezik
	})
	if err := SpremiJezik(dom, JezikEngleski); err != nil {
		t.Fatal(err)
	}
	if Jezik() != JezikEngleski || UcitajJezik(dom, "hr-HR") != JezikEngleski {
		t.Errorf("odabir korisnika mora vrijediti i kad je sustav na hrvatskom")
	}
	if err := SpremiJezik(dom, "fr"); err == nil {
		t.Errorf("nepodržan jezik ne smije se spremiti")
	}
	if err := os.WriteFile(putanjaJezika(dom), []byte("klingonski\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if UcitajJezik(dom, "en-GB") != JezikEngleski {
		t.Errorf("neispravan zapis odabira zamjenjuje jezik sustava")
	}

	blokirano := filepath.Join(t.TempDir(), "datoteka")
	napraviDatoteku(t, blokirano, 1)
	if err := SpremiJezik(blokirano, JezikHrvatski); err == nil {
		t.Errorf("očekivana greška kad se mapa za odabir ne može stvoriti")
	}
	mapaJezika := filepath.Dir(putanjaJezika(dom))
	if err := os.Remove(putanjaJezika(dom)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(putanjaJezika(dom), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SpremiJezik(dom, JezikHrvatski); err == nil || !strings.HasPrefix(mapaJezika, dom) {
		t.Errorf("očekivana greška kad se odabir ne može zapisati")
	}
}

// pozivi T vraća hrvatske tekstove iz poziva T("…") i cistac.T("…") u kodu bez testova.
func poziviT(t *testing.T, mape ...string) map[string]string {
	t.Helper()
	nadjeni := map[string]string{}
	fs := token.NewFileSet()
	for _, mapa := range mape {
		datoteke, err := filepath.Glob(filepath.Join(mapa, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, put := range datoteke {
			if strings.HasSuffix(put, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fs, put, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				poziv, ok := n.(*ast.CallExpr)
				if !ok || len(poziv.Args) != 1 || !jePozivT(poziv.Fun) {
					return true
				}
				if lit, ok := poziv.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					tekst, _ := strconv.Unquote(lit.Value) // literal je ispravan Go niz
					nadjeni[tekst] = fs.Position(lit.Pos()).String()
				}
				return true
			})
		}
	}
	return nadjeni
}

func jePozivT(fun ast.Expr) bool {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name == "T"
	case *ast.SelectorExpr:
		return f.Sel.Name == "T"
	}
	return false
}

func TestSviTekstoviImajuPrijevod(t *testing.T) {
	pozivi := poziviT(t, ".", "../web")
	for tekst, mjesto := range pozivi {
		if _, ok := prijevodiEN[tekst]; !ok {
			t.Errorf("%s: tekst bez engleskog prijevoda: %q", mjesto, tekst)
		}
	}
	for tekst := range prijevodiEN {
		if _, ok := pozivi[tekst]; !ok {
			t.Errorf("prijevod se nigdje ne koristi: %q", tekst)
		}
	}
}

// Prijevod mora imati iste oznake formata istim redom, inače Sprintf ispiše krive vrijednosti.
func TestPrijevodiZadrzavajuOznakeFormata(t *testing.T) {
	oznake := regexp.MustCompile(`%[a-z]`)
	for hr, en := range prijevodiEN {
		if strings.Join(oznake.FindAllString(hr, -1), "") != strings.Join(oznake.FindAllString(en, -1), "") {
			t.Errorf("različite oznake formata: %q i %q", hr, en)
		}
	}
	for jednina, oblici := range pluraliEN {
		if oblici[0] == "" || oblici[1] == "" {
			t.Errorf("nepotpuna množina za %q", jednina)
		}
	}
}
