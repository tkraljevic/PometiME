package cistac

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// TrenutnaInacica definira ugrađenu verziju programa PometiME.
const TrenutnaInacica = "1.0.0"

var (
	urlGitHubReleases         = "https://api.github.com/repos/tkraljevic/PometiME/releases/latest"
	klijentNadogradnje        = &http.Client{Timeout: 8 * time.Second}
	odrediPutanjuAplikacijeFn = odrediPutanjuAplikacije
	executablePutanjaFn       = os.Executable
	stvoriPrivremenuMapuFn    = func() (string, error) { return os.MkdirTemp("", "pometime-nadogradnja-*") }
)

// PostaviNadogradnjuZaTest privremeno mijenja postavke poslužitelja za potrebe testiranja.
func PostaviNadogradnjuZaTest(url string, klijent *http.Client) func() {
	stariURL := urlGitHubReleases
	stariKlijent := klijentNadogradnje
	urlGitHubReleases = url
	klijentNadogradnje = klijent
	return func() {
		urlGitHubReleases = stariURL
		klijentNadogradnje = stariKlijent
	}
}

// PostaviPutanjuAplikacijeZaTest privremeno mijenja funkciju određivanja putanje aplikacije.
func PostaviPutanjuAplikacijeZaTest(fn func() string) func() {
	stara := odrediPutanjuAplikacijeFn
	odrediPutanjuAplikacijeFn = fn
	return func() {
		odrediPutanjuAplikacijeFn = stara
	}
}

type githubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Name    string        `json:"name"`
	Body    string        `json:"body"`
	HTMLURL string        `json:"html_url"`
	Assets  []githubAsset `json:"assets"`
}

// ProvjeriNadogradnju provjerava dostupnost novog izdanja na GitHub repozitoriju.
func ProvjeriNadogradnju() (*InformacijeONadogradnji, error) {
	zahtjev, err := http.NewRequest(http.MethodGet, urlGitHubReleases, nil)
	if err != nil {
		return nil, fmt.Errorf("priprema zahtjeva za GitHub: %w", err)
	}
	zahtjev.Header.Set("User-Agent", "PometiME-Updater")
	zahtjev.Header.Set("Accept", "application/vnd.github.v3+json")

	odgovor, err := klijentNadogradnje.Do(zahtjev)
	if err != nil {
		return nil, fmt.Errorf("povezivanje s GitHub API-jem: %w", err)
	}
	defer func() {
		_ = odgovor.Body.Close() // zatvaranje odgovora GitHub API-ja
	}()

	if odgovor.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API je vratio status %d", odgovor.StatusCode)
	}

	var izdanje githubRelease
	if err := json.NewDecoder(odgovor.Body).Decode(&izdanje); err != nil {
		return nil, fmt.Errorf("dekodiranje GitHub odgovora: %w", err)
	}

	info := &InformacijeONadogradnji{
		TrenutnaInacica:  TrenutnaInacica,
		NajnovijaInacica: strings.TrimPrefix(izdanje.TagName, "v"),
		NazivIzdanja:     izdanje.Name,
		Biljeske:         izdanje.Body,
		Poveznica:        izdanje.HTMLURL,
	}

	info.PoveznicaPreuzimanja, info.VelicinaBajta = pronadjiZipAsset(izdanje.Assets)
	info.ImaNadogradnju = UsporediInacice(TrenutnaInacica, info.NajnovijaInacica)
	return info, nil
}

func pronadjiZipAsset(assets []githubAsset) (string, int64) {
	for _, a := range assets {
		if strings.HasSuffix(a.Name, ".zip") {
			return a.BrowserDownloadURL, a.Size
		}
	}
	return "", 0
}

// UsporediInacice vraća true ako je najnovija inačica veća od trenutne.
func UsporediInacice(trenutna, najnovija string) bool {
	t := dijeloviInacice(trenutna)
	n := dijeloviInacice(najnovija)
	for i := 0; i < 3; i++ {
		if n[i] != t[i] {
			return n[i] > t[i]
		}
	}
	return false
}

func dijeloviInacice(s string) [3]int {
	cisto := strings.TrimPrefix(strings.TrimSpace(s), "v")
	dijelovi := strings.Split(cisto, ".")
	var brojevi [3]int
	for i := 0; i < len(dijelovi) && i < 3; i++ {
		if br, err := strconv.Atoi(dijelovi[i]); err == nil {
			brojevi[i] = br
		}
	}
	return brojevi
}

