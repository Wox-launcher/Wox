# Embedded resources

Wox-owned `go:embed` resources use `wox/internal/assetfs`. Keep source files in
their original format; do not check in compressed copies. New files under an
existing embed pattern are discovered automatically.

```go
//go:embed assets
var rawAssets embed.FS

var assets = assetfs.New(rawAssets)
```

Use `assets.ReadFile`, `assets.ReadDir`, or the standard `io/fs` helpers. Do not
read the raw embed variable. The release packer rejects embed declarations that
are not wrapped with `assetfs.New`. Third-party module assets are not rewritten.

`make build` builds/stages native helpers and skills, then runs `make
pack-resources`. This scans the target's dependency graph with `go list`, gzip
compresses every selected Wox resource, verifies byte-for-byte decompression, and
writes `.build/assets/overlay.json`. The final Go build consumes that overlay.
Source files, names, directory layout, and signed executable bytes are unchanged.
Compression must happen after helper signing; main executable signing remains
after the final build. Generated files are local build artifacts and are ignored
by Git. Always regenerate the overlay after changing resources or target platform.

Plain `go build` and `go test` use the current uncompressed source resources,
without a generation prerequisite. Release-mode checks can be run from `wox.core`:

```sh
make pack-resources
go test -overlay .build/assets/overlay.json -tags sqlite_fts5 ./internal/assetfs ./resource ./plugin/system/emoji
```

The runtime decompresses one file per read, checks gzip integrity, and does not
cache decompressed files. Directory entries report original sizes. Existing
resource extraction still controls destination permissions. Mandatory application
icons are loaded once; other resource consumers keep their existing error paths.

`TestEmbeddedBytesMatchSources` compares every resource against its source file
in either mode. The emoji catalog additionally has a full semantic fingerprint.
`BenchmarkReadEmbeddedResources` measures bulk read overhead, not application
startup time. Files must be smaller than 4 GiB so gzip's size footer is exact.
