# Backend image. Plain `go build`, not Bazel: Bazel is the separate hermetic
# checkpoint signal, and putting it inside a container build would
# mean shipping a Bazel cache to produce a binary `make build` already makes.
# What matters here is that the Go version matches everything else that pins
# one — scripts/check-containers.sh fails the build if this drifts from go.mod and
# MODULE.bazel.
ARG GO_VERSION=1.25.0

FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /src

# Dependencies as their own layer: they change on the order of once a month,
# the source changes every commit.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO off so the binary runs on any base image; -trimpath so the build is
# reproducible from a different checkout path.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gcs ./cmd/gcs

FROM alpine:3.22

# Unprivileged. Nothing this process does needs root, and it terminates
# untrusted MAVLink frames from the network.
RUN adduser -D -u 10001 gcs

COPY --from=build /out/gcs /usr/local/bin/gcs

USER gcs
EXPOSE 8080 14550/udp

ENTRYPOINT ["/usr/local/bin/gcs"]
