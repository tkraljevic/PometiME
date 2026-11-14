package cistac

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// zapisiPokretac sprema LaunchAgent koji pokreće zadani program.
func zapisiPokretac(t *testing.T, dom, ime, program string) {
	t.Helper()
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array><string>%s</string><string>--posluzi</string></array>
</dict></plist>`, ime, program)
	putanja := filepath.Join(dom, "Library", "LaunchAgents", ime+".plist")
	if err := os.MkdirAll(filepath.Dir(putanja), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(putanja, []byte(plist), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Pokretač s programom zadanim samo imenom ili preko ~/ radi, pa ga PometiME
// ne smije ponuditi za uklanjanje kao zaostali.
func TestStatusPokretacaZaRelativnePutanje(t *testing.T) {
	dom := t.TempDir()
	napraviDatoteku(t, filepath.Join(dom, "bin", "moj-alat"), 10)
	ocekivano := map[string]string{
		"hr.ime-u-sustavu":     "aktivan",   // sh je u /bin, gdje ga launchd i traži
		"hr.ime-nepoznato":     "nepoznato", // ime koje PometiME ne pronalazi; možda je u PATH-u pokretača
		"hr.tilda-postoji":     "aktivan",
		"hr.tilda-nema":        "siroce",
		"hr.relativna-putanja": "nepoznato", // ovisi o radnoj mapi pokretača
		"hr.apsolutna-nema":    "siroce",
	}
	zapisiPokretac(t, dom, "hr.ime-u-sustavu", "sh")
	zapisiPokretac(t, dom, "hr.ime-nepoznato", "pometime-nepostojeci-alat")
	zapisiPokretac(t, dom, "hr.tilda-postoji", "~/bin/moj-alat")
	zapisiPokretac(t, dom, "hr.tilda-nema", "~/bin/uklonjeni-alat")
	zapisiPokretac(t, dom, "hr.relativna-putanja", "bin/moj-alat")
	zapisiPokretac(t, dom, "hr.apsolutna-nema", filepath.Join(dom, "uklonjeno", "alat"))
	// Mapa koju PometiME ne smije pregledati: program možda postoji, samo se ne vidi.
	zakljucano := filepath.Join(dom, "zakljucano")
	napraviDatoteku(t, filepath.Join(zakljucano, "alat"), 10)
	if err := os.Chmod(zakljucano, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chmod(zakljucano, 0o755) // TempDir mora se moći očistiti
	}()
	zapisiPokretac(t, dom, "hr.zakljucana-mapa", filepath.Join(zakljucano, "alat"))
	ocekivano["hr.zakljucana-mapa"] = "nepoznato"

	pokretaci := SkenirajPokretace(Postavke{KorijenKorisnika: dom})
	if len(pokretaci) != len(ocekivano) {
		t.Fatalf("očekivano %d pokretača, dobiveno %d", len(ocekivano), len(pokretaci))
	}
	for _, p := range pokretaci {
		ime := p.Naziv[:len(p.Naziv)-len(".plist")]
		if p.Status != ocekivano[ime] {
			t.Errorf("%s (%s): status %q, očekivano %q", ime, p.PutanjaIzvrsne, p.Status, ocekivano[ime])
		}
		if p.PostojiIzvrsna != (p.Status == "aktivan") {
			t.Errorf("%s: program postoji samo kod aktivnog pokretača, dobiveno %v", ime, p.PostojiIzvrsna)
		}
	}
}
