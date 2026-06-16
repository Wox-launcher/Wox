package common

import (
	"context"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
	"wox/util"
)

/*
cpu: Apple M1 Max
BenchmarkConvertIconWithSizeScreenshot/cold-derived-cache-10         	      18	  61236859 ns/op	  32.18 MB/s	         2.537 source_MPx/op	12437475 B/op	    2565 allocs/op
BenchmarkConvertIconWithSizeScreenshot/warm-derived-cache-10         	  377696	      2706 ns/op	728280.61 MB/s	         2.537 source_MPx/op	    1296 B/op	      16 allocs/op
PASS
*/
func BenchmarkConvertIconWithSizeScreenshot(b *testing.B) {
	sourcePath := benchmarkScreenshotPath(b)
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		b.Fatalf("stat benchmark screenshot: %v", err)
	}
	width, height := benchmarkPngSize(b, sourcePath)

	// Size-specific sub-benchmarks add noise here because the list and grid icon sizes
	// exercise the same ConvertIconWithSize path. Keep one representative size so the
	// baseline focuses on cold conversion cost versus warm derived-cache cost.
	size := ResultListIconSize
	b.Run("cold-derived-cache", func(b *testing.B) {
		benchmarkConvertIconColdDerivedCache(b, sourcePath, sourceInfo.Size(), width, height, size)
	})
	b.Run("warm-derived-cache", func(b *testing.B) {
		benchmarkConvertIconWarmDerivedCache(b, sourcePath, sourceInfo.Size(), width, height, size)
	})
}

func benchmarkConvertIconColdDerivedCache(b *testing.B, sourcePath string, sourceBytes int64, width int, height int, size int) {
	ctx := context.Background()
	benchmarkInitConvertIconLocation(b)
	sourcePaths := benchmarkUniqueScreenshotPaths(b, sourcePath)
	b.SetBytes(sourceBytes)
	b.ReportAllocs()

	b.ResetTimer()
	b.ReportMetric(float64(width*height)/1_000_000, "source_MPx/op")
	for i := 0; i < b.N; i++ {
		// Cold-cache timing must avoid reusing the derived crop/resize files. The unique source
		// path keeps the image content realistic while forcing ConvertIconWithSize through the
		// expensive decode, transparent-padding scan, resize, and PNG write path each iteration.
		converted := ConvertIconWithSize(ctx, NewWoxImageAbsolutePath(sourcePaths[i]), "", size)
		if converted.ImageType != WoxImageTypeAbsolutePath || converted.ImageData == "" {
			b.Fatalf("unexpected converted image: %+v", converted)
		}
	}
}

func benchmarkConvertIconWarmDerivedCache(b *testing.B, sourcePath string, sourceBytes int64, width int, height int, size int) {
	ctx := context.Background()
	benchmarkInitConvertIconLocation(b)
	sourceImage := NewWoxImageAbsolutePath(sourcePath)
	if converted := ConvertIconWithSize(ctx, sourceImage, "", size); converted.ImageType != WoxImageTypeAbsolutePath || converted.ImageData == "" {
		b.Fatalf("warm benchmark prefill failed: %+v", converted)
	}
	b.SetBytes(sourceBytes)
	b.ReportAllocs()

	b.ResetTimer()
	b.ReportMetric(float64(width*height)/1_000_000, "source_MPx/op")
	for i := 0; i < b.N; i++ {
		// Warm-cache timing captures the steady query-polish path after ConvertIconWithSize has
		// generated both derived files. This keeps the benchmark useful when optimizing cache hits
		// without mixing them with the first-conversion cost.
		converted := ConvertIconWithSize(ctx, sourceImage, "", size)
		if converted.ImageType != WoxImageTypeAbsolutePath || converted.ImageData == "" {
			b.Fatalf("unexpected converted image: %+v", converted)
		}
	}
}

func benchmarkInitConvertIconLocation(b *testing.B) {
	b.Helper()

	dataDir := b.TempDir()
	b.Setenv(util.TestWoxDataDirEnv, dataDir)
	b.Setenv(util.TestUserDataDirEnv, filepath.Join(dataDir, "user"))
	if err := util.GetLocation().Init(); err != nil {
		b.Fatalf("init benchmark location: %v", err)
	}
	ClearConvertIconPathExistenceCache()
}

func benchmarkScreenshotPath(b *testing.B) string {
	b.Helper()

	workingDir, err := os.Getwd()
	if err != nil {
		b.Fatalf("get working directory: %v", err)
	}

	for dir := workingDir; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "screenshots", "screenshot.png")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}

	b.Fatalf("screenshots/screenshot.png was not found from %s", workingDir)
	return ""
}

func benchmarkPngSize(b *testing.B, sourcePath string) (int, int) {
	b.Helper()

	file, err := os.Open(sourcePath)
	if err != nil {
		b.Fatalf("open benchmark screenshot: %v", err)
	}
	defer file.Close()

	config, err := png.DecodeConfig(file)
	if err != nil {
		b.Fatalf("decode benchmark screenshot config: %v", err)
	}

	return config.Width, config.Height
}

func benchmarkUniqueScreenshotPaths(b *testing.B, sourcePath string) []string {
	b.Helper()

	sourcePaths := make([]string, b.N)
	sourceDir := b.TempDir()
	for i := 0; i < b.N; i++ {
		destination := filepath.Join(sourceDir, fmt.Sprintf("screenshot-%06d.png", i))
		if err := os.Link(sourcePath, destination); err != nil {
			if copyErr := benchmarkCopyFile(sourcePath, destination); copyErr != nil {
				b.Fatalf("prepare benchmark screenshot copy: link=%v copy=%v", err, copyErr)
			}
		}
		sourcePaths[i] = destination
	}

	return sourcePaths
}

func benchmarkCopyFile(sourcePath string, destinationPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(destinationPath)
	if err != nil {
		return err
	}

	if _, err = io.Copy(destination, source); err != nil {
		_ = destination.Close()
		return err
	}

	return destination.Close()
}
