FROM golang:1.25-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/evmbench \
    ./cmd/evmbench

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/evmbench /usr/local/bin/evmbench

ENTRYPOINT ["evmbench"]
