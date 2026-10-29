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
FSEvents. Windows configured roots use the USN journal. Linux uses recursive fsnotify watches. Windows volumes without a journal use
ReadDirectoryChangesW subtree watches. Platform adapters must
not depend on query policy or plugin implementations.

Native NTFS incremental reads publish status changes after applying filename
records, independently of content extraction. A single worker coalesces these
notifications across volumes in 1 second windows without polling while idle or
blocking USN ingestion on UI consumers. File Search uses the existing status
subscription to refresh its toolbar file count from published index statistics.

Filename indexing prioritizes bounded memory, fast enumeration and low-latency
queries. Results use the default relevance ranking without user-selectable sort
orders. Size and modification-time sorting are not offered: the NTFS MFT/USN
fact stream does not include those attributes, and a second whole-volume
directory scan solely to populate them would delay index readiness. File previews
may still read the selected file's attributes. Filename ranking requires no
additional filesystem I/O.

On macOS, empty triggered File Search queries fall back to directory-index
modification times when OS recents are unavailable or empty. This bounded lookup
uses existing metadata, filters before limiting, and does not change ordinary
filename ranking or Windows/Linux OS recents.

Content extraction and its SQLite/FTS database remain independent. Its configured
roots can differ from filename roots. Candidate enumeration and stale-content
reconciliation use the common directory view, not a second SQL filename index.

Roots are only the configured default and user roots; there are no derived
dynamic roots or exclusion ranges. Deleting a root removes its metadata row and
its mapped filename shard. Independently configured roots are retained even when
their paths overlap: each one indexes its whole tree, and change signals route to
the longest matching root.

## Columns and durability

`column` stores entry identifiers, parent rows, interned names, name-to-row
postings, optional metadata, and source versions in pointer-free arrays. File
paths are materialized only when required; directory paths have mapped query columns. A versioned, checksummed snapshot is
mapped read-only; callers hold a store read lease while borrowing mapped names.

The append-only journal writes changes and their source cursor in one
checksummed frame before exposing them in the overlay. Scoped job writes skip
their own fsync (`ApplyUnsynced`); the synced cursor frame that `finishBuild`
writes before the feed cursor advances commits the whole run, so an incremental
run pays one fsync (`F_FULLFSYNC` on macOS) however many jobs it has. Recovery
stops at the first frame that does not decode: a crash can leave unsynced
frames as partial or zeroed pages, and those were never acknowledged, so their
events replay. Compaction writes another immutable generation
and atomically publishes a small manifest. Stale builders cannot discard newer
mutations. A failed directory sync after a successful rename must retain the
generation referenced by the manifest.

The scheduler must coalesce builds, replay events through a known boundary, retry
stale publications, and bound delta growth. Defaults are a 64 MiB journal
or `min(100000, max(10000, base entries / 10))` changed entries before compaction.
Partial enumeration and access denial never imply an empty directory.

Compaction returns its temporary Go heap pages after its build frame exits, on
success or failure. A large background compaction can finish after the launcher
has already performed its last hidden-state cleanup; relying on another hide
would leave the builder's released arrays resident until later GC/scavenging.
The published mapped columns and durable overlay remain owned by the store.

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
It resolves paths only for competitive rows. Explicit path matching intersects persisted directory character and ordered-pair
bitmaps, checks a conservative subsequence condition, then shares the directory
score across its direct files. Ordered pairs require relative order, never
adjacency: fuzzy paths can still span multiple components. Only competitive
results materialize owned strings. ASCII keys use byte searches; Unicode keys
keep normalization-aware matching. Wildcards and quoted-only paths without a
mandatory directory clause retain exhaustive tree traversal.

Directory display paths, normalized keys, rune counts, row ordinals, direct-child
bounds and filter bitmaps are mapped with the immutable generation. Filename
queries borrow these paths for reachability and recycle-bin checks too. A
revision-owned overlay supplies child edges and invalidates old paths only for
ancestry/name/type changes. Metadata updates preserve them. At most 4,096 affected
or newly created directories retain prepared live paths; larger moved subtrees
use uncached traversal, bounding retained overlay memory. Final scoring, AND
clauses, quotes, wildcard rules and ordering remain shared with exhaustive search.

`Store.ReadQuery` prepares the shared overlay on the first search after a change.
Background writes invalidate it without rebuilding it, so multiple file-change
batches while hidden collapse into one preparation when search resumes. Ordinary
`Store.Read` leases for replay, content hooks and status never prepare query data.
Cursor-only commits retain the prepared overlay while still journaling, syncing
and advancing the revision. Read leases pin its borrowed mapped names; concurrent
searches share one preparation, and mutation, publication or close releases the
previous overlay. No additional entry cache or pending-change queue is retained.

Snapshot format 3 persists the two scalar scoring-hint columns, character and
repetition bitmaps, non-ASCII bitmap, frequent-name IDs, and path-ordered postings.
They belong to the same checksummed immutable generation as the filename facts.
Open borrows these arrays directly from the read-only mapping; it does not
allocate another query index on the Go heap. Builders prepare the arrays before
publication. Legacy version 1/2 reads remain write-free and supported; the ordered
`20261013_filesearch_query_columns` migration upgrades published generations before
engine startup. Replacement preserves the manifest, journal, cursors and policy,
and an interrupted migration can safely retry. Orphan generations are not migrated.

Transient development format 4 snapshots are also readable without retaining
their discarded metadata-sort permutations. New publications retain format 3.

Snapshots also persist a child index: every parented row ordered by parent row,
so each directory's children are one contiguous range (4 bytes per row, demand
paged). Format 3 now has an optional eighteenth section for directory query
columns. Older 16/17-section generations still open without writes and prepare
missing columns on the heap. New builds and compactions persist all columns;
this unreleased format change registers no migration. Manifests, journals, source cursors,
content databases and unrelated files remain untouched.

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

