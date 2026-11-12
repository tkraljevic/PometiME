package cistac

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
)

type metaAplikacije struct {
	naziv   string
	paketID string
	inacica string
	putanja string
	grupe   []string
}

// pravoPisanja je W_OK za access(2).
const pravoPisanja = 0x2

var (
	uzorakPlistKljuca = regexp.MustCompile(`(?s)<key>\s*([^<]+?)\s*</key>\s*<string>\s*([^<]*?)\s*</string>`)
	uzorakGrupa       = regexp.MustCompile(`(?s)<key>com\.apple\.security\.application-groups</key>\s*<array>(.*?)</array>`)
	uzorakNiza        = regexp.MustCompile(`<string>\s*([^<]+?)\s*</string>`)

	// Binarni plist (oko 15 % aplikacija) plutil pretvara u XML koji regex razumije.
	pretvoriPlistFn = func(put string) ([]byte, error) {
		return pokreniIzlazNaredbe("/usr/bin/plutil", "-convert", "xml1", "-o", "-", put)
	}
	procitajGrupeFn     = procitajGrupeAplikacije
	mozeSePremjestitiFn = mozeSePremjestiti
)

// citajPlistKaoXML vraća sadržaj plista kao XML tekst, i kad je zapisan binarno.
func citajPlistKaoXML(put string) (string, error) {
	sadrzaj, err := os.ReadFile(put)
	if err != nil {
		return "", err
	}
	if !bytes.HasPrefix(sadrzaj, []byte("bplist")) {
		return string(sadrzaj), nil
	}
	xml, err := pretvoriPlistFn(put)
	return string(xml), err
}

// procitajInfoPlist čita osnovne podatke iz Info.plist datoteke aplikacije.
func procitajInfoPlist(putanjaApp string) metaAplikacije {
	osnovnoIme := strings.TrimSuffix(filepath.Base(putanjaApp), ".app")
	meta := metaAplikacije{naziv: osnovnoIme, putanja: putanjaApp, inacica: "1.0"}

	sadrzaj, err := citajPlistKaoXML(filepath.Join(putanjaApp, "Contents", "Info.plist"))
	if err != nil {
		return meta
	}
	parovi := izvuciPlistParove(sadrzaj)
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

func prikupiSveMetaAplikacija(p Postavke) []metaAplikacije {
	korijeni := []string{
		p.KorijenAplikacija,
		filepath.Join(p.KorijenKorisnika, "Applications"),
	}
	var popis []metaAplikacije
	for _, kor := range korijeni {
		popis = append(popis, prikupiMetaAplikacija(kor)...)
	}
	return popis
}

func prikupiMetaAplikacija(mapaAplikacija string) []metaAplikacije {
	ulazi, err := os.ReadDir(mapaAplikacija)
	if err != nil {
		return nil
	}
	var popis []metaAplikacije
	for _, u := range ulazi {
		popis = append(popis, obradiUlazAplikacije(mapaAplikacija, u)...)
	}
	return popis
}

func obradiUlazAplikacije(mapaAplikacija string, u os.DirEntry) []metaAplikacije {
	ime := u.Name()
	punaPutanja := filepath.Join(mapaAplikacija, ime)
	if strings.HasSuffix(ime, ".app") {
		meta := procitajInfoPlist(punaPutanja)
		if strings.HasPrefix(meta.paketID, "com.apple.") {
			return nil
		}
		return []metaAplikacije{meta}
	}
	if u.IsDir() && !strings.HasPrefix(ime, ".") {
		return prikupiPodmapuAplikacija(punaPutanja)
	}
	return nil
}

func prikupiPodmapuAplikacija(podmapa string) []metaAplikacije {
	ulazi, err := os.ReadDir(podmapa)
	if err != nil {
		return nil
	}
	var popis []metaAplikacije
	for _, u := range ulazi {
		if !strings.HasSuffix(u.Name(), ".app") {
			continue
		}
		meta := procitajInfoPlist(filepath.Join(podmapa, u.Name()))
		if !strings.HasPrefix(meta.paketID, "com.apple.") {
			popis = append(popis, meta)
		}
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
		{"Containers", filepath.Join(lib, "Containers")},
	}
}

// procitajGrupeAplikacije čita grupe aplikacija iz potpisa (application-groups).
// Imena grupa su imena mapa u ~/Library/Group Containers.
func procitajGrupeAplikacije(putanjaApp string) []string {
	izlaz, err := pokreniIzlazNaredbe("/usr/bin/codesign", "-d", "--entitlements", "-", "--xml", putanjaApp)
	if err != nil {
		return nil
	}
	return izvuciGrupe(string(izlaz))
}

func izvuciGrupe(xml string) []string {
	m := uzorakGrupa.FindStringSubmatch(xml)
	if m == nil {
		return nil
	}
	var grupe []string
	for _, n := range uzorakNiza.FindAllStringSubmatch(m[1], -1) {
		grupe = append(grupe, n[1])
	}
	return grupe
}

// dopuniGrupe čita grupe svih aplikacija usporedno; codesign traje oko 0,1 s po aplikaciji.
func dopuniGrupe(mete []metaAplikacije) {
	var wg sync.WaitGroup
	mjesta := make(chan struct{}, radnikaMjerenja)
	for i := range mete {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mjesta <- struct{}{}
			mete[i].grupe = procitajGrupeFn(mete[i].putanja)
			<-mjesta
		}()
	}
	wg.Wait()
}

