# Metrics Exporter for Dell PowerStore

Collect metrics from multiple PowerStore arrays through their REST and bulk APIs.
Prometheus and Zabbix can scrape the exporter directly; Grafana is optional.

This fork includes a dedicated **Zabbix 7.0 template**, collection-health and
freshness checks, filesystem NAS ownership, source-volume filtering, NAS request
caching, and separate rotating request/activity logs. Use this fork's `main` branch
for the current implementation; the original `zabbix7-collection-health` commits
are already merged and do not include subsequent fixes.

## Validation status

The target deployment is **Zabbix 7.0.24 with PowerStoreOS 5.0.0.2**. Automated tests
cover the exporter and template contracts, with CI on Go 1.23 and stable Go.
Private single-array CSV replays have also been performed; no production data is
included in this repository. Live acceptance of the latest changes remains pending.
The retained Zabbix 6.0 template is a legacy option, not the recommended template
for Zabbix 7. See the [pilot checklist](docs/pilot-checklist.md) and
[action list](TODO.md) for outstanding checks.

## Build and run

Use Go 1.23 or later. From the repository root:

```sh
mkdir -p build bulk
go build -o ./build/powerstore-metrics-exporter .
./build/powerstore-metrics-exporter -c config.yml
```

Edit [config.yml](config.yml) before starting. Set `storageList` to the arrays and
credentials you intend to monitor. The default exporter port is 9010. Use a
PowerStore account with the necessary monitoring permissions; bulk enablement
requires the Storage Operator role according to the project's deployment guidance.
Keep production credentials, logs and bulk archives out of Git and image builds.

For PowerStoreOS 5.0.0.2, set `apiVersion: v3` and **`bulkCollector: true`** for each
array. The example's `bulkCollector: false` must be changed for this deployment.
The project requires bulk mode for PowerStoreOS 4.1 and later, and recommends it
when per-object performance requests time out on larger systems.

## Configuration and collection behaviour

| Setting | Behaviour |
| --- | --- |
| `exporter.bulkDir` | Writable bulk cache directory; pre-create it before startup. |
| `exporter.bulkCron` | Bulk download schedule; example/default is every five minutes. |
| `exporter.bulkMaxAge` | Maximum cache/source age; default `15m`. |
| `exporter.inventoryRefresh` | `"0"` or omitted: inventory at startup only. Set e.g. `24h` for periodic refresh. Failed inventory classes retry every minute. |
| `storageList[].nasStatusCacheTTL` | NAS REST status cached per array, including failures; default `60s`. Refresh occurs on the next request after expiry. |
| `storageList[].capacityInterval` | Non-bulk capacity interval; defaults to `Five_Mins`. Also supports `One_Hour` and `One_Day`. Bulk mode uses five-minute source samples. |
| `exporter.reqLimit` | Concurrent-request limit for code paths using the request limiter; omitted/zero defaults to 200. |

Bulk mode enables bulk metrics on the array and downloads complete archives.
Downloads are validated in a temporary file before replacing the existing archive.
A failed refresh preserves the previous good archive; stale samples are not emitted.
Existing archives from a previous run do not need manual deletion: restart requires
a new successful download before treating the cache as ready. Consequently,
`bulk cache has no recent successful download` can appear during startup. If it
persists, inspect activation/download errors rather than deleting files blindly.

Faster monitoring polls do not increase the resolution of five-minute bulk samples.
Inventory refresh and NAS status caching are independent of Zabbix discovery/polling.
Restart or refresh inventory after object or replication-role changes.

### Monitoring scope

- Volumes and volume groups: local Primary/Clone objects with
  `is_replication_destination=false`. Ordinary replication destinations, snapshots
  and proxies are excluded. Unreplicated local objects remain included.
- Metro volumes: **both sides are monitored**, each under its own array identity.
  The API reports the destination flag as false on both sides. Do not sum both
  copies as unique logical capacity.
- Filesystems: Primary inventory, including clones; snapshots are excluded.
  Filesystem metrics include stable filesystem IDs and NAS ownership.
- Volume/group bulk records outside the selected local inventory are ignored.
  Missing supported measurements for selected objects still fail collection.
  No cross-array source-ID correlation is performed.

### TLS: two separate connections

**Exporter to PowerStore:** certificate and hostname verification are enabled by
default for REST and bulk calls. Set `storageList[].tlsCAFile` to a PEM CA bundle
when the array uses an internal CA. Use an array address covered by its certificate.
`tlsInsecureSkipVerify: true` explicitly disables verification; it is not the default.
Old configurations without TLS settings therefore require a trusted array certificate
or an explicit configuration change.

