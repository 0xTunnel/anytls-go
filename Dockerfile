ARG GO_VERSION=1.24
ARG ALPINE_VERSION=3.21

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder

ARG BUILDPLATFORM
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
	go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/anytls-server ./cmd/server

FROM alpine:${ALPINE_VERSION}

LABEL org.opencontainers.image.title="anytls-ppanel" \
	org.opencontainers.image.description="AnyTLS server for PPanel v1 node mode"

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /etc/anytls

RUN mkdir -p /etc/anytls /etc/anytls/log

COPY --from=builder /out/anytls-server /usr/local/bin/anytls-server
COPY node.example.toml /etc/anytls/node.example.toml

STOPSIGNAL SIGTERM

ENTRYPOINT ["/usr/local/bin/anytls-server"]
CMD ["-c", "/etc/anytls/node.toml"]