func brojKoristenjaGrupa(mete []metaAplikacije) map[string]int {
	koristenje := map[string]int{}
	for _, m := range mete {
		for _, g := range m.grupe {
			koristenje[g]++
		}
	}
	return koristenje
}

// grupniKontejneri vraća Group Containers koje koristi samo ova aplikacija.
// Grupu koju dijeli i druga instalirana aplikacija (npr. Office) treba toj aplikaciji.
func grupniKontejneri(dom string, meta metaAplikacije, koristenje map[string]int) []PovezanaPutanja {
	var povezane []PovezanaPutanja
	for _, grupa := range meta.grupe {
		if koristenje[grupa] > 1 || strings.Contains(grupa, "/") || strings.HasPrefix(grupa, ".") {
			continue
		}
		put := filepath.Join(dom, "Library", "Group Containers", grupa)
		if info, err := os.Lstat(put); err == nil && info.IsDir() {
			povezane = append(povezane, PovezanaPutanja{Vrsta: "Group Containers", Putanja: put, VelicinaBajta: IzracunajVelicinu(put)})
		}
	}
	return povezane
}

// pokretaciAplikacije vraća LaunchAgente nazvane po aplikaciji ili one koji
// pokreću program iz njezina paketa.
func pokretaciAplikacije(dom string, meta metaAplikacije) []PovezanaPutanja {
	mapa := filepath.Join(dom, "Library", "LaunchAgents")
	ulazi, err := os.ReadDir(mapa)
	if err != nil {
		return nil
	}
	var povezane []PovezanaPutanja
	for _, u := range ulazi {
		put := filepath.Join(mapa, u.Name())
		if strings.HasSuffix(u.Name(), ".plist") && pokretacPripadaAplikaciji(u.Name(), put, meta) {
			povezane = append(povezane, PovezanaPutanja{Vrsta: "LaunchAgents", Putanja: put, VelicinaBajta: velicinaUlazaPoImenu(put)})
		}
	}
	return povezane
}

func pokretacPripadaAplikaciji(ime, put string, meta metaAplikacije) bool {
	if meta.paketID != "" && strings.Contains(strings.ToLower(ime), strings.ToLower(meta.paketID)) {
		return true
	}
	sadrzaj, err := citajPlistKaoXML(put)
	if err != nil {
		return false
	}
	return strings.HasPrefix(izvuciIzvrsnuPutanju(sadrzaj), meta.putanja+"/")
}

