// Program potpis izrađuje ključ za potpis izdanja i potpisuje arhivu PometiME-macOS.zip.
//
// PometiME instalira nadogradnju samo ako je arhiva potpisana tajnim ključem
// čiji je javni par upisan u internal/cistac/nadogradnja.go. Tajni ključ nikad
// ne ide u repozitorij ni na GitHub.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var errUputa = errors.New("upotreba: potpis novi-kljuc -kljuc DATOTEKA | potpis potpisi -kljuc DATOTEKA -javni KLJUC ARHIVA")

func main() {
	if err := pokreni(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "potpis: %v\n", err) // ispis greške prije izlaza ne može se dalje obraditi
		os.Exit(1)
	}
}

func pokreni(argumenti []string, izlaz io.Writer) error {
	if len(argumenti) == 0 {
		return errUputa
	}
	switch argumenti[0] {
	case "novi-kljuc":
		return noviKljuc(argumenti[1:], izlaz)
	case "potpisi":
		return potpisi(argumenti[1:], izlaz)
	default:
		return errUputa
	}
}

// noviKljuc sprema tajni ključ s dozvolama 0600 i ispisuje javni ključ za aplikaciju.
// Postojeći ključ nikad ne prepisuje: izgubljen ključ znači da instalirane
// aplikacije više ne mogu prihvatiti nadogradnju.
func noviKljuc(argumenti []string, izlaz io.Writer) error {
	zastavice := flag.NewFlagSet("novi-kljuc", flag.ContinueOnError)
	zastavice.SetOutput(io.Discard)
	putanja := zastavice.String("kljuc", "", "datoteka u koju se sprema tajni ključ")
	if err := zastavice.Parse(argumenti); err != nil || *putanja == "" {
		return errUputa
	}
	javni, tajni, err := ed25519.GenerateKey(nil)
	if err != nil {
		return fmt.Errorf("izrada ključa: %w", err)
	}
	if err := spremiNoviTajniKljuc(*putanja, tajni); err != nil {
		return err
	}
	_, err = fmt.Fprintf(izlaz, "Tajni ključ je spremljen u %s. Spremite i sigurnosnu kopiju.\nJavni ključ: %s\n",
		*putanja, base64.StdEncoding.EncodeToString(javni))
	return err
}

func spremiNoviTajniKljuc(putanja string, tajni ed25519.PrivateKey) error {
	if err := os.MkdirAll(filepath.Dir(putanja), 0o700); err != nil {
		return fmt.Errorf("stvaranje mape za ključ: %w", err)
	}
	datoteka, err := os.OpenFile(putanja, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("spremanje tajnog ključa (postojeći ključ se ne prepisuje): %w", err)
	}
	_, errPisanja := fmt.Fprintln(datoteka, base64.StdEncoding.EncodeToString(tajni.Seed()))
	if errZatvaranja := datoteka.Close(); errPisanja == nil {
		errPisanja = errZatvaranja
	}
	return errPisanja
}

// potpisi potpisuje arhivu i sprema potpis u ARHIVA.sig. Odbija potpisati kad
// tajni ključ ne odgovara javnom ključu iz aplikacije, jer bi takvu
// nadogradnju sve instalirane aplikacije odbile.
func potpisi(argumenti []string, izlaz io.Writer) error {
	zastavice := flag.NewFlagSet("potpisi", flag.ContinueOnError)
	zastavice.SetOutput(io.Discard)
	putanjaKljuca := zastavice.String("kljuc", "", "datoteka s tajnim ključem")
	ocekivaniJavni := zastavice.String("javni", "", "javni ključ upisan u aplikaciju")
	if err := zastavice.Parse(argumenti); err != nil || *putanjaKljuca == "" || *ocekivaniJavni == "" || zastavice.NArg() != 1 {
		return errUputa
	}
	tajni, err := ucitajTajniKljuc(*putanjaKljuca)
	if err != nil {
		return err
	}
	javni, _ := tajni.Public().(ed25519.PublicKey) // ed25519.PrivateKey uvijek vraća ed25519.PublicKey
	if base64.StdEncoding.EncodeToString(javni) != *ocekivaniJavni {
		return errors.New("tajni ključ ne odgovara javnom ključu u aplikaciji, pa arhiva nije potpisana")
	}
	arhiva := zastavice.Arg(0)
	sadrzaj, err := os.ReadFile(arhiva)
	if err != nil {
		return fmt.Errorf("čitanje arhive: %w", err)
	}
	potpis := base64.StdEncoding.EncodeToString(ed25519.Sign(tajni, sadrzaj)) + "\n"
	if err := os.WriteFile(arhiva+".sig", []byte(potpis), 0o644); err != nil {
		return fmt.Errorf("spremanje potpisa: %w", err)
	}
	_, err = fmt.Fprintf(izlaz, "Potpis je spremljen u %s.sig. Uz izdanje na GitHubu objavite obje datoteke.\n", arhiva)
	return err
}

func ucitajTajniKljuc(putanja string) (ed25519.PrivateKey, error) {
	tekst, err := os.ReadFile(putanja)
	if err != nil {
		return nil, fmt.Errorf("čitanje tajnog ključa: %w", err)
	}
	sjeme, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(tekst)))
	if err != nil || len(sjeme) != ed25519.SeedSize {
		return nil, fmt.Errorf("datoteka %s ne sadrži ispravan tajni ključ", putanja)
	}
	return ed25519.NewKeyFromSeed(sjeme), nil
}
