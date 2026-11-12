package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"pometime/internal/cistac"
	"pometime/internal/web"
)

func TestPokreniIIspisiURL(t *testing.T) {
	ctx, otkazi := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer otkazi()

	var izlaz bytes.Buffer
	err := pokreni(ctx, []string{"-adresa=127.0.0.1:0", "-korijen=" + t.TempDir()}, &izlaz)
	if err != nil {
		t.Fatalf("neočekivana greška pri pokretanju: %v", err)
	}
	if !strings.Contains(izlaz.String(), "POMETIME_URL=http://127.0.0.1:") || !strings.Contains(izlaz.String(), "/?k=") {
		t.Errorf("očekivan POMETIME_URL s ključem sesije u izlazu, dobiveno %q", izlaz.String())
	}
}

// Pravi poslužitelj odbija API zahtjev bez ključa, a prima onaj s ključem iz ispisane adrese.
func TestKljucSesijeIzAdrese(t *testing.T) {
	slusac, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, otkazi := context.WithCancel(context.Background())
	citac, pisac := io.Pipe()
	gotovo := make(chan error, 1)
	go func() { gotovo <- posluziDoPrekida(ctx, slusac, cistac.ZadanePostavke(t.TempDir()), pisac) }()
	klijent := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	defer func() {
		otkazi()
		<-gotovo
	}()

	redak, err := bufio.NewReader(citac).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	_, kljuc, _ := strings.Cut(strings.TrimSpace(redak), "?k=")
	if kljuc == "" {
		t.Fatalf("ispisana adresa nema ključ sesije: %q", redak)
	}
	adresa := "http://" + slusac.Addr().String() + "/api/kos"
	for _, s := range []struct {
		kljuc  string
		status int
	}{{"", http.StatusForbidden}, {"krivi", http.StatusForbidden}, {kljuc, http.StatusOK}} {
		zah, _ := http.NewRequest(http.MethodGet, adresa, nil) // adresa je ispravna
		zah.Header.Set(web.ZaglavljeKljuca, s.kljuc)
		odg, err := klijent.Do(zah)
		if err != nil {
			t.Fatal(err)
		}
		_ = odg.Body.Close() // tijelo odgovora nije potrebno
		if odg.StatusCode != s.status {
			t.Errorf("ključ %q: očekivan status %d, dobiveno %d", s.kljuc, s.status, odg.StatusCode)
		}
	}
}

func TestDopusteniHostovi(t *testing.T) {
	hostovi := dopusteniHostovi("127.0.0.1:19845")
	for _, h := range []string{"127.0.0.1:19845", "localhost:19845", "[::1]:19845"} {
		if !slices.Contains(hostovi, h) {
			t.Errorf("nedostaje dopušteni host %s u %v", h, hostovi)
		}
	}
	if slices.Contains(hostovi, "napadac.example:19845") {
		t.Errorf("tuđe ime ne smije biti dopušteno")
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
