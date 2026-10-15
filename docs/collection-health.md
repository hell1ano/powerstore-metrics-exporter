# Collection health and bulk cache reliability

The exporter distinguishes a successful HTTP scrape from a successful PowerStore
collection. Existing metric names and endpoint paths are preserved. The additional
metrics are available to both Zabbix and Prometheus consumers.

## Collector health

Each existing `/metrics/<array>/<component>` endpoint includes:

| Metric | Meaning |
| --- | --- |
| `powerstore_collector_success{IP,collector}` | 1 when this collector finishes without a reported API/cache error; otherwise 0. |
| `powerstore_collector_last_success_timestamp_seconds{IP,collector}` | Unix time of its last successful collection; 0 until one succeeds. |
| `powerstore_collector_samples{IP,collector}` | Number of data samples emitted in this collection. |

Collectors report failures even if another collector sharing the endpoint works.
Available partial data is retained alongside `success=0`. Empty required collectors
(cluster, appliance, hardware, capacity and appliance performance) are failures.
Optional resources may legitimately produce zero samples. API errors and missing
bulk modules are failures, including for optional resources: a missing response is
not evidence that an array has no such resources.

Successful collection time is **not source sample time**. In non-bulk mode this
change reports collection errors, but does not assert freshness of historical API
samples. Capacity defaults to the `Five_Mins` API interval in non-bulk mode; historical
intervals can be selected explicitly. Bulk mode uses the validated capacity CSV.

## Bulk freshness

`GET /metrics/<array>/health` exposes local cache status without calling PowerStore:

| Metric | Meaning |
| --- | --- |
| `powerstore_bulk_enabled{IP}` | Whether this array uses bulk collection. |
| `powerstore_bulk_download_success{IP}` | Result of the latest completed refresh; 0 before the first success. |
| `powerstore_bulk_last_success_timestamp_seconds{IP}` | Time of the last validated cache replacement; 0 before one succeeds. |
| `powerstore_bulk_source_timestamp_seconds{IP,module}` | Oldest source timestamp in a module; 0 for a header-only CSV. |
| `powerstore_bulk_module_fresh{IP,module}` | Both download age and source sample age are within `bulkMaxAge`. |

Set `exporter.bulkMaxAge: 15m` (the default if omitted). It must be positive and
should exceed the bulk refresh interval plus expected scheduling/API latency.
Nonempty modules require timezone-aware timestamps: RFC3339 or PowerStore
space-separated timestamps with hour-only or minute offsets, including fractional
seconds. Invalid timestamps and timestamps over one minute in the
future reject the archive. Keep exporter and array clocks synchronized. Header-only
CSVs are structurally valid and checked against download age; collectors additionally
require the corresponding inventory to be empty.

Repeatedly downloading an archive with old source timestamps does not make its data
fresh. `ReadCsvData` rejects stale or missing modules, and the corresponding collector
reports failure instead of repeatedly emitting stale measurements. A failed refresh
may continue serving a previously validated cache only while it remains fresh.

## Atomic publication

1. Serialize refresh attempts for each array.
2. Require HTTP 200; non-200 responses (including 304) do not count as a successful refresh.
3. Download to a private temporary file in the cache directory and flush it.
4. Validate gzip (including checksum), tar structure, recognized CSV modules,
   consistent rows, timestamps and model numeric fields. Duplicate module files are
   rejected. Compressed and expanded archives are each limited to 512 MiB.
5. Close and rename the validated temporary file over the cache while holding the
   per-array write lock. Readers hold the read lock until they finish.
6. Update success metadata only after replacement. On failure, remove the temporary
   file and retain the previous cache and its successful-download timestamp.

Use a local cache filesystem. Archives are read without extracting members. On
restart, an old file is not trusted until a fresh download has been validated.
The bulk directory must exist and be writable by the exporter account.

## Verification

Run `go test ./...`, `go vet ./...`, and `go test -race ./...` on a system with a
supported C compiler for Go's race detector. Tests cover failed HTTP responses,
truncation, checksum errors, malformed CSV/numeric fields, old/mixed/future source
timestamps, missing/empty modules, startup without a cache, recovery and concurrent
refresh/read operations. Collector tests cover partial errors and empty results.

These tests use simulated API responses. Validate real bulk archives and source
timestamp formats on the target PowerStoreOS version before production rollout.

## Inventory and pagination

Inventory loads at startup and publishes an immutable snapshot. Periodic refresh is
disabled by default (`exporter.inventoryRefresh: "0"`). Set a duration such as `24h`
or `1h` to opt in. Restart the exporter after inventory changes when using the
startup-only default. This does not alter the bulk download schedule. Failed
resource queries remain unavailable rather than appearing as an empty inventory.
Bulk collectors reject missing or unmatched objects; newly created objects can
report collection failure until a restart or configured inventory refresh and the
bulk refresh agree.
Hardware node descriptors no longer depend on the startup appliance list.

