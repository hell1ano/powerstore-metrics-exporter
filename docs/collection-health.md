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
samples. Capacity still uses the existing `One_Day` API interval. Inventory refresh,
pagination and changing capacity to bulk collection are separate follow-up work.

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
Nonempty modules require RFC3339 timestamps, including optional fractional seconds
and time-zone offsets. Invalid timestamps and timestamps over one minute in the
future reject the archive. Keep exporter and array clocks synchronized. Header-only
CSVs are valid empty resources and are checked against download age only.

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
