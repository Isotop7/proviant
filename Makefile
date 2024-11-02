# ==================================================================================== #
# HELPERS
# ==================================================================================== #

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
	docker run -t --rm -v ./:/app -w /app golangci/golangci-lint:v1.55.2 golangci-lint run -v -E gocritic --timeout "3m"

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

.PHONY: dockerimage
dockerimage:
	docker build --no-cache --tag=expiro ./

# ==================================================================================== #
# App
# ==================================================================================== #

.PHONY: run
run:
	cd ./src && \
	go run expiro.go

.PHONY: rundocker
rundocker:
	@make dockerimage
	cd ./src && \
	docker run -t --rm -p 5114:5114 -v ./config.yaml.sqlite.tmpl:/app/config.yaml expiro:latest

.PHONY: rundockerdebug
rundockerdebug:
	@make dockerimage
	cd ./src && \
	docker run -it --rm -v ./config.yaml.sqlite.tmpl:/app/config.yaml --entrypoint /bin/sh expiro:latest

# ==================================================================================== #
# Web
# ==================================================================================== #

.PHONY: fonts
fonts:
	cp ./node_modules/bootstrap-icons/font/fonts/bootstrap-icons.woff* ./src/assets/fonts/
	cp ./node_modules/@fontsource-variable/dm-sans/files/dm-sans-latin-wght-normal.woff2 ./src/assets/fonts/

.PHONY: css
css:
	@make fonts
	npm run css