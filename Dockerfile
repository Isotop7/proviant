FROM golang:1.23-alpine AS buildenv
WORKDIR /app

RUN apk add --no-cache --update go gcc g++ npm
COPY ./src/go.mod ./src/go.sum ./
RUN go mod download
COPY ./ ./
RUN npm install && \
    npm run css && \
    cp node_modules/bootstrap-icons/font/fonts/bootstrap-icons.woff* ./src/assets/fonts/ && \
	cp node_modules/@fontsource-variable/dm-sans/files/dm-sans-latin-wght-normal.woff2 ./src/assets/fonts/ && \
    cp node_modules/bootstrap/dist/js/bootstrap.bundle.min.js* ./src/assets/js/ && \
    cp node_modules/html5-qrcode/html5-qrcode.min.js ./src/assets/js/ && \
    cd src && \
    CGO_ENABLED=1 GOOS=linux CGO_CFLAGS="-D_LARGEFILE64_SOURCE" go build -v -o ../proviant

FROM alpine:3.20
WORKDIR /app

COPY --from=buildenv /app/proviant /app/proviant
RUN mkdir /app/data
COPY ./src/config.yaml.sqlite.tmpl /app/config.yaml
ENV GIN_MODE=release
EXPOSE 5050

ENTRYPOINT [ "/app/proviant" ]