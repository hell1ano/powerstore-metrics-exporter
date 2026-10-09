# Monitoring reliability follow-up

## Before production monitoring

- [x] **P1: Accept the PowerStore bulk timestamp format.** In addition to RFC3339,
  support space-separated timestamps with hour-only UTC offsets (synthetic example:
  `2000-01-01 00:00:00+00`). Keep timezone-aware freshness checks and add synthetic
  regression cases. The current RFC3339-only validator rejects this format.
- [x] **P1: Preserve missing CSV values.** Validate required identity/measurement
  columns before cache publication. Missing optional values must remain absent,
  rather than turning into zero-valued measurements. Add schema-variation tests.
- [x] **P1: Detect missing measurements for known objects.** Distinguish an empty
  resource class from a response that omits existing objects. Cover completely
  empty and partially incomplete responses with object-level freshness checks.
- [x] **P2: Surface registry and duplicate-sample failures.** Detect duplicate
  metric identities and ensure collection health reflects exposition failures,
  even when the HTTP handler returns a partial successful response.
- [x] **P2: Align filesystem bulk decoding and template coverage.** Resolve the
  six block/mirror fields declared by the collector/template but absent from the
  bulk model. Account for CSV schemas that do not supply these fields; do not
  manufacture zero values. Test CSV-to-metric-to-template behavior, not only descriptors.

## Pilot acceptance

- [x] Privately validate real PowerStore CSV schemas and timestamps against the
  parser, collectors and Zabbix selectors. Commit only synthetic fixtures and
  sanitized compatibility conclusions; never commit production data or logs.
- [ ] Repeat the private replay after the above fixes, preserving original source
  timestamps for freshness tests. A diagnostic replay with normalized timestamps
  is not proof that unmodified production archives are accepted.
- [ ] Import the new template into Zabbix 7.0.24 and verify supported items,
  discovery, units and recovery using PowerStoreOS 5.0.0.2.
- [ ] Test array disconnection, stale source data, exporter restart and bad TLS
  certificates. Confirm the resulting alerts and recovery behavior.

## Separate improvements

- [ ] Refresh inventory after startup and implement API pagination.
- [ ] Validate capacity source intervals and bulk-mode support on PowerStoreOS 5.
- [ ] Add configurable certificate verification for exporter-to-array requests.
