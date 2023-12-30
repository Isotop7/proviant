# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'


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
	docker run -t --rm -v ./:/app -w /app golangci/golangci-lint:v1.55.2 golangci-lint run -v -E gocritic

# ==================================================================================== #
# Documentation
# ==================================================================================== #

## doc: runs a godoc instance
.PHONY: doc
doc:
	cd ./src && \
	godoc -http=:6060

# ==================================================================================== #
# App
# ==================================================================================== #

.PHONY: run
run:
	cd ./src && \
	go run expiro.go

# ==================================================================================== #
# Deploy
# ==================================================================================== #

.PHONY: dockerimage
dockerimage:
	docker build --tag=expiro ./