// PrimijeniNadogradnju preuzima i instalira najnoviju inačicu u mapu aplikacije.
func PrimijeniNadogradnju(downloadURL string) error {
	if downloadURL == "" {
		return fmt.Errorf("adresa za preuzimanje nije navedena")
	}

	privremenaMapa, err := stvoriPrivremenuMapuFn()
	if err != nil {
		return fmt.Errorf("stvaranje privremene mape: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(privremenaMapa) // čišćenje privremene mape nadogradnje
	}()

	zipPutanja := filepath.Join(privremenaMapa, "pometime-nova.zip")
	if err := preuzmiDatoteku(downloadURL, zipPutanja); err != nil {
		return fmt.Errorf("preuzimanje arhive s GitHuba: %w", err)
	}

	raspakirano := filepath.Join(privremenaMapa, "raspakirano")
	if err := raspakirajZip(zipPutanja, raspakirano); err != nil {
		return fmt.Errorf("raspakiravanje preuzete arhive: %w", err)
	}

	return instalirajAppIzMape(raspakirano)
}

func instalirajAppIzMape(raspakirano string) error {
	noviApp := pronadjiAppPaket(raspakirano)
	if noviApp == "" {
		return fmt.Errorf("u preuzetoj arhivi nije pronađen PometiME.app paket")
	}

	ciljApp := odrediPutanjuAplikacijeFn()
	if err := zamijeniAplikaciju(noviApp, ciljApp); err != nil {
		return fmt.Errorf("zamjena aplikacije na lokaciji %s: %w", ciljApp, err)
	}
	return nil
}

func preuzmiDatoteku(url, cilj string) error {
	odg, err := klijentNadogradnje.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		_ = odg.Body.Close() // zatvaranje odgovora preuzimanja
	}()

	if odg.StatusCode != http.StatusOK {
		return fmt.Errorf("preuzimanje nije uspjelo (status %d)", odg.StatusCode)
	}

	datoteka, err := os.Create(cilj)
	if err != nil {
		return err
	}
	defer func() {
		_ = datoteka.Close() // zatvaranje preuzete datoteke
	}()

	_, err = io.Copy(datoteka, odg.Body)
	return err
}

func raspakirajZip(zipPutanja, odrediste string) error {
	r, err := zip.OpenReader(zipPutanja)
	if err != nil {
		return err
	}
	defer func() {
		_ = r.Close() // zatvaranje zip arhive
	}()

	for _, f := range r.File {
		if err := raspakirajJedanUnos(f, odrediste); err != nil {
			return err
		}
	}
	return nil
}

func raspakirajJedanUnos(f *zip.File, odrediste string) error {
	cistoIme := filepath.Clean(f.Name)
	if strings.HasPrefix(cistoIme, "..") || filepath.IsAbs(cistoIme) {
		return nil
	}
	cilj := filepath.Join(odrediste, cistoIme)
	if f.FileInfo().IsDir() {
		return os.MkdirAll(cilj, 0o755)
	}
	_ = os.MkdirAll(filepath.Dir(cilj), 0o755) // priprema podmapa za datoteku

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() {
		_ = rc.Close() // zatvaranje čitača zip unosa
	}()

	izlaz, err := os.OpenFile(cilj, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer func() {
		_ = izlaz.Close() // zatvaranje raspakirane datoteke
	}()

	_, err = io.Copy(izlaz, rc)
	_ = os.Chmod(cilj, f.Mode()) // očuvanje dozvola izvršavanja datoteke
	return err
}

func pronadjiAppPaket(korijen string) string {
	ulazi, err := os.ReadDir(korijen)
	if err != nil {
		return ""
	}
	for _, u := range ulazi {
		if strings.HasSuffix(u.Name(), ".app") {
			return filepath.Join(korijen, u.Name())
		}
	}
	return ""
}

func zamijeniAplikaciju(noviApp, ciljApp string) error {
	stara := ciljApp + ".stara"
	_ = os.RemoveAll(stara) // uklanjanje zaostale privremene mape prije zamjene

	imaStaru := false
	if _, err := os.Stat(ciljApp); err == nil {
		imaStaru = os.Rename(ciljApp, stara) == nil
	}
	if err := os.Rename(noviApp, ciljApp); err != nil {
		if imaStaru {
			_ = os.Rename(stara, ciljApp) // vraćanje prethodnog stanja ako zamjena ne uspije
		}
		return err
	}
	_ = os.RemoveAll(stara) // brisanje prethodne verzije nakon uspješne zamjene
	return nil
}

func odrediPutanjuAplikacije() string {
	execPutanja, err := executablePutanjaFn()
	if err != nil {
		return "/Applications/PometiME.app"
	}
	return izracunajPutanjuAppa(execPutanja)
}

func izracunajPutanjuAppa(execPutanja string) string {
	if strings.Contains(execPutanja, ".app/Contents/MacOS") {
		idx := strings.Index(execPutanja, ".app")
		return execPutanja[:idx+4]
	}
	return "/Applications/PometiME.app"
}
