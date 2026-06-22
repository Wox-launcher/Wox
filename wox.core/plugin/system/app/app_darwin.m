#import <Cocoa/Cocoa.h>
#import <Foundation/Foundation.h>
#if __has_include(<UniformTypeIdentifiers/UniformTypeIdentifiers.h>)
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#endif
#include <stdlib.h>
#include <stdio.h>
#include <sys/sysctl.h>
#include <libproc.h>

static NSImage *GetWorkspaceIconForExtension(NSString *extension) {
    NSWorkspace *workspace = [NSWorkspace sharedWorkspace];

    if (@available(macOS 11.0, *)) {
#if __has_include(<UniformTypeIdentifiers/UniformTypeIdentifiers.h>)
        if ([extension length] > 0) {
            UTType *contentType = [UTType typeWithFilenameExtension:extension];
            if (contentType != nil) {
                return [workspace iconForContentType:contentType];
            }
        }

        return [workspace iconForContentType:UTTypeData];
#endif
    }

#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    return [workspace iconForFileType:extension];
#pragma clang diagnostic pop
}

static NSImage *CreateTintedImage(NSImage *image, NSColor *color) {
    if (image == nil || color == nil) {
        return image;
    }

    NSImage *tintedImage = [[[NSImage alloc] initWithSize:[image size]] autorelease];
    [tintedImage lockFocus];
    [image drawInRect:NSMakeRect(0, 0, image.size.width, image.size.height)
             fromRect:NSZeroRect
            operation:NSCompositingOperationSourceOver
             fraction:1.0];
    [color set];
    NSRectFillUsingOperation(NSMakeRect(0, 0, image.size.width, image.size.height), NSCompositingOperationSourceAtop);
    [tintedImage unlockFocus];

    return tintedImage;
}

// Helper function to get NSColor from color name string
static NSColor* colorFromName(NSString *colorName) {
    if ([colorName isEqualToString:@"blue"]) return [NSColor systemBlueColor];
    if ([colorName isEqualToString:@"red"]) return [NSColor systemRedColor];
    if ([colorName isEqualToString:@"gray"]) return [NSColor systemGrayColor];
    if ([colorName isEqualToString:@"indigo"]) return [NSColor systemIndigoColor];
    if ([colorName isEqualToString:@"pink"]) return [NSColor systemPinkColor];
    if ([colorName isEqualToString:@"purple"]) return [NSColor systemPurpleColor];
    if ([colorName isEqualToString:@"cyan"]) {
        if (@available(macOS 12.0, *)) {
            return [NSColor systemCyanColor];
        }
        return [NSColor colorWithSRGBRed:0.04 green:0.68 blue:0.80 alpha:1.0];
    }
    if ([colorName isEqualToString:@"orange"]) return [NSColor systemOrangeColor];
    if ([colorName isEqualToString:@"green"]) return [NSColor systemGreenColor];
    if ([colorName isEqualToString:@"teal"]) return [NSColor systemTealColor];
    if ([colorName isEqualToString:@"yellow"]) return [NSColor systemYellowColor];
    if ([colorName isEqualToString:@"brown"]) return [NSColor systemBrownColor];
    return [NSColor systemGrayColor]; // Default fallback
}