`metadata.db` stores the `roots` table (status, progress and feed cursors) and
small `meta` values only. Filename and directory facts have no SQL tables:
traversed directories become structural column rows, so empty directories keep
their place in the hierarchy and history recovery lists them from the columns.
Content candidates use stable shard/entry cursors, so compaction can reorder rows
between pages.

The dirty queue promotes a batch to a root reconcile only for explicit root
signals (a change to the root path itself, a never-indexed root, or a failed
scoped reconcile retried at root scope). It does not count indexed
directories per flush or escalate large path bursts to whole-root scans; those
remain scoped reconciles.

As in fsearch, a metadata event on an existing directory relists only that
directory (a `directory` job). FSEvents file-level events, USN and fsnotify
already report each child's own change, so the relist trusts known child
directories, walks only children new to the index, and prunes only children
that vanished or became files. A directory that was created, renamed or
removed, or whose type is unknown, is still rescanned as a subtree. Siblings are
never merged into a recursive parent rescan: eight touched folders under `~`
used to rescan the whole home directory.

## Directory deltas and FSEvents acknowledgement

Unreadable-scope protection uses stable path IDs and memoized parent ancestry.
It must not compare every indexed row with every unreadable path: system-volume
scans can contain thousands of access-denied scopes. A run-owned one-second
heartbeat advances elapsed status even while diffing or committing produces no
file-count events, and stops before the next run begins.

A scoped directory reconcile stages only the newly observed scope, compares it
with the live columns, and commits inserts, updates and tombstones in one
journal frame. Unchanged records and structural ancestors retain their existing
metadata. Failed scans publish nothing; unreadable scopes retain their prior
facts. Deletion candidates come only from the scope's own stored subtree: the
writer walks the persisted child index plus overlay rows added since the base,
so a refresh costs time proportional to that subtree. The previous whole-shard
sweep materialized paths for every row and cost about 4.7 s per dirty scope on a
3.6M-row index; the subtree walk takes 1.5 ms for `~/Library/Logs` and about
250 ms for all of `~/Library`.

Deleting a directory writes only its own tombstone, like fsearch's dead range:
path resolution, search, counts and compaction already treat every row beneath
a dead (or now-file) directory as gone. Writing one tombstone per descendant
used to push large deletes past the journal frame and into a full republish.
Scoped walks do not descend through dead rows, so a later rescan does not
re-delete them. Whenever a change set revives a dead directory (exact-path
event, structural ancestry, or rescan), its stored children that were not
re-observed are tombstoned in the same frame, so recreating the path cannot
resurrect stale children. A diff larger than one 1 MiB journal frame is published as a new generation with
the changes applied, instead of failing every retry. Exact file events retain their direct-delta path. Only full builds, explicit whole-root replacements and threshold-triggered
maintenance publish complete generations.

FSEvents cursors travel with sealed dirty batches and advance only after durable
root finalization. Full/root reconciles capture the boundary before traversal,
never the live system event head at completion. The feed updates its cached
acknowledgements without restarting an unchanged watch set and filters history
already committed by each root. A partial reconcile cannot acknowledge a known
history gap. Subdirectory must-scan markers require a scoped recovery scan, not
repeated replay of the same incomplete history.

## Change feeds and history recovery

Every feed (FSEvents, USN, fallback) emits three kinds of signals: dirty paths,
`history_lost`, and `feed_unavailable`. A feed reports `history_lost` when it can
no longer say which paths changed. Causes include a kernel drop, an expired or
mismatched cursor, a watcher error or queue overflow, a moved root, and the
fallback feed's initial watch registration.

All feeds deliver through `changeSignalQueue`. Native callbacks never block. A
full channel turns into one `history_lost` marker per root. A helper goroutine
delivers that marker even if no other event arrives. Later signals for the same
root are dropped while its marker is pending, because the recovery relists after
those changes.

The scanner marks the root degraded and schedules a history recovery. A root
gets at most one recovery per minute; later requests are delayed, not dropped.
Recovery never re-crawls the whole root:

1. It lstats every indexed directory in parallel.
2. It relists the root plus every directory whose mtime differs from the index,
   or whose mtime is unknown.
3. Each of those directories is queued as a one-level `directory` relist (see
   above): new child directories are walked, vanished entries are pruned, and
   new or changed files are upserted.
4. The root returns to ready after the flush that applies those relists succeeds.

Creating, deleting or renaming an entry updates the parent's mtime on APFS,
NTFS, ext4 and btrfs. An in-place file edit that loses its event stays stale
until the file changes again. Only a missing root or an unpublished shard falls
back to a root reconcile. Startup restore requests the same recovery for indexed
roots whose cursor is stale.

Removals usually arrive after the path is gone, so their type is unknown. They
are applied as delete-by-path deltas. Deleting a directory row hides all of its
descendants, so the whole-root reconcile previously used for this case is gone.

There is no per-folder relist pacing like fsearch's `Pace`. Wox receives
per-file events, so a hot folder costs cheap `lstat` deltas rather than
repeated listings, and the dirty queue's 2 s quiet window, 5 s maximum wait and
backpressure tiers already bound how often any scope is processed.

Apps subscribes to the existing Windows filename index's resolved service
changes through a bounded in-process broker. There is no second fact stream or
filename index. Only caught-up volumes count as covered; a transport failure,
rebuild or source shutdown activates local subtree watches. Recovery releases
those watches and reports history loss to reconcile the transition gap. File
renames deliver the old and new paths, and directory moves invalidate both scopes.
Apps repairs history-loss signals through debounced directory reconciliation;
its periodic fallback reconciliation checks current coverage rather than the
startup snapshot.

