ARG TARGETOS=linux
ARG TARGETARCH=amd64

# ---------- Builder ----------
FROM --platform=$BUILDPLATFORM golang:1.26 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

ARG REPOSITORY
ARG VERSION=dev
ARG REVISION=none
ARG BUILDTIME

# Copy go.mod and go.sum files from the project root
COPY go.mod go.sum ./
RUN go mod download

# Copy project files
COPY ./apps/default ./apps/default

# Build static binary for target platform
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
     -ldflags="-s -w \
         -X github.com/pitabwire/frame/version.Repository=${REPOSITORY} \
         -X github.com/pitabwire/frame/version.Version=${VERSION} \
         -X github.com/pitabwire/frame/version.Commit=${REVISION} \
         -X github.com/pitabwire/frame/version.Date=${BUILDTIME}" \
     -o /app/binary ./apps/default/cmd/main.go

# ---------- Final ----------
FROM cgr.dev/chainguard/static:latest
# Add Maintainer Info
LABEL maintainer="Bwire Peter <bwire517@gmail.com>"

USER 65532:65532

EXPOSE 80

# Add OCI metadata labels
ARG REPOSITORY
ARG VERSION
ARG REVISION
ARG BUILDTIME
LABEL org.opencontainers.image.title="Files Service"
LABEL org.opencontainers.image.version=$VERSION
LABEL org.opencontainers.image.revision=$REVISION
LABEL org.opencontainers.image.created=$BUILDTIME
LABEL org.opencontainers.image.source=$REPOSITORY

WORKDIR /

COPY --from=builder /app/binary /files
COPY --from=builder /app/apps/default/migrations /migrations

# Run the service command by default when the container starts.
ENTRYPOINT ["/files"]
