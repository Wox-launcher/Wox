# File search architecture

## Refactor target

All platforms use the same filename column store, query grammar, fuzzy/pinyin
matching, and recovery logic in the ordinary Wox process. Full builds and
compaction run in-process on a bounded background worker. No additional build
helper is shipped.

Windows' privileged service only enumerates NTFS MFT records and reads USN
changes over a bounded binary named-pipe stream. It does not own the filename
index or execute searches. Windows service mode retains fixed-NTFS-volume scope;
other modes retain configured roots. macOS uses bulk directory reads and
FSEvents. Linux retains its watcher/reconcile strategy. Platform adapters must
not depend on query policy or plugin implementations.

Content extraction and its SQLite/FTS database remain independent. Its configured
roots can differ from filename roots. Candidate enumeration and stale-content
reconciliation use the common directory view, not a second SQL filename index.

## Columns and durability

`column` stores entry identifiers, parent rows, interned names, name-to-row
postings, optional metadata, and source versions in pointer-free arrays. Full
paths are materialized only when required. A versioned, checksummed snapshot is
mapped read-only; callers hold a store read lease while borrowing mapped names.

The append-only journal commits changes and their source cursor together before
exposing them in the overlay. Recovery ignores an incomplete final frame but
rejects complete corrupt frames. Compaction writes another immutable generation
and atomically publishes a small manifest. Stale builders cannot discard newer
mutations. A failed directory sync after a successful rename must retain the
generation referenced by the manifest.

The scheduler must coalesce builds, replay events through a known boundary, retry
stale publications, and bound delta growth. Defaults are a 64 MiB journal
or `min(100000, max(10000, base entries / 10))` changed entries before compaction.
Partial enumeration and access denial never imply an empty directory.

Every interned name has a 64-bit rejection mask. Non-ASCII masks include
normalized literal characters and every pinyin reading, prepared once before
publication and regenerated during compaction. The filter does not choose a
pronunciation or replace exact scoring. Unprepared older masks safely bypass
non-ASCII rejection.

Default queries match filenames only, including pinyin; a slash or backslash
explicitly opts into path matching. Fewer than the requested number of filename
hits never triggers an implicit parent-path scan. The filename executor compiles
predicates once, filters distinct names with transposed character/repetition
bitmaps, applies conservative score bounds, and retains bounded ranked groups.
It resolves paths only for competitive rows. Explicit path matching has a
separate execution path and is not covered by filename-only latency claims.

Snapshot format 3 persists the two scalar scoring-hint columns, character and
repetition bitmaps, non-ASCII bitmap, frequent-name IDs, and path-ordered postings.
They belong to the same checksummed immutable generation as the filename facts.
Open borrows these arrays directly from the read-only mapping; it does not
allocate another query index on the Go heap. Builders prepare the arrays before
publication. Legacy version 1/2 reads remain write-free and supported; the ordered
`20261013_filesearch_query_columns` migration upgrades published generations before
engine startup. Replacement preserves the manifest, journal, cursors and policy,
and an interrupted migration can safely retry. Orphan generations are not migrated.

The existing hidden-launcher lifecycle releases pinyin memoization after ten
seconds. At thirty seconds it clears memoization again, drops prepared text and
the shared pinyin dictionary, and asks filename stores to release clean mapped
pages under their read-lease locks. Windows uses `VirtualUnlock` on the file
mapping (the documented `ERROR_NOT_LOCKED` result is expected); Unix uses
`MADV_DONTNEED`. Mappings remain valid and subsequent access faults pages back in.
Durable delta records are not discarded. This reduces resident memory rather
than merely moving allocations outside Go's accounting; it does not guarantee
that the OS removes the underlying data from its system file cache.

Cold queries must therefore be measured separately: process startup, first
query, repeated queries, hidden idle, and first query after idle. A copied index
or freshly validated mmap is already warm in the OS file cache, so those tests
must not be described as cold-disk or reboot measurements. Reading and validating
all sections at startup also incurs real I/O, even though only borrowed arrays
remain afterward. Pinyin dictionary reconstruction after idle remains a latency
cost. Numeric performance results are diagnostic and do not fail smoke tests.

`directory.db` stores configured-root and traversal lifecycle metadata only.
Filename facts no longer have SQL entries/FTS tables. Content candidates use
stable shard/entry cursors, so compaction can reorder rows between pages.
Dynamic-root promotion publishes the destination before committing ownership;
startup completes any missing parent tombstone from committed root metadata.

