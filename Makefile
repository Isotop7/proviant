# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## Get container runtime
CONTAINER_RUNTIME := $(shell command -v podman 2>/dev/null || command -v $(CONTAINER_RUNTIME) 2>/dev/null)
# Verify the container runtime is set
ifeq ($(CONTAINER_RUNTIME),)
$(error No container runtime found. Please install podman or $(CONTAINER_RUNTIME).)
endif

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

# ==================================================================================== #
# Install/Setup
# ==================================================================================== #

.PHONY: init
init:
	npm install
	@make css
	@make js
	@make icons
	mkdir -p ./src/assets/js
	cp ./node_modules/bootstrap/dist/js/bootstrap.bundle.min.js* ./src/assets/js/
	cd src/ && go get -u

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	cd ./src && \
	go fmt ./... && \
	go mod tidy -v

## audit: run quality control checks
.PHONY: audit
audit:
	cd ./src && \
	go vet ./... && \
	go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./... && \
	go test -race -vet=off ./... && \
	go mod verify

.PHONY: lint
lint:
	cd ./src && \
	$(CONTAINER_RUNTIME) run -t --rm -v ./:/app:Z -w /app golangci/golangci-lint:v2.6.1 golangci-lint run -v -E gocritic --timeout "3m"

# ==================================================================================== #
# Documentation
# ==================================================================================== #

## packagedoc: generates package documentation as markdown
.PHONY: packagedoc
packagedoc:
	cd ./src && \
	go run github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest --output ../docs/README.md ./...

## apidoc: generates API documentation
.PHONY: apidoc
apidoc:
	cd ./src && \
	go run github.com/swaggo/swag/cmd/swag@latest init -g api/api.go --parseDependency -o ../docs/

.PHONY: doc
doc:
	@make packagedoc
	@make apidoc

# ==================================================================================== #
# Package
# ==================================================================================== #

.PHONY: containerimage
containerimage:
	$(CONTAINER_RUNTIME) build --no-cache --tag=proviant ./

# ==================================================================================== #
# App
# ==================================================================================== #

.PHONY: run
run:
	cd ./src && \
	go run proviant.go

.PHONY: runcontainer
runcontainer:
	@make containerimage
	cd ./src && \
	$(CONTAINER_RUNTIME) run -t --rm -p 5114:5114 -v ./config.yaml.sqlite.tmpl:/app/config.yaml:Z proviant:latest

.PHONY: runcontainerdebug
runcontainerdebug:
	@make containerimage
	cd ./src && \
	$(CONTAINER_RUNTIME) run -it --rm -v ./config.yaml.sqlite.tmpl:/app/config.yaml:Z --entrypoint /bin/sh proviant:latest

# ==================================================================================== #
# Web
# ==================================================================================== #

.PHONY: fonts
fonts:
	cp ./node_modules/bootstrap-icons/font/fonts/bootstrap-icons.woff* ./src/assets/fonts/
	cp ./node_modules/@fontsource-variable/dm-sans/files/dm-sans-latin-wght-normal.woff2 ./src/assets/fonts/
	cp ./node_modules/@fontsource/pacifico/files/pacifico-latin-*.woff2 ./src/assets/fonts/

.PHONY: icons
icons:
	cp ./res/icons/proviant_logo_256.png ./src/assets/icons/
	cp ./res/icons/proviant_logo.ico ./src/assets/icons/favicon.ico
	cp ./res/icons/proviant_logo_256.png ./src/assets/icons/favicon.png
	cp ./res/icons/proviant_hero.png ./src/assets/icons/hero.png

.PHONY: css
css:
	@make fonts
	npm run css

.PHONY: js
js:
	cp ./node_modules/html5-qrcode/html5-qrcode.min.js ./src/assets/js/
	cp ./node_modules/bootstrap/dist/js/bootstrap.bundle.min.js ./src/assets/js/
	cp ./node_modules/bootstrap/dist/js/bootstrap.bundle.min.js.map ./src/assets/js/