// Generate an icon using SF Symbols with a colored background
// iconStyle: "filled" = colored bg + white symbol, "outlined" = white bg + colored symbol
// Returns PNG data, caller must free the returned bytes
const unsigned char *GenerateSFSymbolIcon(const char *symbolName, const char *colorName, const char *iconStyle, size_t *length) {
    @autoreleasepool {
        if (@available(macOS 11.0, *)) {
            NSString *symbol = [NSString stringWithUTF8String:symbolName];
            NSString *color = [NSString stringWithUTF8String:colorName];
            NSString *style = [NSString stringWithUTF8String:iconStyle];
            
            BOOL isOutlined = [style isEqualToString:@"outlined"];
            
            // Configure symbol color based on style
            NSColor *symbolColor = isOutlined ? colorFromName(color) : [NSColor whiteColor];
            CGFloat symbolWeight = NSFontWeightBold;
            CGFloat symbolPointSize = 180;
            NSImageSymbolConfiguration *weightConfig = [NSImageSymbolConfiguration configurationWithPointSize:symbolPointSize weight:symbolWeight scale:NSImageSymbolScaleLarge];
            NSImage *symbolImage = [NSImage imageWithSystemSymbolName:symbol accessibilityDescription:nil];
            
            if (!symbolImage) {
                *length = 0;
                return NULL;
            }
            
            symbolImage = [symbolImage imageWithSymbolConfiguration:weightConfig];
            if (@available(macOS 12.0, *)) {
                NSImageSymbolConfiguration *colorConfig = [NSImageSymbolConfiguration configurationWithPaletteColors:@[symbolColor, symbolColor, symbolColor]];
                symbolImage = [symbolImage imageWithSymbolConfiguration:colorConfig];
            } else {
                symbolImage = CreateTintedImage(symbolImage, symbolColor);
            }
            
            NSSize size = NSMakeSize(256, 256);
            NSImage *icon = [[NSImage alloc] initWithSize:size];
            [icon lockFocus];
            
            // Draw rounded background
            NSBezierPath *bgPath = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(0, 0, 256, 256) xRadius:56 yRadius:56];
            NSColor *bgColor = isOutlined ? [NSColor whiteColor] : colorFromName(color);
            [bgColor set];
            [bgPath fill];
            
            // Draw symbol (centered, preserving aspect ratio)
            // Get natural size of symbol and scale to fit within max bounds while preserving ratio
            NSSize symbolSize = [symbolImage size];
            CGFloat maxSymbolSize = 200; // Max width/height for symbol
            CGFloat scale = MIN(maxSymbolSize / symbolSize.width, maxSymbolSize / symbolSize.height);
            CGFloat scaledWidth = symbolSize.width * scale;
            CGFloat scaledHeight = symbolSize.height * scale;
            CGFloat x = (256 - scaledWidth) / 2;
            CGFloat y = (256 - scaledHeight) / 2;
            NSRect symbolRect = NSMakeRect(x, y, scaledWidth, scaledHeight);
            [symbolImage drawInRect:symbolRect fromRect:NSZeroRect operation:NSCompositingOperationSourceOver fraction:1.0];
            
            [icon unlockFocus];
            
            NSData *tiffData = [icon TIFFRepresentation];
            NSBitmapImageRep *imageRep = [NSBitmapImageRep imageRepWithData:tiffData];
            NSDictionary *imageProps = @{};
            NSData *pngData = [imageRep representationUsingType:NSBitmapImageFileTypePNG properties:imageProps];
            
            *length = [pngData length];
            unsigned char *bytes = (unsigned char *)malloc(*length);
            memcpy(bytes, [pngData bytes], *length);
            // Bug fix: this file is compiled without ARC, so the rendered image
            // must be released after the PNG bytes are copied. Keeping it alive
            // retained native CG image memory in the core process after startup.
            [icon release];
            
            return bytes;
        }
        
        *length = 0;
        return NULL;
    }
}

const unsigned char *GetPrefPaneIcon(const char *prefPanePath, size_t *length) {
    @autoreleasepool {
        NSString *path = [NSString stringWithUTF8String:prefPanePath];

        NSImage *icon = nil;
        NSImage *ownedIcon = nil;

        // NOTE: SF Symbol-based icons are now generated via GenerateSFSymbolIcon called from Go.
        // This function only handles traditional icon loading from plist/resources.
        
        NSString *plistPath = [path stringByAppendingPathComponent:@"Contents/Info.plist"];
        NSDictionary *plist = [NSDictionary dictionaryWithContentsOfFile:plistPath];

        // Try NSPrefPaneIconFile first (specific to prefPane)
        NSString *iconFile = plist[@"NSPrefPaneIconFile"];
        if (iconFile && ![iconFile isEqualToString:@""]) {
            // Try to load from system resources
            icon = [NSImage imageNamed:iconFile];

            // If not found in system, try in prefPane's Resources
            if (!icon) {
                NSString *iconPath = [[path stringByAppendingPathComponent:@"Contents/Resources"] stringByAppendingPathComponent:iconFile];
                if (![[iconPath pathExtension] isEqualToString:@"icns"] && ![[iconPath pathExtension] isEqualToString:@"png"]) {
                    iconPath = [iconPath stringByAppendingPathExtension:@"icns"];
                }
                icon = [[NSImage alloc] initWithContentsOfFile:iconPath];
                ownedIcon = icon;
            }
        }

        // Try CFBundleIconName from CFBundleIcons
        if (!icon) {
            NSDictionary *bundleIcons = plist[@"CFBundleIcons"];
            if (bundleIcons) {
                NSDictionary *primaryIcon = bundleIcons[@"CFBundlePrimaryIcon"];
                if (primaryIcon) {
                    NSString *iconName = primaryIcon[@"CFBundleIconName"];
                    if (iconName) {
                        icon = [NSImage imageNamed:iconName];
                    }
                }
            }
        }

        // Fallback: try to get the actual icon for this specific prefPane file
        // This works for system prefPanes like Security.prefPane where the icon
        // is embedded and can't be accessed via imageNamed
        if (!icon) {
            icon = [[NSWorkspace sharedWorkspace] iconForFile:path];
        }
        
        // Last resort: generic prefPane icon using UTI
        if (!icon) {
            icon = GetWorkspaceIconForExtension(@"prefPane");
        }

        if (icon == nil) {
            return NULL;
        }

        // Render the icon properly - this is needed because NSISIconImageRep
        // (used by system prefPane icons like Security.prefPane) doesn't render
        // correctly when using TIFFRepresentation directly
        // For our generated SF Symbol icon, this redraw re-normalizes it, which is fine.
        NSSize targetSize = NSMakeSize(256, 256);
        NSImage *renderedIcon = [[NSImage alloc] initWithSize:targetSize];
        [renderedIcon lockFocus];
        [[NSGraphicsContext currentContext] setImageInterpolation:NSImageInterpolationHigh];
        [icon drawInRect:NSMakeRect(0, 0, targetSize.width, targetSize.height)
                fromRect:NSZeroRect
               operation:NSCompositingOperationSourceOver
                fraction:1.0];
        [renderedIcon unlockFocus];

        NSData *tiffData = [renderedIcon TIFFRepresentation];
        NSBitmapImageRep *imageRep = [NSBitmapImageRep imageRepWithData:tiffData];
        NSDictionary *imageProps = [NSDictionary dictionaryWithObject:[NSNumber numberWithFloat:1.0] forKey:NSImageCompressionFactor];
        NSData *pngData = [imageRep representationUsingType:NSBitmapImageFileTypePNG properties:imageProps];

        *length = [pngData length];
        unsigned char *bytes = (unsigned char *)malloc(*length);
        memcpy(bytes, [pngData bytes], *length);
        // Bug fix: manual retain/release is used here. Release owned native
        // images after rendering so cached preference-pane icons do not leave
        // decoded CG images resident in wox.core.
        [renderedIcon release];
        if (ownedIcon != nil) {
            [ownedIcon release];
        }

        return bytes;
    }
}