## Transport

`filesearchservice/protocol` is independent of the index and native APIs. Version
3 frames contain a kind, payload length, session, and sequence. File records
carry IDs, parents, USNs, reasons, names, and explicit metadata-validity flags.
Frames are limited to 1 MiB and normally batched around 256 KiB. Cursor frames
follow all records from their corresponding native read. A history gap is an
explicit reset request. Slow consumers must apply backpressure, not create an
unbounded service queue.

The v3 management pipe only accepts health and update requests; it has 16
bounded client slots independent of the fact pipe's 32 streams and single
enumeration slot. Protocol-incompatible installations use the existing elevated
repair/update action. The service has no access to a user-supplied index path.
Ordinary Wox restores its native mappings, serializes builds/compaction across
volumes, retries transport failures, and retains the old generation until a
complete catch-up. Metadata-only USN reads advance a volatile cursor without
writing another journal checkpoint, preventing self-triggered writes.

## macOS enumeration

`directoryread` requests names, types, modification times, permissions and file
lengths through `getattrlistbulk` in 256 KiB buffers. Returned attribute bitmaps
determine metadata validity; absent fields fall back to `Lstat`. Unsupported
filesystems use ordinary directory reads. Link attributes describe the link
itself, preserving cycle avoidance. The decoder follows Apple's
[packed attribute contract](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/man/man2/getattrlistbulk.2)
and is independently fuzzed on all platforms. FSEvents remains the change feed.

## Validation and comparison

The core store and protocol have deterministic tests and decoder fuzz targets.
Native smoke runs fixed temporary filesystem workloads and reports timing,
index size, and memory without a numeric pass/fail threshold. Wrong search
results, broken persistence, and failed cleanup remain functional failures.
Real-directory and external `fd`/`rg` comparisons will remain opt-in through the same
smoke entry point. Ordinary smoke must not install or attach to the production
machine-wide service.

The pre-refactor commit is `243fd98894e3c9f746522ee27440b981b2607491`.
`test/filesearchbench` provides deterministic 100,000-file, index-only fixtures
for both legacy implementations and the replacement. Retained legacy test
binaries and JSON observations live in `artifacts/filesearch-comparison/` at the
repository root. Run the binaries serially without concurrent builds or tests.
Compare result counts as well as timings: the legacy SQLite recall and Windows
fuzzy recall intentionally differ. Windows timings use QPC, and its reported
process memory is private working set, not total resident mapped pages.

## Integration status

Implemented and tested: common column builder, checked mmap format, durable
overlay and publication, binary frame/record codec, cancellable native MFT/USN
fact readers, bounded named-pipe endpoint/client, and source adapter catch-up.
The production engine and configured-root scanner now use common columns and
the shared short-fuzzy/pinyin/AND/phrase/extension/path/wildcard provider. Windows
service mode also searches those columns in ordinary Wox. Service-owned storage,
search and JSON snapshot transfer have been removed. macOS bulk enumeration is
wired into planning and scanning; Linux uses the portable directory reader.
The registered migration retires the derived SQL filename cache. Content scope
and ignore rules are checked independently for crawls and incremental upserts.

Content now shares native and configured directory facts. NTFS-covered content
roots do not cause a second traversal. Metadata-only USN changes still trigger
content extraction even when they do not mutate filename columns. Notification
bursts coalesce into content-root reconciliations after 8,192 pending changes.
Unavailable native shards fail candidate reads rather than presenting an empty
view to stale-content cleanup. Unreadable configured subtrees preserve their
previous filename facts; confirmed missing paths still remove stale facts.

## Smoke entry points

Run from the repository root:

```
make smoke
make smoke launcher/plugin/filesearch
```

Cases 004 and 005 run by default: 10,000 filename fixtures and 1,000 content
fixtures in separate configured roots. They emit the generic `SMOKE_REPORT`
summary. Timings, disk bytes, and memory readings have no numerical failure
threshold. Incorrect results, crashes, failure to complete within the operational
timeout, and failed cleanup still fail the suite.

Optional environment variables use the same entry point:

| Variable | Purpose |
| --- | --- |
| `WOX_FILESEARCH_SMOKE_FILES=1000000` | Increase case 004 to one million temporary files; multiples of 100 between 10,000 and 1,000,000 are accepted. |
| `WOX_FILESEARCH_SMOKE_ROOT` and `WOX_FILESEARCH_SMOKE_QUERY` | Enable case 006 for a real filename root and a known matching query. |
| `WOX_FILESEARCH_SMOKE_CONTENT_ROOT` and `WOX_FILESEARCH_SMOKE_CONTENT_QUERY` | Enable case 007 for real document contents and a known matching query. |

