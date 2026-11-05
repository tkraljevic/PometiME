// Package main pokreće lokalni HTTP poslužitelj aplikacije PometiME.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pometime/internal/cistac"
	"pometime/internal/web"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := pokreni(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Greška pri pokretanju PometiME: %v\n", err)
		os.Exit(1)
	}
}

func pokreni(ctx context.Context, argumenti []string, izlaz io.Writer) error {
	zastavice := flag.NewFlagSet("pometime", flag.ContinueOnError)
	zastavice.SetOutput(io.Discard)
	adresa := zastavice.String("adresa", "127.0.0.1:19845", "Lokalna adresa za PometiME sučelje")
	korijen := zastavice.String("korijen", "", "Korijenska mapa korisnika (zadano $HOME)")
	if err := zastavice.Parse(argumenti); err != nil {
		return fmt.Errorf("neispravni argumenti naredbenog retka: %w", err)
	}

	dom, err := odrediKorijenKorisnika(*korijen)
	if err != nil {
		return err
	}
	slusac, err := net.Listen("tcp", *adresa)
	if err != nil {
		return fmt.Errorf("otvaranje mrežnog priključka %s: %w", *adresa, err)
	}
	return posluziDoPrekida(ctx, slusac, cistac.ZadanePostavke(dom), izlaz)
}

func odrediKorijenKorisnika(zadani string) (string, error) {
	if zadani != "" {
		return zadani, nil
	}
	dom, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("dohvat korisničke mape: %w", err)
	}
	return dom, nil
}

func posluziDoPrekida(ctx context.Context, slusac net.Listener, p cistac.Postavke, izlaz io.Writer) error {
	posluzitelj := &http.Server{
		Handler:           web.NoviPosluzitelj(p),
		ReadHeaderTimeout: 5 * time.Second,
	}
	_, _ = fmt.Fprintf(izlaz, "POMETIME_URL=http://%s\n", slusac.Addr().String()) // ispis URL-a na standardni izlaz

	greskaPosluzitelja := make(chan error, 1)
	go func() {
		if err := posluzitelj.Serve(slusac); err != nil && err != http.ErrServerClosed {
			greskaPosluzitelja <- err
		}
		close(greskaPosluzitelja)
	}()

	select {
	case err := <-greskaPosluzitelja:
		return err
	case <-ctx.Done():
		ctxGasenja, otkazi := context.WithTimeout(context.Background(), 3*time.Second)
		defer otkazi()
		return posluzitelj.Shutdown(ctxGasenja)
	}
}
