# PometiME — Standardi kvalitete koda

Ovaj dokument opisuje obvezne pragove kvalitete koda za **PometiME**, preuzete iz sustava `goCOP` i `goEMM`.

## 1. Pragovi metrika (`quality/config.json`)

- **Ciklomatska složenost (CC):**
  - Opći prag: $\le 10$ po funkciji
  - Kritični put (`^internal/cistac/`): $\le 7$ po funkciji
- **CRAP indeks (`CC^2 * (1 - pokrivenost)^3 + CC`):**
  - Opći prag: $\le 10$
  - Kritični put (`^internal/cistac/`): $\le 7$
- **Pokrivenost testovima (`-race -covermode=atomic`):**
  - Ukupno po projektu: $\ge 85\%$
  - Kritični put (`^internal/cistac/`): $\ge 95\%$
- **Statička analiza (`golangci-lint`):**
  - Nula upozorenja i nula grešaka (`govet`, `errcheck`, `staticcheck`, `gocyclo`, `gocritic`, `revive`, `ineffassign`, `unused`, `misspell`, `errorlint`, `copyloopvar`, `bodyclose`).

## 2. Pravila pisanja koda

1. **Hrvatski identifikatori i poruke:** Sva imena tipova, funkcija, varijabli i komentara pišu se na hrvatskom jeziku. Korisničke poruke moraju biti cijele rečenice s ispravnom množinom (`UzBrojHR`).
2. **Zabrana progutanih grešaka:** Svaka provjera greške mora se ili vratiti pozivatelju s kontekstom (`fmt.Errorf("...: %w", err)`) ili, ako je ignoriranje namjerno, imati objašnjenje u komentaru u istom retku.
3. **Bez globalnih tablica stanja:** Konfiguracijski korijeni i pragovi predaju se kroz strukturu `cistac.Postavke`.

## 3. Pokretanje provjere

```bash
make quality
```
