package cistac

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type metaAplikacije struct {
	naziv   string
	paketID string
	inacica string
	putanja string
}

var uzorakPlistKljuca = regexp.MustCompile(`(?s)<key>\s*([^<]+?)\s*</key>\s*<string>\s*([^<]*?)\s*</string>`)

// procitajInfoPlist čita osnovne podatke iz XML Info.plist datoteke aplikacije.
func procitajInfoPlist(putanjaApp string) metaAplikacije {
	osnovnoIme := strings.TrimSuffix(filepath.Base(putanjaApp), ".app")
	meta := metaAplikacije{naziv: osnovnoIme, putanja: putanjaApp, inacica: "1.0"}

	sadrzaj, err := os.ReadFile(filepath.Join(putanjaApp, "Contents", "Info.plist"))
	if err != nil {
		return meta
	}
	parovi := izvuciPlistParove(string(sadrzaj))
	if id := parovi["CFBundleIdentifier"]; id != "" {
		meta.paketID = id
	}
	if v := parovi["CFBundleShortVersionString"]; v != "" {
		meta.inacica = v
	}
	if ime := odaberiImeIzParova(parovi, osnovnoIme); ime != "" {
		meta.naziv = ime
	}
	return meta
}

func izvuciPlistParove(xmlTekst string) map[string]string {
	izlaz := make(map[string]string)
	podudaranja := uzorakPlistKljuca.FindAllStringSubmatch(xmlTekst, -1)
	for _, m := range podudaranja {
		izlaz[strings.TrimSpace(m[1])] = strings.TrimSpace(m[2])
	}
	return izlaz
}

func odaberiImeIzParova(parovi map[string]string, zadano string) string {
	if disp := parovi["CFBundleDisplayName"]; disp != "" {
		return disp
	}
	if name := parovi["CFBundleName"]; name != "" {
		return name
	}
	return zadano
}

func prikupiMetaAplikacija(mapaAplikacija string) []metaAplikacije {
	ulazi, err := os.ReadDir(mapaAplikacija)
	if err != nil {
		return nil
	}
	var popis []metaAplikacije
	for _, u := range ulazi {
		ime := u.Name()
		if !strings.HasSuffix(ime, ".app") {
			continue
		}
		meta := procitajInfoPlist(filepath.Join(mapaAplikacija, ime))
		if strings.HasPrefix(meta.paketID, "com.apple.") {
			continue
		}
		popis = append(popis, meta)
	}
	return popis
}

func mapePodrskeKorisnika(korijenKorisnika string) []struct{ vrsta, putanja string } {
	lib := filepath.Join(korijenKorisnika, "Library")
	return []struct{ vrsta, putanja string }{
		{"Application Support", filepath.Join(lib, "Application Support")},
		{"Caches", filepath.Join(lib, "Caches")},
		{"Preferences", filepath.Join(lib, "Preferences")},
		{"Logs", filepath.Join(lib, "Logs")},
		{"Saved State", filepath.Join(lib, "Saved Application State")},
		{"WebKit", filepath.Join(lib, "WebKit")},
		{"HTTPStorages", filepath.Join(lib, "HTTPStorages")},
	}
}

func odgovaraAplikaciji(imeDatoteke string, meta metaAplikacije) bool {
	maloIme := strings.ToLower(imeDatoteke)
	maliPaket := strings.ToLower(meta.paketID)
	if maliPaket != "" && strings.Contains(maloIme, maliPaket) {
		return true
	}
	maliNaziv := strings.ToLower(meta.naziv)
	zbiveniNaziv := strings.ReplaceAll(maliNaziv, " ", "")
	return len(zbiveniNaziv) >= 4 && (maloIme == maliNaziv || maloIme == zbiveniNaziv)
}

func pronadiPovezanePutanje(korijenKorisnika string, meta metaAplikacije) ([]PovezanaPutanja, int64) {
	var povezane []PovezanaPutanja
	var ukupnoPodataka int64

	for _, mp := range mapePodrskeKorisnika(korijenKorisnika) {
		stavke, iznos := pretraziJednuMapuPodrske(mp.vrsta, mp.putanja, meta)
		povezane = append(povezane, stavke...)
		ukupnoPodataka += iznos
	}
	return povezane, ukupnoPodataka
}

func pretraziJednuMapuPodrske(vrsta, mapa string, meta metaAplikacije) ([]PovezanaPutanja, int64) {
	ulazi, err := os.ReadDir(mapa)
	if err != nil {
		return nil, 0
	}
	var povezane []PovezanaPutanja
	var zbroj int64
	for _, u := range ulazi {
		if !odgovaraAplikaciji(u.Name(), meta) {
			continue
		}
		put := filepath.Join(mapa, u.Name())
		vel := IzracunajVelicinu(put)
		povezane = append(povezane, PovezanaPutanja{Vrsta: vrsta, Putanja: put, VelicinaBajta: vel})
		zbroj += vel
	}
	return povezane, zbroj
}

