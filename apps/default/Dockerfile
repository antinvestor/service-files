FROM golang:1.25 AS builder

WORKDIR /app

# Copy go.mod and go.sum files from the project root
COPY go.mod go.sum ./
RUN go mod download

# Copy project files
COPY ./apps/default ./apps/default
COPY ./internal ./internal

# Build the binary from the cmd directory
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /app/binary ./apps/default/cmd/main.go

FROM gcr.io/distroless/static:nonroot

USER 65532:65532
EXPOSE 80
EXPOSE 50051

WORKDIR /

COPY --from=builder /app/binary /files
COPY --from=builder /app/apps/default/migrations /migrations
COPY --from=builder /app/apps/default/api /api

# Run the service command by default when the container starts.
ENTRYPOINT ["/files"]
