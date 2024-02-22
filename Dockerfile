FROM golang:1.21-alpine AS buildenv
WORKDIR /app

RUN apk add --no-cache --update go gcc g++
COPY ./src/go.mod ./src/go.sum ./
RUN go mod download
COPY ./src/*.go ./
COPY ./src ./
RUN CGO_ENABLED=1 GOOS=linux CGO_CFLAGS="-D_LARGEFILE64_SOURCE" go build -v -o expiro

FROM alpine:3.19
WORKDIR /app

COPY --from=buildenv /app/expiro /app/expiro
RUN mkdir /app/data
COPY ./src/config.yaml.sqlite.tmpl /app/config.yaml
ENV GIN_MODE=release
EXPOSE 5050

ENTRYPOINT [ "/app/expiro" ]