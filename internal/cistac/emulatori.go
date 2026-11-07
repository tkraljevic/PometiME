package cistac

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	uzorakSimulatorNaziv   = regexp.MustCompile(`(?s)<key>name</key>\s*<string>\s*([^<]+?)\s*</string>`)
	uzorakSimulatorRuntime = regexp.MustCompile(`(?s)<key>runtime</key>\s*<string>\s*([^<]+?)\s*</string>`)
)

func prikupiSimulatorKandidate(mapaDevices string) []kandidatCiscenja {
	ulazi, err := os.ReadDir(mapaDevices)
	if err != nil {
		return nil
	}
	var izlaz []kandidatCiscenja
	for _, u := range ulazi {
		if !u.IsDir() {
			continue
		}
		if k, ok := obradiJedanSimulator(filepath.Join(mapaDevices, u.Name()), u.Name()); ok {
			izlaz = append(izlaz, k)
		}
	}
	return izlaz
}

func obradiJedanSimulator(putanjaDir, dirName string) (kandidatCiscenja, bool) {
	plistPut := filepath.Join(putanjaDir, "device.plist")
	sadrzaj, err := os.ReadFile(plistPut)
	var naziv, runtime string
	if err == nil {
		naziv, runtime = izvuciNazivIRuntimeSimulatora(string(sadrzaj))
	}
	prikazNaziv := formatirajNazivSimulatora(naziv, runtime, dirName)
	return kandidatCiscenja{
		putanja:     putanjaDir,
		naziv:       prikazNaziv,
		kategorija:  "dev_ai",
		opis:        fmt.Sprintf("Lokalni simulator uređaj (%s).", dirName),
		preporuceno: false,
	}, true
}

func izvuciNazivIRuntimeSimulatora(sadrzaj string) (string, string) {
	var naziv, runtime string
	if m := uzorakSimulatorNaziv.FindStringSubmatch(sadrzaj); len(m) > 1 {
		naziv = strings.TrimSpace(m[1])
	}
	if m := uzorakSimulatorRuntime.FindStringSubmatch(sadrzaj); len(m) > 1 {
		runtime = strings.TrimSpace(m[1])
	}
	return naziv, runtime
}

func formatirajNazivSimulatora(naziv, runtime, dirName string) string {
	rt := urediSimulatorRuntime(runtime)
	switch {
	case naziv != "" && rt != "":
		return fmt.Sprintf("iOS Simulator: %s (%s)", naziv, rt)
	case naziv != "":
		return fmt.Sprintf("iOS Simulator: %s", naziv)
	default:
		return fmt.Sprintf("iOS Simulator: %s", dirName)
	}
}

func urediSimulatorRuntime(rt string) string {
	rt = strings.TrimPrefix(rt, "com.apple.CoreSimulator.SimRuntime.")
	dijelovi := strings.Split(rt, "-")
	if len(dijelovi) > 1 {
		return dijelovi[0] + " " + strings.Join(dijelovi[1:], ".")
	}
	return rt
}

func prikupiAvdKandidate(mapaAvd string) []kandidatCiscenja {
	ulazi, err := os.ReadDir(mapaAvd)
	if err != nil {
		return nil
	}
	var izlaz []kandidatCiscenja
	for _, u := range ulazi {
		if !u.IsDir() || !strings.HasSuffix(u.Name(), ".avd") {
			continue
		}
		ime := strings.TrimSuffix(u.Name(), ".avd")
		izlaz = append(izlaz, kandidatCiscenja{
			putanja:     filepath.Join(mapaAvd, u.Name()),
			naziv:       fmt.Sprintf("Android Emulator (AVD): %s", ime),
			kategorija:  "dev_ai",
			opis:        fmt.Sprintf("Lokalni Android virtualni uređaj (%s).", u.Name()),
			preporuceno: false,
		})
	}
	return izlaz
}
