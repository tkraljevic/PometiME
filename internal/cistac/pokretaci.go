package cistac

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	uzorakProgram       = regexp.MustCompile(`(?s)<key>Program</key>\s*<string>\s*([^<]+?)\s*</string>`)
	uzorakArgumenti     = regexp.MustCompile(`(?s)<key>ProgramArguments</key>\s*<array>\s*<string>\s*([^<]+?)\s*</string>`)
	pokreniNaredbu      = func(name string, args ...string) error {
		return exec.Command(name, args...).Run()
	}
	pokreniIzlazNaredbe = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}
)

// PostaviPokretacNaredbiZaTest omogućuje zamjenu izvršavanja sistemskih naredbi u testovima.
func PostaviPokretacNaredbiZaTest(fn func(name string, args ...string) error) func() {
	stara := pokreniNaredbu
	pokreniNaredbu = fn
	return func() {
		pokreniNaredbu = stara
	}
}

// SkenirajPokretace pronalazi LaunchAgente korisnika i sustava te provjerava postojanje njihovih programa.
func SkenirajPokretace(p Postavke) []PokretacSustava {
	var svi []PokretacSustava
	korisnickaMapa := filepath.Join(p.KorijenKorisnika, "Library", "LaunchAgents")
	svi = append(svi, prikupiPokretaceIzMape(p.KorijenKorisnika, korisnickaMapa, true)...)

	sort.Slice(svi, func(i, j int) bool {
		if svi[i].Status != svi[j].Status {
			return svi[i].Status == "siroce"
		}
		return svi[i].Naziv < svi[j].Naziv
	})
	return svi
}

func prikupiPokretaceIzMape(dom, mapa string, jeKorisnicki bool) []PokretacSustava {
	ulazi, err := os.ReadDir(mapa)
	if err != nil {
		return nil
	}
	var izlaz []PokretacSustava
	for _, u := range ulazi {
		if !strings.HasSuffix(u.Name(), ".plist") {
			continue
		}
		put := filepath.Join(mapa, u.Name())
		izlaz = append(izlaz, obradiJedanPokretac(dom, put, u.Name(), jeKorisnicki))
	}
	return izlaz
}

func obradiJedanPokretac(dom, putanja, ime string, jeKorisnicki bool) PokretacSustava {
	sadrzaj, err := os.ReadFile(putanja)
	if err != nil {
		return PokretacSustava{ID: putanja, Naziv: ime, Putanja: putanja, Status: "nepoznato", Opis: T("Datoteka nije čitljiva.")}
	}
	izvrsna := izvuciIzvrsnuPutanju(string(sadrzaj))
	postoji, status, opis := odrediStatusPokretaca(dom, izvrsna)
	return PokretacSustava{
		ID:             putanja,
		Naziv:          ime,
		Putanja:        putanja,
		PutanjaIzvrsne: izvrsna,
		PostojiIzvrsna: postoji,
		JeKorisnicki:   jeKorisnicki,
		Status:         status,
		Opis:           opis,
	}
}

