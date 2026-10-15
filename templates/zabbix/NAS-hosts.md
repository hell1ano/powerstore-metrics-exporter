# One Zabbix host per source NAS

Import `zbx_export_nas_hosts_7.0.yaml` into Zabbix 7.0. It contains a discovery
template and a NAS monitoring template. Deploy the matching exporter first.

1. Configure every candidate PowerStore in the same exporter's `storageList`.
2. Set `exporter.inventoryRefresh: 15m` (or your preferred longer interval).
   The default is startup-only: without periodic refresh, moves require a restart.
3. Create **one discovery host per exporter**, and link **Dell PowerStore NAS
   discovery by exporter HTTP**. Do not link it separately to each array host.
4. Set `{$POWERSTORE.EXPORTER.URL}` on that discovery host to the exporter URL
   reachable from the Zabbix server/proxy, with no trailing slash. Discovered hosts
   inherit this macro. No agent interface is required.
5. Confirm `/discovery/nas` returns the expected sources. Discovery runs every
   15 minutes; adjust `{$POWERSTORE.NAS.DISCOVERY.INTERVAL}` if needed. This request
   reads cached inventory and does not fetch inventory from PowerStore.

The prototype creates hosts in **PowerStore NAS**, with the NAS name as their
visible name. Each child monitors NAS status/performance and its filesystem
capacity/performance. The array template remains available for other components.
Pilot the child hosts before retiring old NAS/filesystem discovery and items on
array hosts; running both produces duplicate monitoring. No existing items or
history are deleted by this change.

## Source selection and movement

NAS names must be unique across the configured arrays and remain unchanged during
a move. The technical host name is `powerstore-nas-<SHA256 of NAS name>`. One
discovery rule owns that stable identity, so a changed array address or NAS UUID
does not create a replacement host. The exporter routes the stable child URLs
`/nas-hosts/<key>/nas` and `/nas-hosts/<key>/file` to the current eligible source.
Ownership changes become visible after inventory refresh. Host tags update after
the next Zabbix discovery run.

NAS async and NAS Metro use the same existing eligibility rule: exclude a NAS
when `is_replication_destination` is true **and** `protection_policy_id` is a
nonempty string. A retained destination flag with a null/empty policy remains
eligible. This is distinct from volume Metro monitoring, which covers both sides.

Duplicate eligible names or incomplete inventory return HTTP 503 rather than
choosing an array or returning an empty discovery result. Independent array
refreshes can briefly produce this condition during failover. A genuinely absent
source is omitted; lost hosts are disabled after one hour and never automatically
deleted. Rediscovery can therefore reuse the host and its history.

Zabbix does not adopt an unrelated manually created host or a host owned by
another discovery rule just because its visible name matches. Review existing
hosts before enabling this bundle; resolve collisions deliberately and preserve
their history. Keep the same discovery parent across moves. Separate exporter
scopes and NAS renames need a planned migration.

NAS host identity and name-keyed NAS item history remain stable. Filesystem items
are keyed by filesystem UUID: if a move changes that UUID, replacement filesystem
items are discovered, with old item retention governed by their discovery rule.

## Collection and pilot checks

Child requests share a 60-second collection cache per array and component,
including failures. Each response contains only the selected NAS's measurements.
Collection-health items are explicitly **array-level**: a failure involving
another NAS on that array can mark collection incomplete. Filesystem data without
known NAS ownership remains available at the array `/file` endpoint and cannot
be safely assigned to a child host.

Local tests cover source movement, duplicate names, incomplete inventory,
metric isolation, cache concurrency and template metric selectors. In the live
pilot, import into Zabbix 7.0.24, verify one host per eligible source and inherited
URL, then move a NAS and confirm the same Zabbix host ID is retained, metrics
resume from the new array, and destination-only NAS objects are not discovered.

See [Zabbix host prototype documentation](https://www.zabbix.com/documentation/7.0/en/manual/discovery/low_level_discovery/host_prototypes)
for ownership, inherited macros and lost-host behavior.
