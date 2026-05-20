//go:build darwin

package ocr

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Vision -framework ImageIO -framework CoreGraphics
#include <stdlib.h>
#include <stdbool.h>
#import <Foundation/Foundation.h>
#import <Vision/Vision.h>
#import <ImageIO/ImageIO.h>

static char* woxOCRDuplicateString(NSString *value) {
    if (value == nil) {
        return strdup("");
    }
    const char *utf8 = [value UTF8String];
    if (utf8 == NULL) {
        return strdup("");
    }
    return strdup(utf8);
}

static char* woxOCRRecognizeVisionText(const char *path, char **errorOut) {
    @autoreleasepool {
        if (@available(macOS 10.15, *)) {
            NSString *pathString = [NSString stringWithUTF8String:path];
            if (pathString == nil || [pathString length] == 0) {
                if (errorOut != NULL) {
                    *errorOut = strdup("image path is empty");
                }
                return NULL;
            }

            NSURL *url = [NSURL fileURLWithPath:pathString];
            CGImageSourceRef source = CGImageSourceCreateWithURL((__bridge CFURLRef)url, NULL);
            if (source == NULL) {
                if (errorOut != NULL) {
                    *errorOut = strdup("failed to open image source");
                }
                return NULL;
            }
            CGImageRef image = CGImageSourceCreateImageAtIndex(source, 0, NULL);
            CFRelease(source);
            if (image == NULL) {
                if (errorOut != NULL) {
                    *errorOut = strdup("failed to decode image");
                }
                return NULL;
            }

            __block NSMutableArray<NSString *> *recognizedLines = [NSMutableArray array];
            VNRecognizeTextRequest *request = [[VNRecognizeTextRequest alloc] initWithCompletionHandler:^(VNRequest *request, NSError *error) {
                if (error != nil) {
                    return;
                }
                NSArray<VNRecognizedTextObservation *> *observations = request.results;
                for (VNRecognizedTextObservation *observation in observations) {
                    VNRecognizedText *candidate = [[observation topCandidates:1] firstObject];
                    if (candidate != nil && candidate.string != nil && candidate.string.length > 0) {
                        [recognizedLines addObject:candidate.string];
                    }
                }
            }];
            request.recognitionLevel = VNRequestTextRecognitionLevelAccurate;
            request.usesLanguageCorrection = YES;

            VNImageRequestHandler *handler = [[VNImageRequestHandler alloc] initWithCGImage:image options:@{}];
            NSError *performError = nil;
            BOOL ok = [handler performRequests:@[request] error:&performError];
            CGImageRelease(image);
            if (!ok) {
                if (errorOut != NULL) {
                    *errorOut = woxOCRDuplicateString([performError localizedDescription]);
                }
                return NULL;
            }

            NSString *joined = [recognizedLines componentsJoinedByString:@"\n"];
            return woxOCRDuplicateString(joined);
        }

        if (errorOut != NULL) {
            *errorOut = strdup("Vision text recognition requires macOS 10.15 or later");
        }
        return NULL;
    }
}
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"
)

func recognizePlatform(ctx context.Context, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	cPath := C.CString(request.ImagePath)
	defer C.free(unsafe.Pointer(cPath))

	var cErr *C.char
	cText := C.woxOCRRecognizeVisionText(cPath, &cErr)
	if cErr != nil {
		defer C.free(unsafe.Pointer(cErr))
	}
	if cText == nil {
		message := "macOS Vision OCR is unavailable"
		if cErr != nil {
			message = C.GoString(cErr)
		}
		return Result{}, fmt.Errorf("%w: %s", ErrUnavailable, message)
	}
	defer C.free(unsafe.Pointer(cText))

	return Result{
		Engine: EngineMacOSVision,
		Text:   C.GoString(cText),
	}, nil
}