Windows fallback uses one overlapped ReadDirectoryChangesW subtree read and one
64 KiB buffer per root, including newly created descendants. It does not enumerate
or register descendant directories, so large trees no longer exhaust the old
512-directory budget. Overflow reports history loss, missing roots are retried,
and removed watches cancel and join pending reads before releasing their buffers.
The original root identity is checked while idle because subtree notifications
do not report the watched root's own rename. Ignore policy still applies to
consumed changes; it cannot exclude subtrees from the kernel's recursive buffer.

macOS Apps uses FSEvents for default and user-added directories. Default app
roots no longer use root-only kqueue watches. Events inside a `.app` bundle
refresh that application; they never trigger a crawl inside the bundle. Event
admission and scoped repairs retain the configured depth, exclusions and ignore
rules, including the union of overlapping user and default roots.

Linux Apps and filename feeds share one reference-counted inotify instance.
Repeated paths consume one native watch; detaching a feed releases only its
references. The pool bounds unique directories to half of `max_user_watches`,
capped at 32,768 (4,096 if the kernel limit cannot be read). Other processes can
still exhaust the shared system quota. Recursive registration filters against
Apps scan depth and exclusions before enqueueing children, reads directories in
256-entry chunks, and bounds queued discoveries. Roots that exceed a budget or
hit `ENOSPC`/`EMFILE` report history loss and retain periodic reconciliation.
Apps checks fallback roots every 30 seconds; the filename feed reports partial
roots every 10 minutes. These are eventual recovery paths, not real-time coverage
of directories beyond the budget.

Neither Linux nor FSEvents allocates a Windows-style read buffer per directory.
Linux has one fsnotify read buffer per native instance, plus kernel watch records;
FSEvents owns a system-managed stream. Apps reserves 256 change signals per feed
on each platform, while bulk filename feeds retain their larger bounded replay
queue. Linux's shared pool has one 256-event delivery queue and one error slot
per subscribing feed, never per directory. Overflow is delivered separately from
that full event queue and triggers recovery even if no further event arrives.

## Full-build temporary memory

The builder retains distinct name strings only in its intern map during
ingestion. It records their final offsets, then allocates the packed name bytes
once at finalization. This avoids both a second live dictionary and repeated
growth of that byte slice. Names no longer referenced after a rename or deletion
retain their dictionary IDs, so finalization does not change postings or ranking.

Finalization drops the name map and tombstones after packing names, then drops
the parent-ID array and ID-to-row table after resolving parents, before allocating
postings. Posting offsets temporarily serve as scatter cursors and are restored
in place. Snapshot writes checksum and write borrowed query-column segments,
including their alignment padding, instead of allocating a serialized copy.
No forced GC or process-wide GC policy changes are used by these optimizations.

The builder's ID lookup stores only `row+1` in a power-of-two scalar table and
compares keys against the existing ID column. Zero denotes an empty slot, so
zero and all other 64-bit IDs remain valid keys. Linear probing uses a mixed
64-bit hash and a maximum 75% load; growth rehashes rows, while deletion packing
rebuilds the table only after every retained ID has reached its new row. Deleted
records still use source-version tombstones until finalization, preserving stale
event and resurrection handling. This table is private to the unpublished build;
query lookup and the persisted ID permutation are unchanged.

The opt-in Windows `TestNativeBuildMemoryDiagnostic` exercises the production
NTFS source, builder, pinyin preparation, durable write, validation and atomic
publication against an isolated temporary store. `WOX_BUILD_DIAGNOSTIC` names
a recording directory, `WOX_BUILD_VOLUME` selects the volume, and
`WOX_BUILD_OUTPUT` names the JSON report. Set `WOX_BUILD_CAPTURE=1` once to
record enumeration and catch-up frames; subsequent invocations replay those
bounded frames. `WOX_BUILD_LIVE=1` instead reads the installed fact service
directly without recording. Production indexes are never opened or changed.

Use `GOGC=50`, `GOMAXPROCS=8` and `GODEBUG=memprofilerate=0` to match the default
production runtime configuration on this host. Compile once with
`go test -c -tags sqlite_fts5,filesearch_diagnostic ./util/filesearch`, then run
the binary with `-test.run=^TestNativeBuildMemoryDiagnostic$`. Alternate separate
before/after processes serially with no concurrent builds or test suites.
Memory is sampled every 10 ms during the build; report heap, working set and
private commit separately. The diagnostic also samples a separate immediate-query
phase and records per-query latency. Full snapshot hashing runs only afterward,
so it cannot prewarm the mapping before those queries. The first two comparisons
below used the earlier build-only sampler and hashed before checking queries.
Replay comparisons verify every snapshot byte and ordered path/score fingerprints
for twelve query families. Live inputs can change between runs, so their hashes
need not match.

These diagnostics measure full filename-index construction in an isolated
backend process. They exclude UI, plugin hosts, the service process's own memory,
and content indexing. Replay timing includes local fact-file reads rather than
live MFT enumeration, and warm OS caches are not cold-disk measurements.

### Windows comparison (2026-10-16)

Baseline source is `0ec489ed2651e7b4c11e933fc7e42de4dc1fa45f`, with the same
diagnostic harness compiled into both arms. Go 1.27.2 windows/amd64, optimized
builds, `sqlite_fts5,filesearch_diagnostic`, and the runtime settings above were
used. Each scenario ran before/after/after/before/before/after in fresh processes,
without concurrent compilation or test suites. Each value below is the median
of three builds' elapsed times or sampled memory peaks, not a post-GC sample.

