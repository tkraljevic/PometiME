package cistac

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
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
const TrenutnaInacica = "1.0.10"

// identifikatorPaketa je CFBundleIdentifier koji preuzeta aplikacija mora imati.
const identifikatorPaketa = "hr.kraljevic.pometime"

// javniKljucNadogradnjeB64 provjerava potpis arhive izdanja. Pripadni tajni
// ključ čuva se izvan repozitorija (vidi odjeljak Izdavanje u README.md).
const javniKljucNadogradnjeB64 = "1sVS6uEP3YTV6tZalhLKjSKugjBNGANfZUVxRMkqe4M="

const (
	najvecaArhiva  = 200 << 20
	najveciPotpis  = 1 << 10
	najveciOpis    = 1 << 20
	rokProvjere    = 8 * time.Second
	rokPreuzimanja = 10 * time.Minute
)

var (
	urlGitHubReleases         = "https://api.github.com/repos/tkraljevic/PometiME/releases/latest"
	prefiksPreuzimanja        = "https://github.com/tkraljevic/PometiME/releases/download/"
	javniKljucNadogradnje     = ucitajJavniKljuc(javniKljucNadogradnjeB64)
	klijentNadogradnje        = noviKlijentNadogradnje()
	odrediPutanjuAplikacijeFn = odrediPutanjuAplikacije
	executablePutanjaFn       = os.Executable
	stvoriPrivremenuMapuFn    = func() (string, error) { return os.MkdirTemp("", "pometime-nadogradnja-*") }
)

// noviKlijentNadogradnje nema ukupni rok, jer bi on prekinuo preuzimanje na
// sporoj vezi. Rokove zadaje kontekst svakog zahtjeva.
func noviKlijentNadogradnje() *http.Client {
	prijenos := http.DefaultTransport.(*http.Transport).Clone()
	prijenos.ResponseHeaderTimeout = 15 * time.Second
	return &http.Client{Transport: prijenos}
}

func ucitajJavniKljuc(b64 string) ed25519.PublicKey {
	kljuc, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(kljuc) != ed25519.PublicKeySize {
		panic("javni ključ nadogradnje nije ispravan")
	}
	return ed25519.PublicKey(kljuc)
}

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

