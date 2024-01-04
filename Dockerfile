FROM golang:1.21-alpine
WORKDIR /app

COPY ./src/go.mod ./src/go.sum ./
RUN go mod download

COPY ./src/*.go ./
COPY ./src ./
COPY ./src/config.yaml.tmpl /app/config.yaml

ENV GIN_MODE=release
RUN go build -v -o /app/expiro

EXPOSE 5050

ENTRYPOINT [ "/app/expiro" ]