func izvuciIzvrsnuPutanju(xmlTekst string) string {
	if m := uzorakProgram.FindStringSubmatch(xmlTekst); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	if m := uzorakArgumenti.FindStringSubmatch(xmlTekst); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// mapeProgramaSustava su mape u kojima launchd i Homebrew drže programe koje
// pokretač može zadati samo imenom, na primjer „node”.
var mapeProgramaSustava = []string{"/usr/bin", "/bin", "/usr/sbin", "/sbin", "/usr/local/bin", "/opt/homebrew/bin"}

// korisnickeMapePrograma su uobičajene mape alata u kućnoj mapi.
var korisnickeMapePrograma = []string{".local/bin", "bin", "go/bin", ".cargo/bin"}

// kandidatiPrograma vraća gdje bi program pokretača mogao biti i je li to
// jedino moguće mjesto. Program zadan imenom ili relativnom putanjom launchd
// može naći i ondje gdje PometiME ne gleda, na primjer u PATH-u pokretača.
func kandidatiPrograma(dom, izvrsna string) ([]string, bool) {
	switch {
	case filepath.IsAbs(izvrsna):
		return []string{izvrsna}, true
	case strings.HasPrefix(izvrsna, "~/"):
		return []string{filepath.Join(dom, izvrsna[2:])}, true
	case strings.Contains(izvrsna, "/"):
		return nil, false
	}
	kandidati := make([]string, 0, len(mapeProgramaSustava)+len(korisnickeMapePrograma))
	for _, mapa := range mapeProgramaSustava {
		kandidati = append(kandidati, filepath.Join(mapa, izvrsna))
	}
	for _, mapa := range korisnickeMapePrograma {
		kandidati = append(kandidati, filepath.Join(dom, mapa, izvrsna))
	}
	return kandidati, false
}

// odrediStatusPokretaca zaostalim proglašava samo pokretač čiji program sigurno
// više nije na disku, jer se zaostali pokretači nude za uklanjanje.
func odrediStatusPokretaca(dom, izvrsna string) (bool, string, string) {
	if izvrsna == "" {
		return true, "aktivan", T("Pokretač koristi sustavske servise ili ugrađeni XPC.")
	}
	kandidati, sigurno := kandidatiPrograma(dom, izvrsna)
	for _, kandidat := range kandidati {
		_, err := os.Stat(kandidat)
		if err == nil {
			return true, "aktivan", T("Aktivni pokretač instaliranog programa.")
		}
		sigurno = sigurno && os.IsNotExist(err)
	}
	if !sigurno {
		return false, "nepoznato", fmt.Sprintf(T("PometiME ne može provjeriti postoji li program %s, pa ovaj pokretač ne nudi za uklanjanje."), izvrsna)
	}
	return false, "siroce", T("Program je uklonjen s diska; preporučeno premjestiti u Koš.")
}

// PopisLokalnihSnimaka dohvaća popis lokalnih Time Machine APFS snimaka.
func PopisLokalnihSnimaka() []string {
	izlaz, err := pokreniIzlazNaredbe("tmutil", "listlocalsnapshots", "/")
	if err != nil {
		return nil
	}
	return izdvojiNaziveSnimaka(string(izlaz))
}

func izdvojiNaziveSnimaka(tekst string) []string {
	var snimke []string
	linije := strings.Split(tekst, "\n")
	for _, l := range linije {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "com.apple.TimeMachine.") {
			snimke = append(snimke, l)
		}
	}
	return snimke
}

// IzvrsiOptimizaciju pokreće odabranu radnju održavanja i ubrzanja sustava macOS-a.
func IzvrsiOptimizaciju(p Postavke, vrsta string) (string, error) {
	switch vrsta {
	case "dns":
		return izvrsiDnsOptimizaciju()
	case "quicklook":
		return izvrsiQuicklookOptimizaciju()
	case "snapshots":
		return izvrsiSnapshotsOptimizaciju()
	case "aerials":
		return OcistiZracnePozadine(p)
	case "otvori_aerials":
		return otvoriMapuAerials()
	case "simulators":
		return izvrsiSimulatorsOptimizaciju()
	default:
		return "", fmt.Errorf(T("nepoznata radnja optimizacije: %s"), vrsta)
	}
}

func izvrsiDnsOptimizaciju() (string, error) {
	if err := pokreniNaredbu("dscacheutil", "-flushcache"); err != nil {
		return "", fmt.Errorf(T("pražnjenje DNS predmemorije: %w"), err)
	}
	return T("DNS predmemorija je uspješno očišćena i osvježena."), nil
}

func izvrsiQuicklookOptimizaciju() (string, error) {
	_ = pokreniNaredbu("qlmanage", "-r", "cache") // reset predmemorije pregleda ne ruši sustav
	_ = pokreniNaredbu("qlmanage", "-r")          // ponovno pokretanje demona pregleda
	return T("QuickLook međuspremnik i ikone Finder-a su uspješno osvježeni."), nil
}

func izvrsiSnapshotsOptimizaciju() (string, error) {
	if err := pokreniNaredbu("tmutil", "thinlocalsnapshots", "/", "999999999999", "4"); err != nil {
		return "", fmt.Errorf(T("oslobađanje Time Machine snimaka: %w"), err)
	}
	return T("Lokalne APFS Time Machine snimke su uspješno pročišćene i oslobođene."), nil
}

func otvoriMapuAerials() (string, error) {
	mapa := filepath.Join(korijenSustavskeKnjiznice, "Application Support", "com.apple.idleassetsd", "Customer")
	_ = pokreniNaredbu("open", mapa) // otvaranje mape u Finderu
	return T("Mapa zračnih pozadina je otvorena u Finderu."), nil
}

func izvrsiSimulatorsOptimizaciju() (string, error) {
	if err := pokreniNaredbu("xcrun", "simctl", "delete", "unavailable"); err != nil {
		return "", fmt.Errorf(T("čišćenje nedostupnih simulatora nije uspjelo (provjerite je li Xcode instaliran): %w"), err)
	}
	return T("Nedostupni iOS simulatori su uspješno očišćeni."), nil
}
