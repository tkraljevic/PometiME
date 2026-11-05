package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestPokreniIIspisiURL(t *testing.T) {
	ctx, otkazi := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer otkazi()

	var izlaz bytes.Buffer
	err := pokreni(ctx, []string{"-adresa=127.0.0.1:0", "-korijen=" + t.TempDir()}, &izlaz)
	if err != nil {
		t.Fatalf("neočekivana greška pri pokretanju: %v", err)
	}
	if !strings.Contains(izlaz.String(), "POMETIME_URL=http://127.0.0.1:") {
		t.Errorf("očekivan POMETIME_URL u izlazu, dobiveno %q", izlaz.String())
	}
}

func TestPokreniGreskeArgumenataIPrikljucka(t *testing.T) {
	var izlaz bytes.Buffer
	if err := pokreni(context.Background(), []string{"-nepoznata-zastavica"}, &izlaz); err == nil {
		t.Errorf("očekivana greška za nepoznatu zastavicu")
	}

	if err := pokreni(context.Background(), []string{"-adresa=neispravna:adresa:xyz"}, &izlaz); err == nil {
		t.Errorf("očekivana greška za neispravnu mrežnu adresu")
	}

	dom, err := odrediKorijenKorisnika("")
	if err != nil || dom == "" {
		t.Errorf("očekivana valjana korisnička mapa, dobiveno %q (%v)", dom, err)
	}
}
