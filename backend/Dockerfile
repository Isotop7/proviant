FROM golang:1.21-alpine as builder
WORKDIR /app

RUN apk add --update gcc musl-dev

COPY ./src/go.mod ./src/go.sum ./
RUN go mod download

COPY ./src/*.go ./
COPY ./src ./

ENV CGO_ENABLED=1
RUN go build -v -o /app/expiro-backend

FROM alpine:3.19

WORKDIR /app
ENV GIN_MODE=release
COPY --from=builder /app/expiro-backend /app/expiro-backend
COPY ./src/config.yaml.tmpl /app/config.yaml

EXPOSE 5050

ENTRYPOINT [ "/app/expiro-backend" ]