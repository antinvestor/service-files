
ARG service_name="boilerplate"
ARG service_port="7000"

FROM golang:1.12 as builder

RUN go get github.com/golang/dep/cmd/dep

ADD Gopkg.* ./
RUN dep ensure --vendor-only

WORKDIR /go/src/bitbucket.org/antinvestor/service-${service_name}

# Copy the local package files to the container's workspace.
ADD . .

# Build the service command inside the container.
RUN go install bitbucket.org/antinvestor/service-${service_name}

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o ${service_name}_binary .

FROM scratch
COPY --from=builder /go/src/bitbucket.org/antinvestor/service-${service_name}/${service_name}_binary /${service_name}
COPY --from=builder /go/src/bitbucket.org/antinvestor/service-${service_name}/migrations /
WORKDIR /

# Run the service command by default when the container starts.
ENTRYPOINT /${service_name}

# Document the port that the service listens on by default.
EXPOSE ${service_port}