| Scenario / metric | Before | After |
| --- | ---: | ---: |
| Frozen facts: elapsed | 6.73 s | 6.29 s |
| Frozen facts: Go heap peak | 367.9 MiB | 313.4 MiB |
| Frozen facts: working-set peak | 542.5 MiB | 499.1 MiB |
| Frozen facts: private-commit peak | 447.2 MiB | 396.8 MiB |
| Live NTFS: elapsed | 16.05 s | 15.43 s |
| Live NTFS: Go heap peak | 363.8 MiB | 320.6 MiB |
| Live NTFS: working-set peak | 548.4 MiB | 500.8 MiB |
| Live NTFS: private-commit peak | 445.7 MiB | 397.9 MiB |

Frozen input contains 2,369,391 rows and 697,739 distinct names. All six snapshots
have SHA-256 `b8a090c67d69cb1d901b3c6b4e53e0055beb293705b21af36ea3f9add0905c77`;
all twelve ordered path/score query fingerprints match, including literal
Chinese, pinyin, multi-term, explicit path and no-hit queries. Live builds contain
2,365,682–2,365,822 rows as the volume changes; their inputs are not identical.

Live elapsed ranges were 15.20–16.13 s before and 12.61–15.90 s after. Working-set
peaks ranged 534.4–557.7 MiB before and 419.5–501.0 MiB after; private-commit peaks
were 431.8–454.5 MiB and 397.7–398.0 MiB. The memory reduction is consistent across
these samples; three runs do not establish a reliable speedup, and total elapsed
time is sensitive to disk/cache and scheduler variation. The isolated process's
absolute peaks should not be equated to the full Wox application's working set.

Column, filename engine, file-search plugin and fuzzy matcher tests pass, as does
the column race suite. Four existing symlink-fixture tests are excluded because
this Windows account lacks symlink creation privilege. Added cases cover deleted
name groups, late/missing parents, an empty index and query-section padding.
No new compatibility layer, migration, global GC tuning or forced collection is
part of the runtime change.

Raw results, frozen fact frames, baseline source overlay, binaries and the serial
runner are in `artifacts/filesearch-comparison/build-memory/`. Only
`final-replay-*.json` and `final-live-*.json` feed `summary.json`; earlier exploratory
or interrupted runs in that folder are excluded. `run-comparison.ps1` repeats
the final runs and `summarize.py` verifies snapshot/query equality and computes
the summary. Regression output is in `tests-final.txt` and `race-final.txt`.

### Compact build ID lookup (second round, 2026-10-16)

Separate diagnostic builds injected heap snapshots at phase boundaries using a
Go source overlay. These attribution runs enable heap sampling and force GC at
each boundary; their timings and peaks are excluded from performance comparisons.
The first-round implementation retained 221.9 MiB of live heap after ingestion.
Sampled allocation attribution identified about 72.5 MiB in the ID-to-row map,
27.8 MiB in the name map and 40.7 MiB in decoded, interned name strings. Remaining
allocations primarily belong to the scalar columns. Parent resolution reduced
live heap to 107.5 MiB, and completed query columns brought it to 146.6 MiB.
After writing, the heap snapshot can become unreachable, but without immediate
collection its memory overlaps the new mapped generation's resident pages.

The compact ID table occupies exactly 16 MiB for the recorded input's 2,369,446
pre-pruning rows. The same phase profile now retains 165.8 MiB after ingestion,
56.1 MiB less than before; later live-heap sizes remain 107.5/146.6 MiB. This
change removes duplicate ID storage and hash-map overhead without moving build
storage into another process or adding forced collection to production code.

The first-round implementation was remeasured as the baseline, with the same
optimized Go 1.27.2 runtime settings and serial alternating order as above.
These are independent three-run medians; compare within this table, rather than
using the earlier run's absolute timings with this run's candidates.

| Scenario / metric | First-round baseline | Compact ID table |
| --- | ---: | ---: |
| Frozen facts: elapsed | 4.56 s | 4.46 s |
| Frozen facts: Go heap peak | 313.4 MiB | 258.7 MiB |
| Frozen facts: working-set peak | 500.3 MiB | 435.4 MiB |
| Frozen facts: private-commit peak | 398.4 MiB | 333.4 MiB |
| Live NTFS: elapsed | 12.89 s | 12.68 s |
| Live NTFS: Go heap peak | 324.4 MiB | 247.5 MiB |
| Live NTFS: working-set peak | 499.0 MiB | 419.0 MiB |
| Live NTFS: private-commit peak | 395.4 MiB | 315.4 MiB |

Frozen ingestion alone increased from 1.25 to 1.34 s, while total build time
remained within the observed variation (4.45–4.58 s before, 4.41–4.54 s after).
Live total times ranged 12.37–16.50 s before and 12.36–12.69 s after; working-set
peaks ranged 486.7–511.0 MiB and 418.0–440.2 MiB, respectively. The result is a
repeatable memory reduction without an observed material total-time regression,
not a claim of statistically established throughput improvement. Live row counts
ranged 2,367,049–2,367,055. Each scenario still uses a fresh isolated index and
excludes the UI and service-process memory.

All frozen snapshots retain the SHA-256 above and identical ordered path/score
fingerprints for all twelve queries. The affected package suites and column race
checks pass with the same four environment-blocked symlink cases excluded. New
tests exercise wrapped collision chains, growth, zero/maximal/high-bit IDs,
packing after deletion, stale updates, resurrection and replacement. No snapshot
format or query algorithm changes are involved.

Artifacts are in `artifacts/filesearch-comparison/build-memory-round2/`, including
`summary.json`, `run-comparison.ps1`, `summarize.py`, baseline source/binaries,
`before-overlay.json`, diagnostic overlays and phase profiles. Only
`final-replay-*.json` and `final-live-*.json` feed the performance summary;
`profile-result.json`, `candidate-probe.json` and `after-profile/` are diagnostic
or exploratory results. The frozen fact stream is shared with the first round.