static void AddLocalizedAppName(NSMutableOrderedSet *names, NSString *name) {
    if (names == nil || name == nil || ![name isKindOfClass:[NSString class]]) {
        return;
    }

    NSString *trimmed = [name stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
    if ([trimmed length] == 0) {
        return;
    }

    [names addObject:trimmed];
}

char* GetLocalizedAppNames(const char *appPath) {
    @autoreleasepool {
        NSString *path = [NSString stringWithUTF8String:appPath];
        if (!path) {
            return NULL;
        }

        NSBundle *bundle = [NSBundle bundleWithPath:path];
        if (!bundle) {
            return NULL;
        }

        NSMutableOrderedSet *names = [NSMutableOrderedSet orderedSet];

        // Bug fix: objectForInfoDictionaryKey only returns one locale-dependent
        // value. Users can search localized names from other macOS languages
        // when Spotlight is disabled, so collect Finder's current display name
        // plus every InfoPlist.loctable/InfoPlist.strings display alias.
        AddLocalizedAppName(names, [[NSFileManager defaultManager] displayNameAtPath:path]);
        AddLocalizedAppName(names, [bundle objectForInfoDictionaryKey:@"CFBundleDisplayName"]);
        AddLocalizedAppName(names, [bundle objectForInfoDictionaryKey:@"CFBundleName"]);

        NSString *loctablePath = [bundle pathForResource:@"InfoPlist" ofType:@"loctable"];
        NSDictionary *loctable = loctablePath ? [NSDictionary dictionaryWithContentsOfFile:loctablePath] : nil;
        for (id localization in loctable) {
            if ([localization isEqual:@"LocProvenance"]) {
                continue;
            }

            NSDictionary *localizedValues = loctable[localization];
            if (![localizedValues isKindOfClass:[NSDictionary class]]) {
                continue;
            }

            AddLocalizedAppName(names, localizedValues[@"CFBundleDisplayName"]);
            AddLocalizedAppName(names, localizedValues[@"CFBundleName"]);
        }

        for (NSString *localization in [bundle localizations]) {
            NSString *stringsPath = [bundle pathForResource:@"InfoPlist" ofType:@"strings" inDirectory:nil forLocalization:localization];
            if (!stringsPath || [stringsPath length] == 0) {
                continue;
            }

            NSDictionary *strings = [NSDictionary dictionaryWithContentsOfFile:stringsPath];
            if (!strings) {
                continue;
            }

            AddLocalizedAppName(names, strings[@"CFBundleDisplayName"]);
            AddLocalizedAppName(names, strings[@"CFBundleName"]);
        }

        if ([names count] == 0) {
            return NULL;
        }

        NSString *joined = [[names array] componentsJoinedByString:@"\n"];
        const char *utf8 = [joined UTF8String];
        if (!utf8) {
            return NULL;
        }

        return strdup(utf8);
    }
}

int get_process_list(struct kinfo_proc **procList, size_t *procCount) {
    int err;
    size_t length;
    static const int name[] = { CTL_KERN, KERN_PROC, KERN_PROC_ALL, 0 };

    if (sysctl((int *)name, 4, NULL, &length, NULL, 0) == -1) {
        return -1;
    }

    *procList = malloc(length);
    if (!*procList) {
        return -1;
    }

    if (sysctl((int *)name, 4, *procList, &length, NULL, 0) == -1) {
        free(*procList);
        return -1;
    }

    *procCount = length / sizeof(struct kinfo_proc);

    return 0;
}

char* get_process_path(pid_t pid) {
    char *path = (char *)malloc(PROC_PIDPATHINFO_MAXSIZE);
    if (!path) {
        return NULL;
    }

    if (proc_pidpath(pid, path, PROC_PIDPATHINFO_MAXSIZE) <= 0) {
        free(path);
        return NULL;
    }

    return path;
}