// SkenirajAplikacije pronalazi instalirane aplikacije i sve njihove pripadajuće mape u ~/Library.
func SkenirajAplikacije(p Postavke) []InstaliranaAplikacija {
	mete := prikupiMetaAplikacija(p.KorijenAplikacija)
	izlaz := make([]InstaliranaAplikacija, 0, len(mete))

	for _, m := range mete {
		velApp := IzracunajVelicinu(m.putanja)
		povezane, velPod := pronadiPovezanePutanje(p.KorijenKorisnika, m)
		izlaz = append(izlaz, InstaliranaAplikacija{
			Naziv:               m.naziv,
			IdentifikatorPaketa: m.paketID,
			Inacica:             m.inacica,
			PutanjaAplikacije:   m.putanja,
			VelicinaAplikacije:  velApp,
			VelicinaPodataka:    velPod,
			UkupnaVelicina:      velApp + velPod,
			PovezanePutanje:     povezane,
		})
	}

	sort.Slice(izlaz, func(i, j int) bool {
		return izlaz[i].UkupnaVelicina > izlaz[j].UkupnaVelicina
	})
	return izlaz
}

var zasticeniProizvodaci = map[string]bool{
	"apple": true, "google": true, "microsoft": true, "mozilla": true,
	"firefox": true, "adobe": true, "claude": true, "anthropic": true,
	"antigravity": true, "parallels": true, "tailscale": true, "obsidian": true,
	"telegram": true, "whatsapp": true, "homebrew": true, "qgis": true,
	"openai": true, "codex": true, "github": true, "docker": true,
	"jetbrains": true, "autodesk": true, "acronis": true, "onedrive": true,
	"zoom": true, "spotify": true, "vlc": true, "crashreporter": true,
	"caches": true, "knowledge": true, "cloud": true,
}

func izgradiSkupPoznatih(mete []metaAplikacije) map[string]bool {
	poznati := make(map[string]bool)
	for k := range zasticeniProizvodaci {
		poznati[k] = true
	}
	for _, m := range mete {
		dodajMetuUPoznate(poznati, m)
	}
	return poznati
}

func dodajMetuUPoznate(poznati map[string]bool, m metaAplikacije) {
	if m.paketID != "" {
		maliID := strings.ToLower(m.paketID)
		poznati[maliID] = true
		dijelovi := strings.Split(maliID, ".")
		if len(dijelovi) >= 2 {
			poznati[dijelovi[1]] = true
		}
	}
	maliNaziv := strings.ToLower(m.naziv)
	poznati[maliNaziv] = true
	poznati[strings.ReplaceAll(maliNaziv, " ", "")] = true
}

func pripadaPoznatojAplikaciji(imeMape string, poznati map[string]bool) bool {
	malo := strings.TrimSuffix(strings.ToLower(imeMape), ".savedstate")
	if strings.HasPrefix(malo, "com.apple.") || strings.HasPrefix(malo, ".") {
		return true
	}
	if poznati[malo] {
		return true
	}
	for kljuc := range poznati {
		if len(kljuc) >= 4 && strings.Contains(malo, kljuc) {
			return true
		}
	}
	return false
}

// SkenirajOstatke pronalazi mape u ~/Library koje ne pripadaju nijednoj instaliranoj aplikaciji.
func SkenirajOstatke(p Postavke) []StavkaCiscenja {
	poznati := izgradiSkupPoznatih(prikupiMetaAplikacija(p.KorijenAplikacija))
	lib := filepath.Join(p.KorijenKorisnika, "Library")
	izvori := []struct{ oznaka, putanja string }{
		{"Application Support", filepath.Join(lib, "Application Support")},
		{"Saved State", filepath.Join(lib, "Saved Application State")},
		{"WebKit", filepath.Join(lib, "WebKit")},
		{"HTTPStorages", filepath.Join(lib, "HTTPStorages")},
	}

	var kandidati []kandidatCiscenja
	for _, izv := range izvori {
		kandidati = append(kandidati, kandidatiOstatakaUMapi(izv.oznaka, izv.putanja, poznati)...)
	}
	return izmjeriKandidate(kandidati, p.NajmanjaVelicinaOstataka)
}

func kandidatiOstatakaUMapi(oznaka, mapa string, poznati map[string]bool) []kandidatCiscenja {
	ulazi, err := os.ReadDir(mapa)
	if err != nil {
		return nil
	}
	var izlaz []kandidatCiscenja
	for _, u := range ulazi {
		ime := u.Name()
		if pripadaPoznatojAplikaciji(ime, poznati) {
			continue
		}
		izlaz = append(izlaz, kandidatCiscenja{
			putanja:     filepath.Join(mapa, ime),
			naziv:       ime + " (" + oznaka + ")",
			kategorija:  "leftovers",
			opis:        "Zaostali podaci u ~/Library/" + oznaka + " bez pronađene aplikacije.",
			preporuceno: false,
		})
	}
	return izlaz
}
