package screen

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa -framework ApplicationServices

typedef struct {
    int width;
    int height;
    int x;
    int y;
} ScreenInfo;

typedef struct {
    unsigned int id;
    int x;
    int y;
    int width;
    int height;
    int workX;
    int workY;
    int workWidth;
    int workHeight;
    int pixelX;
    int pixelY;
    int pixelWidth;
    int pixelHeight;
    int pixelWorkX;
    int pixelWorkY;
    int pixelWorkWidth;
    int pixelWorkHeight;
    double scale;
    int primary;
} ScreenDisplayInfo;

ScreenInfo getMouseScreenSize();
ScreenInfo getActiveScreenSize();
int listDisplays(ScreenDisplayInfo* displays, int maxCount);
*/
import "C"
import (
	"fmt"
)

const maxDisplayCount = 16

func GetMouseScreen() Size {
	screenInfo := C.getMouseScreenSize()
	return Size{
		Width:  int(screenInfo.width),
		Height: int(screenInfo.height),
		X:      int(screenInfo.x),
		Y:      int(screenInfo.y),
	}
}

func GetActiveScreen() Size {
	screenInfo := C.getActiveScreenSize()
	return Size{
		Width:  int(screenInfo.width),
		Height: int(screenInfo.height),
		X:      int(screenInfo.x),
		Y:      int(screenInfo.y),
	}
}

func listDisplays() ([]Display, error) {
	buffer := make([]C.ScreenDisplayInfo, maxDisplayCount)
	count := int(C.listDisplays(&buffer[0], C.int(len(buffer))))
	if count < 0 {
		return nil, fmt.Errorf("failed to enumerate displays")
	}

	displays := make([]Display, 0, count)
	for i := 0; i < count; i++ {
		info := buffer[i]
		displays = append(displays, Display{
			ID:   fmt.Sprintf("%d", uint32(info.id)),
			Name: fmt.Sprintf("Display %d", i+1),
			Bounds: Rect{
				X:      int(info.x),
				Y:      int(info.y),
				Width:  int(info.width),
				Height: int(info.height),
			},
			WorkArea: Rect{
				X:      int(info.workX),
				Y:      int(info.workY),
				Width:  int(info.workWidth),
				Height: int(info.workHeight),
			},
			PixelBounds: Rect{
				X:      int(info.pixelX),
				Y:      int(info.pixelY),
				Width:  int(info.pixelWidth),
				Height: int(info.pixelHeight),
			},
			PixelWorkArea: Rect{
				X:      int(info.pixelWorkX),
				Y:      int(info.pixelWorkY),
				Width:  int(info.pixelWorkWidth),
				Height: int(info.pixelWorkHeight),
			},
			Scale:   float64(info.scale),
			Primary: int(info.primary) == 1,
		})
	}

	return displays, nil
}
