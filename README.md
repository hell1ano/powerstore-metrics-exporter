# Metrics Exporter for Dell PowerStore

Collect metrics from multiple PowerStore arrays through their REST and bulk APIs.
Prometheus and Zabbix can scrape the exporter directly; Grafana is optional.

Replication sessions (asynchronous, synchronous and Metro, all resource types and
roles) are exposed on the existing cluster endpoint. Session and legacy Metro
collectors share one cached REST query, including embedded RPO rules; bulk remains
the performance source. See [replication coverage and code review](docs/replication-review.md)
for metrics, remaining gaps, request costs and rollout checks.

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
| `storageList[].apiLimit` | REST collection page size, not a total object limit; defaults to 2000 and is capped at 2000. All pages are fetched using Content-Range. Does not limit bulk CSV rows. |

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
  Filesystems on confirmed replication-destination NAS servers are also excluded.
  Filesystem metrics include stable filesystem IDs and NAS ownership. Available
  samples survive missing inventory/ownership references, with ID fallback labels,
  per-object warnings and failed collector health for incomplete results.
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

For **Zabbix 7.0 / 7.0.24**, use these complementary template files:

| File | Where to link it |
| --- | --- |
| [Array template](templates/zabbix/zbx_export_templates_7.0.yaml) | Link **Dell PowerStore by exporter HTTP** to one host per PowerStore array. Set `{$POWERSTORE.EXPORTER.URL}` and `{$PST_IP}`. |
| [NAS host bundle](templates/zabbix/zbx_export_nas_hosts_7.0.yaml) | Link **Dell PowerStore NAS discovery by exporter HTTP** to **one discovery host per exporter**, covering all configured arrays. It creates source NAS hosts linked automatically to **Dell PowerStore NAS by exporter HTTP**. |

For array monitoring plus separate source NAS hosts, import both files. Deploy
latest main first: the NAS bundle needs `/discovery/nas` and `/nas-hosts/<key>/...`,
which older exporters do not provide. No Prometheus server or Zabbix agent is needed.

On the NAS discovery host, set `{$POWERSTORE.EXPORTER.URL}` to the exporter URL
reachable from the Zabbix server/proxy (no trailing slash). Add or update this in
the existing exporter configuration, then restart:

```yaml
exporter:
  inventoryRefresh: 15m
```

This periodically refreshes ownership; the default is startup-only. Zabbix NAS
host discovery also defaults to 15 minutes, controlled separately by
`{$POWERSTORE.NAS.DISCOVERY.INTERVAL}`. Do not create a discovery parent per array.
With unique, unchanged NAS names, the same discovered NAS host is retained when
its source moves between configured PowerStores. Filesystem UUID changes can
still create replacement filesystem items.

The array template already contains NAS/filesystem monitoring. After validating
the new child hosts, deliberately retire overlapping array-host discovery/items
to avoid duplicate monitoring; preserve history during migration. Existing manual
hosts are not automatically adopted by discovery.

See the [array setup and migration guide](templates/zabbix/README.md) and
[NAS host setup guide](templates/zabbix/NAS-hosts.md). Live import, alerts and
failover acceptance in Zabbix 7.0.24 remain pilot checks.

The original Zabbix 6.0 template remains available. Do not link it alongside the
7.0 array template on the same host: item keys overlap.

For **Prometheus**, adapt the scrape jobs from
[templates/prometheus/prometheus.yml](templates/prometheus/prometheus.yml) to your
exporter address and arrays, and add them to your configuration's `scrape_configs`.
Review intervals and add per-array health scrapes as needed; the example is a
starting point. Grafana can visualize either monitoring backend.

See [collection health](docs/collection-health.md) for metric semantics, failure
behaviour and limitations. For local verification, run `go test ./...` and
`go vet ./...`; CI also runs the tests with race detection on Linux.

### NAS destination policy condition

For filesystem monitoring, exclude an owning NAS only when
`is_replication_destination=true` **and** its `protection_policy_id` is a non-empty
string. Use the NAS policy, not the filesystem policy. This rule applies to both
asynchronous NAS replication and synchronous NAS Metro. A null, missing or empty
policy keeps available filesystem measurements eligible; missing measurements
still produce warnings and failed collection health. This is the selected monitoring
policy, not proof of a currently active replication session.

Volume Metro remains a separate feature: both volume sides stay monitored.
Restart after upgrading to load the NAS policy field into startup inventory.
Partial filesystem output and stable ID-based item keys remain unchanged.

## Volume performance in Zabbix

Valid samples remain visible when another volume has an object-level error.
Selected volumes absent from a valid, fresh bulk performance module display zero
without a manual unused-volume list, provided inventory supplies their name and
appliance ID. Those values reflect the configured monitoring policy, not measured
zeros: collection health remains failed and named warnings identify missing rows.
Stale/invalid archives do not generate zeros. Keep collection-health alerts enabled
alongside performance items; see [collection health](docs/collection-health.md).

### Idle-object detection

The Zabbix 7 array and NAS templates identify volumes/filesystems with zero
average total IOPS over 24 hours as Information-level problems (`activity: idle`).
The window and minimum sample count are configurable through
`{$POWERSTORE.IDLE.PERIOD}` and `{$POWERSTORE.IDLE.MIN.SAMPLES}` (defaults `24h`
and `288` for five-minute polling). Import the updated templates; see
[configuration and data-coverage limits](templates/zabbix/README.md#idle-volumes-and-filesystems).
Missing filesystem samples do not count as zero; volume fallback zeros do.
# Expected missing object samples

Measured numeric zero is valid data. An absent row is not a measured zero.
For known unused objects, configure exact object UUIDs under the relevant
`storageList` entry (defaults to no exceptions):

```yaml
expectedMissingMetrics:
  filesystem_capacity: ["unused-filesystem-uuid"]
  filesystem_performance: ["unused-filesystem-uuid"]
  volume_performance: ["unused-volume-uuid"]
```

Exceptions only permit absent rows in valid fresh bulk modules or successful empty
direct metric responses. They do not skip collection, change source/destination
selection, or hide API failures, stale/missing bulk modules, invalid samples or
inventory errors. No additional API requests are introduced. Remove an exception
when an object returns to service; returning samples are collected even while listed.
Restart the exporter after changing configuration.

The three collectors emit `powerstore_object_sample_available` and
`powerstore_object_missing_expected`, labelled by `IP`, `collector`, `object_id`,
`name` and `nas_server_id` (empty for volumes):

| Result | Sample available | Missing expected | Collection health |
| --- | --- | --- | --- |
| Measured value, including zero | 1 | 0 | Successful unless another error occurs |
| Absent sample, explicitly configured | 0 | 1 | Successful unless another error occurs |
| Absent sample, not configured | 0 | 0 | Failed |

These diagnostics do not count toward `powerstore_collector_samples`. On request or
archive failure, coverage gauges may be absent; use `powerstore_collector_success`
to detect those failures. Existing volume zero-fill is retained for missing rows,
but the availability gauge makes those synthetic zeros identifiable. Filesystem
measurements are not fabricated. Filesystem coverage also follows NAS host filtering.

The Zabbix 7 array template discovers both coverage items for all three collectors;
the NAS-host template includes the two filesystem collectors. Discovery uses coverage
metrics so objects with absent measurements are included. Items use existing master
feeds and stable object IDs, without additional polling. Missing diagnostics during
request/archive failures are discarded rather than converted to zero; the last item
value remains visible, so always check collector health and the item's timestamp.
Existing collector-health triggers report failures; expected absence creates no new
per-object alert. Update the relevant Zabbix 7 template after upgrading the exporter.
