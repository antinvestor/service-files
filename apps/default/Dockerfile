FROM golang:1.24 AS builder

# Add Maintainer Info
LABEL maintainer="Bwire Peter <bwire517@gmail.com>"

WORKDIR /
COPY ../../go.mod .
COPY ../../go.sum .
RUN go env -w GOFLAGS=-mod=mod && go mod download

ADD ../.. .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o file_binary .

FROM gcr.io/distroless/static:nonroot

USER 65532:65532
EXPOSE 80
EXPOSE 50051

WORKDIR /

COPY --from=builder /file_binary /file
COPY --from=builder /apps/default/migrations /migrations

# Run the service command by default when the container starts.
ENTRYPOINT ["/file"]
