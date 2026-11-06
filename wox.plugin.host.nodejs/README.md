Wox Nodejs Host

CommonJS single-file plugins can `require("@wox-launcher/wox-plugin")` directly.
The host provides its bundled SDK to the plugin entry module; other imports use
normal Node.js resolution, and packaged plugins keep their own dependencies.

`resultActions.ts` owns result callbacks by session, query, result and action ID.
Query, MRU restore, incremental results and form updates share that store. Core
releases query buckets when their result cache expires; releasing a bucket also
invalidates in-flight responses. Plugin unload closes the API's original store.
See `wox.core/plugin/host/README.md` for the internal retention protocol.

Run `pnpm test` and `pnpm exec tsc --noEmit` to verify the host.
After `pnpm build`, run `WOX_TEST_BUNDLED_HOST=1 node --test tests/hostSecurity.test.cjs`
to exercise single-file SDK imports through the distributed host bundle.
