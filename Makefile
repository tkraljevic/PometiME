GOLANGCI_VERSION := v2.14.0
GREMLINS_VERSION := v0.6.0
QUALITY_ROOT := $(CURDIR)
QUALITY_OUT ?= $(QUALITY_ROOT)/.quality
QUALITY_BASELINE ?= $(QUALITY_ROOT)/quality/baseline.json
# Tajni ključ za potpis izdanja čuva se izvan repozitorija.
KLJUC_POTPISA ?= $(HOME)/.config/pometime/nadogradnja-ed25519.key
# Certifikat za potpis koda: macOS uz njega veže dozvole, pa one ostaju nakon nadogradnje.
KEYCHAIN_IZDANJA ?= $(HOME)/.config/pometime/potpisivanje.keychain-db
JAVNI_KLJUC := $(shell sed -n 's/^const javniKljucNadogradnjeB64 = "\(.*\)"$$/\1/p' internal/cistac/nadogradnja.go)

.PHONY: build app certifikat potpisi run quality quality-tools quality-report quality-test quality-baseline quality-puni-race

build:
	go build -o pometime ./cmd/pometime

app: build
	rm -rf dist/PometiME.app dist/PometiME-macOS.zip dist/PometiME-macOS.zip.sig
	mkdir -p dist/PometiME.app/Contents/MacOS dist/PometiME.app/Contents/Resources
	cp pometime dist/PometiME.app/Contents/MacOS/pometime-server
	clang -fobjc-arc -framework Cocoa -framework WebKit -O2 build/macos_webview.m -o dist/PometiME.app/Contents/MacOS/PometiME
	cp build/Info.plist dist/PometiME.app/Contents/Info.plist
	cp build/AppIcon.icns dist/PometiME.app/Contents/Resources/AppIcon.icns
	cp LICENSE dist/PometiME.app/Contents/Resources/LICENSE
	dev/potpis/potpisi-aplikaciju.sh "$(KEYCHAIN_IZDANJA)" dist/PometiME.app
	ditto -c -k --sequesterRsrc --keepParent dist/PometiME.app dist/PometiME-macOS.zip

certifikat:
	dev/potpis/novi-certifikat.sh "$(KEYCHAIN_IZDANJA)"

potpisi:
	@test -f dist/PometiME-macOS.zip || (echo 'Prvo pokrenite: make app'; exit 1)
	@codesign -d -r- dist/PometiME.app 2>&1 | grep -q 'certificate leaf' || (echo 'Izdanje mora biti potpisano certifikatom (make certifikat, pa make app).'; exit 1)
	cd dev/potpis && go run . potpisi -kljuc "$(KLJUC_POTPISA)" -javni "$(JAVNI_KLJUC)" "$(CURDIR)/dist/PometiME-macOS.zip"

run: build
	./pometime

quality-tools:
	GOBIN="$(QUALITY_ROOT)/bin/quality" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)

quality-test:
	cd dev/quality && go test -race ./...
	cd dev/potpis && go test -race ./...

quality: quality-test
	cd dev/quality && go run . -root "$(QUALITY_ROOT)" -out "$(QUALITY_OUT)" -baseline "$(QUALITY_BASELINE)"

quality-report: quality-test
	cd dev/quality && go run . -root "$(QUALITY_ROOT)" -out "$(QUALITY_OUT)" -baseline "$(QUALITY_BASELINE)" -report-only

quality-baseline:
	@test "$(ACCEPT_BASELINE)" = "yes" || (echo 'Potrebno: make quality-baseline ACCEPT_BASELINE=yes'; exit 1)
	cd dev/quality && go run . -root "$(QUALITY_ROOT)" -out "$(QUALITY_OUT)" -baseline "$(QUALITY_BASELINE)" -record-baseline

quality-puni-race:
	cd dev/quality && go run . -root "$(QUALITY_ROOT)" -out "$(QUALITY_OUT)" -puni-race
