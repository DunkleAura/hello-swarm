# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.24.5
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src
COPY go.mod ./
RUN go mod download

COPY main.go main_test.go ./
COPY web ./web

RUN go test -mod=readonly ./...
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -mod=readonly -trimpath -buildvcs=false \
    -ldflags="-s -w -buildid= -X main.version=${VERSION}" \
    -o /out/hello-swarm .

FROM scratch

COPY --from=build /out/hello-swarm /hello-swarm

USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=3s --retries=3 CMD ["/hello-swarm", "healthcheck"]

ENTRYPOINT ["/hello-swarm"]
