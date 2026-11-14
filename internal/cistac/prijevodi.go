package cistac

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// Jezici sučelja. Hrvatski je izvorni jezik teksta u kodu, a engleski se čita
// iz rječnika prijevodiEN u kojem je hrvatski tekst ključ.
const (
	JezikHrvatski = "hr"
	JezikEngleski = "en"
)

var trenutniJezik atomic.Value

func init() {
	trenutniJezik.Store(JezikHrvatski)
}

// Jezik vraća jezik na kojem poslužitelj trenutačno piše poruke.
func Jezik() string {
	jezik, _ := trenutniJezik.Load().(string) // uvijek je spremljen string
	return jezik
}

// PostaviJezik mijenja jezik poruka; poslužitelj ima samo jednog korisnika, pa je jezik zajednički.
func PostaviJezik(jezik string) error {
	if jezik != JezikHrvatski && jezik != JezikEngleski {
		return fmt.Errorf(T("nepodržan jezik: %s"), jezik)
	}
	trenutniJezik.Store(jezik)
	return nil
}

// T vraća tekst na trenutnom jeziku. Tekst bez prijevoda ostaje hrvatski.
func T(tekst string) string {
	if Jezik() != JezikEngleski {
		return tekst
	}
	if prijevod, ok := prijevodiEN[tekst]; ok {
		return prijevod
	}
	return tekst
}

// uzBrojEN vraća engleski oblik uz broj, ili false ako za riječ nema prijevoda.
func uzBrojEN(n int, jednina string) (string, bool) {
	oblici, ok := pluraliEN[jednina]
	if Jezik() != JezikEngleski || !ok {
		return "", false
	}
	if n == 1 {
		return fmt.Sprintf("%d %s", n, oblici[0]), true
	}
	return fmt.Sprintf("%d %s", n, oblici[1]), true
}

func putanjaJezika(dom string) string {
	return filepath.Join(dom, "Library", "Application Support", "PometiME", "jezik")
}

// UcitajJezik vraća jezik koji je korisnik odabrao, a bez odabira jezik sustava:
// hrvatski za hrvatski sustav, engleski za sve ostale. Bez podatka o sustavu hrvatski.
func UcitajJezik(dom, jezikSustava string) string {
	if sadrzaj, err := os.ReadFile(putanjaJezika(dom)); err == nil {
		if odabran := strings.TrimSpace(string(sadrzaj)); odabran == JezikHrvatski || odabran == JezikEngleski {
			return odabran
		}
	}
	if jezikSustava == "" || strings.HasPrefix(strings.ToLower(jezikSustava), JezikHrvatski) {
		return JezikHrvatski
	}
	return JezikEngleski
}

// SpremiJezik pamti odabir korisnika i odmah ga primjenjuje.
func SpremiJezik(dom, jezik string) error {
	if err := PostaviJezik(jezik); err != nil {
		return err
	}
	put := putanjaJezika(dom)
	if err := os.MkdirAll(filepath.Dir(put), 0o755); err != nil {
		return fmt.Errorf(T("spremanje jezika: %w"), err)
	}
	if err := os.WriteFile(put, []byte(jezik+"\n"), 0o644); err != nil {
		return fmt.Errorf(T("spremanje jezika: %w"), err)
	}
	return nil
}

// pluraliEN daje engleski oblik jednine i množine za hrvatsku riječ u jednini.
var pluraliEN = map[string][2]string{
	"stavka": {"item", "items"},
	"dan":    {"day", "days"},
}

