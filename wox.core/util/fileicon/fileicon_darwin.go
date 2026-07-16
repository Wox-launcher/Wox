package fileicon

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa -framework UniformTypeIdentifiers
#include <stdlib.h>

const unsigned char *GetFileTypeIconBytes(const char *ext, int size, size_t *length);
const unsigned char *GetFileIconBytes(const char *path, int size, size_t *length);
*/
import "C"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"unsafe"
	"wox/util"
	"wox/util/imagecache"
	"wox/util/shell"

	"github.com/disintegration/imaging"
	"howett.net/plist"
)

func getFileIconImpl(ctx context.Context, filePath string, size int) (string, error) {
	if filePath == "" {
		return "", errors.New("empty path")
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	cachePath := buildPathCachePath(filePath, size, info.ModTime().UnixNano())
	if cacheInfo, err := os.Stat(cachePath); err == nil {
		imagecache.Touch(ctx, cachePath, cacheInfo)
		return cachePath, nil
	}

	if iconPath, err := ResolveMacAppBundleIconPath(filePath); err == nil {
		// App bundles usually expose multi-size icns assets. Render the requested
		// size from that source first so large grid icons are not built from the
		// normal 48px NSWorkspace fallback.
		if saveErr := saveMacIconToCache(iconPath, cachePath, size); saveErr == nil {
			return cachePath, nil
		}
	}

	return getFileIconFromSystemAPI(filePath, cachePath, size)
}

func getFileTypeIconImpl(ctx context.Context, ext string, size int) (string, error) {
	cachePath := buildCachePath(ext, size)

	if info, err := os.Stat(cachePath); err == nil {
		imagecache.Touch(ctx, cachePath, info)
		return cachePath, nil
	}

	cext := C.CString(ext)
	defer C.free(unsafe.Pointer(cext))

	var length C.size_t
	bytesPtr := C.GetFileTypeIconBytes(cext, C.int(size), &length)
	if bytesPtr == nil || length == 0 {
		return "", errors.New("no icon")
	}
	defer C.free(unsafe.Pointer(bytesPtr))

	data := C.GoBytes(unsafe.Pointer(bytesPtr), C.int(length))
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	if err := imaging.Save(img, cachePath); err != nil {
		return "", err
	}

	return cachePath, nil
}

func getFileIconFromSystemAPI(filePath, cachePath string, size int) (string, error) {
	if err := os.MkdirAll(path.Dir(cachePath), 0o755); err != nil {
		return "", err
	}

	cpath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cpath))

	var length C.size_t
	bytesPtr := C.GetFileIconBytes(cpath, C.int(size), &length)
	if bytesPtr == nil || length == 0 {
		return "", errors.New("no icon")
	}
	defer C.free(unsafe.Pointer(bytesPtr))

	data := C.GoBytes(unsafe.Pointer(bytesPtr), C.int(length))
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	if err := imaging.Save(img, cachePath); err != nil {
		return "", err
	}

	return cachePath, nil
}

