FROM golang:1-alpine AS builder

RUN apk add ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY main.go cleanup.go ./
COPY pkg/ pkg/

ARG VERSION=dev
ARG COMMIT=unknown

RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s -extldflags '-static' \
              -X 'main.Version=${VERSION}' \
              -X 'main.Commit=${COMMIT}'" \
    -o hass-backup \
    .

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /build/hass-backup /hass-backup

ENTRYPOINT ["/hass-backup"]