Real-root cases read existing files without modifying them and restore the
isolated app settings afterward. They optionally report `fd`/`rg` found on PATH;
missing tools are reported rather than failing the case. Wox ranking, ignore
rules, top-result limits and content extraction differ from the tools, so these
are diagnostic observations, not an assertion of equivalent workloads. The
opt-in cases have an eight-minute operational timeout; select individual cases
for large roots. Neither ordinary nor opt-in smoke attaches to the installed
privileged service. The old standalone real-index binaries and four bespoke
Makefile targets have been removed.

## Measured comparison

Windows/amd64, Go 1.27.2; frozen old implementations from the baseline commit
above versus the shared columns. Each implementation ran serially three times
with the same synthetic 100,000-file fixture. Values below are medians of those
three runs; queries use three warmups and twenty measured samples per run.
These are index-only observations, excluding native enumeration, service IPC,
and UI. The SQL backend stores metadata that this filename-only column fixture
does not request. Do not interpret build times as end-to-end filesystem scans.

| Metric | Previous SQLite | Previous Windows columns | Shared columns |
| --- | ---: | ---: | ---: |
| Index bytes | 47,669,248 | 3,231,546 | 3,232,196 |
| Build/publish (ms) | 8,556.47 | 34.04 | 58.54 |
| Go heap after build (bytes) | 2,076,912 | 644,504 | 1,836,184 |
| Private working set after build (bytes) | 24,588,288 | 4,538,368 | 6,070,272 |
| Private working set after queries (bytes) | 25,194,496 | 6,164,480 | 7,118,848 |

Private working set is the Windows monitor's metric, not total resident memory
or mapped-file residency. These are absolute test-process observations, not
combined Wox-plus-service memory or peak build memory. Heap samples force GC.
Shared columns prepare pinyin masks during build; the previous Windows engine
initializes its pinyin dictionary during queries, so after-build memory is not
an equivalent pinyin initialization point.

Query p50 / p95, in milliseconds:

| Query | Previous SQLite | Previous Windows columns | Shared columns |
| --- | ---: | ---: | ---: |
| `report` | 23.448 / 28.153 | 0.107 / 0.121 | 0.250 / 0.323 |
| `document-000104` | 2.118 / 3.251 | 0.273 / 0.286 | 0.558 / 0.590 |
| `rpt` | No results | 0.202 / 0.552 | 0.291 / 0.670 |
| `xiangmu` | 24.419 / 30.845 | 0.182 / 0.371 | 0.151 / 0.345 |
| `xm` | No results | 0.111 / 0.141 | 0.127 / 0.303 |
| `项目` | No results | 3.266 / 6.978 | 0.121 / 0.143 |

The unique document query returns one result; other supported queries return
the top 50. Empty SQL results are recall differences, not speed wins. Shared
columns reduce this fixture's disk size by 93.2% relative to SQLite and are
within 650 bytes of the previous Windows format. They are not universally
faster than the specialized Windows search: Latin queries cost roughly another
0.1–0.3 ms here, while the Chinese literal query is substantially faster.
After-query private working set is about 0.91 MiB higher than the old Windows
test process. Raw observations and the reproducible median summary are under
`artifacts/filesearch-comparison/` (`before-*-{1,2,3}.json`,
`after-column-unicode-{1,2,3}.json`, `median-comparison.json`).

## Verification limits

Windows affected-package tests and targeted race tests pass, including corrupt
frame/journal checks, interrupted replay, stale generation publication, pinyin
recall, shared content scopes, directory rename/delete notifications, bounded
content queue recovery and unreadable-subtree retention. Windows service 2.8.3
builds successfully. The service was not installed or elevated on this host.
Live privileged MFT/USN integration still needs an installation test.

Portable column, protocol and bulk-decoder packages cross-compile for Linux
amd64 and macOS arm64 with CGO disabled; the bulk decoder was fuzzed. The macOS
CGo `getattrlistbulk` bridge and native macOS/Linux watchers require execution
on their respective operating systems. Cross-compilation is not native runtime
verification. NTFS identity still follows the prior single-link-per-file-ID
behavior; this change does not add exhaustive native hard-link enumeration.

