# Upstream contribution notes

Baseline: `564df4d66274f318092529986288baeca12ec798` (`Update template and dashboard`).
The initial work was reviewed on `zabbix7-collection-health` (originally
`codex/zabbix7-collection-health`) and merged into `main` in
`hell1ano/powerstore-metrics-exporter`. Further work in this fork targets `main`.
See [the to-do list](../TODO.md) for remaining monitoring and compatibility work.

The initial implementation was split into two independently reviewable commits:

1. **fix: validate bulk cache publication and expose collection health**
   - Preserve the last valid archive on download, HTTP, gzip, tar and CSV failures.
   - Serialize refreshes and synchronize readers with cache publication.
   - Reject old source samples and expose cache/collector health metrics.
   - Propagate collection failures, reject invalid API JSON and guard empty required
     responses that previously could panic.
   - Include behavioral tests and `docs/collection-health.md`.
2. **feat: add Zabbix 7 template with freshness and TLS checks**
   - Add a separate 7.0 template without modifying the 6.0 export.
   - Correct cluster/port failure expressions and NAS/filesystem metric contracts.
   - Add NAS/filesystem performance, per-appliance service tags, collection health,
     source freshness, witness/NAS alerts and configurable thresholds/port filters.
   - Add configurable HTTP/HTTPS URLs with peer and hostname verification.
   - Include template contract tests, CI and setup/migration documentation.

The second commit requires the health metrics from the first. Existing exporter
metric names and paths are retained. No credentials or environment-specific array
addresses are committed beyond the pre-existing example configuration.

## Verification and outstanding deployment checks

Run:

```sh
go test ./...
go vet ./...
go test -race ./...
go build ./...
```

CI runs tests with race detection, vet and a build on Linux using Go 1.23 and the
current stable release. Local Windows tests and vet pass; the local race detector
requires a C toolchain, so race testing is delegated to CI. Consult the branch's
Actions results for actual CI status rather than treating workflow configuration as
proof of a passing run.

The metric contract tests use actual collector descriptors to check every template
Prometheus selector and discovery label. They also check master-item references,
UUID isolation from the legacy template, HTTPS verification and regression-prone
trigger expressions. They do not emulate the Zabbix import API or trigger engine.

Before an upstream production-readiness claim, perform the documented pilot on
Zabbix 7.0.24 / PowerStoreOS 5.0.0.2. In particular, confirm the real bulk CSV timestamp
format, array permissions, optional resource behavior and capacity API compatibility.
Inventory refresh, pagination, bulk capacity and configurable array TLS verification
are implemented in the follow-up commits below. The operator will perform the
[live pilot](pilot-checklist.md).

## Preparing the upstream pull request

```sh
git log --reverse --format=fuller 564df4d..main
git diff --stat 564df4d...main
```

Preserve the activity commits when rebasing or cherry-picking to an upstream
contribution branch. Describe the concrete failure modes and attach the completed pilot results.
Changes are pushed to the personal fork only; no upstream branch is changed.

## Follow-up activity commits on main

| Commit | Activity | Validation |
| --- | --- | --- |
| `d2d7a73` | PowerStore timestamp formats | Offsets, fractions, invalid formats, source expiry |
| `b9e0abc` | Preserve missing CSV measurements | Sparse columns, empty cells, genuine zero, invalid schemas |
| `3bd49c4` | Detect missing known objects | Complete/partial/empty inventory coverage |
| `c83431f` | Duplicate and registry failure health | Duplicate metrics and HTTP error propagation |
| `e65df26` | Filesystem template coverage | CSV-to-exposition-to-template contract |
| `8fd8dff` | Inventory refresh and pagination | Atomic snapshots, page failures, bounded authentication retries |
| `5b801b4` | Array TLS trust configuration | Trusted/untrusted CA, hostname mismatch, invalid CA files |
| `37f2ab1` | Hardware refresh follow-up | Nodes without startup inventory |
| `ccfeda3` | Bulk capacity and explicit REST intervals | Capacity template selectors and requested intervals |
| `e88e17f` | Direct NAS field correction | NAS size field and mismatched volume-group field |
| `9f26e21` | Outage/restart acceptance tests | Disconnect, fresh-cache fallback, expiry, restart recovery |

The commits are published together to the personal fork's `main`; each commit body
records its trigger, behavior and validation. No upstream push is performed.

The private replay accepted the original timestamp format and verified expiry
without changing original timestamps. A separate temporary replay with current
timestamps exercised ten bulk collectors and all 81 bulk-backed Zabbix selectors,
with no missing metric families. Inventory names were synthetic; this does not
validate live REST inventory, permissions, Zabbix import or trigger behavior.
Production archives, measurements, identifiers and replay logs remain outside Git.

Migration changes: array TLS verification is now enabled by default; configure
`tlsCAFile` for an internal CA. Non-bulk capacity now defaults to five-minute samples,
and bulk mode no longer makes daily capacity REST requests. Six unsupported
filesystem prototypes are removed from the Zabbix 7 defaults. The Zabbix 6 export
is unchanged.

## Filesystem NAS ownership follow-up

- `bda63ea`: Resolve `file_system.nas_server_id` via NAS inventory and attach stable
  filesystem ID, NAS ID and NAS name labels to both collection modes. Synthetic
  tests cover duplicate names, NAS rename, missing ownership and recovery.
- `08e3cda`: Add NAS display names/tags to Zabbix filesystem items and switch keys
  and selectors to filesystem IDs. Document the discovered-item migration.
- Inventory cadence follow-up: startup-only by default, with optional
  `exporter.inventoryRefresh` durations. This supersedes the earlier fixed
  five-minute refresh behavior. Bulk metric downloads keep their existing cadence.

The relationship was checked against the supplied PowerStore OpenAPI definition.
The supplied schema and PDF remain outside Git. Real NAS ownership still needs
live pilot verification; bulk CSVs alone do not contain this mapping.

## Filesystem diagnostics follow-up

Filesystem collection failures now log object IDs and mapping-specific reasons in
both collection modes. Invalid startup inventory reports all offending IDs; bulk
coverage reports every unmatched or missing object. This makes empty-name cases
traceable without treating the exporter label as proof of an unnamed array object.
Synthetic tests assert the emitted diagnostics and failure health. No production
identifiers or logs are included in the commit.