// ResolveMacAppBundleIconPath returns the app bundle's declared icon file.
// App indexing uses the same resolver to distinguish real app icons from the
// generic NSWorkspace fallback, so launchpad can hide entries that effectively
// have no dedicated icon.
func ResolveMacAppBundleIconPath(appPath string) (string, error) {
	plistPath := path.Join(appPath, "Contents", "Info.plist")
	plistFile, openErr := os.Open(plistPath)
	if openErr != nil {
		plistPath = path.Join(appPath, "WrappedBundle", "Info.plist")
		plistFile, openErr = os.Open(plistPath)
		if openErr != nil {
			return "", fmt.Errorf("can't find Info.plist in this app: %s", openErr.Error())
		}
	}
	defer plistFile.Close()

	decoder := plist.NewDecoder(plistFile)
	var plistData map[string]any
	if decodeErr := decoder.Decode(&plistData); decodeErr != nil {
		return "", fmt.Errorf("failed to decode Info.plist: %s", decodeErr.Error())
	}

	iconName, exist := plistData["CFBundleIconFile"].(string)
	if exist {
		if !strings.HasSuffix(iconName, ".icns") {
			iconName = iconName + ".icns"
		}
		iconPath := path.Join(appPath, "Contents", "Resources", iconName)
		if _, statErr := os.Stat(iconPath); os.IsNotExist(statErr) {
			return "", fmt.Errorf("icon file not found: %s", iconPath)
		}
		return iconPath, nil
	}

	icons, cfBundleIconsExist := plistData["CFBundleIcons"].(map[string]any)
	if cfBundleIconsExist {
		primaryIcon, cfBundlePrimaryIconExist := icons["CFBundlePrimaryIcon"].(map[string]any)
		if cfBundlePrimaryIconExist {
			iconFiles, cfBundleIconFilesExist := primaryIcon["CFBundleIconFiles"].([]any)
			if cfBundleIconFilesExist {
				lastIconName := iconFiles[len(iconFiles)-1].(string)
				files, readDirErr := os.ReadDir(path.Dir(plistPath))
				if readDirErr == nil {
					for _, file := range files {
						if strings.HasPrefix(file.Name(), lastIconName) {
							return path.Join(path.Dir(plistPath), file.Name()), nil
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("info plist doesn't have CFBundleIconFile property")
}

func saveMacIconToCache(iconPath, cachePath string, size int) error {
	if err := os.MkdirAll(path.Dir(cachePath), 0o755); err != nil {
		return err
	}

	renderSize := boundedMacIconRenderSize(iconPath, size)

	if strings.HasSuffix(strings.ToLower(iconPath), ".icns") {
		if out, err := shell.RunOutput("sips", "-z", intToString(renderSize), intToString(renderSize), "-s", "format", "png", iconPath, "--out", cachePath); err != nil {
			msg := fmt.Sprintf("failed to convert icns to png: %s", err.Error())
			if out != nil {
				msg = fmt.Sprintf("%s, output: %s", msg, string(out))
			}
			return errors.New(msg)
		}
		return nil
	}

	if strings.HasSuffix(strings.ToLower(iconPath), ".png") {
		isCgbi, detectErr := isCgbiPNG(iconPath)
		if detectErr == nil && isCgbi {
			if out, err := shell.RunOutput("sips", "-z", intToString(renderSize), intToString(renderSize), "-s", "format", "png", iconPath, "--out", cachePath); err != nil {
				msg := fmt.Sprintf("failed to convert CgBI PNG to standard PNG: %s", err.Error())
				if out != nil {
					msg = fmt.Sprintf("%s, output: %s", msg, string(out))
				}
				return errors.New(msg)
			}
			return nil
		}
	}

	if out, err := shell.RunOutput("sips", "-z", intToString(renderSize), intToString(renderSize), "-s", "format", "png", iconPath, "--out", cachePath); err != nil {
		msg := fmt.Sprintf("failed to resize icon to png: %s", err.Error())
		if out != nil {
			msg = fmt.Sprintf("%s, output: %s", msg, string(out))
		}
		return errors.New(msg)
	}
	return nil
}

func boundedMacIconRenderSize(iconPath string, requestedSize int) int {
	if requestedSize <= 0 {
		requestedSize = util.ResultListIconSize
	}

	maxSourceSize := readMacIconMaxPixelSize(iconPath)
	if maxSourceSize <= 0 || maxSourceSize >= requestedSize {
		return requestedSize
	}

	// Do not upscale small native icon assets. Terminal.app, for example, only
	// exposes a 256px icns; forcing sips to create a 512px PNG makes Wox cache
	// a pre-blurred image before UI has a chance to downsample it.
	return maxSourceSize
}

func readMacIconMaxPixelSize(iconPath string) int {
	out, err := shell.RunOutput("sips", "-g", "pixelWidth", "-g", "pixelHeight", iconPath)
	if err != nil || out == nil {
		return 0
	}

	maxPixels := 0
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[0] != "pixelWidth:" && fields[0] != "pixelHeight:" {
			continue
		}
		value, parseErr := strconv.Atoi(fields[1])
		if parseErr == nil && value > maxPixels {
			maxPixels = value
		}
	}

	return maxPixels
}

func isCgbiPNG(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false, err
	}

	cgbiSignature := []byte("CgBI")
	for i := 0; i <= n-4; i++ {
		if bytes.Equal(buffer[i:i+4], cgbiSignature) {
			return true, nil
		}
	}

	return false, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := out.ReadFrom(in); err != nil {
		return err
	}
	return nil
}
