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
	@echo "${GREEN}# Copying icon files...${NOCOLOR}"
	mkdir -p ${INTEGRATIONS_DIR}/Linux
	cp logo/usm.png ${INTEGRATIONS_DIR}/Linux/
	mkdir -p ${INTEGRATIONS_DIR}/MacOS/USM.app/Resources
	cp images/icon.icns ${INTEGRATIONS_DIR}/MacOS/USM.app/Resources/ || echo "Warning: icon.icns not found"

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
