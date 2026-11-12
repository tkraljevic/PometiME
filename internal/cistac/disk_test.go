package cistac

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// zauzeto zbraja zauzete blokove datoteka izravno iz stat, neovisno o kodu koji se testira.
func zauzeto(t *testing.T, putanje ...string) int64 {
	t.Helper()
	var ukupno int64
	for _, put := range putanje {
		var st syscall.Stat_t
		if err := syscall.Lstat(put, &st); err != nil {
			t.Fatal(err)
		}
		ukupno += st.Blocks * 512
	}
	return ukupno
}

func TestRijetkaDatotekaBrojiZauzeteBlokove(t *testing.T) {
	mapa := t.TempDir()
	rijetka := filepath.Join(mapa, "Docker.raw")
	f, err := os.Create(rijetka)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(64 << 20); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("podaci na početku")); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if v := IzracunajVelicinu(mapa); v >= 1<<20 || v != zauzeto(t, rijetka) {
		t.Errorf("rijetka datoteka od 64 MB s malo podataka mora zauzimati samo svoje blokove, dobiveno %d B", v)
	}
}

func TestCvrstaPoveznicaBrojiSeJednom(t *testing.T) {
	mapa := t.TempDir()
	izvor := filepath.Join(mapa, "store", "paket.tgz")
	napraviDatoteku(t, izvor, 100000)
	if err := os.MkdirAll(filepath.Join(mapa, "projekt", "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(izvor, filepath.Join(mapa, "projekt", "node_modules", "paket.tgz")); err != nil {
		t.Fatal(err)
	}
	if v := IzracunajVelicinu(mapa); v != zauzeto(t, izvor) {
		t.Errorf("datoteka s dvije poveznice mora se brojiti jednom (%d), dobiveno %d", zauzeto(t, izvor), v)
	}
}

// infoBezStat je FileInfo bez podataka iz stat, kakav vraćaju neki datotečni sustavi.
type infoBezStat struct{}

func (infoBezStat) Name() string       { return "x" }
func (infoBezStat) Size() int64        { return 1234 }
func (infoBezStat) Mode() fs.FileMode  { return 0 }
func (infoBezStat) ModTime() time.Time { return time.Time{} }
func (infoBezStat) IsDir() bool        { return false }
func (infoBezStat) Sys() any           { return nil }

func TestVelicinaBezPodatakaStat(t *testing.T) {
	if v := velicinaDatoteke(infoBezStat{}); v != 1234 {
		t.Errorf("bez podataka iz stat vrijedi prividna duljina, dobiveno %d", v)
	}
}

// ulazKojiNestaje je stavka koja je obrisana između čitanja mape i dohvata podataka.
type ulazKojiNestaje struct{}

func (ulazKojiNestaje) Name() string      { return "nestala" }
func (ulazKojiNestaje) IsDir() bool       { return false }
func (ulazKojiNestaje) Type() fs.FileMode { return 0 }
func (ulazKojiNestaje) Info() (fs.FileInfo, error) {
	return nil, errors.New("datoteka više ne postoji")
}

func TestVelicinaUlazaKojiNestaje(t *testing.T) {
	z := &zbrajalo{}
	if v := z.velicinaUlaza(ulazKojiNestaje{}, nil); v != 0 {
		t.Errorf("nestala datoteka ne zauzima prostor, dobiveno %d", v)
	}
	if v := z.velicinaUlaza(nil, errors.New("nedostupno")); v != 0 {
		t.Errorf("nedostupna stavka ne zauzima prostor, dobiveno %d", v)
	}
}
