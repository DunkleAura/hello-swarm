# Hello Swarm

Hello Swarm zeigt, welche Container-Instanz eine HTTP-Anfrage beantwortet. Die eingebettete Webseite pollt standardmaessig alle fuenf Sekunden und sammelt die gesehenen Docker-Swarm-Replikate im Speicher des Browsers.

Das Multi-Arch-Image fuer `linux/amd64` und `linux/arm64` basiert auf `scratch`. Es enthaelt nur ein statisch gelinktes Go-Binary, keine Shell und keinen Paketmanager.

## Schnellstart

```bash
docker run --detach \
  --name hello-swarm \
  --publish 8080:8080 \
  --read-only \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  ghcr.io/dunkleaura/hello-swarm:latest
```

Die Webseite ist danach unter <http://localhost:8080> erreichbar.

```bash
docker logs --follow hello-swarm
docker rm --force hello-swarm
```

Fuer Produktion sollte statt `latest` eine exakte, unveraenderliche Version wie `0.1.0` verwendet werden.

## Docker Compose

`compose.yaml` startet eine Instanz des veroeffentlichten Images mit Read-only-Dateisystem und ohne Linux-Capabilities:

```bash
docker compose up --detach
```

Eine feste Version auswaehlen:

```bash
VERSION=0.1.0 docker compose up --detach
```

Logs anzeigen und den Dienst entfernen:

```bash
docker compose logs --follow app
docker compose down
```

Das normale Compose-Beispiel startet bewusst nur eine Instanz. Mehrere Container koennen wegen der festen Bindung von Host-Port `8080` nicht mit `docker compose --scale` betrieben werden; dafuer ist das Swarm-Beispiel vorgesehen.

## Docker Swarm

`compose.swarm.yaml` ist ein eigenstaendiges Stack-Manifest. Es startet drei Replikate, veroeffentlicht Port `8080` ueber das Swarm Routing Mesh und uebergibt Node-, Service-, Task- und Slot-Metadaten.

```bash
docker swarm init
VERSION=0.1.0 docker stack deploy \
  --compose-file compose.swarm.yaml \
  hello
```

Skalieren und entfernen:

```bash
docker service scale hello_app=6
docker stack rm hello
```

Das Image muss fuer alle Swarm-Nodes erreichbar sein. Das oeffentliche GHCR-Image benoetigt keine Registry-Zugangsdaten.

## Replica-Erkennung

Jeder Aufruf von `/api/info` beschreibt nur die antwortende Instanz. Der Server bleibt dadurch vollstaendig zustandslos; die geoeffnete Webseite fuehrt ihre Liste im Arbeitsspeicher und leert sie beim Neuladen.

Swarm verteilt TCP-Verbindungen statt einzelner HTTP-Anfragen. `/api/info` beendet deshalb HTTP/1.1-Verbindungen nach jeder Antwort. Ein vorgeschalteter Proxy darf diesen Endpunkt nicht cachen oder dauerhaft mit Sticky Sessions an dieselbe Instanz binden.

| Status | Bedeutung |
| --- | --- |
| `ONLINE` | Die Instanz hat kuerzlich geantwortet |
| `VERMISST` | Die Instanz wurde trotz mehrerer erfolgreicher Polls nicht gesehen |
| `OFFLINE` | Die Instanz wurde ueber einen laengeren Beobachtungszeitraum nicht gesehen |

Fehlgeschlagene oder pausierte Polls veraendern keinen individuellen Instanzstatus. Antwortet eine vermisste oder offline markierte Instanz erneut, wechselt sie sofort zu `ONLINE`. Offline-Karten bleiben bis zum Neuladen sichtbar.

## Konfiguration

| Variable | Standard | Bedeutung |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | Listen-Adresse des HTTP-Servers |
| `HEALTHCHECK_URL` | `http://127.0.0.1:8080/healthz` | URL fuer den integrierten Healthcheck |
| `SWARM_NODE_NAME` | leer | Name des Swarm-Nodes |
| `SWARM_SERVICE_NAME` | leer | Name des Swarm-Services |
| `SWARM_TASK_NAME` | leer | Taskname und bevorzugte Instanz-ID |
| `SWARM_TASK_SLOT` | leer | Replikat-Slot |

Wenn `SWARM_TASK_NAME` fehlt, dient der Container-Hostname als Instanz-ID. Interne IPv4- und IPv6-Adressen werden aus den aktiven Netzwerkschnittstellen gelesen. Der Docker-Socket wird weder benoetigt noch eingebunden.

## HTTP-Endpunkte

| Pfad | Inhalt |
| --- | --- |
| `/` | Eingebettete Weboberflaeche |
| `/api/info` | Metadaten der antwortenden Instanz als JSON |
| `/healthz` | Readiness-/Liveness-Antwort `ok` |

Informationen zu lokalen Builds, Tests, Multi-Arch-Images und Releases stehen in [BUILDING.md](BUILDING.md).
