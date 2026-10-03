# Recording runtime

`Resolve` is local-only. The recording UI calls `Install` only after the user
confirms the download. Existing PATH installations remain usable.

The runtime comes from the pinned `eugeneware/ffmpeg-static` GitHub release in
`release.go`. The installer uses Wox's HTTP/proxy configuration, verifies the
SHA-256 of the gzip, README and LICENSE, and probes H.264, GIF and animated WebP
encoders before publishing the directory. Downloads are staged beside the final
directory and removed on failure or cancellation. No administrator privileges,
system PATH changes, or package manager are required.

Installed files live under `<Wox data>/runtime/ffmpeg/<version>/<os>-<arch>/`.
README and LICENSE retain the upstream build provenance and license information.

When updating the release, update the version, asset names, compressed sizes and
all digests together from the release assets API. Verify the actual executable
on each supported target; do not infer binary compatibility from the asset name.
The network integration test installs only into a temporary test directory:

```sh
WOX_TEST_FFMPEG_DOWNLOAD=1 go test ./util/ffmpeg -run TestPublishedRelease -count=1
```
