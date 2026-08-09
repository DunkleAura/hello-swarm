# Hello Swarm

Hello Swarm ist ein kleiner, zustandsloser HTTP-Dienst, der sichtbar macht, welche Container-Instanz eine Anfrage beantwortet. Die eingebettete Webseite pollt standardmaessig alle fuenf Sekunden und sammelt die gesehenen Swarm-Replikate im Speicher des Browsers.

Das Laufzeit-Image basiert auf `scratch`. Es enthaelt nur ein statisch gelinktes Go-Binary, keine Shell und keinen Paketmanager.

## Schnellstart

Voraussetzungen: Docker mit Compose-Plugin und BuildKit.

```bash
VERSION=0.1.0 docker compose up --build
```

Danach ist die Seite unter <http://localhost:8080> erreichbar. Logs erscheinen mit `docker compose logs -f app` als JSON auf `stdout`.

```bash
docker compose down
```

## Veroeffentlichtes Image

Stabile Multi-Arch-Images werden unter `ghcr.io/dunkleaura/hello-swarm` veroeffentlicht. Fuer Produktion sollte immer eine exakte Version verwendet werden:

```bash
docker pull ghcr.io/dunkleaura/hello-swarm:0.1.0
docker run --rm -p 8080:8080 ghcr.io/dunkleaura/hello-swarm:0.1.0
```

Verfuegbare Tag-Arten:

| Tag | Bedeutung |
| --- | --- |
| `0.1.0` | Unveraenderliches Release, fuer Produktion empfohlen |
| `0.1` | Neuestes Patch-Release der Reihe `0.1` |
| `latest` | Neuestes stabiles Release |

Es gibt waehrend Major `0` bewusst keinen Tag `0`, weil neue Minor-Versionen inkompatible Aenderungen enthalten duerfen. Pushes auf `main` erzeugen kein Registry-Image.

## Lokal bauen

Der lokale Build benoetigt Go 1.24 oder neuer. Fuer einen reproduzierbaren Binary-Build muessen Quellcode, Go-Version und `VERSION` identisch sein:

```bash
VERSION=0.1.0 make build
sha256sum hello-swarm
```

Der Build deaktiviert CGO und VCS-Metadaten, entfernt lokale Quellpfade und setzt keine variable Go-Build-ID. Die Version wird mit `-X main.version` eingebettet.

Direkt ohne Make:

```bash
CGO_ENABLED=0 go build -mod=readonly -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=0.1.0" \
  -o hello-swarm .
```

Start:

```bash
HTTP_ADDR=:8080 ./hello-swarm
```

## Pruefen

`make check` fuehrt die Schritte in dieser Reihenfolge aus: Formatpruefung, `go vet`, Tests und statischer Build.

```bash
make check
go test -run TestInfoEndpoint ./...
```

Der Docker-Build fuehrt die Tests ebenfalls aus:

```bash
VERSION=0.1.0 make docker-build
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

Das finale Image laeuft als numerischer Non-Root-Benutzer `65532`, besitzt keine Linux-Capabilities und verwendet das Binary selbst fuer den Docker-Healthcheck.

## Releases

Git-Tags sind die einzige Quelle fuer Release-Versionen. Ein stabiler Tag hat das Format `vMAJOR.MINOR.PATCH`; begonnen wird mit `v0.1.0`. Der Workflow entfernt das fuehrende `v`, bettet die Version in das Binary ein und veroeffentlicht das Image fuer `linux/amd64` und `linux/arm64`.

Ein Tag `v0.1.0` erzeugt:

```text
ghcr.io/dunkleaura/hello-swarm:0.1.0
ghcr.io/dunkleaura/hello-swarm:0.1
ghcr.io/dunkleaura/hello-swarm:latest
```

Nach erfolgreichem Image-Push erstellt derselbe Workflow ein GitHub Release mit dem Titel `Hello Swarm 0.1.0`. Es enthaelt automatisch generierte Release Notes, alle drei Image-Tags und den unveraenderlichen Multi-Arch-Digest. Es werden keine separaten Binary-Dateien angehaengt.

Der exakte Image-Tag darf nicht bereits existieren. Release-Tags muessen auf einen Commit in `main` zeigen. Pull Requests und Pushes auf `main` durchlaufen `make check` sowie einen nicht veroeffentlichten Multi-Arch-Docker-Build. Schlaegt Test oder Image-Push fehl, wird kein GitHub Release erstellt.

Nach dem ersten Release muss das GHCR-Package einmalig in den GitHub Package Settings auf `Public` gestellt werden. Das Image traegt OCI-Labels fuer Quell-Repository, Commit und Version sowie SBOM- und Provenance-Attestations.

## Docker Swarm

Das Image muss vor dem Deployment fuer alle Nodes erreichbar sein. Bei einem Multi-Node-Swarm bedeutet das normalerweise: zuerst in eine Registry pushen.

```bash
docker swarm init
IMAGE=ghcr.io/dunkleaura/hello-swarm VERSION=0.1.0 \
  docker stack deploy \
  --compose-file compose.yaml \
  --compose-file compose.swarm.yaml \
  hello
