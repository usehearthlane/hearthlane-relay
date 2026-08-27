FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY main.go ./
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /relay .

FROM alpine:3.21

RUN adduser -D -u 10001 relay \
    && mkdir -p /data \
    && chown relay:relay /data

COPY --from=build /relay /usr/local/bin/relay

USER relay

ENV RELAY_DATA_FILE=/data/state.json

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/relay"]