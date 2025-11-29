//go:build darwin

package emoji

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework CoreText
#include <Cocoa/Cocoa.h>
#include <CoreText/CoreText.h>
#include <stdlib.h>
#include <string.h>

static unsigned char* RenderEmojiPNG(const char* emoji, int size, size_t* length) {
    @autoreleasepool {
        if (emoji == NULL || length == NULL || size <= 0) {
            return NULL;
        }

        NSString* str = [NSString stringWithUTF8String:emoji];
        if (str == nil) {
            return NULL;
        }

        NSFont* font = [NSFont fontWithName:@"AppleColorEmoji" size:size];
        if (!font) {
            font = [NSFont systemFontOfSize:size];
        }
        NSDictionary* attrs = @{ NSFontAttributeName: font };
        NSAttributedString* attrStr = [[NSAttributedString alloc] initWithString:str attributes:attrs];

        NSSize textSize = [attrStr size];
        CGFloat dimension = (CGFloat)size;
        if (textSize.width > dimension) {
            dimension = textSize.width;
        }
        if (textSize.height > dimension) {
            dimension = textSize.height;
        }

        NSBitmapImageRep* rep = [[NSBitmapImageRep alloc]
            initWithBitmapDataPlanes:NULL
                          pixelsWide:(NSInteger)dimension
                          pixelsHigh:(NSInteger)dimension
                       bitsPerSample:8
                     samplesPerPixel:4
                            hasAlpha:YES
                            isPlanar:NO
                      colorSpaceName:NSCalibratedRGBColorSpace
                        bitmapFormat:NSBitmapFormatAlphaFirst
                         bytesPerRow:0
                        bitsPerPixel:0];
        if (!rep) {
            return NULL;
        }

        [NSGraphicsContext saveGraphicsState];
        NSGraphicsContext* ctx = [NSGraphicsContext graphicsContextWithBitmapImageRep:rep];
        [NSGraphicsContext setCurrentContext:ctx];
        [[NSColor clearColor] set];
        NSRect rect = NSMakeRect(0, 0, dimension, dimension);
        NSRectFill(rect);

        CGFloat x = (dimension - textSize.width) / 2.0;
        CGFloat y = (dimension - textSize.height) / 2.0;
        NSRect drawRect = NSMakeRect(x, y, textSize.width, textSize.height);
        [attrStr drawInRect:drawRect];
        [NSGraphicsContext restoreGraphicsState];

        NSData* pngData = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
        if (!pngData) {
            return NULL;
        }

        *length = (size_t)[pngData length];
        unsigned char* buffer = malloc(*length);
        if (!buffer) {
            return NULL;
        }
        memcpy(buffer, [pngData bytes], *length);
        return buffer;
    }
}
*/
import "C"
import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"unsafe"
)

func getNativeEmojiImage(emoji string, size int) (image.Image, error) {
	var length C.size_t
	cstr := C.CString(emoji)
	defer C.free(unsafe.Pointer(cstr))

	data := C.RenderEmojiPNG(cstr, C.int(size), &length)
	if data == nil {
		return nil, errors.New("failed to render emoji via Cocoa")
	}
	defer C.free(unsafe.Pointer(data))

	goData := C.GoBytes(unsafe.Pointer(data), C.int(length))
	return png.Decode(bytes.NewReader(goData))
}