Four existing symlink-fixture tests cannot run on this Windows host because it
lacks symlink creation privilege. The new cache migration test passes; the full
migration package encounters an unrelated temporary `crash.log` cleanup lock.
The core suite reports all cases PASS but its command remains alive afterward
and had to be stopped; this is recorded in `final-core-ui-unit.txt`. The separate
Go UI unit suite completes successfully (`final-ui-unit.txt`).
The complete Windows native smoke suite completed with **130 passed, 3 skipped**
(`final-full-smoke.txt`). The skipped cases were the two opt-in real-root cases
and the per-case performance reference already collected by the full runner.
No filename/content performance threshold was used as a pass/fail condition.

End-to-end fixed-fixture observations from that full run:

| Metric | Filename: 10,000 files | Content: 1,000 documents |
| --- | ---: | ---: |
| Settings action to completed index | 768.19 ms | 1,561.65 ms |
| Completed UI query p50 / p95 | 116.36 / 126.49 ms | 116.90 / 121.31 ms |
| Index disk bytes including live journal/WAL | 3,167,903 | 1,355,296 |

The old native filename fixture reported 814.87 ms to index, UI query
116.24 / 129.35 ms, and 11,321,448 disk bytes. The old content fixture reported
1,587.98 ms, UI query 115.82 / 129.97 ms, and 897,976 disk bytes. These individual
UI observations are not a controlled performance distribution. WAL checkpoint
timing affects live SQLite disk sizes. Full-suite process history differs from
the old targeted captures, so their absolute process/heap samples must not be
used to attribute a memory regression or saving to file search. Use the serial
index-only table above for a more controlled backend comparison. The complete
UI memory samples remain in `final-full-smoke-reports.json` for transparency.

The updated file-search package, including both opt-in entry points, also
completed with **7 passed** against a controlled repository directory
(`final-filesearch-optin-smoke.txt`). Both filename and content queries found
the expected file; optional tool metrics were emitted. The million-file scale
switch is available but was not exercised in this run.
`rg` enumeration and content matching both executed successfully. `fd` was not
installed, so its unavailable-metric path was verified rather than an actual
`fd` run.

## Whole-volume status timeout correction

Status/Settings originally counted visibility by walking every entry's ancestry
through the sorted ID column. On a read-only production snapshot with 2,304,940
rows, the developer build (`all=-N -l`) hit the two-second status deadline before
finishing, so the revision cache never became valid. Parent-directory reachability
is now memoized for the duration of a count; files use their scalar parent rows.
The same snapshot completed in 337.10 ms. A 309,645-row snapshot completed in
26.64 ms versus 363.53 ms, with identical counts. Journals and production files
were not modified (`status-count-readonly.json` records the observations).

Native `f status` diagnostics now read the volume-backed index used by filename
search and Settings, including volume roots, feed cursors and top-root counts.
They no longer substitute the possibly empty configured-root index. The fixed
10,000-file smoke case now checks the actual rendered diagnostic report's counts
and rejects a deadline error. Counter tests compare against exhaustive ancestry
checks across missing parents, structural rows, deletion, reparenting and deltas.

Verification for this correction: affected backend/plugin unit suites and
focused race tests pass. The file-search native smoke package passes all five
default cases (two real-root cases skip by default), including the new status
count assertion. Full smoke stopped in the existing
`Test035LauncherEmptyQueryHidesToolbar`: the host rejected Alt+Space registration,
leaving a hotkey warning in the toolbar (32 passed, 1 failed before other packages
ran). This is recorded in `status-fix-full-smoke.txt`; no global hotkey behavior
was changed to bypass that failure. See `status-fix-filesearch-smoke.txt` for the
completed feature-specific run.

## Mapped query columns and idle memory comparison (2026-10-13)

This comparison is against the immediately preceding optimized filename query
implementation, whose query arrays were heap-resident, not the older SQL or
service-owned search. The frozen real index contains 2,636,569 base records,
794,841 distinct names and 14,974 overlay records across two volumes. Both arms
use exactly those snapshots and complete journal prefixes. A measurement-only
Go build overlay freezes the native source input; the actual Go UI, engine,
ranking and result materialization remain enabled. The installed fact service
is sampled separately while idle. Its enumeration/replay load is not exercised.
Original production index files are not modified.

