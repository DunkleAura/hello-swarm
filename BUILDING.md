# Building Hello Swarm

Diese Datei beschreibt Entwicklung, Verifikation, Image-Builds und Releases. Fuer den normalen Betrieb genuegt die [README](README.md).

## Voraussetzungen

- Go 1.24 oder neuer; CI und `Dockerfile` verwenden Go 1.24.5
- GNU Make fuer die bereitgestellten Kurzbefehle
- Docker mit BuildKit und Buildx fuer Container-Images

Es gibt keine externen Go-Abhaengigkeiten. Die Dateien unter `web/` werden mit `go:embed` in das Binary eingebettet.

## Verifikation

`make check` fuehrt die erforderlichen Schritte in dieser Reihenfolge aus: Formatpruefung, `go vet`, Tests und CGO-freier Build.

```bash
make check
```

Ein einzelner Handler-Test:

```bash
go test -run TestInfoEndpoint ./...
```

Der Docker-Build fuehrt die Go-Tests ebenfalls im Builder-Image aus.

## Lokales Binary

Die Version wird ueber `-ldflags` eingebettet. Fuer einen reproduzierbaren Build muessen Quellcode, Go-Version und `VERSION` identisch sein.

```bash
VERSION=0.1.0 make build
sha256sum hello-swarm
```

Der entsprechende vollstaendige Befehl:

```bash
CGO_ENABLED=0 go build -mod=readonly -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=0.1.0" \
  -o hello-swarm .
```

Start:

```bash
HTTP_ADDR=:8080 ./hello-swarm
```

## Lokales Image

```bash
VERSION=dev make docker-build
```

Das normale Compose-Beispiel verwendet standardmaessig GHCR und zieht das Image bei jedem Start. Fuer das lokale Image werden Registry-Pull und Standardname explizit ueberschrieben:

```bash
IMAGE=hello-swarm VERSION=dev PULL_POLICY=never docker compose up --detach
```

Aufraeumen:

```bash
docker compose down
docker image rm hello-swarm:dev
```

## Multi-Arch-Image

Fuer `linux/amd64` und `linux/arm64` wird ein Registry-Ziel benoetigt, weil Docker nicht beide Plattformen gleichzeitig in den lokalen Image Store laden kann:

```bash
IMAGE=registry.example.com/team/hello-swarm
VERSION=0.1.0
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg VERSION="$VERSION" \
  --tag "$IMAGE:$VERSION" \
  --push .
```

Das finale `scratch`-Image enthaelt nur das statische Binary, laeuft als numerischer Benutzer `65532:65532` und verwendet `hello-swarm healthcheck` ohne externe Werkzeuge.

## CI

`.github/workflows/ci.yml` laeuft fuer Pull Requests und Pushes auf `main`:

1. `make check`
2. Multi-Arch-Build fuer `linux/amd64` und `linux/arm64`
3. Ablage der Build-Layer im GitHub-Actions-Cache

CI meldet keine Images bei GHCR an und veroeffentlicht nichts.

## Releases

Ein stabiler Git-Tag `vMAJOR.MINOR.PATCH` ist die einzige Quelle fuer eine Release-Version. Der Tag muss auf einen Commit in `main` zeigen. `.github/workflows/release.yml` entfernt das fuehrende `v`, fuehrt alle Checks aus und veroeffentlicht das Image fuer amd64 und arm64.

Der Tag `v0.1.0` erzeugt:

```text
ghcr.io/dunkleaura/hello-swarm:0.1.0
ghcr.io/dunkleaura/hello-swarm:0.1
ghcr.io/dunkleaura/hello-swarm:latest
```

Exakte Tags sind unveraenderlich. Es gibt waehrend Major `0` keinen gleitenden Tag `0`, weil neue Minor-Versionen inkompatible Aenderungen enthalten duerfen. `latest` bezeichnet nur das neueste stabile Release, niemals `main`.

Nach erfolgreichem Image-Push erstellt der Workflow ein GitHub Release mit automatisch generierten Notizen, Image-Referenzen und dem Multi-Arch-Digest. Es werden keine eigenstaendigen Binaries angehaengt. Das Image enthaelt OCI-Labels sowie SBOM- und Provenance-Attestations.

Erstes Release:

```bash
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

Nach dem ersten Push muss das GHCR-Package einmalig in den GitHub Package Settings auf `Public` gestellt werden.
