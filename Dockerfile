FROM docker.io/golang:1.26.9-alpine AS buildenv
WORKDIR /app

RUN apk add --no-cache --update g++ gcc go npm
COPY ./src/go.mod ./src/go.sum ./
RUN go mod download
COPY ./ ./
RUN npm ci && \
    npm run css && \
    cp node_modules/bootstrap-icons/font/fonts/bootstrap-icons.woff* ./src/assets/fonts/ && \
	cp node_modules/@fontsource-variable/vend-sans/files/vend-sans-latin-wght-*.woff2 ./src/assets/fonts/ && \
    cp node_modules/@fontsource/dm-mono/files/dm-mono-latin-400-normal.woff2 ./src/assets/fonts/ && \
    cp node_modules/bootstrap/dist/js/bootstrap.bundle.min.js* ./src/assets/js/ && \
    cp node_modules/html5-qrcode/html5-qrcode.min.js ./src/assets/js/ && \
    cp node_modules/chart.js/dist/chart.umd.min.js ./src/assets/js/ && \
    cp node_modules/chart.js/dist/chart.umd.min.js.map ./src/assets/js/ && \
    cp res/icons/proviant_logo_256.png ./src/assets/icons/ && \
    cp res/icons/proviant_logo_512.png ./src/assets/icons/ && \
	cp res/icons/proviant_logo.ico ./src/assets/icons/favicon.ico && \
	cp res/icons/proviant_logo_256.png ./src/assets/icons/favicon.png && \
	cp res/icons/proviant_hero.png ./src/assets/icons/hero.png && \
	cp res/icons/header*.png ./src/assets/icons/ && \
    cd src && \
    CGO_ENABLED=1 GOOS=linux CGO_CFLAGS="-D_LARGEFILE64_SOURCE" go build -v -o ../proviant

FROM docker.io/alpine:3.23
WORKDIR /app

COPY --from=buildenv /app/proviant /app/proviant
RUN addgroup -S proviant && \
    adduser -S -G proviant proviant && \
    mkdir /app/data && \
    apk add --no-cache tesseract-ocr tesseract-ocr-data-deu tesseract-ocr-data-eng && \
    chown -R proviant:proviant /app
COPY --chown=proviant:proviant ./src/config.yaml.sqlite.tmpl /app/config.yaml
ENV GIN_MODE=release
EXPOSE 5114

USER proviant
ENTRYPOINT [ "/app/proviant" ]
