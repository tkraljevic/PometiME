package cistac

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestProvjeriDozvole(t *testing.T) {
	dom := t.TempDir()
	p := ZadanePostavke(dom)
	mape := zasticeneMapeSkeniranja(dom)
	if len(ProvjeriDozvole(p)) != 0 {
		t.Errorf("mape koje ne postoje nisu problem s dozvolom")
	}
	if err := os.MkdirAll(mape[1].Putanja, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mape[0].Putanja, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(mape[0].Putanja, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(mape[0].Putanja, 0o755) // TempDir mora se moći očistiti
	})
	nedostupne := ProvjeriDozvole(p)
	if len(nedostupne) != 1 || nedostupne[0].Naziv != "iOS sigurnosne kopije" {
		t.Errorf("očekivana jedna nedostupna mapa (iOS kopije), dobiveno %+v", nedostupne)
	}
	if izvj := SkenirajSve(p); len(izvj.NedostupneMape) != 1 {
		t.Errorf("skeniranje mora javiti nedostupnu mapu, dobiveno %+v", izvj.NedostupneMape)
	}
}

func TestOtvoriPostavkeDozvole(t *testing.T) {
	var pozvano []string
	vrati := PostaviPokretacNaredbiZaTest(func(name string, args ...string) error {
		pozvano = append([]string{name}, args...)
		return nil
	})
	defer vrati()
	if err := OtvoriPostavkeDozvole(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(pozvano, " ") != "open "+adresaPostavkiDozvole {
		t.Errorf("neočekivana naredba: %v", pozvano)
	}
	_ = PostaviPokretacNaredbiZaTest(func(string, ...string) error { return errors.New("open nije uspio") }) // vraća ga odgođeni vrati()
	if err := OtvoriPostavkeDozvole(); err == nil {
		t.Errorf("očekivana greška kad se postavke ne otvore")
	}
	if jeOdbijenPristup(nil) || !jeOdbijenPristup(os.ErrPermission) {
		t.Errorf("neispravno prepoznavanje odbijenog pristupa")
	}
}
