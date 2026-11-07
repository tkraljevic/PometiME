package cistac

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var korijenSustavskeKnjiznice = "/Library"

// SkenirajZracnePozadine pretražuje preuzete 4K videozapise za čuvare zaslona i pozadine.
func SkenirajZracnePozadine(p Postavke) InformacijeZracnihPozadina {
	mapaCustomer := filepath.Join(korijenSustavskeKnjiznice, "Application Support", "com.apple.idleassetsd", "Customer")
	info := InformacijeZracnihPozadina{
		Putanja: mapaCustomer,
	}

	if st, err := os.Stat(mapaCustomer); err == nil && st.IsDir() {
		info.PostojiMapa = true
		ukupno, broj := prebrojiVideoZapise(mapaCustomer)
		info.UkupnoBajta += ukupno
		info.BrojVidea += broj
	}

	mapaUserCaches := filepath.Join(p.KorijenKorisnika, "Library", "Containers", "com.apple.wallpaper.agent", "Data", "Library", "Caches", "com.apple.wallpaper.caches")
	if st, err := os.Stat(mapaUserCaches); err == nil && st.IsDir() {
		info.UkupnoBajta += IzracunajVelicinu(mapaUserCaches)
	}

	return info
}

func prebrojiVideoZapise(korijen string) (int64, int) {
	var ukupno int64
	var broj int
	_ = filepath.WalkDir(korijen, obradiVideoUlaz(&ukupno, &broj)) // nedostupne podmape se preskaču pri prebrojavanju
	return ukupno, broj
}

func obradiVideoUlaz(ukupno *int64, broj *int) fs.WalkDirFunc {
	return func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".mov") {
			if info, errStat := d.Info(); errStat == nil {
				*ukupno += info.Size()
				*broj++
			}
		}
		return nil
	}
}

// OcistiZracnePozadine uklanja preuzete .mov 4K videozapise i međuspremnik pozadina.
func OcistiZracnePozadine(p Postavke) (string, error) {
	info := SkenirajZracnePozadine(p)
	if info.UkupnoBajta == 0 && info.BrojVidea == 0 {
		return "Nema preuzetih 4K videozapisa ni predmemorije pozadina za čišćenje.", nil
	}

	// 1. Očisti korisnički međuspremnik pozadina
	mapaUserCaches := filepath.Join(p.KorijenKorisnika, "Library", "Containers", "com.apple.wallpaper.agent", "Data", "Library", "Caches", "com.apple.wallpaper.caches")
	_ = os.RemoveAll(mapaUserCaches) // uklanjanje korisničke predmemorije bez prekida

	// 2. Očisti .mov videozapise u Customer mapi
	mapaCustomer := filepath.Join(korijenSustavskeKnjiznice, "Application Support", "com.apple.idleassetsd", "Customer")
	if info.PostojiMapa && info.BrojVidea > 0 {
		if err := obrisiMovDatoteke(mapaCustomer); err != nil {
			skripta := fmt.Sprintf("do shell script %q with administrator privileges", `find '/Library/Application Support/com.apple.idleassetsd/Customer' -type f -name '*.mov' -delete`)
			if errAuth := pokreniNaredbu("osascript", "-e", skripta); errAuth != nil {
				return "", fmt.Errorf("za brisanje 4K videozapisa iz /Library potrebna je administratorska potvrda: %w", errAuth)
			}
		}
	}

	return "Preuzete 4K zračne pozadine i predmemorija su uspješno očišćeni.", nil
}

func obrisiMovDatoteke(korijen string) error {
	var prvaGreska error
	_ = filepath.WalkDir(korijen, ukloniMovUlaz(&prvaGreska)) // pregled i brisanje pojedinačnih videozapisa
	return prvaGreska
}

func ukloniMovUlaz(prvaGreska *error) fs.WalkDirFunc {
	return func(put string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".mov") {
			if errRem := os.Remove(put); errRem != nil && *prvaGreska == nil {
				*prvaGreska = errRem
			}
		}
		return nil
	}
}
