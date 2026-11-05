# PometiME

**PometiME** je nativna macOS aplikacija napisana u Go-u i Cocoa/WebKit-u za sigurno čišćenje diska, razvojnih i AI predmemorija te potpunu deinstalaciju aplikacija s njihovim ostacima u `~/Library`.

## Značajke

- **Pregled i Brzo Čišćenje:** Prikaz slobodnog prostora na disku i preporučene stavke za sigurno oslobađanje prostora.
- **Predmemorija i Zapisnici:** Čišćenje `~/Library/Caches` i `~/Library/Logs` uz automatsku zaštitu osjetljivih sustavskih mapa (`com.apple.*`, `CloudKit`).
- **Razvojni i AI Alati:** Čišćenje predmemorija za Go, Homebrew, NPM, Python Pip, Rust Cargo i završenih Claude Code sesija (`subagents`, `tool-results`) uz trajnu zaštitu mape `memory`.
- **Ostaci Aplikacija:** Pronalazak zaboravljenih mapa u `~/Library/Application Support`, `Saved Application State`, `WebKit` i `HTTPStorages` koje ne pripadaju nijednoj instaliranoj aplikaciji.
- **Deinstalater Aplikacija:** Potpuno uklanjanje `.app` paketa iz `/Applications` zajedno sa svim pripadajućim podacima u `~/Library`.

## Izgradnja i pokretanje

```bash
# Izgradnja nativnog macOS paketa dist/PometiME.app
make app

# Pokretanje aplikacije
open dist/PometiME.app

# Provjera kvalitete koda (STATUS: HEALTHY)
make quality
```
