//go:build darwin

#import "floating_material_darwin.h"
#import <Accelerate/Accelerate.h>
#include <math.h>
#include <stdlib.h>

// Blur only the physical sample region, leaving all output clipping to CoreGraphics.
// A half-logical-resolution sample and three box passes approximate the shared
// Gaussian without a GPU device or persistent window-sized scratch buffers.
bool wox_darwin_blur_material(CGContextRef context, float scale, CGRect bounds, float radius, float sigma, float margin, uint8_t red, uint8_t green, uint8_t blue) {
  if (context == NULL || scale <= 0 || sigma <= 0 || CGRectIsEmpty(bounds)) {
    return false;
  }
  size_t width = CGBitmapContextGetWidth(context);
  size_t height = CGBitmapContextGetHeight(context);
  size_t stride = CGBitmapContextGetBytesPerRow(context);
  uint8_t *pixels = CGBitmapContextGetData(context);
  if (pixels == NULL || CGRectIsEmpty(CGRectIntersection(bounds, CGContextGetClipBoundingBox(context)))) {
    return false;
  }
  size_t left = fmin(width, fmax(0, floor((CGRectGetMinX(bounds) - margin) * scale)));
  size_t top = fmin(height, fmax(0, floor((CGRectGetMinY(bounds) - margin) * scale)));
  size_t right = fmin(width, fmax(0, ceil((CGRectGetMaxX(bounds) + margin) * scale)));
  size_t bottom = fmin(height, fmax(0, ceil((CGRectGetMaxY(bounds) + margin) * scale)));
  if (right <= left || bottom <= top) {
    return false;
  }
  size_t region_width = right - left;
  size_t region_height = bottom - top;
  // Blur removes detail at this scale anyway; avoid CPU convolution and tone mapping
  // at Retina resolution for every frame of a scrolling result list.
  size_t sample_width = fmax(1, ceil(region_width / scale / 2));
  size_t sample_height = fmax(1, ceil(region_height / scale / 2));
  size_t sample_stride = sample_width * 4;
  size_t size = sample_stride * sample_height;
  uint8_t *source = malloc(size);
  uint8_t *scratch = malloc(size);
  if (source == NULL || scratch == NULL) {
    free(source);
    free(scratch);
    return false;
  }
  CGContextFlush(context);
  vImage_Buffer backdrop = {pixels + top * stride + left * 4, region_height, region_width, stride};
  vImage_Buffer input = {source, sample_height, sample_width, sample_stride};
  vImage_Buffer output = {scratch, sample_height, sample_width, sample_stride};
  vImage_Error status = vImageScale_ARGB8888(&backdrop, &input, NULL, kvImageNoFlags);
  // Three box kernels of width 2*sigma have total variance approximately sigma².
  uint32_t kernel_width = (uint32_t)fmax(1, floor(2 * sigma * scale * sample_width / region_width)) | 1;
  uint32_t kernel_height = (uint32_t)fmax(1, floor(2 * sigma * scale * sample_height / region_height)) | 1;
  for (int pass = 0; pass < 3 && status == kvImageNoError; pass++) {
    status = vImageBoxConvolve_ARGB8888(&input, &output, NULL, 0, 0, kernel_height, kernel_width, NULL, kvImageEdgeExtend);
    vImage_Buffer swap = input;
    input = output;
    output = swap;
  }
  if (status != kvImageNoError) {
    free(source);
    free(scratch);
    return false;
  }
  // Match the Windows backdrop contrast/chroma compression in premultiplied BGRA.
  // The bias scales with sampled alpha, so transparent content stays transparent.
  const float tint[] = {blue, green, red};
  const float tint_luma = 0.2126f * red + 0.7152f * green + 0.0722f * blue;
  uint8_t *blurred = input.data;
  for (size_t offset = 0; offset < size; offset += 4) {
    float alpha = blurred[offset + 3] / 255.0f;
    float luma = 0.2126f * blurred[offset + 2] + 0.7152f * blurred[offset + 1] + 0.0722f * blurred[offset];
    for (int channel = 0; channel < 3; channel++) {
      float value = 0.125f * (blurred[offset + channel] + luma) + (tint[channel] - 0.125f * (tint[channel] + tint_luma)) * alpha;
      blurred[offset + channel] = (uint8_t)fminf(blurred[offset + 3], fmaxf(0, roundf(value)));
    }
  }
  CGColorSpaceRef color_space = CGBitmapContextGetColorSpace(context);
  CGContextRef sample = CGBitmapContextCreate(blurred, sample_width, sample_height, 8, sample_stride, color_space, kCGImageAlphaPremultipliedFirst | kCGBitmapByteOrder32Little);
  CGImageRef image = sample != NULL ? CGBitmapContextCreateImage(sample) : NULL;
  bool painted = image != NULL;
  if (painted) {
    CGFloat corner = fmin(fmax(0, radius), fmin(bounds.size.width, bounds.size.height) / 2);
    CGPathRef path = CGPathCreateWithRoundedRect(bounds, corner, corner, NULL);
    CGContextSaveGState(context);
    CGContextAddPath(context, path);
    CGContextClip(context);
    // Replace sampled alpha rather than compositing a second copy of the backdrop.
    CGContextSetBlendMode(context, kCGBlendModeCopy);
    CGContextSetInterpolationQuality(context, kCGInterpolationLow);
    CGRect target = CGRectMake(left / scale, top / scale, region_width / scale, region_height / scale);
    CGContextTranslateCTM(context, target.origin.x, CGRectGetMaxY(target));
    CGContextScaleCTM(context, 1, -1);
    CGContextDrawImage(context, CGRectMake(0, 0, target.size.width, target.size.height), image);
    CGContextRestoreGState(context);
    CGPathRelease(path);
    CGImageRelease(image);
  }
  if (sample != NULL) {
    CGContextRelease(sample);
  }
  free(source);
  free(scratch);
  return painted;
}
