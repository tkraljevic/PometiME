#!/bin/sh
# Jednom izrađuje certifikat kojim se potpisuje kod aplikacije PometiME.
#
# macOS dozvole (npr. Potpuni pristup disku) za ad-hoc potpisanu aplikaciju veže
# uz sažetak koda, pa ih svaka nadogradnja tiho poništi. Uz stalni certifikat
# dozvola se veže uz certifikat i ostaje nakon nadogradnje.
#
# Certifikat i ključ spremaju se u zaseban keychain bez lozinke s dozvolama 0600.
# Login keychain i postavke povjerenja sustava ostaju netaknuti.
set -eu

KEYCHAIN="${1:?upotreba: novi-certifikat.sh PUTANJA_KEYCHAINA}"
IME="PometiME izdanja"

if [ -e "$KEYCHAIN" ]; then
	echo "Keychain $KEYCHAIN već postoji; postojeći certifikat se ne prepisuje." >&2
	exit 1
fi

RAD=$(mktemp -d)
trap 'rm -rf "$RAD"' EXIT

cat >"$RAD/cert.cnf" <<EOF
[req]
distinguished_name = dn
x509_extensions = ext
prompt = no
[dn]
CN = $IME
[ext]
basicConstraints = critical, CA:false
keyUsage = critical, digitalSignature
extendedKeyUsage = critical, codeSigning
subjectKeyIdentifier = hash
EOF

# Sustavski /usr/bin/openssl (LibreSSL) izrađuje PKCS#12 koji security import prihvaća.
/usr/bin/openssl req -x509 -newkey rsa:2048 -nodes -days 7300 \
	-keyout "$RAD/kljuc.pem" -out "$RAD/cert.pem" -config "$RAD/cert.cnf" 2>/dev/null
/usr/bin/openssl pkcs12 -export -inkey "$RAD/kljuc.pem" -in "$RAD/cert.pem" \
	-out "$RAD/id.p12" -passout pass:prijenos -name "$IME"

mkdir -p "$(dirname "$KEYCHAIN")"
chmod 700 "$(dirname "$KEYCHAIN")"
POPIS=$(security list-keychains -d user | tr -d '"')
security create-keychain -p "" "$KEYCHAIN"
# create-keychain dodaje novi keychain na popis pretraživanja; vraća se zatečeni popis.
# shellcheck disable=SC2086
security list-keychains -d user -s $POPIS
chmod 600 "$KEYCHAIN"
security set-keychain-settings "$KEYCHAIN"
security import "$RAD/id.p12" -k "$KEYCHAIN" -P prijenos -T /usr/bin/codesign >/dev/null
security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "" "$KEYCHAIN" >/dev/null

echo "Certifikat \"$IME\" spremljen je u $KEYCHAIN. Spremite i sigurnosnu kopiju."
security find-identity -p codesigning "$KEYCHAIN" | sed -n 's/^ *1) \([0-9A-F]*\).*/SHA-1: \1/p'
