# Repository Guide

## Verification

- Run `make check` for the required order: formatting check, `go vet`, tests, then a CGO-free build. A focused handler test is `go test -run TestInfoEndpoint ./...`.
- The local toolchain must be Go 1.24 or newer. `Dockerfile` pins the builder to Go 1.24.5 and also runs tests.
- Embed releases with `VERSION=x.y.z make build`; changing the version intentionally changes the otherwise reproducible binary.
- `.github/workflows/ci.yml` checks `main` and pull requests without publishing. `.github/workflows/release.yml` is the only GHCR publisher and creates the matching GitHub Release after a successful image push.

## Releases

- Stable Git tags use `vMAJOR.MINOR.PATCH`; the workflow embeds the version without `v` and publishes `ghcr.io/dunkleaura/hello-swarm` for amd64 and arm64.
- A release publishes exact (`0.1.0`), minor (`0.1`), and `latest` tags. Do not publish a floating `0` tag or publish `main`; exact version tags are immutable.
- GitHub Releases contain generated notes, image references, and the multi-arch digest; standalone binaries are not attached.
- Release commits must belong to `main`. The first GHCR package version must be made public manually in GitHub's package settings.

## Runtime Design

- `main.go` is the entire server; `web/` is compiled into the executable with `go:embed`. The `scratch` runtime must remain shell-free and CGO-free.
- The server is stateless. Replica discovery happens only in the browser's in-memory map; do not add server-side aggregation or a Docker socket dependency.
- Replica state advances only on successful `/api/info` responses. Failed or paused polls provide no evidence about an individual replica and must not age it toward offline.
- `/api/info` must remain uncached and close HTTP/1.1 connections. Swarm balances connections rather than individual requests, so connection reuse can prevent discovery of other replicas.
- `compose.yaml` pulls the published image for one local instance; use `IMAGE=hello-swarm VERSION=dev PULL_POLICY=never` with `make docker-build` for source builds.
- `compose.swarm.yaml` is a standalone three-replica stack manifest. Do not merge it with `compose.yaml`; deploy it directly with `docker stack deploy -c compose.swarm.yaml hello`.
- The image healthcheck invokes the same binary as `hello-swarm healthcheck`; if `HTTP_ADDR` changes from port 8080, update `HEALTHCHECK_URL` too.
