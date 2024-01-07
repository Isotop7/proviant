FROM golang:1.21-alpine as buildenv
WORKDIR /tmp/expiro

COPY ./src/go.mod ./src/go.sum ./
RUN go mod download
COPY ./src/*.go ./
COPY ./src ./
RUN go build -v -o expiro

FROM alpine:3.19
WORKDIR /app

COPY --from=buildenv /tmp/expiro/expiro /app/expiro
COPY ./src/config.yaml.tmpl /app/config.yaml
ENV GIN_MODE=release
EXPOSE 5050

ENTRYPOINT [ "/app/expiro" ]