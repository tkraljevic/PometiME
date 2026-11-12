package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var uzorakJavnog = regexp.MustCompile(`Javni ključ: (\S+)`)

func TestNoviKljucIPotpis(t *testing.T) {
	mapa := t.TempDir()
	kljuc := filepath.Join(mapa, "kljucevi", "nadogradnja.key")
	var izlaz bytes.Buffer
	if err := pokreni([]string{"novi-kljuc", "-kljuc", kljuc}, &izlaz); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(kljuc)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("tajni ključ mora imati dozvole 0600: %v %v", info, err)
	}
	m := uzorakJavnog.FindStringSubmatch(izlaz.String())
	if m == nil {
		t.Fatalf("u izlazu nema javnog ključa: %q", izlaz.String())
	}
	if err := pokreni([]string{"novi-kljuc", "-kljuc", kljuc}, &izlaz); err == nil {
		t.Errorf("postojeći tajni ključ ne smije se prepisati")
	}

	arhiva := filepath.Join(mapa, "PometiME-macOS.zip")
	if err := os.WriteFile(arhiva, []byte("sadržaj arhive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := pokreni([]string{"potpisi", "-kljuc", kljuc, "-javni", m[1], arhiva}, &izlaz); err != nil {
		t.Fatal(err)
	}
	tekst, err := os.ReadFile(arhiva + ".sig")
	if err != nil {
		t.Fatal(err)
	}
	potpis, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(string(tekst))) // neispravan zapis pada na provjeri ispod
	javni, _ := base64.StdEncoding.DecodeString(m[1])                              // ključ je upravo ispisao alat
	if !ed25519.Verify(javni, []byte("sadržaj arhive"), potpis) {
		t.Errorf("potpis se ne može provjeriti javnim ključem")
	}
}

func TestPotpisOdbijaKrivKljucIArgumente(t *testing.T) {
	mapa := t.TempDir()
	kljuc := filepath.Join(mapa, "nadogradnja.key")
	var izlaz bytes.Buffer
	if err := pokreni([]string{"novi-kljuc", "-kljuc", kljuc}, &izlaz); err != nil {
		t.Fatal(err)
	}
	arhiva := filepath.Join(mapa, "a.zip")
	if err := os.WriteFile(arhiva, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	drugi, _, _ := ed25519.GenerateKey(nil) // nil koristi crypto/rand
	slucajevi := map[string][]string{
		"bez naredbe":       nil,
		"nepoznata naredba": {"nesto"},
		"novi bez putanje":  {"novi-kljuc"},
		"potpis bez arhive": {"potpisi", "-kljuc", kljuc, "-javni", "x"},
		"drugi javni ključ": {"potpisi", "-kljuc", kljuc, "-javni", base64.StdEncoding.EncodeToString(drugi), arhiva},
		"nema ključa":       {"potpisi", "-kljuc", filepath.Join(mapa, "nema"), "-javni", "x", arhiva},
		"nema arhive":       {"potpisi", "-kljuc", kljuc, "-javni", uzorakJavnog.FindStringSubmatch(izlaz.String())[1], filepath.Join(mapa, "nema.zip")},
	}
	for naziv, argumenti := range slucajevi {
		if err := pokreni(argumenti, &izlaz); err == nil {
			t.Errorf("%s: očekivana greška", naziv)
		}
	}
	if err := os.WriteFile(kljuc, []byte("nije ključ"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ucitajTajniKljuc(kljuc); err == nil {
		t.Errorf("očekivana greška za neispravan tajni ključ")
	}
}
