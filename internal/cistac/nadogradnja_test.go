package cistac

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testniKljuc potpisuje arhive u testovima; pravi tajni ključ nije u repozitoriju.
var testniKljuc = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))

func testniJavniKljuc() ed25519.PublicKey {
	javni, _ := testniKljuc.Public().(ed25519.PublicKey) // ed25519.PrivateKey uvijek vraća ed25519.PublicKey
	return javni
}

func testniInfoPlist(id, verzija string) string {
	return `<?xml version="1.0"?><plist version="1.0"><dict>` +
		`<key>CFBundleIdentifier</key><string>` + id + `</string>` +
		`<key>CFBundleShortVersionString</key><string>` + verzija + `</string></dict></plist>`
}

func zipIzDatoteka(t *testing.T, datoteke map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for ime, sadrzaj := range datoteke {
		w, err := zw.Create(ime)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(sadrzaj)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func potpisiTestno(sadrzaj []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(testniKljuc, sadrzaj)) + "\n")
}

// pokreniLaznoIzdanje poslužuje GitHub API i datoteke jednog izdanja. Datoteka
// sa sadržajem nil vraća 404. Za datoteke iz mape vanjske adresa je zadana:
// puna (http…) ili relativna prema stranici izdanja.
func pokreniLaznoIzdanje(t *testing.T, oznaka string, datoteke map[string][]byte, vanjske map[string]string) {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/release" {
			var assets []githubAsset
			for ime, sadrzaj := range datoteke {
				adresa := srv.URL + "/download/" + ime
				if vanjska, ok := vanjske[ime]; ok && strings.HasPrefix(vanjska, "http") {
					adresa = vanjska
				} else if ok {
					adresa = srv.URL + "/download/" + vanjska
				}
				assets = append(assets, githubAsset{Name: ime, Size: int64(len(sadrzaj)), BrowserDownloadURL: adresa})
			}
			_ = json.NewEncoder(w).Encode(githubRelease{TagName: oznaka, Assets: assets}) // kodiranje lažnog odgovora GitHuba
			return
		}
		sadrzaj := datoteke[strings.TrimPrefix(r.URL.Path, "/download/")]
		if sadrzaj == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(sadrzaj) // slanje lažne datoteke izdanja
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(PostaviNadogradnjuZaTest(srv.URL+"/release", srv.Client()))
	t.Cleanup(PostaviProvjeruIzdanjaZaTest(srv.URL+"/download/", testniJavniKljuc()))
}

func TestProvjeriNadogradnju(t *testing.T) {
	posluzitelj := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
			return
		case "/neispravan-json":
			_, _ = w.Write([]byte("{neispravan")) // namjerno neispravan JSON
			return
		}
		assets := []githubAsset{{Name: "PometiME-macOS.zip", Size: 7000000, BrowserDownloadURL: "https://github.com/tkraljevic/PometiME/releases/download/v1.2.0/PometiME-macOS.zip"}}
		if r.URL.Path == "/potpisano" {
			assets = append(assets, githubAsset{Name: "PometiME-macOS.zip.sig", Size: 89})
		}
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v1.2.0", Name: "PometiME 1.2.0", Assets: assets}) // kodiranje lažnog odgovora
	}))
	defer posluzitelj.Close()
	vrati := PostaviNadogradnjuZaTest(posluzitelj.URL+"/potpisano", posluzitelj.Client())
	defer vrati()

	info, err := ProvjeriNadogradnju()
	if err != nil {
		t.Fatalf("očekivana uspješna provjera, dobiveno: %v", err)
	}
	if !info.ImaNadogradnju || info.NajnovijaInacica != "1.2.0" || info.VelicinaBajta != 7000000 || info.PoveznicaPreuzimanja == "" {
		t.Errorf("neočekivani podaci o nadogradnji: %+v", info)
	}

	_ = PostaviNadogradnjuZaTest(posluzitelj.URL+"/nepotpisano", posluzitelj.Client()) // vraća ga odgođeni vrati()
	info, err = ProvjeriNadogradnju()
	if err != nil || !info.ImaNadogradnju || info.PoveznicaPreuzimanja != "" {
		t.Errorf("izdanje bez potpisa ne smije nuditi automatsku instalaciju: %+v (%v)", info, err)
	}

	for _, adresa := range []string{posluzitelj.URL + "/error", posluzitelj.URL + "/neispravan-json", "http://127.0.0.1:0/nepostojece", "http://\x7f"} {
		_ = PostaviNadogradnjuZaTest(adresa, posluzitelj.Client()) // vraća ga odgođeni vrati()
		if _, err := ProvjeriNadogradnju(); err == nil {
			t.Errorf("očekivana greška za %q", adresa)
		}
	}
}

