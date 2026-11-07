// Package web poslužuje grafičko sučelje PometiME i JSON API za skeniranje i čišćenje.
package web

import (
	_ "embed"
	"encoding/json"
	"net/http"

	"pometime/internal/cistac"
)

//go:embed sucelje.html
var ugradenoSucelje []byte

// Posluzitelj povezuje HTTP rute s postavkama čistača diska.
type Posluzitelj struct {
	postavke cistac.Postavke
	usmjerivac *http.ServeMux
}

type zahtjevCiscenja struct {
	Putanje []string `json:"paths"`
}

type zahtjevDeinstalacije struct {
	PutanjaAplikacije string   `json:"app_path"`
	PovezanePutanje   []string `json:"related_paths"`
}

type zahtjevOptimizacije struct {
	Akcija string `json:"action"`
}

type zahtjevPrimjeneNadogradnje struct {
	DownloadURL string `json:"download_url"`
}

type odgovorGreske struct {
	Greska string `json:"error"`
}

// NoviPosluzitelj stvara HTTP poslužitelj s registriranim rutama sučelja i API-ja.
func NoviPosluzitelj(p cistac.Postavke) *Posluzitelj {
	mux := http.NewServeMux()
	pos := &Posluzitelj{
		postavke:   p,
		usmjerivac: mux,
	}
	mux.HandleFunc("GET /", pos.posluziSucelje)
	mux.HandleFunc("GET /api/skeniraj", pos.obradiSkeniraj)
	mux.HandleFunc("GET /api/aplikacije", pos.obradiAplikacije)
	mux.HandleFunc("GET /api/pokretaci", pos.obradiPokretace)
	mux.HandleFunc("GET /api/snimke", pos.obradiSnimke)
	mux.HandleFunc("GET /api/aerials", pos.obradiAerials)
	mux.HandleFunc("GET /api/nadogradnja", pos.obradiProvjeruNadogradnje)
	mux.HandleFunc("GET /api/kos", pos.obradiKos)
	mux.HandleFunc("POST /api/ocisti", pos.obradiOcisti)
	mux.HandleFunc("POST /api/vrati", pos.obradiVrati)
	mux.HandleFunc("POST /api/deinstaliraj", pos.obradiDeinstaliraj)
	mux.HandleFunc("POST /api/optimiziraj", pos.obradiOptimiziraj)
	mux.HandleFunc("POST /api/nadogradnja/primijeni", pos.obradiPrimjenuNadogradnje)
	return pos
}

// ServeHTTP implementira http.Handler sučelje.
func (p *Posluzitelj) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.usmjerivac.ServeHTTP(w, r)
}

func (p *Posluzitelj) posluziSucelje(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		posluziJSON(w, http.StatusNotFound, odgovorGreske{Greska: "Tražena stranica nije pronađena."})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(ugradenoSucelje) // prekid veze klijenta pri čitanju HTML-a ne zahtijeva oporavak
}

func (p *Posluzitelj) obradiSkeniraj(w http.ResponseWriter, _ *http.Request) {
	izvjestaj := cistac.SkenirajSve(p.postavke)
	posluziJSON(w, http.StatusOK, izvjestaj)
}

func (p *Posluzitelj) obradiAplikacije(w http.ResponseWriter, _ *http.Request) {
	aplikacije := cistac.SkenirajAplikacije(p.postavke)
	posluziJSON(w, http.StatusOK, aplikacije)
}

func (p *Posluzitelj) obradiPokretace(w http.ResponseWriter, _ *http.Request) {
	pokretaci := cistac.SkenirajPokretace(p.postavke)
	posluziJSON(w, http.StatusOK, pokretaci)
}

func (p *Posluzitelj) obradiKos(w http.ResponseWriter, _ *http.Request) {
	stavke := cistac.IzlistajKos(p.postavke)
	posluziJSON(w, http.StatusOK, stavke)
}