### Publication residency (third round, 2026-10-16)

Publication previously called the ordinary `Open` path after writing. Its checksum
and structural scans faulted the entire new mapping into the working set while
the now-unreachable build heap could still be resident. The new `writeAndOpen`
path retains the writer's 512-byte header, derived from structurally validated
heap columns, and requires the reopened header and every section checksum to
match that output. The file cannot supply its own trusted checksum authority.
Per-entry columns are verified through a reusable 64 KiB read buffer without
walking their mapped rows again. The name dictionary, posting offsets and query
filter columns are checksummed through the mapping to keep them warm for the
first filename query. Mapping bounds and query-column validation remain enabled.
Normal startup `Open` still performs full structural validation of existing files.

This reduces, rather than eliminates, heap/mapping residency overlap. It does not
force GC, trim the process heap, skip file verification or change the immutable
format. Stale-publication rejection, read leases, durable sync and manifest
replacement retain their existing behavior. A fully demand-paged experiment
reduced the build peak further, but increased the first `readme` query from about
6 to 22 ms in an exploratory pair; the selected implementation keeps name-side
columns warm as a compromise.

The baseline is the second-round implementation, rebuilt with the same enhanced
diagnostic harness as the candidate. Go 1.27.2, runtime settings and serial
before/after/after/before/before/after order are unchanged. Each metric is a
three-run median; earlier rounds' absolute timings are not paired controls.

| Scenario / metric | Second-round baseline | Selective publication verification |
| --- | ---: | ---: |
| Frozen facts: build elapsed | 5.31 s | 5.09 s |
| Frozen facts: build working-set peak | 406.3 MiB | 324.9 MiB |
| Frozen facts: build Go heap peak | 249.5 MiB | 249.6 MiB |
| Frozen facts: build private-commit peak | 333.2 MiB | 337.2 MiB |
| Frozen facts: build + immediate queries working-set peak | 407.7 MiB | 366.0 MiB |
| Frozen facts: first `readme` query | 8.73 ms | 15.38 ms |
| Live NTFS: build elapsed | 15.73 s | 14.20 s |
| Live NTFS: build working-set peak | 449.4 MiB | 344.8 MiB |
| Live NTFS: build Go heap peak | 283.3 MiB | 247.4 MiB |
| Live NTFS: build private-commit peak | 346.1 MiB | 329.7 MiB |
| Live NTFS: build + immediate queries working-set peak | 462.1 MiB | 386.2 MiB |
| Live NTFS: first `readme` query | 10.55 ms | 13.11 ms |

The controlled frozen-input comparison shows the intended residency reduction,
not a reduction in live column size or private committed allocation. The OS may
retain the verified file data in its cache; process working set is not unique
machine-wide physical RAM. Later queries still fault in needed per-entry pages.
Including all twelve immediate queries preserves a lower observed combined peak,
but the first query has a measured small latency cost. This is not a cold-disk
measurement or a guarantee about arbitrary workloads and long-running UI sessions.

Frozen build times ranged 5.10–5.75 s before and 4.98–5.16 s after; combined
working-set peaks ranged 399.4–462.5 MiB and 362.4–373.4 MiB. Live times ranged
14.94–20.00 s and 13.49–14.34 s; combined peaks ranged 456.6–463.9 MiB and
372.7–408.1 MiB. Live row counts ranged 2,369,392–2,369,406. All six frozen outputs
retain the same snapshot SHA-256 and ordered query fingerprints as earlier rounds.

Column, filename engine, plugin and fuzzy matcher suites pass, as do column race
checks, with the same four symlink-privilege cases excluded. New publication tests
cover every corrupted section, forged/rechecksummed headers, invalid heap rows,
invalid query IDs, and byte-identical normal reopen. Runtime changes are confined
to common column storage/publication; no platform mapping APIs were changed.

Artifacts are in `artifacts/filesearch-comparison/build-memory-round3/`.
`summary.json` includes build, immediate-query and combined peaks plus all query
latencies; `run-comparison.ps1` and `summarize.py` reproduce and check the results.
The source overlay and `before.exe` preserve the second-round control. Only
`final-replay-*.json` and `final-live-*.json` feed the summary; `probe-*.json` records
exploratory runs. The isolated processes do not include Wox UI or service memory.

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

Settings restores each native volume's last successful full-build duration from
`metadata.db` metadata and sums the currently indexed volumes. Active builds
report live elapsed time; journal replay, compaction and failed builds do not
replace the completed statistic. Manual index reset clears these duration keys.
Older native indexes without recorded timings remain unknown until their next
successful full build; restoring mappings is not counted as building an index.

Native full builds publish scalar live counts after each fact frame, independently
of the still-unpublished filename generation. A build-scoped 250 ms heartbeat
updates toolbar elapsed time even while waiting for facts or finalizing columns;
it is stopped and joined at completion or cancellation. Progress reads worker
state only, so they do not recount mapped entries or walk disk files. Published
index statistics remain separate. Serial volume builds share one run clock and
keep completed volumes' live counts while the next volume is being ingested.

## Configured-root scan streams

Subtree and direct-files scans list directories in parallel workers and emit
pre-resolved column records. Each queued directory carries its own and its
parent's entry IDs, so a child's `ParentID` comes from the listing that produced
it and every path is hashed exactly once, in a worker. The single writer checks
scope ownership once per batch, adds synthetic ancestry for the scope only, and
inserts the records. It does not re-validate, re-hash or walk ancestry per path.
Path-based `Entries` remain for reconciler snapshots and direct deltas.

