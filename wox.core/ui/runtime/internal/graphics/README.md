# Images and geometry

This package owns immutable renderer images, GIF frame retention, physical pixel
buffers, packed BGRX conversion and portable geometry. It has no window, input,
capture or renderer-lifecycle dependency.

`Image.NativePixels` returns shared immutable storage for upload; encoders retain
the image and any external capture backing it and never write the slice. Native format values remain the
renderer's existing RGBA/BGRX contract. Logical `Point`, `Rect` and `Size` stay
distinct from physical `PixelSize` and image buffer dimensions.
