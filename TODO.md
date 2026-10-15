# Monitoring reliability follow-up

## Current rollout status - 2026-10-15

All currently requested implementation work is included in main through ffa2f27,
including partial volume performance (d9d1cc2) and restored NAS host discovery.
Earlier sections retain the investigation history; current remaining activities are:

- [ ] Build/deploy latest main and verify partial volume performance in production.
- [ ] Import/test both Zabbix 7 template options, item values, units and health alerts.
- [ ] Configure one NAS discovery parent and periodic inventoryRefresh; verify one
  host per eligible source and the same host ID after NAS source movement.
- [ ] Investigate PowerStore's missing volume performance rows using direct API
  comparison. Partial output is fixed; absent measurements are not recovered.
- [ ] Complete live outage/recovery, stale-data, TLS, readiness, pagination,
  inventory-change and volume Metro role-change checks from the pilot checklist.
- [ ] Confirm live capacity interval semantics, especially if direct mode is used.
- [ ] Inspect client access logs for duplicate monitoring callers; NAS REST cache
  timing itself is verified in the latest run.

The operator accepts warnings for three test filesystems; no suppression was
requested or implemented. Their incomplete data can still fail filesystem health.
Host log persistence and rotation are handled. Docker packaging changes remain
explicitly deferred. The old-archive recovery suspicion is unconfirmed and did
not recur after startup in the latest log; investigate only if it recurs.

## Before production monitoring

- [x] **P1: Accept the PowerStore bulk timestamp format.** In addition to RFC3339,
  support space-separated timestamps with hour-only UTC offsets (synthetic example:
  `2000-01-01 00:00:00+00`). Keep timezone-aware freshness checks and add synthetic
  regression cases. The previous RFC3339-only validator rejected this format.
- [x] **P1: Preserve missing CSV values.** Validate required identity/measurement
  columns before cache publication. Missing optional values must remain absent,
  rather than turning into zero-valued measurements. Add schema-variation tests.
- [x] **P1: Detect missing measurements for known objects.** Distinguish an empty
  resource class from a response that omits existing objects. Cover completely
  empty and partially incomplete responses with object-level freshness checks.
- [x] **P2: Surface registry and duplicate-sample failures.** Detect duplicate
  metric identities and ensure collection health reflects exposition failures,
  with collector failure flags and HTTP errors for endpoint-level failures.
- [x] **P2: Align filesystem bulk decoding and template coverage.** Resolve the
  six block/mirror fields declared by the collector/template but absent from the
  bulk model. Account for CSV schemas that do not supply these fields; do not
  manufacture zero values. Test CSV-to-metric-to-template behavior, not only descriptors.

## Pilot acceptance

- [x] Privately validate real PowerStore CSV schemas and timestamps against the
  parser, collectors and Zabbix selectors. Commit only synthetic fixtures and
  sanitized compatibility conclusions; never commit production data or logs.
- [x] Repeat the private replay after the above fixes, preserving original source
  timestamps for freshness tests. A diagnostic replay with normalized timestamps
  is not proof that unmodified production archives are accepted.
- [ ] Import the new template into Zabbix 7.0.24 and verify supported items,
  discovery, units and recovery using PowerStoreOS 5.0.0.2.
- [x] Locally test array disconnection, stale source data, exporter restart and
  certificate trust/hostname failures with synthetic responses.
- [ ] Confirm the resulting Zabbix alerts and recovery in the live pilot.

## Separate improvements

- [x] Implement API pagination and optional inventory refresh; default to startup-only.
- [x] Implement and locally validate capacity source intervals and bulk-mode support.
- [ ] Confirm capacity interval semantics against the live PowerStoreOS 5 API.
- [x] Add configurable certificate verification for exporter-to-array requests.

The live pilot is assigned to the operator. See [pilot steps](docs/pilot-checklist.md).

- [x] Add filesystem NAS ownership to metrics and Zabbix discovery using stable IDs.

## Second code review fixes

- [x] Reject direct samples without supported numeric measurements and incomplete
  NAS/hardware health responses; log missing object IDs.
- [x] Retry failed inventory classes without rediscovering successful classes.
- [x] Handle Auto and malformed port speeds without aborting collection.
- [x] Normalize Mbps to Gbps and enable fractional speed values in Zabbix 7.
- [x] Default omitted/zero request limits safely and reject negative limits.
- [ ] During the operator-led pilot, verify inventory recovery, missing-object
  alerts and the updated floating-point speed items after template reimport.

## Application readiness follow-up

- [x] Exit nonzero for startup/listener errors and validate configuration early.
- [x] Retry failed bulk activation with capped backoff and respect warm-up.
- [x] Check inventory coverage for all monitored hardware classes.
- [x] Add stdout-only logging and safe file-open failure fallback.
- [x] Serve liveness/readiness while arrays initialize in the background.
- [ ] Operator pilot: verify startup during an array outage, activation recovery,
  hardware missing-object alerts and readiness transitions.

Docker packaging findings are intentionally deferred at the operator's request.

## Live pilot findings — 2026-10-14

- [ ] **Investigate bulk-cache readiness if it recurs (unconfirmed issue).**
  The observed error was "bulk cache has no recent successful download".
  This means no validated download has been published in this process yet, or
  the last successful download exceeds bulkMaxAge (default 15 minutes). It does
  not establish that an existing archive prevented replacement. The operator
  deleted the file and later saw recovery, but now considers that association
  potentially coincidental with startup/NAS recovery. Remove the confirmed-bug
  assumption. If repeated after startup, capture activation/download/validation/
  replacement errors and bulk last-success time before changing files. Do not
  prescribe deletion based on this message alone; NAS causality is unconfirmed.