On a 430,622-entry `~/Library` tree, serial writer time fell from 2.13 s with
path entries to 104 ms with records, while the parallel walk took 24.4 s.

## macOS enumeration

`directoryread` requests names, types, modification times, permissions and file
lengths through `getattrlistbulk` in pooled 256 KiB buffers. Returned attribute
bitmaps determine metadata validity; absent fields fall back to `Lstat`. Darwin
never returns file lengths for directories, so a directory is complete without
one: it would otherwise cost an extra `Lstat` per directory during full scans.
The index stores zero as every directory's size on all platforms, so bulk-listed
children and `Lstat`-ed scope roots produce identical rows. Unsupported
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

## Path-query execution comparison (2026-10-18)

The frozen two-volume fixture contains 2,636,569 base records and 14,974 overlay
records. The before/after comparison uses Windows, optimized test builds with
`sqlite_fts5,filesearch_diagnostic`, `GOMAXPROCS=8`, and a five-result limit matching
the global-search setting on this machine. A Go build overlay selects the previous
executor without changing production data. Snapshots are copied before opening;
the OS file cache is warm. Before figures are one observation per query; after
figures are medians of three sequential rounds in one process.

| Query | Before (ms) | After (ms) | Before allocated (MiB) | After allocated (MiB) |
| --- | ---: | ---: | ---: | ---: |
| Rooted directory path containing spaces | 5,521.43 | 54.34 | 2,062.55 | 13.11 |
| Rooted missing-file path containing spaces | 5,507.81 | 54.57 | 2,048.52 | 13.10 |
| `Desktop/test` | 5,258.99 | 51.16 | 2,009.67 | 13.23 |
| `dev/wox` | 5,169.85 | 24.48 | 2,003.93 | 8.00 |
| `C:\` | 4,708.58 | 1.07 | 1,997.57 | 1.26 |

Allocation is the cumulative Go allocation during one backend query, not retained
heap or whole-process working set. These are backend timings, not UI latency,
Delve timings, or cold-disk measurements. Larger result limits and unanchored
quoted/wildcard queries are not covered by this table. Result-set/score fingerprints
match in every round; exhaustive-scoring tests separately verify ordering, entry
kinds, metadata, Unicode, cross-component matching and live overlay changes.

Raw inputs, build overlays, individual observations and `path-comparison.json`
are under `artifacts/memory-20261018-1623/`. The related package tests pass with
five Windows symlink-privilege cases and one fallback-watcher event-order case
excluded. The unfiltered run encountered those host/event-order failures; no
symlink or watcher implementation changed. The core production build passes.

## Directory query columns comparison (2026-10-18)

Windows amd64, Go 1.27.2, optimized diagnostic binaries, `GOMAXPROCS=8`,
100-result limit. The frozen two-volume fixture contains 2,636,569 base rows,
568,834 directories and 14,974 overlay records. Sequential before/after/after/before
runs each replay fourteen query families for 26 rounds. The first round in each
process is excluded below, leaving 50 warm observations per family per arm.
Snapshot copying, upgrades and open/replay are outside query timing. Copies warm
the OS file cache; these are backend measurements, not UI or cold-disk timings.

| Query | Before p50/p95 (ms) | After p50/p95 (ms) | Before median allocation (KiB) | After median allocation (KiB) |
| --- | ---: | ---: | ---: | ---: |
| `qqyy` | 3.52 / 5.71 | 2.51 / 4.23 | 1529.5 | 55.9 |
| `readme` | 7.01 / 10.44 | 4.00 / 5.85 | 1527.4 | 258.7 |
| `test go` | 10.13 / 14.44 | 6.78 / 9.06 | 1061.7 | 272.7 |
| `a` | 6.52 / 7.54 | 4.98 / 5.79 | 924.6 | 584.9 |
| `ss` | 7.73 / 10.30 | 5.04 / 6.39 | 1018.1 | 524.4 |
| `*.txt` | 4.51 / 6.47 | 2.52 / 5.80 | 1121.5 | 295.7 |
| `readme .md` | 12.80 / 17.01 | 6.50 / 8.22 | 1828.3 | 255.2 |
| `项目` | 3.00 / 4.04 | 2.50 / 4.17 | 489.5 | 497.5 |
| `xiangmu` | 3.29 / 4.02 | 2.64 / 4.03 | 848.8 | 162.5 |
| `Desktop/test` | 46.55 / 54.59 | 4.98 / 7.52 | 11283.6 | 73.9 |
| `dev/wox` | 23.05 / 31.10 | 3.52 / 5.09 | 8491.7 | 308.0 |
| `C:\Users\qianl\Desktop\test - folder\` | 55.16 / 60.96 | 6.64 / 7.61 | 13420.7 | 36.2 |
| `C:\Users\qianl\Desktop\test - folder\sadf.txt` | 53.13 / 65.82 | 6.49 / 8.87 | 13414.3 | 31.5 |
| `C:\` | 300.24 / 318.44 | 3.53 / 4.83 | 148231.0 | 412.1 |

All result-path/score fingerprints match across all 1,456 observations. Exhaustive
retrieval tests separately compare complete ordering, kinds and metadata, Unicode,
cross-component matching, live renames, metadata updates, file-to-directory
promotion, orphan ancestry and large moves beyond the live-path cache cap.

Every sampled warm query family has p95 below 10 ms; this is not an all-query
latency guarantee. 3 of the 700 warm after samples exceed 10 ms (maximum
14.99 ms). Process-first `qqyy` takes 9.25 and 9.23 ms in the after runs. A separate
mapped-page/pinyin-release experiment measures first resumed queries at 10.58 ms
(`qqyy`), 10.24 ms (`readme`), 10.65 ms (`test go`), 11.51 ms (`Desktop/test`) and
5.74 ms (`dev/wox`). The resumed `qqyy` allocates 8.61 MiB while reconstructing
pinyin state. These numbers precede the pinyin follow-up below, which reduces
that reconstruction cost. Unanchored quoted/wildcard path scans and content
search are outside the sub-10-ms figures.

Frozen snapshot files grow from 135.36 to 347.69 MiB. The input predates persisted
query columns and child postings, so this delta also includes those existing
arrays; it is not exclusively directory path storage. Rewriting diagnostic copies takes roughly
3.1 seconds combined in one observed run. This is fixture preparation, not a
registered application migration. New publications map the prepared arrays;
legacy reads remain write-free but must prepare missing arrays on the heap.
Build peak memory and whole-process resident memory were not measured.
The larger mapping is not a claim of equally large resident/private memory.

The core production build and affected column, filename, fuzzy matcher, plugin,
and focused file-search migration tests pass. Focused race tests cover shared
query state and idle release. Five symlink cases cannot run on this host without
Windows symlink privilege; one fallback watcher event-order failure reproduces
with the preserved baseline binary. The complete migration suite also encounters
an unrelated dictation-test log-file cleanup failure; focused file-search
migrations pass. Column tests cross-compile for Linux amd64 and macOS arm64;
native performance and idle-page behavior there remain unmeasured.

Raw samples, CPU/allocation profiles, preserved binaries and `summary.json` are
under `artifacts/filesearch-query-20261018/`. Set `WOX_QUERY_DIAGNOSTIC_REWRITE=1`
to rewrite only diagnostic copies, and `WOX_QUERY_DIAGNOSTIC_LIMIT` to select a
result cap; neither setting changes the production index.

## Pinyin reconstruction follow-up (2026-10-18)

The directory format is unreleased, so its new migration and production upgrade
entry point have been removed. Existing unrelated migrations are unchanged.
Diagnostic fixture rewriting is test-only; compatibility reads never modify
stored snapshots. New builds and compactions write the prepared directory columns.

A repeated-release allocation profile attributes most of the former cold-search
cost to per-name `getPinYin` graphs, growing segment slices, literal builders and
cache insertion, rather than solely to the character dictionary. Compiled filename
matching now borrows bounded pooled scratch, shares dictionary syllables/initials
and source substrings, and clears pointer-bearing slots before returning scratch.
It creates no per-name memoization entries. General repeated-title matching keeps
its existing bounded cache; prepared text still owns its prepared graph.

The character dictionary uses sparse 256-rune pages and flat reading/initial arrays
instead of thousands of map values and slices. Immutable strings borrow the owned
resource read, including in compressed-asset builds. Idle release still unpublishes
the dictionary. All 8,105 characters, 8,458 readings and their order retain the
existing semantic fingerprint. Tests also retain borrowed readings through forced
GC/release and exercise supplementary Han, invalid runes/UTF-8, large scratch growth,
polyphonic names and concurrent cleanup.

Matcher deduplication uses at most `min(segment count, pattern rune count)` matched
segments because each match consumes at least one pattern rune. Final grown/swapped
state buffers return to the pool instead of repeatedly discarding their capacity.

Cold diagnostics use the same Windows/Go configuration, frozen two-volume data,
100-result limit and sequential before/after/after/before order as above. Each
process performs 20 cycles of mapped-page release, pinyin cache/dictionary release,
`debug.FreeOSMemory`, then `qqyy`: 40 measured cold queries per plain arm. Setup and
rewriting the copied snapshots are outside timing. The OS file cache is warm.

| Build | Samples | p50 / p95 (ms) | Median Go allocation |
| --- | ---: | ---: | ---: |
| Before pinyin change | 40 | 10.93 / 12.68 | 9458.1 KiB |
| After, plain assets | 40 | 7.32 / 10.43 | 475.1 KiB |
| After, compressed assets | 20 | 7.78 / 9.35 | 628.7 KiB |

Plain cold-query allocation falls about 95%, from 9.24 MiB to 475 KiB. The packed
arm is a separate 20-cycle confirmation, not a packed before/after comparison.
Cold tails still cross 10 ms (plain maximum 11.43 ms; packed maximum 10.38 ms).
These backend allocation counters are cumulative allocation, not retained heap or
whole-process resident memory, and these runs are not cold-disk measurements.

Separate warm mixed runs retain 50 observations per family per arm. Every tested
result-path/score fingerprint matches, including all cold and packed observations.
Pinyin-sensitive warm allocation changes include:

| Query | Before median allocation (KiB) | After median allocation (KiB) | Before / after p50 (ms) |
| --- | ---: | ---: | ---: |
| `qqyy` | 55.9 | 58.4 | 2.85 / 3.30 |
| `a` | 586.4 | 216.6 | 5.14 / 5.79 |
| `ss` | 529.0 | 312.4 | 5.60 / 5.95 |
| `项目` | 501.0 | 9.7 | 2.70 / 3.12 |
| `xiangmu` | 269.3 | 38.7 | 3.14 / 3.41 |

Warm mixed-run p50s rise about 0.1-0.65 ms in this run, including path queries whose
executor did not change; this does not establish a warm speedup. All sampled warm
families still have p95 below 10 ms. Repeated-title microbenchmarks stay around
2.3 microseconds/op with 0 B/op before and after. The compiled transient-name
microbenchmark is around 0.59 microseconds/op with 0 B/op. Dictionary loading drops
from approximately 938,851 B and 8,142 allocations to 362,128 B and 8 allocations;
its measured load time is about 0.49 versus 0.29 ms. No dictionary or scratch buffer
is kept solely to evade idle release.

Affected filename/index/plugin suites, fuzzy-matching tests, focused and full
fuzzy-matcher race checks, compressed-resource tests and the production build pass.
The existing five symlink privilege cases and baseline watcher-order failure remain
excluded from the native filename suite as documented above. Runtime changes in
this follow-up do not add filename-index columns or enlarge the index files.

Samples, profiles, benchmark logs and plain/packed binaries are preserved under
`artifacts/filesearch-query-20261018/pinyin-*`. The summaries are
`pinyin-cold-summary.json` and `pinyin-warm-summary.json`.

### Actual `maibn.go` latency follow-up (2026-10-18)

The reported 26 ms was real plugin work: the live log records 25 ms in search
and 1 ms building 53 results. The running VS Code/Delve binary uses
`-gcflags=all=-N -l`, whereas the earlier backend measurements used normal Go
compiler optimizations. Compiler mode alone does not remove the search hotspot.

CPU profiles of this exact query identified large numbers of impossible ASCII
subsequences reaching fuzzy scoring. The existing character bitmaps encode
membership, not order. New snapshots add a nineteenth section with transposed
ordered-pair bitmaps over letters and dots. Pairs express relative order across
arbitrary gaps; they are not bigram or substring restrictions. Normalized Unicode
text and every pinyin pronunciation contribute conservative alternatives. Query
phrases, wildcard literals and AND clauses preserve their independent ordering.
Final matching, scores and tie ordering remain scorer-owned.

The remaining increment scan recalculated character masks for approximately
15,000 records on every query. Revision-owned scalar metadata now stores live
change IDs, masks, ASCII flags and unchanged base-name IDs. Queries borrow the
base pair filters for unchanged names, reject before map lookup, and score only
eligible live records. This adds at most about 351 KiB of scalar metadata for
the frozen fixture's 14,974 overlay records, released with the revision. No
query-result cache or filename string copy is retained for this optimization.

On that fixture, the pair section grows the two snapshots from 347.69 MiB to
416.77 MiB (69.08 MiB, about 20%). It is mapped read-only, not copied into the Go
heap when opened. Builds/compaction prepare it before publication. Older
snapshots remain readable without constructing the pair index on the heap or
rewriting files. They receive the increment optimization but require a normal
index rebuild/compaction for the pair-filter benefit. There is no new migration.

Sequential before/after/after/before optimized backend runs replay nineteen
query families for 21 rounds per process. The first round is excluded, leaving
40 warm observations per family per arm, on the same two-volume fixture and
100-result cap. Copying, opening and preparing snapshots are outside timing;
OS file caches are warm. All result-path/score fingerprints match.

| Query | Before p50 / p95 (ms) | After p50 / p95 (ms) | Before / after median Go allocation (KiB) |
| --- | ---: | ---: | ---: |
| `maibn.go` | 6.98 / 8.66 | 3.00 / 3.85 | 125.4 / 120.1 |
| `maibn.gos` | 6.04 / 7.68 | 2.01 / 3.04 | 29.1 / 29.2 |
| `maibn.g` | 7.68 / 9.04 | 3.05 / 4.07 | 165.6 / 165.9 |
| `maibn` | 11.39 / 13.57 | 7.92 / 9.63 | 266.3 / 268.8 |
| `main.go` | 6.41 / 8.00 | 2.53 / 4.07 | 242.0 / 234.2 |
| `qqyy` | 4.53 / 6.07 | 1.77 / 2.53 | 63.8 / 59.3 |
| `readme` | 6.91 / 8.09 | 4.04 / 5.05 | 262.0 / 260.4 |
| `test go` | 10.07 / 12.73 | 6.43 / 7.56 | 287.7 / 282.1 |

A separate final 40-round diagnostic in the same unoptimized compiler mode as
Delve records `maibn.go` backend p50/p95 of 5.49/7.68 ms; the optimized arm in
that run records 1.60/3.02 ms. These isolated backend numbers must not be presented
as complete UI latency. `readme .md` still has an 11.80 ms p95 in the mixed run;
the long absolute-path families still cross 10 ms. Allocation is largely
unchanged in this follow-up; the primary improvement is avoiding unnecessary CPU.

The actual Wox UI was also tested in an isolated data directory with fresh
settings, the copied full-size index and a diagnostic-only native adapter that
opens the copies without attaching to the live NTFS service. Neither the user's
running debug app nor its persisted index was restarted/replaced. Both ordinary
plugin result processing and the native UI are exercised; the adapter is supplied
through a Go build overlay under artifacts and is absent from the normal build.

For 25 final matching `maibn.go` queries in the optimized production-mode UI,
plugin logs record search p50/p95 4/5 ms, result construction 2/3 ms and total
Plugin.Query 6/8 ms, maximum 9 ms, with 53 results. The automation driver observing
completed UI state has p50/p95 34.84/53.85 ms when alternating `f maibn.g` and
`f maibn.go` without clearing between inputs. Repeated empty-to-results queries
also resize the window and measure about 70 ms. UI response processing and native
presentation remain additional costs; this work does not establish a sub-10-ms
complete visible frame or solve those UI costs. Automation samples include RPC
and 5-ms polling overhead, so they are not precise native presentation timestamps.

Mapped-filter recall is checked against exhaustive scoring, including the
reported typo, gaps, wildcard/AND forms, normalization and polyphonic pinyin.
Affected search/index/plugin suites and focused race tests pass, retaining the
previously documented Windows symlink/watcher exclusions. The normal optimized
production binary builds without the diagnostic adapter. Evidence lives under
`artifacts/filesearch-query-20261018/maibn-*`; `maibn-summary.json` contains the
mixed comparison and `maibn-ui-release-stage-summary.json` contains plugin stages.
