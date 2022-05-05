FROM golang:1.18 as builder

# Add Maintainer Info
LABEL maintainer="Bwire Peter <bwire517@gmail.com>"

WORKDIR /
COPY go.mod .
COPY go.sum .
RUN go env -w GOFLAGS=-mod=mod && go mod download

ADD . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o file_binary .

FROM scratch
COPY --from=builder /file_binary /file
COPY --from=builder /migrations /migrations
WORKDIR /

# Run the service command by default when the container starts.
ENTRYPOINT ["/file"]

# Document the port that the service listens on by default.
EXPOSE 7513
