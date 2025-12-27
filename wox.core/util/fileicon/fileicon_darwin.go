package fileicon

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa
#include <stdlib.h>

const unsigned char *GetFileTypeIconBytes(const char *ext, size_t *length);
const unsigned char *GetFileIconBytes(const char *path, size_t *length);
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
	"strings"
	"unsafe"
	"wox/util/shell"

	"github.com/disintegration/imaging"
	"howett.net/plist"
)

func getFileIconImpl(ctx context.Context, filePath string) (string, error) {
	const size = 48
	cachePath := buildPathCachePath(filePath, size)

	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	if filePath == "" {
		return "", errors.New("empty path")
	}
	if _, err := os.Stat(filePath); err != nil {
		return "", err
	}

	if iconPath, err := parseMacAppIconFromInfoPlist(filePath); err == nil {
		if saveErr := saveMacIconToCache(iconPath, cachePath); saveErr == nil {
			return cachePath, nil
		}
	}

	return getFileIconFromSystemAPI(filePath, cachePath)
}

func getFileTypeIconImpl(ctx context.Context, ext string) (string, error) {
	const size = 48
	cachePath := buildCachePath(ext, size)

	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	cext := C.CString(ext)
	defer C.free(unsafe.Pointer(cext))

	var length C.size_t
	bytesPtr := C.GetFileTypeIconBytes(cext, &length)
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

func getFileIconFromSystemAPI(filePath, cachePath string) (string, error) {
	if err := os.MkdirAll(path.Dir(cachePath), 0o755); err != nil {
		return "", err
	}

	cpath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cpath))

	var length C.size_t
	bytesPtr := C.GetFileIconBytes(cpath, &length)
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

func parseMacAppIconFromInfoPlist(appPath string) (string, error) {
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

func saveMacIconToCache(iconPath, cachePath string) error {
	if err := os.MkdirAll(path.Dir(cachePath), 0o755); err != nil {
		return err
	}

	if strings.HasSuffix(strings.ToLower(iconPath), ".icns") {
		if out, err := shell.RunOutput("sips", "-s", "format", "png", iconPath, "--out", cachePath); err != nil {
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
			if out, err := shell.RunOutput("sips", "-s", "format", "png", iconPath, "--out", cachePath); err != nil {
				msg := fmt.Sprintf("failed to convert CgBI PNG to standard PNG: %s", err.Error())
				if out != nil {
					msg = fmt.Sprintf("%s, output: %s", msg, string(out))
				}
				return errors.New(msg)
			}
			return nil
		}
	}

	return copyFile(iconPath, cachePath)
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
