# Zabbix 7.0.24 / PowerStoreOS 5.0.0.2 pilot

Local synthetic tests and private CSV replay are complete. The operator performs
these live checks before production rollout. Keep real configurations, archives
and result logs outside this public repository.

1. Build the fork's `main` and configure a read-only monitoring account, bulk
   collection, a writable private bulk directory and the correct array address.
   Configure `tlsCAFile` for the array CA and use a certificate-matching hostname
   or IP. Keep `tlsInsecureSkipVerify: false`.
2. Start the exporter and allow the initial bulk collection to become available.
   Check `/metrics/<array>/health`: download success and populated module freshness
   should be 1. Check each component endpoint's collector success.
3. Compare REST inventory with the array UI, including Ethernet/FC port and drive
   eligibility. Confirm optional empty classes are truly empty. Add/rename/remove
   a test object if permitted, restart the exporter (or use an explicitly configured
   inventory refresh), and verify discovery after bulk collection. Confirm NAS names
   on filesystem items, including identically named filesystems on different NAS servers. Check pagination on an inventory larger than one page.
4. Import `templates/zabbix/zbx_export_templates_7.0.yaml`. Set the exporter URL
   and array macro as described in the template README. When updating a previously
   imported version, remove the six obsolete filesystem block/mirror prototypes
   (including their discovered items) from that template; an update-only import
   can otherwise leave them behind. Preserve any intentional custom items.
5. Check discovery, item support, units and values against the array UI. Check
   latency conversion, bytes versus ratios, wear percentages and capacity values.
   Bulk capacity `last_*` metrics mean the latest five-minute sample, not a daily
   maximum. No `max_*` values should be fabricated from raw bulk samples.
6. In an isolated test, interrupt exporter-to-array connectivity. Download health
   must fail; fresh cached data may remain available until `bulkMaxAge`, after
   which collection must fail. Restore access and verify recovery.
7. Repeat with stale source samples, an exporter restart, an untrusted certificate
   and a hostname mismatch. Verify Zabbix health/no-data alerts and their recovery,
   allowing the configured trigger delays.
8. If non-bulk capacity will be used, validate `Five_Mins`, and any explicitly
   selected `One_Hour`/`One_Day` interval, against the live API. Non-bulk performance
   collection currently reports request success rather than source-age freshness.

Record the exporter commit, template version, pass/fail per step and any missing
resource classes. Share only sanitized conclusions upstream.

## First live results — 2026-10-14

Operator-reported observations; deployed revision is not yet confirmed. These are
endpoint data-availability results, not full validation of every metric, collector
health flag, discovery rule or alert. No production identifiers or logs are stored.

| Endpoint suffix | Observed result |
| --- | --- |
| `/cluster` | Data returned |
| `/appliance` | Data returned |
| `/capacity` | Data returned |
| `/hardware` | Data returned |
| `/volume` | Data returned |
| `/volumeGroup` | Data returned |
| `/port` | Data returned |
| `/nas` | Data returned |
| `/file` | No data; operator reports file last success as zero |

Before these checks, a pre-existing old data file caused old-data complaints.
The operator reports that manually deleting it was necessary to continue. The
specific file and failure mechanism have not been captured; this is tracked as
an open reported bug in the action list, separate from the filesystem failure.

Next diagnostics: collect the raw `/file` response, `filesystem collection failed`
log entries (including collector and error), and `/health` entries for
`SpaceMetricsByFilesystem` and `PerformanceMetricsByFileSystem`. Record the old
file's path/type and associated error separately. Keep original logs and archives
private; only sanitized conclusions belong in this repository.

## Follow-up full-log review

The longer private log shows that NAS performance recovers after initialization,
but filesystem capacity/performance fail strict coverage. The operator identified
one missing filesystem object as Snapshot; the exporter now filters inventory to
Primary. Rebuild and restart, then check both filesystem collector-success values,
filesystem values and NAS labels. Unexplained missing primary data must still fail.

The log also shows volume performance inventory mismatches and missing volume-group
measurements on repeated checks. Earlier data-returned results for `/volume` and
`/volumeGroup` apply to endpoint availability only, not performance completeness.
These remain open pilot issues. No raw logs or object identifiers are included here.