func (p *Posluzitelj) obradiOcisti(w http.ResponseWriter, r *http.Request) {
	var zah zahtjevCiscenja
	if err := json.NewDecoder(r.Body).Decode(&zah); err != nil {
		posluziJSON(w, http.StatusBadRequest, odgovorGreske{Greska: "Neispravan JSON zahtjev za čišćenje."})
		return
	}
	rezultat := cistac.OcistiPutanje(p.postavke, zah.Putanje)
	posluziJSON(w, http.StatusOK, rezultat)
}

func (p *Posluzitelj) obradiVrati(w http.ResponseWriter, r *http.Request) {
	var zah zahtjevCiscenja
	if err := json.NewDecoder(r.Body).Decode(&zah); err != nil {
		posluziJSON(w, http.StatusBadRequest, odgovorGreske{Greska: "Neispravan JSON zahtjev za povrat iz Koša."})
		return
	}
	rezultat := cistac.VratiIzKosa(p.postavke, zah.Putanje)
	posluziJSON(w, http.StatusOK, rezultat)
}

func (p *Posluzitelj) obradiDeinstaliraj(w http.ResponseWriter, r *http.Request) {
	var zah zahtjevDeinstalacije
	if err := json.NewDecoder(r.Body).Decode(&zah); err != nil {
		posluziJSON(w, http.StatusBadRequest, odgovorGreske{Greska: "Neispravan JSON zahtjev za deinstalaciju."})
		return
	}
	rezultat := cistac.DeinstalirajAplikaciju(p.postavke, zah.PutanjaAplikacije, zah.PovezanePutanje)
	posluziJSON(w, http.StatusOK, rezultat)
}

func (p *Posluzitelj) obradiOptimiziraj(w http.ResponseWriter, r *http.Request) {
	var zah zahtjevOptimizacije
	if err := json.NewDecoder(r.Body).Decode(&zah); err != nil {
		posluziJSON(w, http.StatusBadRequest, odgovorGreske{Greska: "Neispravan JSON zahtjev za optimizaciju."})
		return
	}
	poruka, err := cistac.IzvrsiOptimizaciju(p.postavke, zah.Akcija)
	if err != nil {
		posluziJSON(w, http.StatusBadRequest, odgovorGreske{Greska: err.Error()})
		return
	}
	posluziJSON(w, http.StatusOK, cistac.RezultatOptimizacije{Uspjeh: true, Poruka: poruka})
}

func (p *Posluzitelj) obradiSnimke(w http.ResponseWriter, _ *http.Request) {
	snimke := cistac.PopisLokalnihSnimaka()
	if snimke == nil {
		snimke = []string{}
	}
	posluziJSON(w, http.StatusOK, snimke)
}

func (p *Posluzitelj) obradiAerials(w http.ResponseWriter, _ *http.Request) {
	info := cistac.SkenirajZracnePozadine(p.postavke)
	posluziJSON(w, http.StatusOK, info)
}

func (p *Posluzitelj) obradiProvjeruNadogradnje(w http.ResponseWriter, _ *http.Request) {
	info, err := cistac.ProvjeriNadogradnju()
	if err != nil {
		posluziJSON(w, http.StatusBadGateway, odgovorGreske{Greska: err.Error()})
		return
	}
	posluziJSON(w, http.StatusOK, info)
}

func (p *Posluzitelj) obradiPrimjenuNadogradnje(w http.ResponseWriter, r *http.Request) {
	var zah zahtjevPrimjeneNadogradnje
	if err := json.NewDecoder(r.Body).Decode(&zah); err != nil {
		posluziJSON(w, http.StatusBadRequest, odgovorGreske{Greska: "Neispravan JSON zahtjev za nadogradnju."})
		return
	}
	if err := cistac.PrimijeniNadogradnju(zah.DownloadURL); err != nil {
		posluziJSON(w, http.StatusInternalServerError, odgovorGreske{Greska: err.Error()})
		return
	}
	posluziJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "PometiME je uspješno ažuriran na najnoviju verziju. Pokrenite aplikaciju ponovno.",
	})
}

func posluziJSON(w http.ResponseWriter, status int, podaci any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(podaci) // greška pri pisanju u mrežni tok klijenta se ne može naknadno ispraviti
}
