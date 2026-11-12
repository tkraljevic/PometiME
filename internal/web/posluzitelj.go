// Package web poslužuje grafičko sučelje PometiME i JSON API za skeniranje i čišćenje.
package web

import (
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"mime"
	"net/http"
	"regexp"
	"strings"

	"pometime/internal/cistac"
)

//go:embed sucelje.html
var ugradenoSucelje []byte

// ZaglavljeKljuca nosi ključ sesije u svakom zahtjevu prema API-ju.
const ZaglavljeKljuca = "X-PometiME-Kljuc"

// najveceTijeloZahtjeva ograničava JSON zahtjev; i dugi popis putanja stane u 1 MB.
const najveceTijeloZahtjeva = 1 << 20

var uzorakSkripte = regexp.MustCompile(`(?s)<script>(.*?)</script>`)

// politikaSadrzaja dopušta samo ugrađenu skriptu sučelja. Ako se u stranicu
// ipak ubaci HTML, preglednik ne pokreće njegov kod.
var politikaSadrzaja = izracunajPolitikuSadrzaja(ugradenoSucelje)

// Zastita određuje po čemu poslužitelj prepoznaje zahtjeve vlastitog prozora.
type Zastita struct {
	// Kljuc je tajni ključ sesije koji sučelje šalje u zaglavlju X-PometiME-Kljuc.
	Kljuc string
	// Hostovi su dopuštene vrijednosti zaglavlja Host, npr. "127.0.0.1:19845".
	// Zahtjev preko tuđeg imena (DNS rebinding) ne prolazi.
	Hostovi []string
}

// Posluzitelj povezuje HTTP rute s postavkama čistača diska.
type Posluzitelj struct {
	postavke   cistac.Postavke
	zastita    Zastita
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

type odgovorGreske struct {
	Greska string `json:"error"`
}

// NoviPosluzitelj stvara HTTP poslužitelj s registriranim rutama sučelja i API-ja.
// API odgovara samo na zahtjeve koji nose ključ sesije i stižu na dopušteni Host.
func NoviPosluzitelj(p cistac.Postavke, z Zastita) *Posluzitelj {
	mux := http.NewServeMux()
	pos := &Posluzitelj{
		postavke:   p,
		zastita:    z,
		usmjerivac: mux,
	}
	mux.HandleFunc("GET /", pos.posluziSucelje)
	mux.HandleFunc("GET /api/skeniraj", pos.obradiSkeniraj)
	mux.HandleFunc("GET /api/aplikacije", pos.obradiAplikacije)
	mux.HandleFunc("GET /api/pokretaci", pos.obradiPokretace)
	mux.HandleFunc("GET /api/snimke", pos.obradiSnimke)
	mux.HandleFunc("GET /api/aerials", pos.obradiAerials)
	mux.HandleFunc("GET /api/stablo", pos.obradiStablo)
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
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if status, poruka := p.provjeriZahtjev(r); status != 0 {
		posluziJSON(w, status, odgovorGreske{Greska: poruka})
		return
	}
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, najveceTijeloZahtjeva)
	}
	p.usmjerivac.ServeHTTP(w, r)
}

// provjeriZahtjev vraća status i poruku odbijanja, ili 0 kad zahtjev smije proći.
// Sučelje (GET /) ne sadrži ništa tajno, pa za njega vrijedi samo provjera Hosta.
func (p *Posluzitelj) provjeriZahtjev(r *http.Request) (int, string) {
	if !p.jeDopustenHost(r.Host) {
		return http.StatusForbidden, "Zahtjev nije poslan na adresu ove aplikacije."
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		return 0, ""
	}
	if !p.jeIspravanKljuc(r.Header.Get(ZaglavljeKljuca)) {
		return http.StatusForbidden, "Zahtjev nije poslan iz prozora PometiME. Zatvorite PometiME i otvorite ga ponovno."
	}
	if r.Method == http.MethodPost && !jeJSONZahtjev(r) {
		return http.StatusUnsupportedMediaType, "Zahtjev mora biti poslan kao JSON."
	}
	return 0, ""
}

func (p *Posluzitelj) jeDopustenHost(host string) bool {
	for _, dopusten := range p.zastita.Hostovi {
		if strings.EqualFold(host, dopusten) {
			return true
		}
	}
	return false
}

// jeIspravanKljuc odbija i prazan ključ, da poslužitelj bez ključa ne bi bio otvoren svima.
func (p *Posluzitelj) jeIspravanKljuc(poslani string) bool {
	ocekivani := p.zastita.Kljuc
	return ocekivani != "" && subtle.ConstantTimeCompare([]byte(poslani), []byte(ocekivani)) == 1
}

// jeJSONZahtjev traži Content-Type application/json. Takav zahtjev preglednik
// s druge stranice ne smije poslati bez CORS dozvole, koju poslužitelj ne daje.
func jeJSONZahtjev(r *http.Request) bool {
	vrsta, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && vrsta == "application/json"
}

// izracunajPolitikuSadrzaja dopušta samo ugrađene skripte, prepoznate po SHA-256 sažetku.
func izracunajPolitikuSadrzaja(html []byte) string {
	var skripte []string
	for _, m := range uzorakSkripte.FindAllSubmatch(html, -1) {
		zbroj := sha256.Sum256(m[1])
		skripte = append(skripte, "'sha256-"+base64.StdEncoding.EncodeToString(zbroj[:])+"'")
	}
	if len(skripte) == 0 {
		skripte = []string{"'none'"}
	}
	return "default-src 'none'; script-src " + strings.Join(skripte, " ") +
		"; style-src 'unsafe-inline'; img-src 'self' pometime-icon:; connect-src 'self'" +
		"; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
}

func (p *Posluzitelj) posluziSucelje(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		posluziJSON(w, http.StatusNotFound, odgovorGreske{Greska: "Tražena stranica nije pronađena."})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", politikaSadrzaja)
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

func (p *Posluzitelj) obradiStablo(w http.ResponseWriter, r *http.Request) {
	putanja := r.URL.Query().Get("putanja")
	if putanja == "" {
		posluziJSON(w, http.StatusBadRequest, odgovorGreske{Greska: "Parametar putanja je obavezan."})
		return
	}
	izvj, err := cistac.PregledajStablo(p.postavke, putanja)
	if err != nil {
		posluziJSON(w, http.StatusBadRequest, odgovorGreske{Greska: err.Error()})
		return
	}
	posluziJSON(w, http.StatusOK, izvj)
}

func (p *Posluzitelj) obradiProvjeruNadogradnje(w http.ResponseWriter, _ *http.Request) {
	info, err := cistac.ProvjeriNadogradnju()
	if err != nil {
		posluziJSON(w, http.StatusBadGateway, odgovorGreske{Greska: err.Error()})
		return
	}
	posluziJSON(w, http.StatusOK, info)
}

// obradiPrimjenuNadogradnje ne prima adresu arhive: poslužitelj je sam dohvaća
// s GitHuba i instalira je samo ako je potpis ispravan.
func (p *Posluzitelj) obradiPrimjenuNadogradnje(w http.ResponseWriter, _ *http.Request) {
	if err := cistac.PrimijeniNadogradnju(); err != nil {
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
