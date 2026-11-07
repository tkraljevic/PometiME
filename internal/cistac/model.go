// Package cistac sadrži pravila skeniranja diska, provjere sigurnosti putanja i brisanja.
package cistac

import (
	"fmt"
	"path/filepath"
)

// Postavke određuju korijenske mape i pragove veličina za skeniranje i brisanje.
type Postavke struct {
	KorijenKorisnika             string
	KorijenAplikacija            string
	NajmanjaVelicinaPredmemorije int64
	NajmanjaVelicinaRazvojnih    int64
	NajmanjaVelicinaOstataka     int64
	NajmanjaVelicinaKopija       int64
}

// ZadanePostavke vraća standardne postavke za zadanu korisničku mapu na macOS-u.
func ZadanePostavke(korijenKorisnika string) Postavke {
	return Postavke{
		KorijenKorisnika:             filepath.Clean(korijenKorisnika),
		KorijenAplikacija:            "/Applications",
		NajmanjaVelicinaPredmemorije: 2 * 1024 * 1024,
		NajmanjaVelicinaRazvojnih:    512 * 1024,
		NajmanjaVelicinaOstataka:     256 * 1024,
		NajmanjaVelicinaKopija:       512 * 1024,
	}
}

// StanjeDiska opisuje zauzeće i slobodan prostor na disku.
type StanjeDiska struct {
	UkupnoBajta    int64   `json:"total_bytes"`
	ZauzetoBajta   int64   `json:"used_bytes"`
	SlobodnoBajta  int64   `json:"avail_bytes"`
	PostotakZauzet float64 `json:"used_percent"`
	TockaMontaze   string  `json:"mount_point"`
}

// StavkaCiscenja predstavlja jednu mapu ili datoteku koja se može očistiti.
type StavkaCiscenja struct {
	ID            string `json:"id"`
	Naziv         string `json:"name"`
	Kategorija    string `json:"category"`
	Putanja       string `json:"path"`
	VelicinaBajta int64  `json:"size_bytes"`
	Opis          string `json:"description"`
	Preporuceno   bool   `json:"recommended"`
}

// PovezanaPutanja predstavlja pripadajuću mapu aplikacije u ~/Library.
type PovezanaPutanja struct {
	Vrsta         string `json:"kind"`
	Putanja       string `json:"path"`
	VelicinaBajta int64  `json:"size_bytes"`
}

// InstaliranaAplikacija opisuje aplikaciju u /Applications i njezine podatke u ~/Library.
type InstaliranaAplikacija struct {
	Naziv                string            `json:"name"`
	IdentifikatorPaketa  string            `json:"bundle_id"`
	Inacica              string            `json:"version"`
	PutanjaAplikacije    string            `json:"app_path"`
	VelicinaAplikacije   int64             `json:"app_size_bytes"`
	VelicinaPodataka     int64             `json:"support_size_bytes"`
	UkupnaVelicina       int64             `json:"total_size_bytes"`
	PovezanePutanje      []PovezanaPutanja `json:"related_paths"`
}

// IzvjestajSkeniranja objedinjuje stanje diska i sve pronađene kategorije za čišćenje.
type IzvjestajSkeniranja struct {
	Disk                StanjeDiska      `json:"disk"`
	PredmemorijaSustava []StavkaCiscenja `json:"system_caches"`
	RazvojniAlati       []StavkaCiscenja `json:"dev_ai_junk"`
	OstaciAplikacija    []StavkaCiscenja `json:"leftovers"`
	KopijeIPrivici      []StavkaCiscenja `json:"backups_attachments"`
	UkupnoZaOsloboditi  int64            `json:"total_reclaimable_bytes"`
}

// StavkaKosa opisuje stavku premještenu u Koš za smeće s izvornom putanjom za povrat.
type StavkaKosa struct {
	ID              string `json:"id"`
	Naziv           string `json:"name"`
	IzvornaPutanja  string `json:"original_path"`
	PutanjaUKosu    string `json:"trash_path"`
	VelicinaBajta   int64  `json:"size_bytes"`
	VrijemeBrisanja string `json:"deleted_at"`
}

