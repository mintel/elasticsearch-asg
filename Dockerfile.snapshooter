###############################
# STAGE 1 test and build binary
###############################

FROM golang:1.13 AS builder

# Set Go build env vars
ARG GOOS=linux
ARG GOARCH=amd64
ENV GOOS=${GOOS} GOARCH=${GOARCH}

# Create a non-root user
ARG GOUSER=appuser
RUN adduser --gecos '' --disabled-password --no-create-home ${GOUSER}

WORKDIR /go/src/app

# Fetch dependencies
# Do this first for caching sake
ENV GO111MODULE=on
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Install/Update packages (after src COPY so it always happens)
RUN apt-get update -y \
    && apt-get upgrade -y \
    && apt-get install -y ca-certificates tzdata \
    && update-ca-certificates

# Build the binary
RUN CGO_ENABLED=0 go build -ldflags='-w -s' -trimpath -o /go/bin/snapshooter ./cmd/snapshooter

##########################
# STAGE 2 deployment image
##########################
FROM scratch

# Import from the builder.
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy our static executable.
COPY --from=builder /go/bin/snapshooter /usr/bin/snapshooter

# Use an unprivileged user.
USER ${GOUSER}

# Expose healthcheck port.
ENV PORT=8080
EXPOSE $PORT

ENTRYPOINT ["/usr/bin/snapshooter"]
