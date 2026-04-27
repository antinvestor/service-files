# build-platforms: linux/amd64
# ^ Honored by antinvestor/common/.github/workflows/docker-release.yml.
#   CGO + libtesseract/libleptonica via QEMU emulation hits a known
#   "gcc: unrecognized command-line option '-m64'" issue when cross-building
#   for arm64. Pinning to amd64 until a proper aarch64 cross-toolchain is set
#   up in the builder.
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# ---------- Builder ----------
# Note: no --platform=$BUILDPLATFORM here. CGO links against libtesseract /
# libleptonica which must match the target arch. Building under emulation
# (QEMU via buildx) keeps the toolchain and libs consistent with TARGETARCH.
FROM golang:1.26 AS builder
ARG TARGETOS
ARG TARGETARCH

RUN apt-get update -qq && apt-get install -y ca-certificates libtesseract-dev libleptonica-dev

WORKDIR /app

ARG REPOSITORY
ARG VERSION=dev
ARG REVISION=none
ARG BUILDTIME

# Copy go.mod and go.sum files from the project root
COPY go.mod go.sum ./
RUN go mod download

# Copy project files
COPY ./apps/ocr ./apps/ocr

# Build binary with CGO enabled for Tesseract integration
RUN CGO_ENABLED=1 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
     -ldflags="-s -w \
         -X github.com/pitabwire/frame/version.Repository=${REPOSITORY} \
         -X github.com/pitabwire/frame/version.Version=${VERSION} \
         -X github.com/pitabwire/frame/version.Commit=${REVISION} \
         -X github.com/pitabwire/frame/version.Date=${BUILDTIME}" \
     -o /app/binary ./apps/ocr/cmd/main.go

# ---------- Final ----------
FROM ubuntu:22.04

RUN apt-get update -qq && apt-get install -y ca-certificates libtesseract-dev libleptonica-dev tesseract-ocr-eng

# Add Maintainer Info
LABEL maintainer="Bwire Peter <bwire517@gmail.com>"

EXPOSE 80

# Add OCI metadata labels
ARG REPOSITORY
ARG VERSION
ARG REVISION
ARG BUILDTIME
LABEL org.opencontainers.image.title="OCR Service"
LABEL org.opencontainers.image.version=$VERSION
LABEL org.opencontainers.image.revision=$REVISION
LABEL org.opencontainers.image.created=$BUILDTIME
LABEL org.opencontainers.image.source=$REPOSITORY

WORKDIR /

COPY --from=builder /app/binary /ocr
COPY --from=builder /app/apps/ocr/migrations /migrations

# Run the service command by default when the container starts.
ENTRYPOINT ["/ocr"]