// prijevodiEN su engleski prijevodi tekstova koje korisnik vidi. Ključ je
// hrvatski tekst točno kako stoji u kodu; test provjerava da svaki T(...) ima prijevod.
var prijevodiEN = map[string]string{
	// Jezik
	"nepodržan jezik: %s":  "unsupported language: %s",
	"spremanje jezika: %w": "saving the language: %w",

	// Aplikacije, dozvole, emulatori, instaleri
	"Zaostali podaci u ~/Library/%s bez pronađene aplikacije.": "Leftover data in ~/Library/%s with no matching app.",
	"iOS sigurnosne kopije":                                        "iOS backups",
	"Privici iz Maila":                                             "Mail attachments",
	"postavke sustava nisu otvorene: %w":                           "System Settings did not open: %w",
	"Lokalni simulator uređaj (%s).":                               "Local simulator device (%s).",
	"Lokalni Android virtualni uređaj (%s).":                       "Local Android virtual device (%s).",
	"Instalacijski paket: ":                                        "Installer: ",
	"Nedavno preuzeti instalacijski paket (%s), preuzet danas.":    "Recently downloaded installer (%s), downloaded today.",
	"Nedavno preuzeti instalacijski paket (%s), preuzet prije %s.": "Recently downloaded installer (%s), downloaded %s ago.",
	"Instalacijski paket (%s) preuzet prije %s. Preporučeno za uklanjanje ako je aplikacija instalirana.": "Installer (%s) downloaded %s ago. Safe to remove if the app is already installed.",

	// Koš
	"priprema mape Koša %s: %w":                       "preparing the Trash folder %s: %w",
	"premještanje u Koš %s: %w":                       "moving %s to the Trash: %w",
	"nevaljana putanja za povrat: %s":                 "invalid restore path: %s",
	"stvaranje izvorne mape za %s: %w":                "creating the original folder for %s: %w",
	"povrat %s: %w":                                   "restoring %s: %w",
	"stavka %s nije u Košu, pa nije izbrisana":        "%s is not in the Trash, so it was not deleted",
	"trajno brisanje %s: %w":                          "permanently deleting %s: %w",
	"Nijedna stavka nije premještena u Koš za smeće.": "No items were moved to the Trash.",
	"Uspješno premješteno u Koš %s (%s). Po potrebi ih možete vratiti iz kartice Koš i Povrat.": "Moved %s to the Trash (%s). You can restore them from the Trash & Restore tab.",
	"Nijedna stavka nije vraćena iz Koša za smeće.":                                             "No items were restored from the Trash.",
	"Uspješno vraćeno na izvorno mjesto %s (%s).":                                               "Restored %s to their original location (%s).",
	"Nijedna stavka nije trajno izbrisana.":                                                     "No items were permanently deleted.",
	"Trajno izbrisano %s. Oslobođeno je %s na disku.":                                           "Permanently deleted %s. Freed %s of disk space.",

	// Nadogradnja
	"priprema zahtjeva za GitHub: %w":                                                                          "preparing the GitHub request: %w",
	"povezivanje s GitHub API-jem: %w":                                                                         "connecting to the GitHub API: %w",
	"GitHub API je vratio status %d":                                                                           "the GitHub API returned status %d",
	"dekodiranje GitHub odgovora: %w":                                                                          "reading the GitHub response: %w",
	"nema novije verzije za instalaciju":                                                                       "there is no newer version to install",
	"izdanje %s nema potpisanu arhivu, pa ga preuzmite ručno sa stranice izdanja":                              "release %s has no signed archive, so download it manually from the releases page",
	"arhiva nije na stranici izdanja PometiME, pa nadogradnja nije pokrenuta":                                  "the archive is not on the PometiME releases page, so the update was not started",
	"preuzimanje potpisa nije uspjelo: %w":                                                                     "downloading the signature failed: %w",
	"preuzimanje arhive nije uspjelo: %w":                                                                      "downloading the archive failed: %w",
	"poslužitelj je vratio status %d":                                                                          "the server returned status %d",
	"datoteka je veća od %s, pa preuzimanje nije nastavljeno":                                                  "the file is larger than %s, so the download was stopped",
	"potpis arhive ne odgovara, pa nadogradnja nije instalirana; preuzmite PometiME ručno sa stranice izdanja": "the archive signature does not match, so the update was not installed; download PometiME manually from the releases page",
	"stvaranje privremene mape: %w":                                                                            "creating a temporary folder: %w",
	"raspakiravanje preuzete arhive: %w":                                                                       "unpacking the downloaded archive: %w",
	"u preuzetoj arhivi nije pronađen PometiME.app paket":                                                      "PometiME.app was not found in the downloaded archive",
	"zamjena aplikacije na lokaciji %s: %w":                                                                    "replacing the app at %s: %w",
	"preuzeta aplikacija nije PometiME":                                                                        "the downloaded app is not PometiME",
	"preuzeta verzija %s nije novija od instalirane verzije %s":                                                "the downloaded version %s is not newer than the installed version %s",

	// Pokretači i održavanje sustava
	"Datoteka nije čitljiva.":                                                              "The file cannot be read.",
	"Pokretač koristi sustavske servise ili ugrađeni XPC.":                                 "This launch agent uses system services or a built-in XPC service.",
	"Aktivni pokretač instaliranog programa.":                                              "Active launch agent of an installed program.",
	"Program je uklonjen s diska; preporučeno premjestiti u Koš.":                          "The program was removed from the disk; moving this agent to the Trash is recommended.",
	"nepoznata radnja optimizacije: %s":                                                    "unknown maintenance action: %s",
	"pražnjenje DNS predmemorije: %w":                                                      "flushing the DNS cache: %w",
	"DNS predmemorija je uspješno očišćena i osvježena.":                                   "The DNS cache was flushed.",
	"QuickLook međuspremnik i ikone Finder-a su uspješno osvježeni.":                       "The Quick Look cache and Finder icons were refreshed.",
	"oslobađanje Time Machine snimaka: %w":                                                 "thinning Time Machine snapshots: %w",
	"Lokalne APFS Time Machine snimke su uspješno pročišćene i oslobođene.":                "Local APFS Time Machine snapshots were thinned.",
	"Mapa zračnih pozadina je otvorena u Finderu.":                                         "The aerial wallpapers folder is open in Finder.",
	"čišćenje nedostupnih simulatora nije uspjelo (provjerite je li Xcode instaliran): %w": "removing unavailable simulators failed (check that Xcode is installed): %w",
	"Nedostupni iOS simulatori su uspješno očišćeni.":                                      "Unavailable iOS simulators were removed.",

	// Zauzeće prostora i Finder
	"mapu %s nije moguće pročitati: %w":                             "the folder %s cannot be read: %w",
	"mapa nije u vašoj korisničkoj mapi: %s":                        "the folder is not in your home folder: %s",
	"mapa ne postoji ili nije mapa: %s":                             "the folder does not exist or is not a folder: %s",
	"stavka nije u vašoj korisničkoj mapi ni među aplikacijama: %s": "the item is not in your home folder or among your apps: %s",
	"stavka više ne postoji: %s":                                    "the item no longer exists: %s",
	"Finder nije otvoren: %w":                                       "Finder did not open: %w",

	// Razvojni alati
	"Homebrew: stare verzije i preuzimanja":                               "Homebrew: old versions and downloads",
	"Stare verzije paketa i preuzete arhive koje Homebrew više ne treba.": "Old package versions and downloaded archives that Homebrew no longer needs.",
	"Procjena nije dostupna, ali naredbu možete pokrenuti.":               "No estimate is available, but you can still run the command.",
	"Trenutačno nema ničega za ukloniti.":                                 "There is nothing to remove right now.",
	"Oslobodit će oko %s.":                                                "It will free about %s.",
	"Go moduli":                                                           "Go modules",
	"Preuzeti Go moduli. Projekti ih ponovno preuzmu pri sljedećoj izgradnji, za što treba internet.": "Downloaded Go modules. Projects download them again on the next build, which needs an internet connection.",
	"pnpm: nekorišteni paketi": "pnpm: unused packages",
	"Briše samo pakete iz spremišta koje nijedan projekt više ne koristi.": "Removes only packages in the store that no project uses anymore.",
	"Spremište zauzima %s, a oslobodit će se samo nekorišteni dio.":        "The store takes up %s; only the unused part will be freed.",
	"nepoznat alat: %s":                                       "unknown tool: %s",
	"program %s nije pronađen na ovom Macu":                   "%s was not found on this Mac",
	"naredba %s nije uspjela: %s":                             "the command %s failed: %s",
	"Naredba %s je završila. Slobodnog prostora ima %s više.": "The command %s finished. You now have %s more free space.",

	// Sigurnost putanja i deinstalacija
	"putanja odbijena iz sigurnosnih razloga: %s":                       "path refused for safety reasons: %s",
	"provjera putanje %s: %w":                                           "checking the path %s: %w",
	"Deinstalacija odbijena jer putanja nije valjana aplikacija.":       "Uninstall refused because the path is not a valid app.",
	"nevaljana putanja aplikacije: %s":                                  "invalid app path: %s",
	"Uklanjanje podataka odbijeno jer putanja nije valjana aplikacija.": "Removing data refused because the path is not a valid app.",

	// Skeniranje
	"Privremeni instalacijski paketi ažuriranja aplikacije.":                          "Temporary update packages for an app.",
	"Predmemorija web-preglednika (preporučeno zatvoriti preglednik prije čišćenja).": "Web browser cache (close the browser before cleaning).",
	"Korisnička predmemorija aplikacije u ~/Library/Caches.":                          "App cache in ~/Library/Caches.",
	" Aplikacija je pokrenuta i koristi ovu predmemoriju.":                            " The app is running and using this cache.",
	" Brzo bi se ponovno napunila.":                                                   " It would fill up again quickly.",
	"Mijenjana je danas.":                                                             "Changed today.",
	"Mijenjana je prije %s.":                                                          "Changed %s ago.",
	"Logovi: ":                                                                        "Logs: ",
	"Zapisnici i dijagnostički izvještaji u ~/Library/Logs.":                          "Logs and diagnostic reports in ~/Library/Logs.",
	"Međuspremnik prevedenih Xcode projekata i indeksa.":                              "Build products and indexes of Xcode projects.",
	"Stare arhive izdanja iz Xcodea.":                                                 "Old release archives from Xcode.",
	"Simboli za otklanjanje grešaka starih iOS verzija.":                              "Debug symbols for old iOS versions.",
	"Korisnička predmemorija za iOS simulator.":                                       "Cache for the iOS Simulator.",
	"Međuspremnik preuzetih iOS/macOS CocoaPods paketa.":                              "Cache of downloaded iOS/macOS CocoaPods packages.",
	"Međuspremnik prevedenih Go paketa (go clean -cache).":                            "Cache of compiled Go packages (go clean -cache).",
	"Arhive preuzetih Go modula (go clean -modcache).":                                "Archives of downloaded Go modules (go clean -modcache).",
	"Preuzete instalacijske arhive u Homebrewu.":                                      "Installer archives downloaded by Homebrew.",
	"Međuspremnik preuzetih Node.js paketa.":                                          "Cache of downloaded Node.js packages.",
	"Međuspremnik preuzetih Yarn paketa.":                                             "Cache of downloaded Yarn packages.",
	"Globalno spremište PNPM paketa.":                                                 "Global PNPM package store.",
	"Međuspremnik preuzetih Python paketa.":                                           "Cache of downloaded Python packages.",
	"Preuzete arhive Rust paketa.":                                                    "Downloaded Rust package archives.",
	"Međuspremnik preuzetih Java/Android Gradle paketa.":                              "Cache of downloaded Java/Android Gradle packages.",
	"Lokalno spremište Java Maven ovisnosti.":                                         "Local repository of Java Maven dependencies.",
	"Privremeni indeks za goimports alat.":                                            "Temporary index for the goimports tool.",
	"Međuspremnik za statičku analizu Go koda.":                                       "Cache for static analysis of Go code.",
	"Privremena predmemorija za Go Language Server.":                                  "Temporary cache for the Go language server.",
	"Predmemorija za Codex AI aplikaciju.":                                            "Cache for the Codex AI app.",
	"Privremena okruženja za Codex CLI.":                                              "Temporary environments for the Codex CLI.",
	"Privremeni zapisi pod-agenata i alata (mapa memory ostaje sačuvana).":            "Temporary subagent and tool records (the memory folder is kept).",
	"iOS Sigurnosna Kopija: ":                                                         "iOS Backup: ",
	"Lokalna sigurnosna kopija iPhone ili iPad uređaja.":                              "Local backup of an iPhone or iPad.",
	"Mail Privitak: ":                                                                 "Mail Attachment: ",
	"Lokalna kopija privitaka preuzetih unutar aplikacije Apple Mail.":                "Local copy of attachments downloaded in Apple Mail.",

	// Stablo izvora i 4K pozadine
	"putanja nije dopuštena za pregled: %s":                                           "this path cannot be viewed: %s",
	"čitanje putanje: %w":                                                             "reading the path: %w",
	"Nema preuzetih 4K videozapisa ni predmemorije pozadina za čišćenje.":             "There are no downloaded 4K videos or wallpaper caches to clean.",
	"za brisanje 4K videozapisa iz /Library potrebna je administratorska potvrda: %w": "deleting 4K videos from /Library needs administrator approval: %w",
	"Preuzete 4K zračne pozadine i predmemorija su uspješno očišćeni.":                "Downloaded 4K aerial wallpapers and their cache were removed.",

	// Poslužitelj
	"Zahtjev nije poslan na adresu ove aplikacije.":                                      "The request was not sent to this app's address.",
	"Zahtjev nije poslan iz prozora PometiME. Zatvorite PometiME i otvorite ga ponovno.": "The request did not come from the PometiME window. Quit PometiME and open it again.",
	"Zahtjev mora biti poslan kao JSON.":                                                 "The request must be sent as JSON.",
	"Tražena stranica nije pronađena.":                                                   "The page was not found.",
	"Neispravan JSON zahtjev za čišćenje alata.":                                         "Invalid JSON request to clean a tool.",
	"Neispravan JSON zahtjev za prikaz u Finderu.":                                       "Invalid JSON request to show an item in Finder.",
	"Neispravan JSON zahtjev za čišćenje.":                                               "Invalid JSON cleanup request.",
	"Neispravan JSON zahtjev za povrat iz Koša.":                                         "Invalid JSON request to restore from the Trash.",
	"Neispravan JSON zahtjev za trajno brisanje.":                                        "Invalid JSON request to delete permanently.",
	"Neispravan JSON zahtjev za deinstalaciju.":                                          "Invalid JSON uninstall request.",
	"Neispravan JSON zahtjev za optimizaciju.":                                           "Invalid JSON maintenance request.",
	"Neispravan JSON zahtjev za jezik.":                                                  "Invalid JSON language request.",
	"Parametar putanja je obavezan.":                                                     "The putanja parameter is required.",
	"PometiME je uspješno ažuriran na najnoviju verziju. Pokrenite aplikaciju ponovno.":  "PometiME was updated to the latest version. Relaunch the app.",
}