// PostaviProvjeruIzdanjaZaTest privremeno mijenja dopuštenu adresu arhiva i javni ključ potpisa.
func PostaviProvjeruIzdanjaZaTest(prefiks string, kljuc ed25519.PublicKey) func() {
	stariPrefiks := prefiksPreuzimanja
	stariKljuc := javniKljucNadogradnje
	prefiksPreuzimanja = prefiks
	javniKljucNadogradnje = kljuc
	return func() {
		prefiksPreuzimanja = stariPrefiks
		javniKljucNadogradnje = stariKljuc
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
// Adresu za preuzimanje vraća samo kad izdanje ima i potpis arhive.
func ProvjeriNadogradnju() (*InformacijeONadogradnji, error) {
	izdanje, err := dohvatiZadnjeIzdanje()
	if err != nil {
		return nil, err
	}
	info := &InformacijeONadogradnji{
		TrenutnaInacica:  TrenutnaInacica,
		NajnovijaInacica: strings.TrimPrefix(izdanje.TagName, "v"),
		NazivIzdanja:     izdanje.Name,
		Biljeske:         izdanje.Body,
		Poveznica:        izdanje.HTMLURL,
	}
	if arhiva, _, ok := potpisanaArhiva(izdanje.Assets); ok {
		info.PoveznicaPreuzimanja, info.VelicinaBajta = arhiva.BrowserDownloadURL, arhiva.Size
	}
	info.ImaNadogradnju = UsporediInacice(TrenutnaInacica, info.NajnovijaInacica)
	return info, nil
}

func dohvatiZadnjeIzdanje() (*githubRelease, error) {
	ctx, otkazi := context.WithTimeout(context.Background(), rokProvjere)
	defer otkazi()
	zahtjev, err := http.NewRequestWithContext(ctx, http.MethodGet, urlGitHubReleases, nil)
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
	if err := json.NewDecoder(io.LimitReader(odgovor.Body, najveciOpis)).Decode(&izdanje); err != nil {
		return nil, fmt.Errorf("dekodiranje GitHub odgovora: %w", err)
	}
	return &izdanje, nil
}

// potpisanaArhiva vraća zip arhivu izdanja i njezin potpis (ime arhive + ".sig").
func potpisanaArhiva(assets []githubAsset) (githubAsset, githubAsset, bool) {
	for _, arhiva := range assets {
		if !strings.HasSuffix(arhiva.Name, ".zip") {
			continue
		}
		for _, potpis := range assets {
			if potpis.Name == arhiva.Name+".sig" {
				return arhiva, potpis, true
			}
		}
	}
	return githubAsset{}, githubAsset{}, false
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

// PrimijeniNadogradnju sama dohvaća zadnje izdanje s GitHuba, provjerava potpis
// arhive i tek tada zamjenjuje aplikaciju. Adresu arhive nikad ne prima od
// sučelja, pa je ne može podmetnuti ni stranica ni drugi program.
func PrimijeniNadogradnju() error {
	izdanje, err := dohvatiZadnjeIzdanje()
	if err != nil {
		return err
	}
	if !UsporediInacice(TrenutnaInacica, izdanje.TagName) {
		return fmt.Errorf("nema novije verzije za instalaciju")
	}
	arhiva, potpis, ok := potpisanaArhiva(izdanje.Assets)
	if !ok {
		return fmt.Errorf("izdanje %s nema potpisanu arhivu, pa ga preuzmite ručno sa stranice izdanja", izdanje.TagName)
	}
	sadrzaj, err := preuzmiProvjerenuArhivu(arhiva, potpis)
	if err != nil {
		return err
	}
	return instalirajIzArhive(sadrzaj)
}

func preuzmiProvjerenuArhivu(arhiva, potpis githubAsset) ([]byte, error) {
	if !jeAdresaIzdanja(arhiva.BrowserDownloadURL) || !jeAdresaIzdanja(potpis.BrowserDownloadURL) {
		return nil, fmt.Errorf("arhiva nije na stranici izdanja PometiME, pa nadogradnja nije pokrenuta")
	}
	tekstPotpisa, err := preuzmi(potpis.BrowserDownloadURL, najveciPotpis)
	if err != nil {
		return nil, fmt.Errorf("preuzimanje potpisa nije uspjelo: %w", err)
	}
	sadrzaj, err := preuzmi(arhiva.BrowserDownloadURL, najvecaArhiva)
	if err != nil {
		return nil, fmt.Errorf("preuzimanje arhive nije uspjelo: %w", err)
	}
	if err := provjeriPotpis(sadrzaj, tekstPotpisa); err != nil {
		return nil, err
	}
	return sadrzaj, nil
}

func jeAdresaIzdanja(adresa string) bool {
	return strings.HasPrefix(adresa, prefiksPreuzimanja) && !strings.Contains(adresa, "..")
}

// preuzmi čita najviše zadani broj bajtova. Rok vrijedi za cijelo preuzimanje,
// a dovoljno je dug i za spore veze.
func preuzmi(adresa string, najvise int64) ([]byte, error) {
	ctx, otkazi := context.WithTimeout(context.Background(), rokPreuzimanja)
	defer otkazi()
	zahtjev, err := http.NewRequestWithContext(ctx, http.MethodGet, adresa, nil)
	if err != nil {
		return nil, err
	}
	odg, err := klijentNadogradnje.Do(zahtjev)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = odg.Body.Close() // zatvaranje odgovora preuzimanja
	}()
	if odg.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("poslužitelj je vratio status %d", odg.StatusCode)
	}
	sadrzaj, err := io.ReadAll(io.LimitReader(odg.Body, najvise+1))
	if err != nil {
		return nil, err
	}
	if int64(len(sadrzaj)) > najvise {
		return nil, fmt.Errorf("datoteka je veća od %s, pa preuzimanje nije nastavljeno", FormatirajBajte(najvise))
	}
	return sadrzaj, nil
}

func provjeriPotpis(sadrzaj, tekstPotpisa []byte) error {
	potpis, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(tekstPotpisa)))
	if err != nil || !ed25519.Verify(javniKljucNadogradnje, sadrzaj, potpis) {
		return fmt.Errorf("potpis arhive ne odgovara, pa nadogradnja nije instalirana; preuzmite PometiME ručno sa stranice izdanja")
	}
	return nil
}

func instalirajIzArhive(sadrzaj []byte) error {
	privremenaMapa, err := stvoriPrivremenuMapuFn()
	if err != nil {
		return fmt.Errorf("stvaranje privremene mape: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(privremenaMapa) // čišćenje privremene mape nadogradnje
	}()

	raspakirano := filepath.Join(privremenaMapa, "raspakirano")
	if err := raspakirajZip(sadrzaj, raspakirano); err != nil {
		return fmt.Errorf("raspakiravanje preuzete arhive: %w", err)
	}
	return instalirajAppIzMape(raspakirano)
}

func instalirajAppIzMape(raspakirano string) error {
	noviApp := pronadjiAppPaket(raspakirano)
	if noviApp == "" {
		return fmt.Errorf("u preuzetoj arhivi nije pronađen PometiME.app paket")
	}
	if err := provjeriNoviPaket(noviApp); err != nil {
		return err
	}
	ciljApp := odrediPutanjuAplikacijeFn()
	if err := zamijeniAplikaciju(noviApp, ciljApp); err != nil {
		return fmt.Errorf("zamjena aplikacije na lokaciji %s: %w", ciljApp, err)
	}
	return nil
}

// provjeriNoviPaket sprječava da se potpisana, ali starija arhiva instalira
// preko novije verzije.
func provjeriNoviPaket(noviApp string) error {
	meta := procitajInfoPlist(noviApp)
	if meta.paketID != identifikatorPaketa {
		return fmt.Errorf("preuzeta aplikacija nije PometiME")
	}
	if !UsporediInacice(TrenutnaInacica, meta.inacica) {
		return fmt.Errorf("preuzeta verzija %s nije novija od instalirane verzije %s", meta.inacica, TrenutnaInacica)
	}
	return nil
}

func raspakirajZip(sadrzaj []byte, odrediste string) error {
	r, err := zip.NewReader(bytes.NewReader(sadrzaj), int64(len(sadrzaj)))
	if err != nil {
		return err
	}
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
