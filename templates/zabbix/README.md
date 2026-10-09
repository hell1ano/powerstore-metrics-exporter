# Zabbix templates

| File | Template | Purpose |
| --- | --- | --- |
| `zbx_export_templates.yaml` | Template API Storage Device EMC PowerStore Exporter | Existing Zabbix 6.0 export, retained unchanged. |
| `zbx_export_templates_7.0.yaml` | Dell PowerStore by exporter HTTP | New Zabbix 7.0 export with collection health, freshness, corrected file/NAS mappings and TLS configuration. |

The new template targets Zabbix 7.0, including 7.0.24. Its YAML, references, UUIDs,
metric names and labels are tested in the repository. A live import and end-to-end
PowerStoreOS 5.0.0.2 validation are still required; this is not a compatibility
certification. No Prometheus server or Zabbix agent is required.

## Setup

1. Build and deploy the updated exporter first. Pre-create its bulk cache directory.
   For PowerStoreOS 5.0.0.2, start from the project's documented newer-OS settings:
   `apiVersion: v3`, `bulkCollector: true`, `bulkCron: "*/5 * * * *"`, and
   `bulkMaxAge: 15m`. The project documents a Storage Operator account for bulk mode.
   Enabling bulk mode calls the array's bulk-metrics enable endpoint at startup.
2. Confirm the Zabbix server/proxy can reach `/metrics/<array>/health` and the nine
   existing component endpoints. The array address must match `storageList.ip`.
3. Import `zbx_export_templates_7.0.yaml` under **Data collection → Templates**.
4. Create one Zabbix host per array and link **Dell PowerStore by exporter HTTP**.
5. Set `{$POWERSTORE.EXPORTER.URL}` and `{$PST_IP}` on that host. A Zabbix host
   interface is not required for these HTTP items.
6. Exclude intentionally unused Ethernet/FC ports with
   `{$POWERSTORE.PORT.NOT_MATCHES}` before enabling notifications. Review optional
   NAS/filesystem/Metro health items against features actually enabled on the array.

## HTTPS

Set `{$POWERSTORE.EXPORTER.URL}` to, for example,
`https://powerstore-exporter.example.net:9010` (no trailing slash). Enable HTTPS and
configure the server certificate/key in the exporter. All HTTP master items in the
new template verify both the peer certificate and hostname. Install the issuing CA
in the trust store used by the Zabbix server/proxy and use a matching DNS hostname.

The default URL remains HTTP for compatibility with the exporter's default config.
Verification flags take effect when an HTTPS URL is selected. The template does not
add authentication to the exporter. If a reverse proxy requires authentication,
configure the HTTP items accordingly. This change also does not alter the separate
exporter-to-PowerStore TLS client; its existing certificate-verification limitation
remains outside the scope of this template update.

## Monitoring behavior

- Hardware, cluster and port endpoints: every minute. Appliance, volume,
  volume-group, NAS and filesystem endpoints: every five minutes. Capacity: every
  ten minutes. Health endpoint: every minute. Polling faster does not increase the
  five-minute bulk sample resolution or change capacity's source interval.
- Each collector has success and last-success items. Failure is detected on the
  next scrape; missing health is detected after `{$POWERSTORE.NODATA}` (30m by
  default). Exporter health endpoint loss has a separate five-minute trigger.
- Bulk refresh failure and stale source/cache data have separate alerts. Align
  `{$POWERSTORE.BULK.MAX.AGE}` (seconds, default 900) with `bulkMaxAge`.
- Cluster/port triggers use actual bad states, including persistent bad states.
  They no longer require a positive numeric change on a 1-to-0 failure transition.
- Filesystem capacity uses `logical_provisioned` and `logical_used`, with the `name`
  label actually exported. NAS status uses `name`; NAS performance uses `nas_id`
  (which currently contains the NAS name). No nonexistent appliance label is used.
- NAS/filesystem performance discovery includes all metrics listed by their
  collectors. Latency is converted from microseconds to milliseconds. Validate
  units and values against PowerStore Manager during the pilot.
- NAS state and Metro witness state/connectivity have triggers. Physical capacity
  and appliance latency thresholds are host macros. Service tags are discovered
  per appliance to avoid ambiguous extraction from multi-appliance arrays.
- Large raw HTTP responses are not stored in history. Dependent items store the
  extracted data. Existing item keys are retained where their meaning is unchanged.

## Migration and rollback

The new template has a different name and UUIDs. Importing it does not overwrite the
6.0 template. **Do not link both templates to the same host:** many item keys overlap.
Pilot on a separate host with notifications disabled, then plan the production
cutover and history retention. Unlinking and clearing the old template removes its
inherited items and their history; do not do that without an explicit retention
decision. Unlinking without clearing can leave items that conflict with the new
template. A separate new host is the simplest migration when preserving the old
host's historical data matters.

The existing Grafana dashboard targets legacy names/keys; review its queries when
using the new template, particularly NAS/filesystem and per-appliance service tags.

For rollback, keep the old host/template available and return polling to it. The
exporter additions preserve existing metric names and paths. The old template does
not use the new health metrics and retains its known trigger/mapping limitations.

## Pilot acceptance checks

- Import into Zabbix 7.0.24 and confirm all expected dependent items are supported.
- Compare discovery counts, capacity, latency and bandwidth with PowerStore Manager.
- Interrupt array access while leaving the exporter running: collector/bulk failure
  alarms must fire even if HTTP endpoints still respond.
- Stop bulk refresh or replay an old archive: stale-source alarms must fire and
  stale performance samples must stop updating. Restore access and verify recovery.
- Test an untrusted/mismatched exporter certificate; HTTP checks must fail.
- Test NAS/FC/Ethernet monitoring only where those resources are present; configure
  excluded ports and maintenance behavior before paging operators.

See [collection health](../../docs/collection-health.md) for metric semantics and
remaining limitations, including startup-only inventory and non-bulk source age.

## Filesystem field availability

The Zabbix 7 template includes the twelve common filesystem performance fields.
Six block/mirror write metrics are omitted from its default prototypes because
the validated bulk schema does not supply them. The exporter emits these optional
fields only when present in the source; it never substitutes zero. Add custom
items only after confirming those fields exist on the target array.

Capacity uses five-minute bulk samples when bulk mode is enabled. Existing
`powerstore_cap_last_*` selectors refer to the latest sample; the exporter does not
substitute current values for daily maxima. Non-bulk requests default to
`capacityInterval: Five_Mins`; see [collection health](../../docs/collection-health.md)
for the migration from the previous daily default.
