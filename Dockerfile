FROM golang:1.21-alpine AS buildenv
WORKDIR /app

COPY ./src/go.mod ./src/go.sum ./
RUN go mod download
COPY ./src/*.go ./
COPY ./src ./
RUN go build -v -o expiro

FROM alpine:3.19
WORKDIR /app

COPY --from=buildenv /app/expiro /app/expiro
COPY ./src/config.yaml.tmpl /app/config.yaml
ENV GIN_MODE=release
EXPOSE 5050

ENTRYPOINT [ "/app/expiro" ]