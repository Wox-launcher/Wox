package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/parsiya/golnk"
	"image"
	"image/color"
	"strings"
	"syscall"
	"unsafe"
	"wox/plugin"
	"wox/util"
)

var (
	// Load shell32.dll instead of user32.dll
	shell32 = syscall.NewLazyDLL("shell32.dll")
	// Get the address of ExtractIconExW from shell32.dll
	extractIconEx = shell32.NewProc("ExtractIconExW")
)

var appRetriever = &WindowsRetriever{}

type WindowsRetriever struct {
	api plugin.API
}

func (a *WindowsRetriever) UpdateAPI(api plugin.API) {
	a.api = api
}

func (a *WindowsRetriever) GetPlatform() string {
	return util.PlatformWindows
}

func (a *WindowsRetriever) GetAppDirectories(ctx context.Context) []appDirectory {
	return []appDirectory{
		{
			Path:           "C:\\ProgramData\\Microsoft\\Windows\\Start Menu\\Programs",
			Recursive:      true,
			RecursiveDepth: 2,
		},
	}
}

func (a *WindowsRetriever) GetAppExtensions(ctx context.Context) []string {
	return []string{"exe", "lnk"}
}

func (a *WindowsRetriever) ParseAppInfo(ctx context.Context, path string) (appInfo, error) {
	if strings.HasSuffix(path, ".lnk") {
		return a.parseShortcut(ctx, path)
	}

	return appInfo{}, errors.New("not implemented")
}

func (a *WindowsRetriever) parseShortcut(ctx context.Context, path string) (appInfo, error) {
	f, lnkErr := lnk.File(path)
	if lnkErr != nil {
		return appInfo{}, lnkErr
	}

	var targetPath = ""
	if f.LinkInfo.LocalBasePath != "" {
		targetPath = f.LinkInfo.LocalBasePath
	}
	if f.LinkInfo.LocalBasePathUnicode != "" {
		targetPath = f.LinkInfo.LocalBasePathUnicode
	}
	if targetPath == "" || !strings.HasSuffix(targetPath, ".exe") {
		return appInfo{}, errors.New("no target path found")
	}

	// use default icon if no icon is found
	icon := appIcon
	img, iconErr := a.GetAppIcon(ctx, targetPath)
	if iconErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Error getting icon for %s: %s", targetPath, iconErr.Error()))
	} else {
		woxIcon, imgErr := plugin.NewWoxImage(img)
		if imgErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("Error converting icon for %s: %s", targetPath, imgErr.Error()))
		} else {
			icon = woxIcon
		}
	}

	return appInfo{
		Name: f.StringData.NameString,
		Path: targetPath,
		Icon: icon,
	}, nil
}

func (a *WindowsRetriever) GetAppIcon(ctx context.Context, path string) (image.Image, error) {
	// 将文件路径转换为UTF16
	lpIconPath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	// 使用ExtractIconEx获取图标句柄
	var largeIcon win.HICON
	var smallIcon win.HICON
	ret, _, _ := extractIconEx.Call(
		uintptr(unsafe.Pointer(lpIconPath)),
		0,
		uintptr(unsafe.Pointer(&largeIcon)),
		uintptr(unsafe.Pointer(&smallIcon)),
		1,
	)
	if ret == 0 {
		return nil, fmt.Errorf("no icons found in file")
	}
	defer win.DestroyIcon(largeIcon) // 确保释放图标资源

	// 获取图标信息
	var iconInfo win.ICONINFO
	if win.GetIconInfo(largeIcon, &iconInfo) == false {
		return nil, fmt.Errorf("failed to get icon info")
	}
	defer win.DeleteObject(win.HGDIOBJ(iconInfo.HbmColor))
	defer win.DeleteObject(win.HGDIOBJ(iconInfo.HbmMask))

	// 创建设备无关位图(DIB)来接收图像数据
	hdc := win.GetDC(0)
	defer win.ReleaseDC(0, hdc)

	var bmpInfo win.BITMAPINFO
	bmpInfo.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmpInfo.BmiHeader))
	bmpInfo.BmiHeader.BiWidth = int32(iconInfo.XHotspot * 2)
	bmpInfo.BmiHeader.BiHeight = -int32(iconInfo.YHotspot * 2) // 负值表示自顶向下的DIB
	bmpInfo.BmiHeader.BiPlanes = 1
	bmpInfo.BmiHeader.BiBitCount = 32
	bmpInfo.BmiHeader.BiCompression = win.BI_RGB

	// 分配内存来存储位图数据
	bits := make([]byte, iconInfo.XHotspot*2*iconInfo.YHotspot*2*4)
	if win.GetDIBits(hdc, win.HBITMAP(iconInfo.HbmColor), 0, uint32(iconInfo.YHotspot*2), &bits[0], &bmpInfo, win.DIB_RGB_COLORS) == 0 {
		return nil, fmt.Errorf("failed to get DIB bits")
	}

	width := int(iconInfo.XHotspot * 2)
	height := int(iconInfo.YHotspot * 2)
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Copy the bitmap data into the img.Pix slice.
	// Note: Windows bitmaps are stored in BGR format, so we need to swap the red and blue channels.
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			base := y*width*4 + x*4
			// The bitmap data in bits is in BGRA format.
			b := bits[base+0]
			g := bits[base+1]
			r := bits[base+2]
			a := bits[base+3]
			// Set the pixel in the image.
			// Note: image.RGBA expects data in RGBA format, so we swap R and B.
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	return img, nil
}

func (a *WindowsRetriever) GetExtraApps(ctx context.Context) ([]appInfo, error) {
	return []appInfo{}, nil
}
