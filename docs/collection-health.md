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
rather than assigning a guessed NAS name. Equal names on different NAS servers
remain distinct through their IDs.

Zabbix dependent discovery processes the exporter response on its own schedule;
it does not trigger an array inventory scan. `inventoryRefresh` controls the
exporter's REST inventory requests only.