// PokretacSustava opisuje stavku iz LaunchAgents koja se pokreće sa sustavom ili prijavom.
type PokretacSustava struct {
	ID             string `json:"id"`
	Naziv          string `json:"name"`
	Putanja        string `json:"path"`
	PutanjaIzvrsne string `json:"executable_path"`
	PostojiIzvrsna bool   `json:"executable_exists"`
	JeKorisnicki   bool   `json:"is_user"`
	Status         string `json:"status"`
	Opis           string `json:"description"`
}

// RezultatOptimizacije opisuje ishod akcije održavanja sustava (npr. DNS, QuickLook).
type RezultatOptimizacije struct {
	Uspjeh bool   `json:"success"`
	Poruka string `json:"message"`
}

// InformacijeZracnihPozadina opisuje veličinu i broj preuzetih 4K video snimaka za pozadine i screensavere.
type InformacijeZracnihPozadina struct {
	UkupnoBajta int64  `json:"total_bytes"`
	BrojVidea   int    `json:"video_count"`
	Putanja     string `json:"path"`
	PostojiMapa bool   `json:"folder_exists"`
}

// RezultatCiscenja opisuje ishod premještanja u Koš ili povrata odabranih putanja.
type RezultatCiscenja struct {
	ObrisanoStavki  int         `json:"removed_count"`
	OslobodenoBajta int64       `json:"freed_bytes"`
	Poruka          string      `json:"message"`
	Greske          []string    `json:"errors"`
	Disk            StanjeDiska `json:"disk"`
}

// UzBrojHR vraća ispravan hrvatski oblik imenice uz zadani cijeli broj.
func UzBrojHR(n int, jednina, dvojina, mnozina string) string {
	apsolutni := n
	if apsolutni < 0 {
		apsolutni = -apsolutni
	}
	zadnjeDvije := apsolutni % 100
	if zadnjeDvije >= 11 && zadnjeDvije <= 14 {
		return fmt.Sprintf("%d %s", n, mnozina)
	}
	return oblikPremaZadnjojZnamenci(n, apsolutni%10, jednina, dvojina, mnozina)
}

func oblikPremaZadnjojZnamenci(n, zadnja int, jednina, dvojina, mnozina string) string {
	if zadnja == 1 {
		return fmt.Sprintf("%d %s", n, jednina)
	}
	if zadnja >= 2 && zadnja <= 4 {
		return fmt.Sprintf("%d %s", n, dvojina)
	}
	return fmt.Sprintf("%d %s", n, mnozina)
}

// PorukaRezultata vraća cijelu rečenicu o premještanju stavki u Koš s ispravnim oblikom uz broj.
func PorukaRezultata(obrisano int, oslobodenoBajta int64) string {
	if obrisano == 0 {
		return "Nijedna stavka nije premještena u Koš za smeće."
	}
	stavkeTekst := UzBrojHR(obrisano, "stavka", "stavke", "stavki")
	velicinaTekst := FormatirajBajte(oslobodenoBajta)
	return fmt.Sprintf("Uspješno premješteno u Koš %s (%s). Po potrebi ih možete vratiti iz kartice Koš i Povrat.", stavkeTekst, velicinaTekst)
}

// PorukaPovrata vraća cijelu rečenicu o povratu stavki iz Koša za smeće na izvorno mjesto.
func PorukaPovrata(vraceno int, vracenoBajta int64) string {
	if vraceno == 0 {
		return "Nijedna stavka nije vraćena iz Koša za smeće."
	}
	stavkeTekst := UzBrojHR(vraceno, "stavka", "stavke", "stavki")
	velicinaTekst := FormatirajBajte(vracenoBajta)
	return fmt.Sprintf("Uspješno vraćeno na izvorno mjesto %s (%s).", stavkeTekst, velicinaTekst)
}

// FormatirajBajte pretvara broj bajtova u čitljiv prikaz (KB, MB, GB).
func FormatirajBajte(bajta int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	if bajta >= gb {
		return fmt.Sprintf("%.2f GB", float64(bajta)/float64(gb))
	}
	if bajta >= mb {
		return fmt.Sprintf("%.1f MB", float64(bajta)/float64(mb))
	}
	if bajta >= kb {
		return fmt.Sprintf("%.0f KB", float64(bajta)/float64(kb))
	}
	return fmt.Sprintf("%d B", bajta)
}