- [x] **Earlier all-or-nothing filesystem failure addressed.** Partial output and
  NAS policy filtering are implemented. The operator reports the latest run looks
  good and accepts the three remaining test-filesystem warnings. Validate values
  and expected failed health in the template; do not equate completion logs with
  complete filesystem coverage.

See [pilot observations](docs/pilot-checklist.md#first-live-results--2026-10-14).

## Requested deployment improvements

- [x] **Persist exporter log files on the host filesystem.** Handled by the
  operator; no further host logging or rotation action is required.

## NAS polling and request visibility

- [x] Cache NAS REST status per array with configurable `nasStatusCacheTTL: 60s`;
  share refreshes and preserve failure health while limiting retries.
- [x] Log incoming exporter callers and distinguish NAS REST refreshes from cache
  reads; move routine NAS collection-start messages to debug.
- [x] Latest pilot log confirms NAS REST requests are at least 60 seconds apart
  after startup. Caller-log review for duplicate monitoring clients remains open.

## Full-log review and filesystem fix

- [x] Exclude Snapshot objects from monitored filesystem inventory using the
  documented Primary filter (which includes clones); preserve NAS ownership.
- [x] Bound bulk coverage errors and suppress unchanged repeated filesystem logs
  without suppressing collection-failure health.
- [ ] Rebuild/restart and retest filesystem capacity and performance after filtering;
  the operator confirmed one missing object was Snapshot, not every missing ID.
- [ ] **P1: Volume performance inventory mismatch.** Full pilot log shows both
  metric IDs absent from inventory and known inventory objects missing measurements
  on repeated checks. Inspect the volume inventory endpoint and object eligibility;
  status data availability does not establish performance-collector success.
- [x] **Volume-group performance recovered in the latest run.** All 23 logged
  attempts reported success. Earlier missing-group findings are closed for that
  observed run; template values and broader role-change checks remain pending.

NAS performance recovered after startup in the full log. No download or file
replacement errors in that capture establish the cause of the earlier stale-file
workaround; retain that suspicion only as a recurrence check. Production logs remain private.


## Source-only monitoring policy

- [x] Select local Primary/Clone volumes and groups with
  is_replication_destination=false; omit other objects without cross-array joins.
- [x] Test selected-source coverage and replay private CSVs using an artificial
  eligible subset and normalized timestamps; no production data committed.
- [ ] Pilot: verify inventory, performance and capacity after restart. Keep the
  earlier volume/group findings open until live recovery is confirmed.
- [ ] Check Metro scope separately: the destination flag is false at both ends.
  Verify eligibility after role changes and inventory refresh.


## Separate logs and retention

- [x] Separate incoming client requests from PowerStore/API and exporter activity.
- [x] Add configurable size rotation, bounded backups, age cleanup and compression.
- [x] Document host directory mounts and disabling duplicate stdout output.
- [x] Host logging and rotation handled by the operator; close the deployment
  follow-up at their request.
- [x] Metro policy confirmed: retain monitoring on both local Metro endpoints.


## Filesystem pilot follow-up

- [x] Exclude filesystems on confirmed replication-destination NAS servers.
- [x] Preserve available filesystem samples when other measurements/references
  are missing, with fallback labels, per-object logs and failed health.
- [ ] Rebuild/restart and verify source filesystem values, destination exclusion
  and any remaining per-object warnings. One destination was confirmed; the
  other missing objects have not all been classified.

- [x] Refine filesystem exclusion to NAS destination=true AND non-empty NAS
  protection policy, for async and synchronous NAS replication. Null policy
  remains eligible; volume Metro retains both sides.
- [ ] Pilot the refined rule after restart and inspect remaining missing data.

## Source NAS host discovery

- [x] Add exporter-wide discovery, stable NAS-name identities and scoped child metrics.
- [x] Add Zabbix 7 NAS host bundle, movement/conflict/cache tests and migration instructions.
- [ ] Live pilot: import bundle, verify inherited URL and same host ID after source movement.

## Partial volume performance

- [x] Retain available bulk volume performance when selected volumes are missing;
  report failed health and named per-volume warnings. Later policy below adds
  automatic zeros for absent rows in valid, fresh bulk data.
- [x] Test 40 missing objects, real zero values, source filtering, warning
  suppression and recovery; privately replay the supplied CSV with synthetic
  inventory and normalized timestamps. No production data committed.
- [ ] Rebuild/redeploy and confirm valid volume performance in the live pilot.
- [ ] Investigate why PowerStore omits selected volume performance rows; compare
  the direct metrics API with the bulk archive. This fix does not recover absent data.

## Automatic missing-volume zeros

- [x] Display zero performance for selected volumes absent from a valid, fresh
  bulk module, using inventory name/appliance identity and no manual idle list.
- [x] Keep genuine measurements and other volumes visible on object-level errors;
  log missing/invalid objects and preserve failed health for inferred zeros.
- [x] Test zero labels/values, per-object isolation, stale-cache rejection and
  inventory appliance identity. Source filtering remains unchanged.
- [ ] Rebuild/restart and verify zero-valued idle volumes and active-volume values
  in Zabbix; confirm missing-row warnings and failed health remain visible.

## Idle-object detection

- [x] Add Information-level zero-average total IOPS triggers for volumes and
  filesystems in Zabbix 7 array/NAS templates, default 24h with configurable
  window/minimum sample count and a recent-data guard.
- [ ] Reimport templates and verify idle/active/recovery behavior after enough
  history; review Information-severity notification actions.
- Missing filesystem samples do not qualify as zeros. Filesystem performance
  zero filling has not been implemented; capacity must not be zero-filled.
