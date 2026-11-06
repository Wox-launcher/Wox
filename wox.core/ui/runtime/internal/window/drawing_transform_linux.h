// Shared GL drawing state; all coordinates here are logical target coordinates.
// A texture buffer keeps convex clips independent of the polygon-fill vertex limit.
#define WOX_GL_CLIP_SOURCE \
    "uniform samplerBuffer u_clip_planes;\n" \
    "uniform int u_clip_count;\n" \
    "uniform vec2 u_clip_target;\n" \
    "bool wox_inside_clip() {\n" \
    "  if (u_clip_count < 0) return false;\n" \
    "  vec2 p = vec2(gl_FragCoord.x, u_clip_target.x - gl_FragCoord.y) / u_clip_target.y;\n" \
    "  for (int i = 0; i < u_clip_count; i++) {\n" \
    "    vec4 edge = texelFetch(u_clip_planes, i);\n" \
    "    if (dot(p - edge.xy, edge.zw) < -0.00001) return false;\n" \
    "  }\n" \
    "  return true;\n" \
    "}\n"

static const float linux_identity_transform[6] = {1, 0, 0, 1, 0, 0};

static const float *linux_paint_transform(const WoxLinuxRenderer *renderer) {
  return renderer->transform_initialized ? renderer->transform : linux_identity_transform;
}

// Group bounds retain the matrix at entry, independently of subsequent child paints.
static bool linux_transformed_rect(const WoxLinuxRenderer *renderer, float x, float y, float width, float height, float points[8], float bounds[4]) {
  const float *m = linux_paint_transform(renderer);
  const double xs[4] = {x, (double)x + width, (double)x + width, x};
  const double ys[4] = {y, y, (double)y + height, (double)y + height};
  for (int i = 0; i < 4; i++) {
    points[2*i] = m[0]*xs[i] + m[2]*ys[i] + m[4];
    points[2*i+1] = m[1]*xs[i] + m[3]*ys[i] + m[5];
    if (!isfinite(points[2*i]) || !isfinite(points[2*i+1])) return false;
    if (i == 0) {
      bounds[0] = bounds[2] = points[0];
      bounds[1] = bounds[3] = points[1];
    } else {
      bounds[0] = fminf(bounds[0], points[2*i]);
      bounds[1] = fminf(bounds[1], points[2*i+1]);
      bounds[2] = fmaxf(bounds[2], points[2*i]);
      bounds[3] = fmaxf(bounds[3], points[2*i+1]);
    }
  }
  return true;
}

// Each convex polygon contributes inward half-planes; all scopes intersect naturally.
static bool append_linux_clip_planes(GArray *planes, const float *points, unsigned count) {
  double area = 0;
  for (unsigned i = 1; i + 1 < count; i++) {
    area += ((double)points[2*i] - points[0]) * ((double)points[2*i+3] - points[1]) -
            ((double)points[2*i+1] - points[1]) * ((double)points[2*i+2] - points[0]);
  }
  if (area == 0 || !isfinite(area)) return false;
  double sign = area > 0 ? 1 : -1;
  for (unsigned i = 0; i < count; i++) {
    unsigned j = (i + 1) % count;
    double dx = (double)points[2*j] - points[2*i];
    double dy = (double)points[2*j+1] - points[2*i+1];
    double length = hypot(dx, dy);
    if (length == 0) continue;
    float plane[4] = {points[2*i], points[2*i+1], -sign*dy/length, sign*dx/length};
    g_array_append_vals(planes, plane, 4);
  }
  return true;
}

// Upload only when a clip scope changes, preserving the current image texture unit.
static int32_t rebuild_linux_clip(WoxLinuxRenderer *renderer) {
  renderer->clip_plane_count = 0;
  if (renderer->edge_fade_depth == 0 && (renderer->clip_points == NULL || renderer->clip_points->len == 0)) return 0;
  if (renderer->clip_planes == NULL) renderer->clip_planes = g_array_new(FALSE, FALSE, sizeof(float));
  g_array_set_size(renderer->clip_planes, 0);
  for (unsigned i = 0; i < renderer->edge_fade_depth; i++) {
    WoxLinuxEdgeFade *fade = &g_array_index(renderer->edge_fades, WoxLinuxEdgeFade, i);
    if (!append_linux_clip_planes(renderer->clip_planes, fade->polygon, 4)) {
      renderer->clip_plane_count = -1;
      return 0;
    }
  }
  if (renderer->clip_points != NULL && renderer->clip_points->len > 0 &&
      !append_linux_clip_planes(renderer->clip_planes, (float *)renderer->clip_points->data, renderer->clip_points->len / 2)) {
    renderer->clip_plane_count = -1;
    return 0;
  }
  GLint limit, active;
  glGetIntegerv(GL_MAX_TEXTURE_BUFFER_SIZE, &limit);
  if (renderer->clip_planes->len / 4 > (unsigned)limit) return -1;
  glGetIntegerv(GL_ACTIVE_TEXTURE, &active);
  glActiveTexture(GL_TEXTURE2);
  if (renderer->clip_buffer == 0) glGenBuffers(1, &renderer->clip_buffer);
  if (renderer->clip_texture == 0) glGenTextures(1, &renderer->clip_texture);
  glBindBuffer(GL_TEXTURE_BUFFER, renderer->clip_buffer);
  glBufferData(GL_TEXTURE_BUFFER, renderer->clip_planes->len * sizeof(float), renderer->clip_planes->data, GL_STREAM_DRAW);
  renderer->clip_buffer_bytes = renderer->clip_planes->len * sizeof(float);
  glBindTexture(GL_TEXTURE_BUFFER, renderer->clip_texture);
  glTexBuffer(GL_TEXTURE_BUFFER, GL_RGBA32F, renderer->clip_buffer);
  glBindBuffer(GL_TEXTURE_BUFFER, 0);
  glActiveTexture(active);
  renderer->clip_plane_count = renderer->clip_planes->len / 4;
  return glGetError() == GL_NO_ERROR ? 0 : -1;
}

// Presentation chrome bypasses command transforms and clips; paint uses both.
static void apply_linux_drawing_state(WoxLinuxRenderer *renderer, GLuint program, const float *transform, bool paint) {
  const float *m = transform != NULL ? transform : linux_paint_transform(renderer);
  float matrix[9] = {m[0], m[1], 0, m[2], m[3], 0, m[4], m[5], 1};
  glUniformMatrix3fv(glGetUniformLocation(program, "u_transform"), 1, GL_FALSE, matrix);
  glUniform1i(glGetUniformLocation(program, "u_clip_planes"), 2);
  glUniform1i(glGetUniformLocation(program, "u_clip_count"), paint ? renderer->clip_plane_count : 0);
  glUniform2f(glGetUniformLocation(program, "u_clip_target"), renderer->frame_height, renderer->scale);
  if (paint && renderer->clip_plane_count > 0) {
    GLint active;
    glGetIntegerv(GL_ACTIVE_TEXTURE, &active);
    glActiveTexture(GL_TEXTURE2);
    glBindTexture(GL_TEXTURE_BUFFER, renderer->clip_texture);
    glActiveTexture(active);
  }
}

// Reset abandoned groups after failed encodes as well as ordinary frame boundaries.
static void reset_linux_drawing_state(WoxLinuxRenderer *renderer) {
  renderer->transform_initialized = false;
  renderer->edge_fade_depth = 0;
  renderer->clip_plane_count = 0;
  if (renderer->clip_points != NULL) g_array_set_size(renderer->clip_points, 0);
}
