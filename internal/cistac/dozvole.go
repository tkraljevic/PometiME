package cistac

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// adresaPostavkiDozvole otvara Postavke sustava na popisu Potpuni pristup disku.
const adresaPostavkiDozvole = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"

// NedostupnaMapa je mapa koju macOS ne da pročitati bez Potpunog pristupa disku.
// Bez ove obavijesti takva mapa u sučelju izgleda kao da je prazna.
type NedostupnaMapa struct {
	Naziv   string `json:"name"`
	Putanja string `json:"path"`
}

func zasticeneMapeSkeniranja(dom string) []NedostupnaMapa {
	return []NedostupnaMapa{
		{Naziv: "iOS sigurnosne kopije", Putanja: filepath.Join(dom, "Library", "Application Support", "MobileSync", "Backup")},
		{Naziv: "Privici iz Maila", Putanja: filepath.Join(dom, "Library", "Containers", "com.apple.mail", "Data", "Library", "Mail Downloads")},
	}
}

// ProvjeriDozvole vraća mape koje skeniranje treba, a macOS ih ne da pročitati.
// Mapa koja ne postoji nije problem: korisnik takvih podataka nema.
func ProvjeriDozvole(p Postavke) []NedostupnaMapa {
	nedostupne := []NedostupnaMapa{}
	for _, mapa := range zasticeneMapeSkeniranja(p.KorijenKorisnika) {
		if _, err := os.ReadDir(mapa.Putanja); jeOdbijenPristup(err) {
			nedostupne = append(nedostupne, mapa)
		}
	}
	return nedostupne
}

// jeOdbijenPristup prepoznaje i EPERM (zaštita privatnosti) i EACCES (dozvole datoteke).
func jeOdbijenPristup(err error) bool {
	return errors.Is(err, fs.ErrPermission)
}

// OtvoriPostavkeDozvole otvara Postavke sustava na mjestu gdje se PometiME-u daje pristup.
func OtvoriPostavkeDozvole() error {
	if err := pokreniNaredbu("open", adresaPostavkiDozvole); err != nil {
		return fmt.Errorf("postavke sustava nisu otvorene: %w", err)
	}
	return nil
}
