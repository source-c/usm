.PHONY: default build generate-integrations build-with-integrations package-linux package-macos clean clean-all help
#.SILENT:

# COLORS.
GREEN:=\033[0;1;32m
NOCOLOR:=\033[0m

# GO ENV.
export GOSUMDB=off
export GO111MODULE=on
export ENV=local

BIN := $(CURDIR)/bin
BUILD_DIR := $(CURDIR)/build
INTEGRATIONS_DIR := $(BUILD_DIR)/integrations
MOCKS_DIR := $(CURDIR)/internal/mocks

DEFAULT_TARGET ?= $(BUILD_DIR)/usm
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

default: clean lint build
	@:

clean:
	@echo "${GREEN}# Removing old binaries...${NOCOLOR}"
	$(RM) ${DEFAULT_TARGET}
	$(RM) -r ${BUILD_DIR}
	$(RM) -r ${MOCKS_DIR}

clean-all: clean
	$(RM) -r ${INTEGRATIONS_DIR}
	$(RM) -r ${BIN}

.build-mockery:
	@if [ ! -f $(BIN)/mockery ]; then \
		echo "${GREEN}# Installing mockery binary...${NOCOLOR}"; \
		GOBIN=$(BIN) go install github.com/vektra/mockery/v2@v2.53.4; \
	fi

# Set which go files to ignore while testing.
GO_TEST_FILES := `go list ./... | grep -v ./cmd | grep -v ./internal/storage | grep -v ./pkg | grep -v ./scripts | grep -v ./internal/mocks`

.build-golangci-lint:
	@if [ ! -f ./bin/golangci-lint ]; then \
    	echo "${GREEN}# Installing golangci-lint binary...${NOCOLOR}"; \
        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ./bin; \
    fi

.build-govulncheck:
	go install golang.org/x/vuln/cmd/govulncheck@latest

lint: .build-golangci-lint
	@echo "${GREEN}# Running configured linters...${NOCOLOR}"
	./bin/golangci-lint run --config=.golangci.yml ./...

vuln: .build-govulncheck
	govulncheck ./...

test:
	@echo "${GREEN}# Running app tests...${NOCOLOR}"
	go test --coverprofile=.coverage.out $(GO_TEST_FILES)
	go tool cover -func=.coverage.out

deps:
	@echo "${GREEN}# Preparing dependencies...${NOCOLOR}"
	go mod download && go mod tidy && go mod verify

check: deps test lint vuln

generate-mocks: .build-mockery
	@echo "${GREEN}# Generating mocks...${NOCOLOR}"
	$(BIN)/mockery

build:
	@echo "${GREEN}# Building all binaries...${NOCOLOR}"
	go build -o ${DEFAULT_TARGET} ./main.go

generate-integrations:
	@echo "${GREEN}# Generating desktop integration files...${NOCOLOR}"
	go run scripts/gen-integrations.go -output ${INTEGRATIONS_DIR} -version ${VERSION}
	@echo "${GREEN}# Generating macOS .icns from PNG (preserving transparency)...${NOCOLOR}"
	${MAKE} .build-macos-icns
	@echo "${GREEN}# Copying icon files...${NOCOLOR}"
	mkdir -p ${INTEGRATIONS_DIR}/Linux
	cp logo/usm.png ${INTEGRATIONS_DIR}/Linux/
	mkdir -p ${INTEGRATIONS_DIR}/MacOS/USM.app/Contents/Resources
	# Ensure .icns is named to match CFBundleIconFile (usm)
	@if [ -f ${BUILD_DIR}/icons/usm.icns ]; then \
		cp ${BUILD_DIR}/icons/usm.icns ${INTEGRATIONS_DIR}/MacOS/USM.app/Contents/Resources/usm.icns; \
	elif [ -f images/icon.icns ]; then \
		echo "${GREEN}# Using fallback images/icon.icns...${NOCOLOR}"; \
		cp images/icon.icns ${INTEGRATIONS_DIR}/MacOS/USM.app/Contents/Resources/usm.icns; \
	else \
		echo "Warning: no .icns available to copy"; \
	fi

build-with-integrations: build generate-integrations
	@echo "${GREEN}# Copying binaries to integration directories...${NOCOLOR}"
	cp ${DEFAULT_TARGET} ${INTEGRATIONS_DIR}/MacOS/USM.app/Contents/MacOS/usm
	chmod +x ${INTEGRATIONS_DIR}/MacOS/USM.app/Contents/MacOS/usm

package-linux: build-with-integrations
	@echo "${GREEN}# Creating Linux package...${NOCOLOR}"
	mkdir -p ${BUILD_DIR}/linux-package/usr/local/bin
	mkdir -p ${BUILD_DIR}/linux-package/usr/share/applications
	mkdir -p ${BUILD_DIR}/linux-package/usr/share/pixmaps
	cp ${DEFAULT_TARGET} ${BUILD_DIR}/linux-package/usr/local/bin/
	go run scripts/gen-integrations.go -output ${BUILD_DIR}/linux-desktop -version ${VERSION} -install /usr/local/bin
	cp ${BUILD_DIR}/linux-desktop/Linux/usm.desktop ${BUILD_DIR}/linux-package/usr/share/applications/
	cp logo/usm.png ${BUILD_DIR}/linux-package/usr/share/pixmaps/usm.png
	cd ${BUILD_DIR} && tar czf usm-${VERSION}-linux.tar.gz -C linux-package .

package-macos: build-with-integrations
	@echo "${GREEN}# Creating macOS app bundle...${NOCOLOR}"
	cp -R ${INTEGRATIONS_DIR}/MacOS/USM.app ${BUILD_DIR}/
	cd ${BUILD_DIR} && zip -r usm-${VERSION}-macos.zip USM.app

# Make formating less painful
.build-fmt-tools:
	@if [ ! -f $(GOPATH)/bin/gci ]; then \
		echo "${GREEN}# Installing gci...${NOCOLOR}"; \
		go install github.com/daixiang0/gci@latest; \
	fi
	@if [ ! -f $(GOPATH)/bin/gofumpt ]; then \
		echo "${GREEN}# Installing gofumpt...${NOCOLOR}"; \
		go install mvdan.cc/gofumpt@latest; \
	fi

fmt: .build-fmt-tools
	@echo "${GREEN}# Running gci fmt${NOCOLOR}"
	gci write --skip-generated -s standard -s default -s 'prefix(pkg.hiveon.dev)' -s alias --custom-order .
	@echo "${GREEN}# Running gofumpt${NOCOLOR}"
	gofumpt -w -l .

help:
	@echo "Available targets:"
	@echo "  default                  - Clean, lint, and build (default target)"
	@echo "  build                    - Build the main binary only"
	@echo "  generate-integrations    - Generate desktop integration files from templates"
	@echo "  build-with-integrations  - Build binary and generate integrations"
	@echo "  package-linux           - Create Linux distribution package"
	@echo "  package-macos           - Create macOS app bundle"
	@echo "  test                    - Run tests"
	@echo "  lint                    - Run linters"
	@echo "  fmt                     - Format code"
	@echo "  clean                   - Remove build artifacts"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION                 - Set version for packages (default: git describe or 'dev')"

.build-macos-icns:
	@echo "${GREEN}# Building .icns from logo/usm.png (cross-platform)...${NOCOLOR}"
	mkdir -p ${BUILD_DIR}/icons/usm.iconset
	# Generate iconset PNGs using sips (macOS) or ImageMagick convert (Linux/Windows)
	@if command -v sips >/dev/null 2>&1; then \
		echo "Using sips to generate iconset"; \
		sips -s format png -z 16 16 logo/usm.png --out ${BUILD_DIR}/icons/usm.iconset/icon_16x16.png >/dev/null; \
		sips -s format png -z 32 32 logo/usm.png --out ${BUILD_DIR}/icons/usm.iconset/icon_16x16@2x.png >/dev/null; \
		sips -s format png -z 32 32 logo/usm.png --out ${BUILD_DIR}/icons/usm.iconset/icon_32x32.png >/dev/null; \
		sips -s format png -z 64 64 logo/usm.png --out ${BUILD_DIR}/icons/usm.iconset/icon_32x32@2x.png >/dev/null; \
		sips -s format png -z 128 128 logo/usm.png --out ${BUILD_DIR}/icons/usm.iconset/icon_128x128.png >/dev/null; \
		sips -s format png -z 256 256 logo/usm.png --out ${BUILD_DIR}/icons/usm.iconset/icon_128x128@2x.png >/dev/null; \
		sips -s format png -z 256 256 logo/usm.png --out ${BUILD_DIR}/icons/usm.iconset/icon_256x256.png >/dev/null; \
		sips -s format png -z 512 512 logo/usm.png --out ${BUILD_DIR}/icons/usm.iconset/icon_256x256@2x.png >/dev/null; \
		sips -s format png -z 512 512 logo/usm.png --out ${BUILD_DIR}/icons/usm.iconset/icon_512x512.png >/dev/null; \
		sips -s format png -z 1024 1024 logo/usm.png --out ${BUILD_DIR}/icons/usm.iconset/icon_512x512@2x.png >/dev/null; \
	else \
		echo "Using ImageMagick convert to generate iconset"; \
		if command -v magick >/dev/null 2>&1; then \
			MAGICK=magick; \
		else \
			MAGICK=convert; \
		fi; \
		$$MAGICK logo/usm.png -resize 16x16 ${BUILD_DIR}/icons/usm.iconset/icon_16x16.png; \
		$$MAGICK logo/usm.png -resize 32x32 ${BUILD_DIR}/icons/usm.iconset/icon_16x16@2x.png; \
		$$MAGICK logo/usm.png -resize 32x32 ${BUILD_DIR}/icons/usm.iconset/icon_32x32.png; \
		$$MAGICK logo/usm.png -resize 64x64 ${BUILD_DIR}/icons/usm.iconset/icon_32x32@2x.png; \
		$$MAGICK logo/usm.png -resize 128x128 ${BUILD_DIR}/icons/usm.iconset/icon_128x128.png; \
		$$MAGICK logo/usm.png -resize 256x256 ${BUILD_DIR}/icons/usm.iconset/icon_128x128@2x.png; \
		$$MAGICK logo/usm.png -resize 256x256 ${BUILD_DIR}/icons/usm.iconset/icon_256x256.png; \
		$$MAGICK logo/usm.png -resize 512x512 ${BUILD_DIR}/icons/usm.iconset/icon_256x256@2x.png; \
		$$MAGICK logo/usm.png -resize 512x512 ${BUILD_DIR}/icons/usm.iconset/icon_512x512.png; \
		$$MAGICK logo/usm.png -resize 1024x1024 ${BUILD_DIR}/icons/usm.iconset/icon_512x512@2x.png; \
	fi
	# Produce .icns using iconutil (macOS) or png2icns (Linux/Windows)
	@if command -v iconutil >/dev/null 2>&1; then \
		iconutil --convert icns ${BUILD_DIR}/icons/usm.iconset --output ${BUILD_DIR}/icons/usm.icns; \
		echo "${GREEN}# .icns generated with iconutil at ${BUILD_DIR}/icons/usm.icns${NOCOLOR}"; \
	elif command -v png2icns >/dev/null 2>&1; then \
		echo "Using png2icns to build .icns"; \
		png2icns ${BUILD_DIR}/icons/usm.icns \
			${BUILD_DIR}/icons/usm.iconset/icon_16x16.png \
			${BUILD_DIR}/icons/usm.iconset/icon_16x16@2x.png \
			${BUILD_DIR}/icons/usm.iconset/icon_32x32.png \
			${BUILD_DIR}/icons/usm.iconset/icon_32x32@2x.png \
			${BUILD_DIR}/icons/usm.iconset/icon_128x128.png \
			${BUILD_DIR}/icons/usm.iconset/icon_128x128@2x.png \
			${BUILD_DIR}/icons/usm.iconset/icon_256x256.png \
			${BUILD_DIR}/icons/usm.iconset/icon_256x256@2x.png \
			${BUILD_DIR}/icons/usm.iconset/icon_512x512.png \
			${BUILD_DIR}/icons/usm.iconset/icon_512x512@2x.png; \
		echo "${GREEN}# .icns generated with png2icns at ${BUILD_DIR}/icons/usm.icns${NOCOLOR}"; \
	else \
		echo "Warning: neither iconutil nor png2icns found; skipping .icns generation"; \
	fi