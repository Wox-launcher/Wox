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

static char* woxOCRDuplicateError(NSError *error) {
    if (error == nil) {
        return strdup("");
    }
    // Feature fix: Vision can return an NSError whose localized description is empty.
    // Falling back to the full NSError description keeps OCR failures diagnosable instead
    // of surfacing an "unavailable" error with no platform reason.
    NSString *message = [error localizedDescription];
    if (message == nil || [message length] == 0) {
        message = [error description];
    }
    return woxOCRDuplicateString(message);
}

static void woxOCRAddLanguageCandidate(NSMutableArray<NSString *> *languages, NSMutableSet<NSString *> *seen, NSString *language) {
    if (language == nil) {
        return;
    }
    NSString *trimmed = [language stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
    if ([trimmed length] == 0) {
        return;
    }
    NSString *key = [trimmed lowercaseString];
    if ([seen containsObject:key]) {
        return;
    }
    [seen addObject:key];
    [languages addObject:trimmed];
}

static NSArray<NSString *> *woxOCRVisionLanguageCandidates(const char *languages) {
    NSMutableArray<NSString *> *candidates = [NSMutableArray array];
    NSMutableSet<NSString *> *seen = [NSMutableSet set];
    BOOL hasRequestedLanguages = NO;
    if (languages != NULL) {
        NSString *languageBlob = [NSString stringWithUTF8String:languages];
        for (NSString *language in [languageBlob componentsSeparatedByString:@"\n"]) {
            NSUInteger beforeCount = [candidates count];
            woxOCRAddLanguageCandidate(candidates, seen, language);
            hasRequestedLanguages = hasRequestedLanguages || [candidates count] > beforeCount;
        }
    }
    if (hasRequestedLanguages) {
        return candidates;
    }

    // Feature fix: Vision's implicit default can ignore the user's OCR language context and
    // misread non-English screenshots as English-like glyphs. Use macOS language preferences as
    // the default hint list so OCR follows the user's operating-system language instead of a
    // hard-coded language set.
    for (NSString *language in [NSLocale preferredLanguages]) {
        woxOCRAddLanguageCandidate(candidates, seen, language);
    }
    return candidates;
}

static NSString *woxOCRCanonicalSupportedLanguage(NSString *candidate, NSArray<NSString *> *supportedLanguages) {
    for (NSString *supportedLanguage in supportedLanguages) {
        if ([supportedLanguage caseInsensitiveCompare:candidate] == NSOrderedSame) {
            return supportedLanguage;
        }
        // Feature fix: preferred macOS language tags may include regional suffixes, while Vision
        // only accepts its canonical supported tag. Return Vision's tag so zh-Hans-CN style
        // candidates do not make performRequests fail after the support check passes.
        if ([supportedLanguage rangeOfString:candidate options:NSCaseInsensitiveSearch].location == 0 ||
            [candidate rangeOfString:supportedLanguage options:NSCaseInsensitiveSearch].location == 0) {
            return supportedLanguage;
        }
    }
    return nil;
}

static NSArray<NSString *> *woxOCRSupportedVisionLanguages(VNRecognizeTextRequest *request, NSArray<NSString *> *candidates) {
    if ([candidates count] == 0) {
        return candidates;
    }
    if (![request respondsToSelector:@selector(supportedRecognitionLanguagesAndReturnError:)]) {
        return @[];
    }

    NSError *languageError = nil;
    NSArray<NSString *> *supportedLanguages = [request supportedRecognitionLanguagesAndReturnError:&languageError];
    if (languageError != nil || [supportedLanguages count] == 0) {
        // Feature fix: unverified language hints are worse than no hints because Vision rejects
        // unsupported tags at perform time. If support discovery fails, keep the platform default
        // instead of risking a hard OCR failure.
        return @[];
    }

    NSMutableArray<NSString *> *filteredLanguages = [NSMutableArray array];
    NSMutableSet<NSString *> *seenLanguages = [NSMutableSet set];
    for (NSString *candidate in candidates) {
        NSString *supportedLanguage = woxOCRCanonicalSupportedLanguage(candidate, supportedLanguages);
        if (supportedLanguage == nil) {
            continue;
        }
        NSString *key = [supportedLanguage lowercaseString];
        if ([seenLanguages containsObject:key]) {
            continue;
        }
        [seenLanguages addObject:key];
        [filteredLanguages addObject:supportedLanguage];
    }
    return filteredLanguages;
}

static char* woxOCRRecognizeVisionText(const char *path, const char *languages, char **errorOut) {
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
            NSArray<NSString *> *candidateLanguages = woxOCRVisionLanguageCandidates(languages);
            NSArray<NSString *> *supportedLanguages = woxOCRSupportedVisionLanguages(request, candidateLanguages);
            if ([supportedLanguages count] > 0) {
                request.recognitionLanguages = supportedLanguages;
            }

            VNImageRequestHandler *handler = [[VNImageRequestHandler alloc] initWithCGImage:image options:@{}];
            NSError *performError = nil;
            BOOL ok = [handler performRequests:@[request] error:&performError];
            CGImageRelease(image);
            if (!ok) {
                if (errorOut != NULL) {
                    *errorOut = woxOCRDuplicateError(performError);
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
	"strings"
	"unsafe"
)

func recognizePlatform(ctx context.Context, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	cPath := C.CString(request.ImagePath)
	defer C.free(unsafe.Pointer(cPath))
	cLanguages := C.CString(strings.Join(request.Languages, "\n"))
	defer C.free(unsafe.Pointer(cLanguages))

	var cErr *C.char
	cText := C.woxOCRRecognizeVisionText(cPath, cLanguages, &cErr)
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
