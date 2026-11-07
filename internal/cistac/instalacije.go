package cistac

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SkenirajInstalacije pronalazi preuzete instalacijske pakete (.dmg, .pkg, .iso, .xip) u mapi Downloads.
func SkenirajInstalacije(p Postavke) []StavkaCiscenja {
	mapa := filepath.Join(p.KorijenKorisnika, "Downloads")
	kandidati := prikupiKandidateInstalacija(mapa)
	return izmjeriKandidate(kandidati, p.NajmanjaVelicinaInstalacija)
}

func prikupiKandidateInstalacija(mapaDownloads string) []kandidatCiscenja {
	ulazi, err := os.ReadDir(mapaDownloads)
	if err != nil {
		return nil
	}
	var kandidati []kandidatCiscenja
	for _, u := range ulazi {
		if kan, ok := obradiUlazInstalacije(mapaDownloads, u); ok {
			kandidati = append(kandidati, kan)
		}
	}
	return kandidati
}

func obradiUlazInstalacije(mapa string, u os.DirEntry) (kandidatCiscenja, bool) {
	if u.IsDir() || strings.HasPrefix(u.Name(), ".") || !jeInstalacijskaDatoteka(u.Name()) {
		return kandidatCiscenja{}, false
	}
	info, err := u.Info()
	if err != nil {
		return kandidatCiscenja{}, false
	}
	return stvoriKandidataInstalacije(mapa, u.Name(), info), true
}

func jeInstalacijskaDatoteka(ime string) bool {
	ext := strings.ToLower(filepath.Ext(ime))
	return ext == ".dmg" || ext == ".pkg" || ext == ".iso" || ext == ".xip"
}

func stvoriKandidataInstalacije(mapa, ime string, info os.FileInfo) kandidatCiscenja {
	dani := int(time.Since(info.ModTime()).Hours() / 24)
	opis, preporuceno := opisInstalacije(ime, dani)
	return kandidatCiscenja{
		putanja:     filepath.Join(mapa, ime),
		naziv:       "Instalacijski paket: " + ime,
		kategorija:  "installers",
		opis:        opis,
		preporuceno: preporuceno,
	}
}

func opisInstalacije(ime string, dani int) (string, bool) {
	ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(ime), "."))
	if dani <= 0 {
		return fmt.Sprintf("Nedavno preuzeti instalacijski paket (%s), preuzet danas.", ext), false
	}
	daniTekst := UzBrojHR(dani, "dan", "dana", "dana")
	if dani < 7 {
		return fmt.Sprintf("Nedavno preuzeti instalacijski paket (%s), preuzet prije %s.", ext, daniTekst), false
	}
	return fmt.Sprintf("Instalacijski paket (%s) preuzet prije %s. Preporučeno za uklanjanje ako je aplikacija instalirana.", ext, daniTekst), true
}
