package cistac

import (
	"io/fs"
	"path/filepath"
	"syscall"
)

// DohvatiStanjeDiska čita zauzeće diska za zadanu putanju pomoću sustavskog poziva Statfs.
func DohvatiStanjeDiska(putanja string) StanjeDiska {
	cilj := odaberiPutanjuZaStatfs(putanja)
	var stat syscall.Statfs_t
	if err := syscall.Statfs(cilj, &stat); err != nil {
		return StanjeDiska{TockaMontaze: cilj}
	}
	return izracunajStanjeIzStatfs(cilj, stat.Blocks, stat.Bavail, uint64(stat.Bsize))
}

func odaberiPutanjuZaStatfs(putanja string) string {
	if putanja == "" {
		return "/"
	}
	return filepath.Clean(putanja)
}

func izracunajStanjeIzStatfs(cilj string, blokova, dostupno, velicinaBloka uint64) StanjeDiska {
	ukupno := int64(blokova * velicinaBloka)
	slobodno := int64(dostupno * velicinaBloka)
	zauzeto := ukupno - slobodno
	var postotak float64
	if ukupno > 0 {
		postotak = (float64(zauzeto) / float64(ukupno)) * 100.0
	}
	return StanjeDiska{
		UkupnoBajta:    ukupno,
		ZauzetoBajta:   zauzeto,
		SlobodnoBajta:  slobodno,
		PostotakZauzet: postotak,
		TockaMontaze:   cilj,
	}
}

// IzracunajVelicinu zbraja veličinu svih običnih datoteka unutar zadane putanje.
func IzracunajVelicinu(putanja string) int64 {
	var ukupno int64
	_ = filepath.WalkDir(putanja, dodajVelicinuUlaza(&ukupno)) // nedostupne podmape se preskaču pri zbrajanju
	return ukupno
}

func dodajVelicinuUlaza(ukupno *int64) fs.WalkDirFunc {
	return func(_ string, d fs.DirEntry, err error) error {
		*ukupno += velicinaUlaza(d, err)
		return nil
	}
}

func velicinaUlaza(d fs.DirEntry, err error) int64 {
	if err != nil || d == nil || !d.Type().IsRegular() {
		return 0
	}
	info, infoErr := d.Info()
	if infoErr != nil {
		return 0
	}
	return info.Size()
}
