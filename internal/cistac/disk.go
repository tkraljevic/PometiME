package cistac

import (
	"io/fs"
	"path/filepath"
	"syscall"
)

// velicinaBloka je jedinica u kojoj stat vraća broj zauzetih blokova (st_blocks).
const velicinaBloka = 512

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

// IzracunajVelicinu zbraja prostor koji datoteke unutar putanje stvarno zauzimaju na disku.
func IzracunajVelicinu(putanja string) int64 {
	var ukupno int64
	_ = filepath.WalkDir(putanja, dodajVelicinuUlaza(&ukupno, &zbrajalo{})) // nedostupne podmape se preskaču pri zbrajanju
	return ukupno
}

func dodajVelicinuUlaza(ukupno *int64, z *zbrajalo) fs.WalkDirFunc {
	return func(_ string, d fs.DirEntry, err error) error {
		*ukupno += z.velicinaUlaza(d, err)
		return nil
	}
}

// inode jednoznačno određuje datoteku na računalu, bez obzira na broj poveznica.
type inode struct {
	uredaj uint64
	broj   uint64
}

// zbrajalo broji prostor na disku tijekom jednog mjerenja. Datoteku s više
// čvrstih poveznica (npr. pnpm store i node_modules) broji samo jednom.
type zbrajalo struct {
	vidjene map[inode]bool
}

func (z *zbrajalo) velicinaUlaza(d fs.DirEntry, err error) int64 {
	if err != nil || d == nil || !d.Type().IsRegular() {
		return 0
	}
	info, infoErr := d.Info()
	if infoErr != nil {
		return 0
	}
	return z.velicina(info)
}

// velicina vraća zauzete blokove, a ne prividnu duljinu: rijetka datoteka
// (Docker.raw) i datoteka iz oblaka koja nije preuzeta zauzimaju manje od duljine.
func (z *zbrajalo) velicina(info fs.FileInfo) int64 {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return info.Size()
	}
	if st.Nlink > 1 {
		kljuc := inode{uredaj: uint64(st.Dev), broj: st.Ino}
		if z.vidjene[kljuc] {
			return 0
		}
		if z.vidjene == nil {
			z.vidjene = map[inode]bool{}
		}
		z.vidjene[kljuc] = true
	}
	return st.Blocks * velicinaBloka
}

// velicinaUlaza mjeri jednu stavku, bez pamćenja poveznica.
func velicinaUlaza(d fs.DirEntry, err error) int64 {
	return (&zbrajalo{}).velicinaUlaza(d, err)
}

// velicinaDatoteke mjeri jednu datoteku, bez pamćenja poveznica.
func velicinaDatoteke(info fs.FileInfo) int64 {
	return (&zbrajalo{}).velicina(info)
}
