GOLANGCI_VERSION := v2.14.0
GREMLINS_VERSION := v0.6.0
QUALITY_ROOT := $(CURDIR)
QUALITY_OUT ?= $(QUALITY_ROOT)/.quality
QUALITY_BASELINE ?= $(QUALITY_ROOT)/quality/baseline.json

.PHONY: build app run quality quality-tools quality-report quality-test quality-baseline quality-puni-race

build:
	go build -o pometime ./cmd/pometime

app: build
	mkdir -p dist/PometiME.app/Contents/MacOS dist/PometiME.app/Contents/Resources
	cp pometime dist/PometiME.app/Contents/MacOS/pometime-server
	clang -fobjc-arc -framework Cocoa -framework WebKit -O2 build/macos_webview.m -o dist/PometiME.app/Contents/MacOS/PometiME
	cp build/Info.plist dist/PometiME.app/Contents/Info.plist

run: build
	./pometime

quality-tools:
	GOBIN="$(QUALITY_ROOT)/bin/quality" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)

quality-test:
	cd dev/quality && go test -race ./...

quality: quality-test
	cd dev/quality && go run . -root "$(QUALITY_ROOT)" -out "$(QUALITY_OUT)" -baseline "$(QUALITY_BASELINE)"

quality-report: quality-test
	cd dev/quality && go run . -root "$(QUALITY_ROOT)" -out "$(QUALITY_OUT)" -baseline "$(QUALITY_BASELINE)" -report-only

quality-baseline:
	@test "$(ACCEPT_BASELINE)" = "yes" || (echo 'Potrebno: make quality-baseline ACCEPT_BASELINE=yes'; exit 1)
	cd dev/quality && go run . -root "$(QUALITY_ROOT)" -out "$(QUALITY_OUT)" -baseline "$(QUALITY_BASELINE)" -record-baseline

quality-puni-race:
	cd dev/quality && go run . -root "$(QUALITY_ROOT)" -out "$(QUALITY_OUT)" -puni-race