```

`compose.swarm.yaml` startet drei Replikate und uebergibt Node-, Service-, Task- und Slot-Metadaten ueber Swarm-Templates. Skalieren:

```bash
docker service scale hello_app=6
```

Die Webseite entdeckt Replikate nur, wenn der vorgeschaltete Load Balancer neue Verbindungen auf unterschiedliche Tasks verteilt. `/api/info` antwortet deshalb mit `Connection: close`, damit der Browser fuer den naechsten Poll eine neue HTTP/1.1-Verbindung aufbaut. Ein externer Proxy darf den Endpunkt weder cachen noch mit Sticky Sessions dauerhaft an dieselbe Instanz binden.

## Konfiguration

| Variable | Standard | Bedeutung |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | Listen-Adresse des HTTP-Servers |
| `HEALTHCHECK_URL` | `http://127.0.0.1:8080/healthz` | URL fuer den internen `healthcheck`-Befehl |
| `SWARM_NODE_NAME` | leer | Name des Swarm-Nodes |
| `SWARM_SERVICE_NAME` | leer | Name des Swarm-Services |
| `SWARM_TASK_NAME` | leer | Taskname und bevorzugte Instanz-ID |
| `SWARM_TASK_SLOT` | leer | Replikat-Slot |

Wenn `SWARM_TASK_NAME` fehlt, dient der Container-Hostname als Instanz-ID. Interne IPv4- und IPv6-Adressen werden direkt aus den aktiven Netzwerkschnittstellen gelesen. Der Docker-Socket wird nicht benoetigt und sollte nicht eingebunden werden.

## HTTP-Endpunkte

| Pfad | Inhalt |
| --- | --- |
| `/` | Eingebettete Weboberflaeche |
| `/api/info` | Metadaten der antwortenden Instanz als JSON |
| `/healthz` | Readiness-/Liveness-Antwort `ok` |

Beispielantwort von `/api/info`:

```json
{
  "instance_id": "hello_app.2.example",
  "hostname": "72cc44cde91a",
  "ip_addresses": ["10.0.1.12", "172.18.0.4"],
  "version": "0.1.0",
  "go_version": "go1.24.5",
  "os": "linux",
  "architecture": "arm64",
  "started_at": "2026-07-26T12:00:00Z",
  "node_name": "worker-1",
  "service_name": "hello_app",
  "task_name": "hello_app.2.example",
  "task_slot": "2"
}
```

Der Server speichert keine Requests oder Instanzdaten. Nur die geoeffnete Webseite haelt ihre Liste entdeckter Instanzen; ein Neuladen leert sie.

## Lebenszyklus entdeckter Instanzen

Die Webseite bewertet Instanzen anhand erfolgreicher Polls, nicht allein anhand der verstrichenen Zeit. Ein erfolgreicher Poll ist eine Gelegenheit, bei der der Swarm Load Balancer eine bekannte Replik haette auswaehlen koennen. Fehlgeschlagene und pausierte Polls veraendern deshalb keinen Instanzstatus.

| Status | Bedingung | Darstellung |
| --- | --- | --- |
| `ONLINE` | Kuerzlich geantwortet | Gruen |
| `VERMISST` | Seit `2 * aktive Instanzen` erfolgreichen Polls nicht gesehen, mindestens 3 Polls | Gelb |
| `OFFLINE` | Seit `5 * aktive Instanzen` erfolgreichen Polls nicht gesehen, mindestens 6 Polls | Rot |

Antwortet eine vermisste oder offline markierte Instanz erneut, wechselt sie sofort zu `ONLINE`. Offline-Karten bleiben als sichtbare Historie erhalten, bis die Webseite neu geladen wird. Bei einem Swarm-Neustart erhaelt ein Task eine neue Task-ID; dadurch wird der alte Task spaeter rot dargestellt und der Ersatz erscheint als neue gruene Karte.

## Aufgabenplanung

- [x] Zustandsloser Metadaten-Endpunkt und responsive Weboberflaeche
- [x] Einstellbares AJAX-Polling und clientseitige Instanzsammlung
- [x] Statisches Multi-Arch-Binary und minimales `scratch`-Image
- [x] Compose- und Swarm-Konfiguration
- [x] Automatisierter CI- und Release-Build fuer Multi-Arch-Images auf GHCR
- [x] Mehrstufiger Status fuer nicht mehr gesehene Instanzen
