#!/bin/sh
# Potpisuje PometiME.app certifikatom iz keychaina izdanja (vidi novi-certifikat.sh).
# Bez keychaina potpisuje ad-hoc: to je dovoljno za razvoj, ali ne i za izdanje,
# jer bi korisnici nakon svake nadogradnje izgubili dozvole.
set -eu

KEYCHAIN="${1:?upotreba: potpisi-aplikaciju.sh PUTANJA_KEYCHAINA PUTANJA_APLIKACIJE}"
APLIKACIJA="${2:?upotreba: potpisi-aplikaciju.sh PUTANJA_KEYCHAINA PUTANJA_APLIKACIJE}"
IME="PometiME izdanja"

if [ ! -f "$KEYCHAIN" ]; then
	echo "Upozorenje: nema keychaina $KEYCHAIN, potpisujem ad-hoc (samo za razvoj)." >&2
	codesign --force --deep --sign - "$APLIKACIJA"
	exit 0
fi

# codesign traži identitet samo u keychainima s popisa pretraživanja, pa se
# keychain izdanja dodaje na popis samo dok traje potpisivanje.
POPIS=$(security list-keychains -d user | tr -d '"')
# shellcheck disable=SC2086
vrati_popis() { security list-keychains -d user -s $POPIS; }
trap vrati_popis EXIT
security unlock-keychain -p "" "$KEYCHAIN"
# shellcheck disable=SC2086
security list-keychains -d user -s $POPIS "$KEYCHAIN"

codesign --force --deep --sign "$IME" --keychain "$KEYCHAIN" "$APLIKACIJA"
codesign --verify --deep --strict "$APLIKACIJA"
codesign -d -r- "$APLIKACIJA" 2>&1 | sed -n 's/^designated => /Zahtjev potpisa: /p'