GET collections follow `Content-Range` and use stable ID ordering, with a maximum
requested page size of 2,000. Incomplete, inconsistent or failed pages fail the
whole request. Authentication is retried once. See Dell's
[pagination contract](https://www.dell.com/support/manuals/en-sg/powerstore-9000/pwrstr-apidevg/pagination?guid=guid-55d82d5c-baf1-4b96-9ad0-ead603800af8&lang=en-us).

## Array certificate verification

Both REST and bulk connections now verify the array certificate and hostname by
default. Set `tlsCAFile` per storage entry to a PEM CA bundle when using an internal
CA, and use an IP or DNS name covered by the certificate. A missing or invalid CA
file is a configuration error. This is a change from the previous insecure default.

For a temporary migration only, `tlsInsecureSkipVerify: true` explicitly restores
the old behavior. It disables certificate and hostname checks on both clients.
Zabbix-to-exporter TLS configuration is separate and remains verified by default.

## Capacity sources and compatibility

Bulk capacity uses `SpaceMetricsByAppliance`, with the same source-age checks as
performance. The existing `powerstore_cap_last_*` names remain latest-sample aliases
for the raw CSV fields. `max_*` metrics are emitted only when explicitly supplied
by an API response; five-minute samples are never advertised as daily maxima. The
Zabbix 7 capacity selectors are covered by a synthetic CSV-to-exposition test.

Without bulk collection, `capacityInterval` defaults to `Five_Mins`. Choose
`One_Hour` or `One_Day` explicitly only when historical aggregation is intended.
Changing from the old daily default changes the meaning of capacity trends; record
the upgrade time when interpreting historical charts. The legacy Zabbix 6 template
is unchanged. Live interval behavior still requires the PowerStoreOS 5 pilot.

Filesystem metrics retain the `name` label and add `file_system_id`,
`nas_server_id` and `nas_server_name`. The filesystem-to-NAS join uses the same
inventory snapshot as the names. Missing ownership reports collection failure
while preserving available samples with NAS ID or `unknown` display labels.
Missing filesystem names use the filesystem ID. Equal names on different NAS servers
remain distinct through their IDs.

Zabbix dependent discovery processes the exporter response on its own schedule;
it does not trigger an array inventory scan. `inventoryRefresh` controls the
exporter's REST inventory requests only.

## Troubleshooting unnamed or unmatched filesystems

An empty exporter name does not establish that a filesystem has no name on the
array. It can mean the metric's filesystem ID was not found in the exporter's
inventory snapshot. Both filesystem collectors log failures at warning level with
array IP, collector, filesystem ID, known filesystem name, NAS ID and the reason.
Filesystem coverage warnings identify individual affected IDs. Invalid
inventory responses log at error level with the resource class and offending IDs.

Reasons distinguish an unknown filesystem ID, an empty inventory name, missing
NAS ownership, unavailable NAS inventory, an unknown NAS ID and an empty NAS name.
Missing samples and request errors also identify the object where known. Failures
still set collector health to zero; no unnamed filesystem metric is emitted.
The completion message no longer claims success after a partial failure.

Logs go to standard output and the configured `log.path` (the example configuration
uses `./logs/powerstore.log`). Warning diagnostics are visible at the default
`info` level. Search for `filesystem collection failed` or `invalid inventory response`,
then use the reported ID with the array's read-only filesystem instance query.
When inventory is startup-only, restart after verifying an inventory change. Logs
contain operational identifiers and should remain private; full API responses and
credentials are not included in these diagnostics.

## Failed inventory recovery

A recovery timer checks once per minute for inventory classes that failed to load
and retries only those classes. Successfully loaded classes, including empty ones,
make no API requests from this timer. This preserves startup-only discovery by
default; `inventoryRefresh` still controls deliberate full rediscovery. Failed
scheduled refreshes also recover this way. Loads are serialized and snapshots
remain immutable. Synthetic tests verify selective recovery, restored filesystem
NAS ownership, and no further requests after recovery.

## Port speed units

Port speeds are normalized to Gbps: `100_Mbps` exports `0.1`, and `25_Gbps`
exports `25`. Auto/null exports zero (no numeric speed available). The Zabbix 7
speed prototypes now use floating-point values and fixed Gbps units. Reimport
the updated template and allow discovery to update existing items. Historical
Mbps samples retain their old incorrect values; history is not rewritten. Legacy
template users must also change speed items to floating point to retain fractions.
Validation includes speed conversion and Zabbix prototype contract tests.

## Process health and startup

`GET /livez` returns 200 while the HTTP service is running. `GET /readyz` returns
503 until every array has complete initial inventory (including at least one
appliance and node), and each bulk-enabled array has a successful, fresh cache.
It returns 200 when those conditions hold and 503 again if a bulk refresh fails
or the cache becomes stale. Probes read local state and do not contact arrays.
Readiness is not a live connectivity or hardware-health check: continue using
collector health and Zabbix alerts. REST-only readiness reflects loaded inventory.

Array login, inventory loading and bulk activation run in the background, so a
slow array no longer blocks HTTP startup. Metric endpoints remain accessible while
initializing and report their existing failure health until data is available.
Successful inventory discovery remains startup-only by default. Failed activation
uses capped backoff; successful activation gets a one-minute warm-up before download.

Use `log.path: "-"` (or empty) for stdout-only logging. Invalid startup configuration
and HTTP/TLS listener errors exit nonzero. Docker packaging is unchanged.

## NAS status request caching

Set `nasStatusCacheTTL: 60s` in each `storageList` entry. Omitted values default to
60 seconds; valid durations must be at least one second. This caches the NAS REST
status response per array and exporter process. Refresh is demand-driven: the
first poll after expiry fetches a new response; idle exporters do not poll on a
background timer. Concurrent polls share the refresh. Pagination/authentication
may require several HTTP requests within one logical refresh.

A failed refresh caches the error for the same interval, so repeated polls cannot
cause a retry storm. Previous successful data is not returned after a failed
refresh. Incomplete responses still fail collector validation. Cached successful
status may be up to the configured interval old; collection last-success remains
scrape success, not a new source acquisition timestamp. Inventory discovery and
bulk performance downloads retain their separate schedules. Other REST status
collectors are not cached by this NAS-specific setting.

Info logs identify real NAS REST refreshes; cache hits and routine NAS collection
starts are debug-level. Existing old configuration files receive the 60s default.

## Incoming request logs

Info-level `exporter HTTP request` entries show the socket peer IP, User-Agent,
method, endpoint path, status, duration and a generated request ID also returned in
`X-Request-ID`. Query strings, authorization headers, cookies and bodies are not
logged. The peer may be a proxy; forwarded headers are deliberately not trusted.
User-Agent is caller-supplied, so it is a diagnostic clue rather than proof of
identity. Request IDs identify HTTP access entries; collector logs identify array
and source but do not currently carry that request ID. Access logging does not
change scrape frequency; the NAS cache controls the rate of NAS status refreshes.

## Filesystem scope and diagnostic volume

Filesystem inventory requests `filesystem_type=eq.Primary`, covering normal
filesystems and clones, excluding snapshots. Restart after upgrading so the
startup inventory uses this filter. Confirmed destination NAS ownership excludes
the filesystem from both capacity and performance, in bulk and direct modes.
Missing measurements or references fail health but no longer discard other data.

Bulk coverage errors show a total count and at most ten sorted examples. Identical
filesystem object/error pairs repeat no more than once per five minutes, while health
metrics continue to indicate failure on every affected scrape. Successful recovery
resets the log suppression. Large-volume diagnostic changes supersede the earlier
statement that every offending ID is included in each log entry.

## Source-only volume monitoring

Volume and volume-group inventory uses the local canonical REST resources with
`type=in.(Primary,Clone)&is_replication_destination=eq.false`. This includes local
nonreplicated objects and replication sources; it excludes replication destinations,
snapshots and proxies. Volume status uses the same REST filter. Volume-group capacity
continues to use the CMA view, restricted to the selected local inventory IDs.

Bulk volume/group performance rows outside that inventory are ignored, including
unknown IDs. Selected objects must still have supported numeric measurements;
missing measurements remain collection failures. A successfully loaded empty
eligible inventory produces no performance samples without requiring a bulk module.
No cross-array lookup or source_id remapping is performed.

Inventory is startup-only by default. Restart after replication role changes or
configure inventoryRefresh to refresh eligibility periodically. Previously discovered
Zabbix destination items follow the template's lost-resource retention settings.

The supplied API schema says is_replication_destination is false at both ends of
Metro volumes: this filter does not distinguish Metro endpoints. Live validation
of the filtered endpoints on PowerStoreOS 5.0.0.2 remains required.

## Separate request logs and automatic rotation

`log.path` holds PowerStore calls (`stream=powerstore_api`) and collection/exporter
messages (`stream=exporter`). `log.accessPath` holds incoming monitoring HTTP requests
(`stream=client_request`), including peer IP, User-Agent, path, status and request ID.
Actual outbound HTTP attempts are logged once each, including pagination, login,
retries and bulk downloads. Cached reads do not produce outbound-call entries.
API entries include array IP, REST/bulk kind, method, path, status and time until
response headers arrive. Status zero with transport_error=true means no HTTP
response. Bodies, headers, query strings and raw transport errors are omitted from
these new request entries. Existing collection diagnostics provide failure details.
Incoming request IDs are not propagated to scheduled or shared backend operations.

The example configuration sets paths under `./logs/`. Existing `log.path` settings
still work: if accessPath is omitted, `exporter.log` produces `exporter.access.log`.
An empty path or `-` sends that stream to stdout; with both streams on stdout, use
the stream field to distinguish them. Existing combined log history stays in place;
only new incoming requests move to the access file.

Each file rotates independently before exceeding maxSizeMB (default 50 MiB), keeps
at most maxBackups (default 5) archives, and removes archives older than maxAgeDays
(default 7) during asynchronous cleanup after opening/rotation. Compression defaults
to true. Zero/omitted numeric limits use these bounded defaults; negatives fail
startup. Active files are not deleted merely because seven days elapsed. Cleanup
and compression are asynchronous, so disk usage can temporarily exceed the retained
limit, especially when an existing oversized log is first archived. One process
must own each pair of log files; do not share paths between replicas or combine
this with external rotation of the same files.

Missing parent directories are created when permissions allow. Failed file writes
fall back to stdout with an stderr warning. `log.stdout: false` disables ordinary
file-to-stdout duplication; it does not disable fallback or stdout-only streams.
Exporter rotation does not rotate Podman's own stdout logs.

### Persist logs with Podman Compose

Merge this volume into the existing exporter service (keep its current image,
configuration mount and other volumes):

```yaml
services:
  exporter:
    volumes:
      - ./logs:/var/log/powerstore-exporter:Z
```

Use these settings in the mounted exporter configuration:

```yaml
log:
  type: logfmt
  level: info
  path: /var/log/powerstore-exporter/powerstore.log
  accessPath: /var/log/powerstore-exporter/requests.log
  stdout: false
  rotation:
    maxSizeMB: 50
    maxBackups: 5
    maxAgeDays: 7
    compress: true
```

Create the host directory before deployment (`mkdir -p ./logs`) and give the
container's configured UID write access, accounting for rootless UID mapping.
`:Z` labels this private mount on SELinux hosts; omit it where not applicable.
Mount the directory, not individual files, because rotation renames files.
Verify both host files appear, trigger a monitoring request, and confirm the
request entry goes only into requests.log. Recreate the container and verify
history remains. Host permissions, SELinux and persistence need a live Podman pilot.

## Partial filesystem collection

Filesystem collection retains usable numeric samples even if other expected IDs
are missing or a sample has no matching inventory/NAS reference. Such samples
keep file_system_id; name falls back to that ID, nas_server_name falls back to the
known NAS ID or `unknown`, and unavailable nas_server_id stays empty. Warnings
identify affected IDs and the known filesystem name/NAS ID. Collector success is
zero and last-success does not advance while errors remain. Missing measurements
are not replaced with zeros. Zabbix can display values while reporting incomplete
collection; ID-based item keys remain unchanged when names recover.

NAS inventory now requests is_replication_destination. Only confirmed destination
ownership excludes a filesystem. Unknown references cannot prove replication role
or snapshot eligibility, so those available rows are retained with failure health
instead of silently classified. Inventory remains startup-only by default: restart
to load the new role field. NAS status monitoring itself is unchanged.

Archive integrity and freshness checks still apply before rows are emitted; a
corrupt, invalid or stale archive is not accepted for partial collection. Direct
mode needs inventory IDs to request samples. Per-object logging is throttled for
five minutes per identical error (up to 4096 tracked pairs); changed errors log
immediately and a fully successful scrape clears suppression.

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

### Partial volume performance

Bulk volume performance now retains usable samples for selected inventory objects
when other selected volumes have no performance row. Missing volumes produce no
fabricated zero values. Each affected volume is logged with array address, ID and
inventory name; identical warnings are limited to once per five minutes per
object/error, while collection health fails on every incomplete scrape. A fully
successful scrape resets warning suppression and advances last success.

Source-only selection, both-sides volume Metro eligibility and archive integrity
and freshness checks remain unchanged. Missing inventory or invalid/stale archives
still fail collection. Volume-group collection is unchanged. Rebuild and redeploy
the exporter for this fix; no YAML parameter or Zabbix template changes are needed.
This preserves available data; it does not recover measurements omitted by PowerStore.