**Monitoring client to exporter:** enable HTTPS with `exporter.https.enable`,
`exporter.https.crtPath` and `exporter.https.keyPath`. Configure the monitoring client
to trust the issuing CA. The Zabbix 7 template verifies certificate and hostname
when its exporter URL uses HTTPS. The exporter does not add HTTP authentication.

### Logs and rotation

The example configuration writes two files:

- `log.path: ./logs/powerstore.log`: actual outbound PowerStore calls, collection
  diagnostics and exporter events.
- `log.accessPath: ./logs/requests.log`: incoming Zabbix/Prometheus requests,
  including peer IP, User-Agent, endpoint, status, duration and request ID.

Outbound entries use `stream=powerstore_api`; collection/exporter entries use
`stream=exporter`; incoming requests use `stream=client_request`. A collector-start
message is not evidence of a new API call. Outbound entries omit query strings,
headers and bodies; their duration ends when response headers arrive.

Both files rotate independently. `log.rotation` defaults to `maxSizeMB: 50`,
`maxBackups: 5`, `maxAgeDays: 7`, `compress: true`. Zero/omitted numeric limits use
those defaults. Archive cleanup/compression is asynchronous. See
[rotation details and Podman host mounts](docs/collection-health.md#separate-request-logs-and-automatic-rotation).

Old `log.path` settings still work: an omitted accessPath derives a sibling such as
`powerstoreExporter.out.access.log`. Empty/`-` paths select stdout. File logging also
copies to stdout unless `log.stdout: false`; failures fall back to stdout. Container
stdout retention is separate from exporter file rotation.

## Container deployment

The existing Dockerfile packages a prebuilt binary; it does not compile Go code.
Build a **Linux binary matching the container architecture** before building the
image. On a matching Linux build host, for example:

```sh
mkdir -p build bulk https
go build -o ./build/powerstore-metrics-exporter .
podman build -t powerstore-exporter:latest .
```

The current Dockerfile also copies the `bulk` and `https` directories: use a clean
build checkout without production archives or private keys. Mount runtime files
instead. Its working directory is `/powerstore_exporter`; mount configuration at
`/powerstore_exporter/config.yml`, the cache at `/powerstore_exporter/bulk`, and
certificates at the paths configured in that file. Match the published port to
`exporter.port`. Mount a log **directory**, not individual log files, for rotation.
See the [Podman Compose mount example](docs/collection-health.md#persist-logs-with-podman-compose).
Docker packaging changes remain deferred; container acceptance belongs to the pilot.

## Endpoints

Each array uses `http://<exporter>:9010/metrics/<storageList.ip>/<component>`.

| Component | Path suffix |
| --- | --- |
| Cluster | `cluster` |
| Appliance | `appliance` |
| Capacity | `capacity` |
| Hardware | `hardware` |
| Volume | `volume` |
| Volume group | `volumeGroup` |
| Port | `port` |
| NAS | `nas` |
| Filesystem | `file` |
| Bulk download/cache health | `health` |

Example: `http://127.0.0.1:9010/metrics/10.0.0.1/file`.
Component endpoints also expose their collector health. `/livez` reports that the
HTTP service is alive; `/readyz` requires initialized inventory and, for bulk-enabled
arrays, usable bulk caches. Readiness is not proof that every collector succeeds or
that the array is currently reachable. `/performance` exposes exporter process metrics.

## Monitoring setup

For **Zabbix 7.0**, import
[templates/zabbix/zbx_export_templates_7.0.yaml](templates/zabbix/zbx_export_templates_7.0.yaml)
and follow the [setup and migration guide](templates/zabbix/README.md). Create one
host per array and set its exporter URL and array-address macros. No Prometheus
server or Zabbix agent is required.

The original Zabbix 6.0 template remains available. Do not link both templates to
the same host: item keys overlap. Review filesystem name-to-ID migration and
history retention before replacing templates.

For **Prometheus**, adapt the scrape jobs from
[templates/prometheus/prometheus.yml](templates/prometheus/prometheus.yml) to your
exporter address and arrays, and add them to your configuration's `scrape_configs`.
Review intervals and add per-array health scrapes as needed; the example is a
starting point. Grafana can visualize either monitoring backend.

See [collection health](docs/collection-health.md) for metric semantics, failure
behaviour and limitations. For local verification, run `go test ./...` and
`go vet ./...`; CI also runs the tests with race detection on Linux.
