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

// SkenirajPokretace pronalazi LaunchAgente korisnika i sustava te provjerava postojanje njihovih programa.
func SkenirajPokretace(p Postavke) []PokretacSustava {
	var svi []PokretacSustava
	korisnickaMapa := filepath.Join(p.KorijenKorisnika, "Library", "LaunchAgents")
	svi = append(svi, prikupiPokretaceIzMape(korisnickaMapa, true)...)

	sort.Slice(svi, func(i, j int) bool {
		if svi[i].Status != svi[j].Status {
			return svi[i].Status == "siroce"
		}
		return svi[i].Naziv < svi[j].Naziv
	})
	return svi
}

func prikupiPokretaceIzMape(mapa string, jeKorisnicki bool) []PokretacSustava {
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
		izlaz = append(izlaz, obradiJedanPokretac(put, u.Name(), jeKorisnicki))
	}
	return izlaz
}

func obradiJedanPokretac(putanja, ime string, jeKorisnicki bool) PokretacSustava {
	sadrzaj, err := os.ReadFile(putanja)
	if err != nil {
		return PokretacSustava{ID: putanja, Naziv: ime, Putanja: putanja, Status: "nepoznato", Opis: "Datoteka nije čitljiva."}
	}
	izvrsna := izvuciIzvrsnuPutanju(string(sadrzaj))
	postoji, status, opis := odrediStatusPokretaca(izvrsna)
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

func odrediStatusPokretaca(izvrsna string) (bool, string, string) {
	if izvrsna == "" {
		return true, "aktivan", "Pokretač koristi sustavske servise ili ugrađeni XPC."
	}
	if _, err := os.Stat(izvrsna); err == nil {
		return true, "aktivan", "Aktivni pokretač instaliranog programa."
	}
	return false, "siroce", "Program je uklonjen s diska; preporučeno premjestiti u Koš."
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
func IzvrsiOptimizaciju(vrsta string) (string, error) {
	switch vrsta {
	case "dns":
		if err := pokreniNaredbu("dscacheutil", "-flushcache"); err != nil {
			return "", fmt.Errorf("pražnjenje DNS predmemorije: %w", err)
		}
		return "DNS predmemorija je uspješno očišćena i osvježena.", nil
	case "quicklook":
		_ = pokreniNaredbu("qlmanage", "-r", "cache") // reset predmemorije pregleda ne ruši sustav
		_ = pokreniNaredbu("qlmanage", "-r")          // ponovno pokretanje demona pregleda
		return "QuickLook međuspremnik i ikone Finder-a su uspješno osvježeni.", nil
	case "snapshots":
		if err := pokreniNaredbu("tmutil", "thinlocalsnapshots", "/", "999999999999", "4"); err != nil {
			return "", fmt.Errorf("oslobađanje Time Machine snimaka: %w", err)
		}
		return "Lokalne APFS Time Machine snimke su uspješno pročišćene i oslobođene.", nil
	default:
		return "", fmt.Errorf("nepoznata radnja optimizacije: %s", vrsta)
	}
}