// mozeSePremjestiti provjerava smije li korisnik premjestiti paket aplikacije.
// Paket u vlasništvu roota (instalacijski paket, App Store) traži administratorsku lozinku.
func mozeSePremjestiti(putanjaApp string) bool {
	return syscall.Access(putanjaApp, pravoPisanja) == nil && syscall.Access(filepath.Dir(putanjaApp), pravoPisanja) == nil
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
	mete := prikupiSveMetaAplikacija(p)
	dopuniGrupe(mete)
	koristenje := brojKoristenjaGrupa(mete)
	izlaz := make([]InstaliranaAplikacija, 0, len(mete))
	for _, m := range mete {
		izlaz = append(izlaz, opisiAplikaciju(p.KorijenKorisnika, m, koristenje))
	}

	sort.Slice(izlaz, func(i, j int) bool {
		return izlaz[i].UkupnaVelicina > izlaz[j].UkupnaVelicina
	})
	return izlaz
}

func opisiAplikaciju(dom string, m metaAplikacije, koristenje map[string]int) InstaliranaAplikacija {
	velApp := IzracunajVelicinu(m.putanja)
	povezane, velPod := pronadiPovezanePutanje(dom, m)
	for _, dodatna := range append(grupniKontejneri(dom, m, koristenje), pokretaciAplikacije(dom, m)...) {
		povezane = append(povezane, dodatna)
		velPod += dodatna.VelicinaBajta
	}
	return InstaliranaAplikacija{
		Naziv:               m.naziv,
		IdentifikatorPaketa: m.paketID,
		Inacica:             m.inacica,
		PutanjaAplikacije:   m.putanja,
		VelicinaAplikacije:  velApp,
		VelicinaPodataka:    velPod,
		UkupnaVelicina:      velApp + velPod,
		PovezanePutanje:     povezane,
		TrebaAdmin:          !mozeSePremjestitiFn(m.putanja),
	}
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
	"rtk": true, "go": true, "cargo": true, "rustup": true,
	"npm": true, "pnpm": true, "yarn": true, "pip": true,
	"brew": true, "git": true, "vode": true, "agent-hv": true,
	"askpermissiond": true, "familycircled": true, "networkserviceproxy": true,
	"syncservices": true, "animoji": true, "passkit": true, "gamekit": true,
	"siritts": true, "biome": true, "replaykit": true, "fileprovider": true,
	"mobilesync": true,
}

func izgradiSkupPoznatih(mete []metaAplikacije) map[string]bool {
	poznati := make(map[string]bool)
	for k := range zasticeniProizvodaci {
		poznati[k] = true
	}
	for _, m := range mete {
		dodajMetuUPoznate(poznati, m)
	}
	ucitajImenaIzMapa(poznati, []string{"/opt/homebrew/Cellar", "/opt/homebrew/Caskroom", "/usr/local/Cellar"})
	return poznati
}

func ucitajImenaIzMapa(poznati map[string]bool, mape []string) {
	for _, m := range mape {
		ulazi, err := os.ReadDir(m)
		if err != nil {
			continue
		}
		for _, u := range ulazi {
			poznati[strings.ToLower(u.Name())] = true
		}
	}
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
	if strings.HasPrefix(malo, "com.apple.") || strings.HasPrefix(malo, ".") || poznati[malo] {
		return true
	}
	return sadrziPoznatiKljuc(malo, poznati)
}

func sadrziPoznatiKljuc(malo string, poznati map[string]bool) bool {
	for kljuc := range poznati {
		if len(kljuc) >= 4 && strings.Contains(malo, kljuc) {
			return true
		}
	}
	return false
}

// SkenirajOstatke pronalazi mape u ~/Library koje ne pripadaju nijednoj instaliranoj aplikaciji.
func SkenirajOstatke(p Postavke) []StavkaCiscenja {
	poznati := izgradiSkupPoznatih(prikupiSveMetaAplikacija(p))
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