func TestPrimijeniNadogradnjuInstalira(t *testing.T) {
	arhiva := zipIzDatoteka(t, map[string]string{
		"PometiME.app/Contents/Info.plist":   testniInfoPlist(identifikatorPaketa, "9.0.0"),
		"PometiME.app/Contents/MacOS/Pometi": "bin",
		"../izvan.txt":                       "ne smije izaći iz privremene mape",
		"/apsolutno.txt":                     "ne smije se zapisati",
	})
	pokreniLaznoIzdanje(t, "v9.0.0", map[string][]byte{
		"PometiME-macOS.zip":     arhiva,
		"PometiME-macOS.zip.sig": potpisiTestno(arhiva),
	}, nil)
	cilj := filepath.Join(t.TempDir(), "PometiME.app")
	t.Cleanup(PostaviPutanjuAplikacijeZaTest(func() string { return cilj }))

	for _, pokusaj := range []string{"nova instalacija", "zamjena postojeće"} {
		if err := PrimijeniNadogradnju(); err != nil {
			t.Fatalf("%s: očekivana uspješna nadogradnja, dobiveno: %v", pokusaj, err)
		}
		if _, err := os.Stat(filepath.Join(cilj, "Contents", "Info.plist")); err != nil {
			t.Fatalf("%s: instalirani paket nema Info.plist: %v", pokusaj, err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(cilj), "izvan.txt")); err == nil {
		t.Errorf("unos s ../ ne smije se raspakirati izvan privremene mape")
	}

	t.Cleanup(PostaviPutanjuAplikacijeZaTest(func() string { return "/dev/null/nemoguce/PometiME.app" }))
	if err := PrimijeniNadogradnju(); err == nil {
		t.Errorf("očekivana greška pri instalaciji na nemoguću lokaciju")
	}

	stara := stvoriPrivremenuMapuFn
	t.Cleanup(func() { stvoriPrivremenuMapuFn = stara })
	stvoriPrivremenuMapuFn = func() (string, error) { return "", fmt.Errorf("nema mjesta") }
	if err := PrimijeniNadogradnju(); err == nil || !strings.Contains(err.Error(), "privremene mape") {
		t.Errorf("očekivana greška privremene mape, dobiveno %v", err)
	}
}

func TestPrimijeniNadogradnjuOdbija(t *testing.T) {
	ispravna := zipIzDatoteka(t, map[string]string{"PometiME.app/Contents/Info.plist": testniInfoPlist(identifikatorPaketa, "9.0.0")})
	tudja := zipIzDatoteka(t, map[string]string{"Drugi.app/Contents/Info.plist": testniInfoPlist("com.example.drugi", "9.0.0")})
	starija := zipIzDatoteka(t, map[string]string{"PometiME.app/Contents/Info.plist": testniInfoPlist(identifikatorPaketa, "1.0.0")})
	bezAppa := zipIzDatoteka(t, map[string]string{"obican.txt": "tekst"})
	ostecena := []byte("ovo nije zip")
	const zip, sig = "PometiME-macOS.zip", "PometiME-macOS.zip.sig"

	slucajevi := []struct {
		naziv, oznaka string
		datoteke      map[string][]byte
		vanjske       map[string]string
		ocekivano     string
	}{
		{"nema novije verzije", "v" + TrenutnaInacica, map[string][]byte{zip: ispravna, sig: potpisiTestno(ispravna)}, nil, "nema novije verzije"},
		{"izdanje bez potpisa", "v9.0.0", map[string][]byte{zip: ispravna}, nil, "nema potpisanu arhivu"},
		{"arhiva na tuđoj adresi", "v9.0.0", map[string][]byte{zip: ispravna, sig: potpisiTestno(ispravna)}, map[string]string{zip: "https://napadac.example/a.zip"}, "nije na stranici izdanja"},
		{"adresa s ..", "v9.0.0", map[string][]byte{zip: ispravna, sig: potpisiTestno(ispravna)}, map[string]string{sig: "x/../a.sig"}, "nije na stranici izdanja"},
		{"potpis se ne može preuzeti", "v9.0.0", map[string][]byte{zip: ispravna, sig: nil}, nil, "preuzimanje potpisa"},
		{"arhiva se ne može preuzeti", "v9.0.0", map[string][]byte{zip: nil, sig: potpisiTestno(ispravna)}, nil, "preuzimanje arhive"},
		{"prevelik potpis", "v9.0.0", map[string][]byte{zip: ispravna, sig: bytes.Repeat([]byte("a"), najveciPotpis+1)}, nil, "veća od"},
		{"potpis druge arhive", "v9.0.0", map[string][]byte{zip: ispravna, sig: potpisiTestno(bezAppa)}, nil, "potpis arhive ne odgovara"},
		{"potpis nije base64", "v9.0.0", map[string][]byte{zip: ispravna, sig: []byte("%%%")}, nil, "potpis arhive ne odgovara"},
		{"oštećena arhiva", "v9.0.0", map[string][]byte{zip: ostecena, sig: potpisiTestno(ostecena)}, nil, "raspakiravanje"},
		{"arhiva bez aplikacije", "v9.0.0", map[string][]byte{zip: bezAppa, sig: potpisiTestno(bezAppa)}, nil, "nije pronađen"},
		{"tuđa aplikacija", "v9.0.0", map[string][]byte{zip: tudja, sig: potpisiTestno(tudja)}, nil, "nije PometiME"},
		{"starija potpisana verzija", "v9.0.0", map[string][]byte{zip: starija, sig: potpisiTestno(starija)}, nil, "nije novija"},
	}
	for _, s := range slucajevi {
		t.Run(s.naziv, func(t *testing.T) {
			pokreniLaznoIzdanje(t, s.oznaka, s.datoteke, s.vanjske)
			cilj := filepath.Join(t.TempDir(), "PometiME.app")
			t.Cleanup(PostaviPutanjuAplikacijeZaTest(func() string { return cilj }))
			err := PrimijeniNadogradnju()
			if err == nil || !strings.Contains(err.Error(), s.ocekivano) {
				t.Fatalf("očekivana greška s %q, dobiveno %v", s.ocekivano, err)
			}
			if _, statErr := os.Stat(cilj); statErr == nil {
				t.Errorf("odbijena nadogradnja ne smije ništa instalirati")
			}
		})
	}

	t.Run("GitHub nije dostupan", func(t *testing.T) {
		t.Cleanup(PostaviNadogradnjuZaTest("http://127.0.0.1:0/nema", http.DefaultClient))
		if err := PrimijeniNadogradnju(); err == nil {
			t.Errorf("očekivana greška kad GitHub nije dostupan")
		}
	})
}

func TestPomocneFunkcijeNadogradnje(t *testing.T) {
	if _, err := preuzmi("http://\x7f", 10); err == nil {
		t.Errorf("očekivana greška za neispravnu adresu")
	}
	if _, err := preuzmi("http://127.0.0.1:0/nema", 10); err == nil {
		t.Errorf("očekivana mrežna greška")
	}
	prekinuto := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte("samo dio")) // veza se prekida prije najavljene duljine
	}))
	defer prekinuto.Close()
	if _, err := preuzmi(prekinuto.URL, 2000); err == nil {
		t.Errorf("očekivana greška kad se veza prekine usred preuzimanja")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Errorf("neispravan javni ključ mora zaustaviti program pri pokretanju")
			}
		}()
		ucitajJavniKljuc("nije-kljuc")
	}()
	if len(ucitajJavniKljuc(javniKljucNadogradnjeB64)) != ed25519.PublicKeySize {
		t.Errorf("ugrađeni javni ključ nije ispravne duljine")
	}

	if p := izracunajPutanjuAppa("/Applications/PometiME.app/Contents/MacOS/PometiME"); p != "/Applications/PometiME.app" {
		t.Errorf("očekivano /Applications/PometiME.app, dobiveno %s", p)
	}
	if p := izracunajPutanjuAppa("/usr/local/bin/pometime"); p != "/Applications/PometiME.app" {
		t.Errorf("očekivano /Applications/PometiME.app, dobiveno %s", p)
	}
	if odrediPutanjuAplikacije() == "" {
		t.Errorf("odrediPutanjuAplikacije vratila prazan niz")
	}
	stariExec := executablePutanjaFn
	t.Cleanup(func() { executablePutanjaFn = stariExec })
	executablePutanjaFn = func() (string, error) { return "", fmt.Errorf("greška putanje") }
	if p := odrediPutanjuAplikacije(); p != "/Applications/PometiME.app" {
		t.Errorf("očekivan fallback na /Applications/PometiME.app, dobiveno %s", p)
	}

	mapa := t.TempDir()
	if pronadjiAppPaket(filepath.Join(mapa, "nepostoji")) != "" {
		t.Errorf("očekivan prazan paket za nepostojeću mapu")
	}
	blokirano := filepath.Join(mapa, "blokirano")
	if err := os.WriteFile(blokirano, []byte("datoteka"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := raspakirajZip(zipIzDatoteka(t, map[string]string{"a/b.txt": "x"}), blokirano); err == nil {
		t.Errorf("očekivana greška pri raspakiravanju u datoteku umjesto mape")
	}

	if err := zamijeniAplikaciju(filepath.Join(mapa, "novi.app"), "/dev/null/nemoguce/PometiME.app"); err == nil {
		t.Errorf("očekivana greška pri nemogućoj zamjeni aplikacije")
	}
	postojeci := filepath.Join(mapa, "ZaRollback.app")
	if err := os.MkdirAll(postojeci, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := zamijeniAplikaciju(filepath.Join(mapa, "nepostojeci-novi.app"), postojeci); err == nil {
		t.Errorf("očekivana greška pri zamjeni s nepostojećim novim paketom")
	}
	if _, err := os.Stat(postojeci); err != nil {
		t.Errorf("neuspjela zamjena mora vratiti staru aplikaciju: %v", err)
	}

	// Unos s nepoznatom metodom kompresije ne može se otvoriti.
	sirovo := zipIzDatoteka(t, map[string]string{"nepodrzan.bin": "podaci"})
	for i := 0; i < len(sirovo)-4; i++ {
		if bytes.Equal(sirovo[i:i+4], []byte{0x50, 0x4b, 0x01, 0x02}) {
			sirovo[i+10], sirovo[i+11] = 0x99, 0x09
		}
	}
	r, err := zip.NewReader(bytes.NewReader(sirovo), int64(len(sirovo)))
	if err != nil {
		t.Fatal(err)
	}
	if err := raspakirajJedanUnos(r.File[0], t.TempDir()); err == nil {
		t.Errorf("očekivana greška za nepodržanu zip kompresiju")
	}
}
