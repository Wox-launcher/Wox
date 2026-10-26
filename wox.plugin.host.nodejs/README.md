Wox Nodejs Host

`resultActions.ts` owns result callbacks by session, query, result and action ID.
Query, MRU restore, incremental results and form updates share that store. Core
releases query buckets when their result cache expires; releasing a bucket also
invalidates in-flight responses. Plugin unload closes the API's original store.
See `wox.core/plugin/host/README.md` for the internal retention protocol.

Run `pnpm test` and `pnpm exec tsc --noEmit` to verify the host.
