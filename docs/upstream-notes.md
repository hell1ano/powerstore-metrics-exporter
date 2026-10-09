# Upstream contribution notes

Baseline: `564df4d66274f318092529986288baeca12ec798` (`Update template and dashboard`).
Development branch: `codex/zabbix7-collection-health` in
`hell1ano/powerstore-metrics-exporter`.

The work is split into two independently reviewable commits:

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
Startup-only inventory, pagination, capacity source intervals and array-side TLS
verification are intentionally separate follow-up work.

## Preparing the upstream pull request

```sh
git log --reverse --format=fuller 564df4d..codex/zabbix7-collection-health
git diff --stat 564df4d...codex/zabbix7-collection-health
```

Keep the two commits when rebasing or cherry-picking to an upstream contribution
branch. Describe the concrete failure modes and attach the completed pilot results.
This branch is pushed to the personal fork only; no upstream branch is changed.