Both UI binaries use the production optimization/allocator configuration with
`sqlite_fts5,wox_automation`, eight Go processors and heap profiling disabled.
Three fresh-process pairs alternate order. Each process runs 25 `f qqyy`
queries (23 results each), waits 45 seconds after hiding, and runs one resume
query. Every memory checkpoint is the median of three samples. Figures below
are medians of the three rounds. Working-set and private-commit figures sum
the Wox app and installed index service; Go heap describes only the app. The
working-set sum can count shared system pages twice and is not unique physical
RAM across the machine. Plugin-host processes and the test driver are excluded.

| Metric | Heap query arrays | Mapped query arrays |
| --- | ---: | ---: |
| Pre-query total working set (MiB) | 359.66 | 344.41 |
| After query block total working set (MiB) | 407.51 | 391.84 |
| Hidden idle total working set (MiB) | 333.05 | 172.72 |
| Hidden idle private working set (MiB) | 86.89 | 62.05 |
| Hidden idle private committed bytes (MiB) | 189.43 | 166.90 |
| Hidden idle live Go heap (MiB) | 37.32 | 14.86 |
| First UI query completion (ms) | 79.12 | 97.66 |
| Repeated UI query completion p50 (ms) | 77.10 | 78.22 |
| Repeated UI query completion p95 (ms) | 93.43 | 97.46 |
| First UI query after hidden idle (ms) | 73.51 | 78.77 |

The first sample is excluded from each repeated-query distribution. The p95 row
is the median of three per-round p95 values, not a pooled percentile. The mapped
arm's individual p95 values were 97.46, 96.20 and 144.76 ms; this tail variation
must not be hidden by the aggregate. UI times include query scheduling, result
materialization, semantics frames and automation polling, not only index lookup.
The private-commit reduction demonstrates a real removal of retained allocation;
the larger total-working-set reduction additionally reflects clean file pages
leaving the process working set. Those pages may remain in the OS cache.

Separate unprofiled backend runs alternate before/after/after/before and exercise
eleven query families for 25 rounds each. All 275 result/ranking fingerprints
match between the first pair. Repeated `qqyy` p50 is 3.00 ms in both arms; p95
is 5.22 versus 4.74 ms. Process-first `qqyy` takes 5.00/6.52 ms before and
6.28/12.75 ms after. With mapped pages and pinyin caches explicitly released,
some backend queries still exceed 10 ms (including an 11.02 ms `qqyy` and
14.44 ms `readme`). Broad/multi-term warm queries also exceed 10 ms in some
tails. This change does not establish an all-query, cold-query sub-10-ms claim.
The OS file cache is warm from copying/validation; these are not cold-disk tests.

The copied indexes grew from 135.36 to 156.20 MiB on disk. Their initial registered
query-column migration took about 0.97 seconds in the observed startup; subsequent
starts map the persisted arrays without rebuilding them. Migration/build peak
memory was not sampled. The normal production executable is built separately
without the automation tag or frozen-source overlay.

Raw per-process samples, individual query samples, preserved binaries, frozen
inputs and the UI runner are under `artifacts/filesearch-comparison/mmap-memory/`.
`ui-round-summary.json` retains all rounds; `ui-summary.json` holds the table's
medians; `backend-summary.json` retains every query family's warm p50/p95. The
opt-in backend diagnostic uses `WOX_FILESEARCH_REAL_INDEX`,
`WOX_QUERY_DIAGNOSTIC_QUERIES` (pipe-separated; `<idle>` releases query working
memory), and `WOX_QUERY_DIAGNOSTIC_OUTPUT` with:

```
go test -tags sqlite_fts5,filesearch_diagnostic ./util/filesearch -run '^TestReadOnlyNativeQueryDiagnostic$' -count=1 -v
```

Set `GOMAXPROCS=8` to match production on this host. Diagnostic reads copy the
source before opening a store, so journal-tail recovery never writes to the
production index. Copying and setup are outside query timing.

Validation for this change: affected column, fuzzy matcher, filename engine and
file-search plugin tests pass; migration and focused idle-release race tests pass.
The concurrent-read test checks mapped names through repeated page eviction;
the idle round-trip test preserves pinyin readings, ranking and durable overlay
hits. The native file-search smoke suite reports five passed and two opt-in cases
skipped. Numeric timings remain informational. Four unrelated existing Windows
symlink-fixture tests are excluded because this host lacks the required privilege.
The column package cross-compiles for Linux amd64 and macOS arm64; their idle
page-release behavior still requires native testing on those operating systems.
