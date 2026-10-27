# Replication and monitoring review — 2026-10-27

## Implemented

The existing cluster endpoint now includes all replication sessions: Asynchronous,
Synchronous and Metro_Active_Active, every resource type (including NAS parents and
filesystem children), and both source and destination roles. New types and state
strings remain visible. Session identity is array IP plus session ID, independent
of volume inventory. This does not add destination volumes to volume monitoring.

The existing Metro query is broadened to:

```
replication_session?select=*,replication_rule(id,name,rpo,alert_threshold)
```

Legacy Metro metrics and the session collector share this response with a
one-minute cache, including errors and concurrent requests. No separate rule,
per-session, or metrics/generate requests are added. The existing witness request
remains. Including all sessions can require more pagination requests; authentication
retries can also add requests. The cache is demand-driven, not a background poller.

The locally supplied PowerStore 5.0 schema defines the rule association. Dell
documents the nested select in its
[related-resource query guide](https://www.dell.com/support/manuals/en-uk/powerstore-3000/pwrstr-apidevg/including-data-from-a-related-resource-type-in-a-query?guid=guid-19d536c5-3c4f-4459-933c-aed616ff43de&lang=en-us).
The supplied live combined-query response was replayed locally on 2026-10-27:
all 100 supplied sessions were exported with collection success 1, and 58 had
embedded RPO targets without a separate rule lookup. The remaining 42 were Metro
sessions without rule references. This validates the response format and embedded
association on the supplied system. Exactly 100 rows may represent one API page;
the file has no HTTP pagination headers, so full-array coverage still needs a
live exporter check. A rejected query fails collection rather than falling back
to extra REST requests.

A subsequent larger-limit response contained 196 unique sessions (105 Metro,
82 synchronous, 9 asynchronous), including 96 IDs absent from the first sample.
Replaying that response without locally joining rules exported all 196 sessions,
91 configured RPO targets, and collection success 1. All supplied session states
were OK. The JSON file alone has no HTTP status/Content-Range headers; live total
coverage should still be checked against the response headers or Manager count.

New metrics use the `powerstore_replication_session_` prefix:

| Suffix | Meaning |
| --- | --- |
| `count` | Number of returned sessions, including destination sessions |
| `info` | Type, role, resource IDs, parent and rule IDs; value 1 |
| `state` | Raw current state as a label; value 1, not a health code |
| `data_transfer_state`, `data_connection_state` | Raw reported states as labels |
| `local_resource_state`, `auto_failover_state`, `error_code` | Optional diagnostics |
| `rpo_seconds`, `rpo_available` | Configured target and explicit availability |
| `alert_threshold_seconds` | Configured delay before an RPO compliance alert |
| `last_sync_timestamp_seconds`, `next_sync_timestamp_seconds`, `estimated_completion_timestamp_seconds` | Reported timestamps where supported |
| `last_sync_duration_seconds`, `progress_percentage`, `failover_test_in_progress` | Optional operation details |

Every per-session metric includes session_id; every metric includes IP. Collection
health uses collector="replication". Success means a usable response, not healthy
replication. A valid empty list succeeds with count 0; API/validation errors fail.
RPO enums convert explicitly to seconds; unknown values fail collection health.
Filesystem children can inherit a parent's embedded rule within the same response.
Missing RPO sets availability 0, never a fabricated zero target. A reported Zero
target is not proof of current protection.

Missing optional measurements are omitted. The schema excludes NAS-parent sync
times, duration, progress and failover-test details. Metro sync timestamps describe
asynchronous copy phases only. None of these are advertised as destination lag or
used to manufacture an RPO-compliance alarm.

The Zabbix 7 array template discovers sessions using its existing cluster master
item. It monitors state, type, role, connection/transfer state and configured RPO,
with collection-health and missing-status alarms. Discovery fails when collection
fails, preserving existing discovered resources instead of marking all as deleted.
States other than OK/Synchronizing raise a warning, including transitions: use
maintenance handling for planned operations. Optional states display Unavailable
when absent. Interpret RPO with its availability item. NAS-host-only monitoring
does not include cluster sessions: retain an array host using the array template.
The legacy Zabbix 6 template is unchanged.

## Volume selection and related fixes

Normal volume monitoring retains Primary/Clone and is_replication_destination=false.
The existing volume status response now refreshes volume name/appliance/source
inventory without another REST query. New sources and role changes are picked up
while /volume is polled. A concurrent performance collector can use the previous
immutable snapshot once; the next scrape uses the refreshed inventory. A failed
status request does not update inventory. This is not instantaneous failover tracking.

Both Metro volume ends have is_replication_destination=false in the supplied
schema. Both remain monitored: Metro_Non_Preferred is not an ordinary destination.

Missing/invalid volume status values no longer become fabricated zeros. Unknown
legacy witness states now map to -1 instead of healthy 0. Witness connection
emission checks the connection's state instead of the parent's state.

## Other findings and remaining coverage

The review covered collectors, transport/inventory, bulk decoding/cache, routes,
health, logging/configuration and monitoring template contracts. It is code/schema
review and synthetic verification, not proof that every PowerStore feature is
covered or a replacement for live acceptance tests.

| Finding | Consequence / follow-up |
| --- | --- |
| Supplied bulk sample has no session-ID CSV or RPO/lag fields | Its four copy CSVs aggregate by appliance, cluster, remote system or volume group. They cannot reliably be assigned to individual sessions; no REST performance fallback was added. |
| Only ten bulk CSV modules are decoded | Host, host group, initiator, node, protocol, extra space and copy metrics in the supplied archive are not all exported. These are additional features, not session status. |
| NAS/filesystem/volume-group inventory remains startup-only unless inventoryRefresh is configured | Ownership, new objects and group role changes still require refresh. Reusing volume status fixes the volume path only. |
| Filesystem health remains array-wide on NAS host endpoints | Unused test filesystems can keep both collectors at 0 and propagate that health to otherwise working NAS hosts. No suppression or changed zero policy was introduced. |
| Missing bulk volume rows retain the existing zero-fill policy | Collection success remains 0; a filled zero does not prove a volume is idle. |
| Port/volume-group/cluster validation is less complete than NAS/hardware validation | Some missing fields/objects can still yield incomplete data without failed collection health. Volume numeric validation was fixed here; broader checks need dedicated fixtures and semantics. |
| Several legacy `_id` labels actually hold display names | Renames change series identity; correcting these requires a template/history migration. New session metrics use actual IDs. |
| Legacy latency HELP mixes microseconds and ms; bandwidth HELP mixes bytes/s and bps | Check displayed units against firmware before changing numeric values or template multipliers. No unverified rescaling was introduced. |
| General array alerts, protection compliance, remote-system health and some protocol/network objects are not collected | Session status alone is not comprehensive array health/protection monitoring. Additional REST endpoints need a separate traffic/coverage decision. |

## Verification and rollout

### Pagination verification

All 21 public REST collection paths (including inventory, cached NAS, sessions,
Metro and witness) are exercised with a server-imposed 100-row page size for 196
objects. The shared client follows Content-Range rather than incrementing offset
by the requested apiLimit. Filters, selected fields and stable ID ordering survive
all pages. Limits 1, 100, 2000, omitted and above 2000 are tested with 2205 objects.
apiLimit is a page size; reqLimit controls concurrency, not returned row count.

Repeated IDs, missing IDs in paginated responses, invalid/overflowing ranges,
changed totals, inconsistent row counts, missing headers on partial responses,
and failed later pages reject the whole collection. A headerless 200 final page
is accepted only when its row count exactly fills the previously advertised total.
Authentication renewal on a later page preserves its offset. The 10000-page
safety bound returns an explicit error rather than a truncated result.

All 11 metrics/generate methods are tested as POST actions with no GET pagination
parameters; partial responses fail without extra generate requests. Login and bulk
enable are also action/session endpoints, not collection queries. Bulk download
reads one complete archive: a 2205-row CSV is tested with one download request.
HTTP 206 or an unexpected Content-Range is rejected; existing archive integrity,
freshness and 512 MiB compressed/expanded size bounds remain enforced.

The private 196-session response was also replayed through a local TLS server
forcing 100+96 rows. The real collector fetched two pages and exported all 196
sessions, 91 embedded RPO values and success 1 without local rule completion.
A synthetic integration test parses actual Prometheus HTTP exposition from both
session and Metro collectors: two shared session page requests, 105 Metro witness
states and no separate rule requests. A failed second page yields failed collector
health and no misleading partial session count or session metrics.

These checks assume the array reports pagination honestly. HTTP 200 without a
Content-Range on the initial page is the API's complete-response contract; a
silently truncated response that falsely claims completeness cannot be detected
from that response alone. Concurrent changes that replace objects without changing
the total or producing duplicate IDs also require a later poll for convergence.

Synthetic tests cover all session types, source/destination roles, NAS parents,
filesystem children, future enums, optional fields, RPO inheritance, unit conversion,
malformed/duplicate sessions, shared paginated requests, cached failures and source
role changes. Template contracts check selectors against collector descriptors.

Local Go 1.23.12 validation: `go test ./...`, `go vet ./...` and `go build` passed.
Race testing could not run on this Windows host because the C compiler (gcc) is
absent. The existing Linux CI workflow runs `go test -race ./...`; its result for
these uncommitted changes is not yet available.

Build/restart the exporter and reimport the Zabbix 7 array template. Check the
existing /metrics/<array>/cluster response and request logs: one shared session
query per cache refresh plus pagination, no per-volume/session lookups. Compare
counts/types/roles with PowerStore Manager on both arrays, including NAS children.
Verify embedded RPO, failure/recovery, planned failover, Zabbix import and discovery
retention before relying on alarms. Production logs, schemas and archives stay
outside version control.
