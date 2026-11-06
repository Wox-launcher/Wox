# Images and geometry

This package owns immutable renderer images, GIF frame retention, physical pixel
buffers, packed BGRX conversion and portable geometry. It has no window, input,
capture or renderer-lifecycle dependency.

`Image.NativePixels` returns shared immutable storage for upload; encoders retain
the image and any external capture backing it and never write the slice. Native format values remain the
renderer's existing RGBA/BGRX contract. Logical `Point`, `Rect` and `Size` stay
distinct from physical `PixelSize` and image buffer dimensions.

`AffineTransform` uses column-vector coordinates with explicit application-order
composition (`local.Then(parent).Then(display)`). Translations carry the caller's
units; scale and rotation do not perform an implicit DPI conversion. Its zero
value collapses geometry; use `IdentityTransform` for an unchanged scene.
Inverse calculation rejects singular/nonfinite maps without discarding valid
small scales. Exact transformed rectangle polygons are separate from conservative
axis-aligned paint bounds.

`ClipConvexPolygon` intersects finite convex clips with either winding, including
reflections and negative desktop origins. Its output is not limited by the small
fill-polygon command's vertex budget: intersecting many rotated rectangular clips
can require many more vertices. These helpers have no native/window dependencies.
