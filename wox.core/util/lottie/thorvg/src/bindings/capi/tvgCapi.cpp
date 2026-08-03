/*
 * Copyright (c) 2020 - 2026 ThorVG project. All rights reserved.

 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:

 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.

 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

#include "config.h"
#include <string>
#include <thorvg.h>
#include "thorvg_capi.h"
#ifdef THORVG_LOTTIE_LOADER_SUPPORT
#include <thorvg_lottie.h>
#endif

using namespace std;
using namespace tvg;

#ifdef __cplusplus
extern "C" {
#endif


/************************************************************************/
/* Engine API                                                           */
/************************************************************************/

TVG_API Tvg_Result tvg_engine_init(unsigned threads)
{
    return (Tvg_Result) Initializer::init(threads);
}


TVG_API Tvg_Result tvg_engine_term()
{
    return (Tvg_Result) Initializer::term();
}


TVG_API Tvg_Result tvg_engine_version(uint32_t* major, uint32_t* minor, uint32_t* micro, const char** version)
{
    if (version) *version = Initializer::version(major, minor, micro);
    return TVG_RESULT_SUCCESS;
}

/************************************************************************/
/* Canvas API                                                           */
/************************************************************************/

TVG_API Tvg_Canvas tvg_swcanvas_create(Tvg_Engine_Option op)
{
    return (Tvg_Canvas) SwCanvas::gen(static_cast<EngineOption>(op));
}


TVG_API Tvg_Canvas tvg_glcanvas_create(Tvg_Engine_Option op)
{
    return (Tvg_Canvas) GlCanvas::gen(static_cast<EngineOption>(op));
}


TVG_API Tvg_Canvas tvg_wgcanvas_create(Tvg_Engine_Option op)
{
    return (Tvg_Canvas) WgCanvas::gen(static_cast<EngineOption>(op));
}


TVG_API Tvg_Result tvg_canvas_destroy(Tvg_Canvas canvas)
{
    if (canvas) {
        delete(reinterpret_cast<Canvas*>(canvas));
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_swcanvas_set_target(Tvg_Canvas canvas, uint32_t* buffer, uint32_t stride, uint32_t w, uint32_t h, Tvg_Colorspace cs)
{
    if (canvas) return (Tvg_Result) reinterpret_cast<SwCanvas*>(canvas)->target(buffer, stride, w, h, static_cast<ColorSpace>(cs));
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_glcanvas_set_target(Tvg_Canvas canvas, void* display, void* surface, void* context, int32_t id, uint32_t w, uint32_t h, Tvg_Colorspace cs)
{
    if (canvas) return (Tvg_Result) reinterpret_cast<GlCanvas*>(canvas)->target(display, surface, context, id, w, h, static_cast<ColorSpace>(cs));
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_wgcanvas_set_target(Tvg_Canvas canvas, void* device, void* instance, void* target, uint32_t w, uint32_t h, Tvg_Colorspace cs, int type)
{
    if (canvas) return (Tvg_Result) reinterpret_cast<WgCanvas*>(canvas)->target(device, instance, target, w, h, static_cast<ColorSpace>(cs), type);
    return TVG_RESULT_INVALID_ARGUMENT;
}

TVG_API Tvg_Result tvg_wgcanvas_set_target_with_context(Tvg_Canvas canvas, const Tvg_WgContext* context, void* target, uint32_t w, uint32_t h, Tvg_Colorspace cs, int type)
{
    if (canvas && context) {
        auto ctx = reinterpret_cast<WgCanvas::Context*>(const_cast<Tvg_WgContext*>(context));
        return (Tvg_Result) reinterpret_cast<WgCanvas*>(canvas)->target(*ctx, target, w, h, static_cast<ColorSpace>(cs), type);
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}

TVG_API Tvg_Result tvg_canvas_add(Tvg_Canvas canvas, Tvg_Paint paint)
{
    if (canvas && paint) return (Tvg_Result) reinterpret_cast<Canvas*>(canvas)->add((Paint*)paint);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_canvas_insert(Tvg_Canvas canvas, Tvg_Paint target, Tvg_Paint at)
{
    if (canvas && target && at) return (Tvg_Result) reinterpret_cast<Canvas*>(canvas)->add((Paint*)target, (Paint*) at);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_canvas_remove(Tvg_Canvas canvas, Tvg_Paint paint)
{
    if (canvas) return (Tvg_Result) reinterpret_cast<Canvas*>(canvas)->remove((Paint*) paint);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_canvas_update(Tvg_Canvas canvas)
{
    if (canvas) return (Tvg_Result) reinterpret_cast<Canvas*>(canvas)->update();
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_canvas_draw(Tvg_Canvas canvas, bool clear)
{
    if (canvas) return (Tvg_Result) reinterpret_cast<Canvas*>(canvas)->draw(clear);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_canvas_sync(Tvg_Canvas canvas)
{
    if (canvas) return (Tvg_Result) reinterpret_cast<Canvas*>(canvas)->sync();
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_canvas_set_viewport(Tvg_Canvas canvas, int32_t x, int32_t y, int32_t w, int32_t h)
{
    if (canvas) return (Tvg_Result) reinterpret_cast<Canvas*>(canvas)->viewport(x, y, w, h);
    return TVG_RESULT_INVALID_ARGUMENT;
}


/************************************************************************/
/* Paint API                                                            */
/************************************************************************/

TVG_API Tvg_Paint tvg_paint_get_parent(const Tvg_Paint paint)
{
    return (Tvg_Paint) reinterpret_cast<const Paint*>(paint)->parent();
}


TVG_API Tvg_Result tvg_paint_rel(Tvg_Paint paint)
{
    Paint::rel(reinterpret_cast<Paint*>(paint));
    return TVG_RESULT_SUCCESS;
}


TVG_API Tvg_Result tvg_paint_set_visible(Tvg_Paint paint, bool visible)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Paint*>(paint)->visible(visible);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API bool tvg_paint_get_visible(const Tvg_Paint paint)
{
    if (paint) return reinterpret_cast<const Paint*>(paint)->visible();
    return false;
}

TVG_API uint32_t tvg_paint_get_id(const Tvg_Paint paint)
{
    if (paint) return reinterpret_cast<const Paint*>(paint)->id;
    return 0;
}

TVG_API Tvg_Result tvg_paint_set_id(Tvg_Paint paint, uint32_t id)
{
    if (paint) {
        reinterpret_cast<Paint*>(paint)->id = id;
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}

TVG_API uint16_t tvg_paint_ref(Tvg_Paint paint)
{
    if (paint) return reinterpret_cast<Paint*>(paint)->ref();
    return 0;
}


TVG_API uint16_t tvg_paint_unref(Tvg_Paint paint, bool free)
{
    if (paint) return reinterpret_cast<Paint*>(paint)->unref(free);
    return 0;
}


TVG_API uint16_t tvg_paint_get_ref(const Tvg_Paint paint)
{
    if (paint) return reinterpret_cast<const Paint*>(paint)->refCnt();
    return 0;
}


TVG_API Tvg_Result tvg_paint_scale(Tvg_Paint paint, float factor)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Paint*>(paint)->scale(factor);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_paint_rotate(Tvg_Paint paint, float degree)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Paint*>(paint)->rotate(degree);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_paint_translate(Tvg_Paint paint, float x, float y)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Paint*>(paint)->translate(x, y);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_paint_set_transform(Tvg_Paint paint, const Tvg_Matrix* m)
{
    if (paint && m) return (Tvg_Result) reinterpret_cast<Paint*>(paint)->transform(*(reinterpret_cast<const Matrix*>(m)));
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_paint_get_transform(Tvg_Paint paint, Tvg_Matrix* m)
{
    if (paint && m) {
        *reinterpret_cast<Matrix*>(m) = reinterpret_cast<Paint*>(paint)->transform();
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Paint tvg_paint_duplicate(Tvg_Paint paint)
{
    if (paint) return (Tvg_Paint) reinterpret_cast<Paint*>(paint)->duplicate();
    return nullptr;
}

TVG_API bool tvg_paint_intersects(Tvg_Paint paint, int32_t x, int32_t y, int32_t w, int32_t h)
{
    return tvg_paint_intersects_region(paint, x, y, w, h, false);
}

TVG_API bool tvg_paint_intersects_region(Tvg_Paint paint, int32_t x, int32_t y, int32_t w, int32_t h, bool visibleOnly)
{
    if (paint) return reinterpret_cast<Paint*>(paint)->intersects(x, y, w, h, visibleOnly);
    return false;
}

TVG_API Tvg_Result tvg_paint_set_opacity(Tvg_Paint paint, uint8_t opacity)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Paint*>(paint)->opacity(opacity);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_paint_get_opacity(const Tvg_Paint paint, uint8_t* opacity)
{
    if (paint && opacity) {
        *opacity = reinterpret_cast<const Paint*>(paint)->opacity();
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_paint_get_aabb(Tvg_Paint paint, float* x, float* y, float* w, float* h)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Paint*>(paint)->bounds(x, y, w, h);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_paint_get_obb(Tvg_Paint paint, Tvg_Point* pt4)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Paint*>(paint)->bounds((Point*)pt4);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_paint_set_mask_method(Tvg_Paint paint, Tvg_Paint target, Tvg_Mask_Method method)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Paint*>(paint)->mask((Paint*)target, (MaskMethod)method);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_paint_get_mask_method(const Tvg_Paint paint, const Tvg_Paint target, Tvg_Mask_Method* method)
{
    if (paint && target && method) {
        *reinterpret_cast<MaskMethod*>(method) = reinterpret_cast<const Paint*>(paint)->mask(reinterpret_cast<const Paint**>(target));
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_paint_set_blend_method(Tvg_Paint paint, Tvg_Blend_Method method)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Paint*>(paint)->blend((BlendMethod)method);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_paint_get_type(const Tvg_Paint paint, Tvg_Type* type)
{
    if (paint && type) {
        *type = static_cast<Tvg_Type>(reinterpret_cast<const Paint*>(paint)->type());
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_paint_set_clip(Tvg_Paint paint, Tvg_Paint clipper)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Paint*>(paint)->clip((Shape*)(clipper));
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Paint tvg_paint_get_clip(const Tvg_Paint paint)
{
   if (paint) return (Tvg_Paint) reinterpret_cast<const Paint*>(paint)->clip();
   return nullptr;
}


/************************************************************************/
/* Shape API                                                            */
/************************************************************************/

TVG_API Tvg_Paint tvg_shape_new()
{
    return (Tvg_Paint) Shape::gen();
}


TVG_API Tvg_Result tvg_shape_reset(Tvg_Paint paint)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->reset();
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_move_to(Tvg_Paint paint, float x, float y)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->moveTo(x, y);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_line_to(Tvg_Paint paint, float x, float y)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->lineTo(x, y);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_cubic_to(Tvg_Paint paint, float cx1, float cy1, float cx2, float cy2, float x, float y)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->cubicTo(cx1, cy1, cx2, cy2, x, y);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_close(Tvg_Paint paint)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->close();
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_append_rect(Tvg_Paint paint, float x, float y, float w, float h, float rx, float ry, bool cw)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->appendRect(x, y, w, h, rx, ry, cw);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_append_circle(Tvg_Paint paint, float cx, float cy, float rx, float ry, bool cw)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->appendCircle(cx, cy, rx, ry, cw);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_append_path(Tvg_Paint paint, const Tvg_Path_Command* cmds, uint32_t cmdCnt, const Tvg_Point* pts, uint32_t ptsCnt)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->appendPath((const PathCommand*)cmds, cmdCnt, (const Point*)pts, ptsCnt);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_get_path(const Tvg_Paint paint, const Tvg_Path_Command** cmds, uint32_t* cmdsCnt, const Tvg_Point** pts, uint32_t* ptsCnt)
{
    if (paint) return (Tvg_Result) reinterpret_cast<const Shape*>(paint)->path((const PathCommand**)cmds, cmdsCnt, (const Point**)pts, ptsCnt);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_set_stroke_width(Tvg_Paint paint, float width)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->strokeWidth(width);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_get_stroke_width(const Tvg_Paint paint, float* width)
{
    if (paint && width) {
        *width = reinterpret_cast<const Shape*>(paint)->strokeWidth();
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_set_stroke_color(Tvg_Paint paint, uint8_t r, uint8_t g, uint8_t b, uint8_t a)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->strokeFill(r, g, b, a);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_get_stroke_color(const Tvg_Paint paint, uint8_t* r, uint8_t* g, uint8_t* b, uint8_t* a)
{
    if (paint) return (Tvg_Result) reinterpret_cast<const Shape*>(paint)->strokeFill(r, g, b, a);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_set_stroke_gradient(Tvg_Paint paint, Tvg_Gradient gradient)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->strokeFill((Fill*)(gradient));
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_get_stroke_gradient(const Tvg_Paint paint, Tvg_Gradient* gradient)
{
    if (paint && gradient) {
        *gradient = (Tvg_Gradient)(reinterpret_cast<const Shape*>(paint)->strokeFill());
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_set_stroke_dash(Tvg_Paint paint, const float* dashPattern, uint32_t cnt, float offset)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->strokeDash(dashPattern, cnt, offset);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_get_stroke_dash(const Tvg_Paint paint, const float** dashPattern, uint32_t* cnt, float* offset)
{
    if (paint) {
        *cnt = reinterpret_cast<const Shape*>(paint)->strokeDash(dashPattern, offset);
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_set_stroke_cap(Tvg_Paint paint, Tvg_Stroke_Cap cap)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->strokeCap((StrokeCap)cap);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_get_stroke_cap(const Tvg_Paint paint, Tvg_Stroke_Cap* cap)
{
    if (paint && cap) {
        *cap = (Tvg_Stroke_Cap) reinterpret_cast<const Shape*>(paint)->strokeCap();
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_set_stroke_join(Tvg_Paint paint, Tvg_Stroke_Join join)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->strokeJoin((StrokeJoin)join);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_get_stroke_join(const Tvg_Paint paint, Tvg_Stroke_Join* join)
{
    if (paint && join) {
        *join = (Tvg_Stroke_Join) reinterpret_cast<const Shape*>(paint)->strokeJoin();
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_set_stroke_miterlimit(Tvg_Paint paint, float ml)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->strokeMiterlimit(ml);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_get_stroke_miterlimit(const Tvg_Paint paint, float* ml)
{
    if (paint && ml) {
        *ml = reinterpret_cast<const Shape*>(paint)->strokeMiterlimit();
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_set_trimpath(Tvg_Paint paint, float begin, float end, bool simultaneous)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->trimpath(begin, end, simultaneous);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_set_fill_color(Tvg_Paint paint, uint8_t r, uint8_t g, uint8_t b, uint8_t a)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->fill(r, g, b, a);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_get_fill_color(const Tvg_Paint paint, uint8_t* r, uint8_t* g, uint8_t* b, uint8_t* a)
{
    if (paint) return (Tvg_Result) reinterpret_cast<const Shape*>(paint)->fill(r, g, b, a);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_set_fill_rule(Tvg_Paint paint, Tvg_Fill_Rule rule)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->fillRule((FillRule)rule);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_get_fill_rule(const Tvg_Paint paint, Tvg_Fill_Rule* rule)
{
    if (paint && rule) {
        *rule = (Tvg_Fill_Rule) reinterpret_cast<const Shape*>(paint)->fillRule();
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_set_paint_order(Tvg_Paint paint, bool strokeFirst)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->order(strokeFirst);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_set_gradient(Tvg_Paint paint, Tvg_Gradient gradient)
{
    if (paint) return (Tvg_Result) reinterpret_cast<Shape*>(paint)->fill((Fill*)gradient);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_shape_get_gradient(const Tvg_Paint paint, Tvg_Gradient* gradient)
{
    if (paint && gradient) {
        *gradient = (Tvg_Gradient)(reinterpret_cast<const Shape*>(paint)->fill());
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


/************************************************************************/
/* Picture API                                                          */
/************************************************************************/

TVG_API Tvg_Paint tvg_picture_new()
{
    return (Tvg_Paint) Picture::gen();
}


TVG_API Tvg_Result tvg_picture_load(Tvg_Paint picture, const char* path)
{
    if (picture) return (Tvg_Result) reinterpret_cast<Picture*>(picture)->load(path);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_picture_load_raw(Tvg_Paint picture, const uint32_t *data, uint32_t w, uint32_t h, Tvg_Colorspace cs, bool copy)
{
    if (picture) return (Tvg_Result) reinterpret_cast<Picture*>(picture)->load(data, w, h, static_cast<ColorSpace>(cs), copy);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_picture_load_data(Tvg_Paint picture, const char *data, uint32_t size, const char *mimetype, const char* rpath, bool copy)
{
    if (picture) return (Tvg_Result) reinterpret_cast<Picture*>(picture)->load(data, size, mimetype ? mimetype : "", rpath ? rpath : "", copy);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_picture_set_asset_resolver(Tvg_Paint picture, Tvg_Picture_Asset_Resolver resolver, void* data)
{
    if (!picture) return TVG_RESULT_INVALID_ARGUMENT;
    if (!resolver) return (Tvg_Result) reinterpret_cast<Picture*>(picture)->resolver(nullptr, nullptr);
    return (Tvg_Result) reinterpret_cast<Picture*>(picture)->resolver([resolver](Paint* paint, const char* src, void* data) -> bool {
        return resolver(reinterpret_cast<Tvg_Paint>(paint), src, data);
    }, data);
}


TVG_API Tvg_Result tvg_picture_set_size(Tvg_Paint picture, float w, float h)
{
    if (picture) return (Tvg_Result) reinterpret_cast<Picture*>(picture)->size(w, h);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_picture_get_size(const Tvg_Paint picture, float* w, float* h)
{
    if (picture) return (Tvg_Result) reinterpret_cast<const Picture*>(picture)->size(w, h);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API const Tvg_Paint tvg_picture_get_paint(Tvg_Paint picture, uint32_t id)
{
    if (picture) return (Tvg_Paint) reinterpret_cast<Picture*>(picture)->paint(id);
    return nullptr;
}

TVG_API Tvg_Result tvg_picture_set_accessible(Tvg_Paint picture, bool accessible)
{
    if (picture) {
        reinterpret_cast<Picture*>(picture)->accessible = accessible;
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}

TVG_API Tvg_Result tvg_picture_set_origin(Tvg_Paint picture, float x, float y)
{
    if (picture) return (Tvg_Result) reinterpret_cast<Picture*>(picture)->origin(x, y);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_picture_get_origin(const Tvg_Paint picture, float* x, float* y)
{
    if (picture) return (Tvg_Result) reinterpret_cast<const Picture*>(picture)->origin(x, y);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_picture_set_filter(Tvg_Paint picture, Tvg_Filter_Method method)
{
    if (picture) return (Tvg_Result) reinterpret_cast<Picture*>(picture)->filter(FilterMethod(method));
    return TVG_RESULT_INVALID_ARGUMENT;
}

/************************************************************************/
/* Gradient API                                                         */
/************************************************************************/

TVG_API Tvg_Gradient tvg_linear_gradient_new()
{
    return (Tvg_Gradient)LinearGradient::gen();
}


TVG_API Tvg_Gradient tvg_radial_gradient_new()
{
    return (Tvg_Gradient)RadialGradient::gen();
}


TVG_API Tvg_Gradient tvg_gradient_duplicate(Tvg_Gradient grad)
{
    if (grad) return (Tvg_Gradient) reinterpret_cast<Fill*>(grad)->duplicate();
    return nullptr;
}


TVG_API Tvg_Result tvg_gradient_del(Tvg_Gradient grad)
{
    if (grad) {
        delete(reinterpret_cast<Fill*>(grad));
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_linear_gradient_set(Tvg_Gradient grad, float x1, float y1, float x2, float y2)
{
    if (grad) return (Tvg_Result) reinterpret_cast<LinearGradient*>(grad)->linear(x1, y1, x2, y2);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_linear_gradient_get(Tvg_Gradient grad, float* x1, float* y1, float* x2, float* y2)
{
    if (grad) return (Tvg_Result) reinterpret_cast<LinearGradient*>(grad)->linear(x1, y1, x2, y2);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_radial_gradient_set(Tvg_Gradient grad, float cx, float cy, float r, float fx, float fy, float fr)
{
    if (grad) return (Tvg_Result) reinterpret_cast<RadialGradient*>(grad)->radial(cx, cy, r, fx, fy, fr);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_radial_gradient_get(Tvg_Gradient grad, float* cx, float* cy, float* r, float* fx, float* fy, float* fr)
{
    if (grad) return (Tvg_Result) reinterpret_cast<RadialGradient*>(grad)->radial(cx, cy, r, fx, fy, fr);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_gradient_set_color_stops(Tvg_Gradient grad, const Tvg_Color_Stop* color_stop, uint32_t cnt)
{
    if (grad) return (Tvg_Result) reinterpret_cast<Fill*>(grad)->colorStops(reinterpret_cast<const Fill::ColorStop*>(color_stop), cnt);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_gradient_get_color_stops(const Tvg_Gradient grad, const Tvg_Color_Stop** color_stop, uint32_t* cnt)
{
    if (grad && color_stop && cnt) {
        *cnt = reinterpret_cast<const Fill*>(grad)->colorStops(reinterpret_cast<const Fill::ColorStop**>(color_stop));
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_gradient_set_spread(Tvg_Gradient grad, const Tvg_Stroke_Fill spread)
{
    if (grad) return (Tvg_Result) reinterpret_cast<Fill*>(grad)->spread((FillSpread)spread);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_gradient_get_spread(const Tvg_Gradient grad, Tvg_Stroke_Fill* spread)
{
    if (grad && spread) {
        *spread = (Tvg_Stroke_Fill) reinterpret_cast<const Fill*>(grad)->spread();
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_gradient_set_transform(Tvg_Gradient grad, const Tvg_Matrix* m)
{
    if (grad && m) return (Tvg_Result) reinterpret_cast<Fill*>(grad)->transform(*(reinterpret_cast<const Matrix*>(m)));
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_gradient_get_transform(const Tvg_Gradient grad, Tvg_Matrix* m)
{
    if (grad && m) {
        *reinterpret_cast<Matrix*>(m) = reinterpret_cast<Fill*>(const_cast<Tvg_Gradient>(grad))->transform();
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_gradient_get_type(const Tvg_Gradient grad, Tvg_Type* type)
{
    if (grad && type) {
        *type = static_cast<Tvg_Type>(reinterpret_cast<const Fill*>(grad)->type());
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


/************************************************************************/
/* Scene API                                                            */
/************************************************************************/

TVG_API Tvg_Paint tvg_scene_new()
{
    return (Tvg_Paint) Scene::gen();
}


TVG_API Tvg_Result tvg_scene_add(Tvg_Paint scene, Tvg_Paint paint)
{
    if (scene && paint) return (Tvg_Result) reinterpret_cast<Scene*>(scene)->add((Paint*)paint);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_scene_insert(Tvg_Paint scene, Tvg_Paint paint, Tvg_Paint at)
{
    if (scene && paint && at) return (Tvg_Result) reinterpret_cast<Scene*>(scene)->add((Paint*)paint, (Paint*)at);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_scene_remove(Tvg_Paint scene, Tvg_Paint paint)
{
    if (scene) return (Tvg_Result) reinterpret_cast<Scene*>(scene)->remove((Paint*)paint);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_scene_clear_effects(Tvg_Paint scene)
{
    if (scene) return (Tvg_Result) reinterpret_cast<Scene*>(scene)->add(SceneEffect::Clear);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_scene_add_effect_drop_shadow(Tvg_Paint scene, int r, int g, int b, int a, double angle, double distance, double sigma, int quality)
{
    if (scene) return (Tvg_Result) reinterpret_cast<Scene*>(scene)->add(SceneEffect::DropShadow, r, g, b, a, angle, distance, sigma, quality);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_scene_add_effect_gaussian_blur(Tvg_Paint scene, double sigma, int direction, int border, int quality)
{
    if (scene) return (Tvg_Result) reinterpret_cast<Scene*>(scene)->add(SceneEffect::GaussianBlur, sigma, direction, border, quality);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_scene_add_effect_fill(Tvg_Paint scene, int r, int g, int b, int a)
{
    if (scene) return (Tvg_Result) reinterpret_cast<Scene*>(scene)->add(SceneEffect::Fill, r, g, b, a);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_scene_add_effect_tint(Tvg_Paint scene, int black_r, int black_g, int black_b, int white_r, int white_g, int white_b, double intensity)
{
    if (scene) return (Tvg_Result) reinterpret_cast<Scene*>(scene)->add(SceneEffect::Tint, black_r, black_g, black_b, white_r, white_g, white_b, intensity);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_scene_add_effect_tritone(Tvg_Paint scene, int shadow_r, int shadow_g, int shadow_b, int midtone_r, int midtone_g, int midtone_b, int highlight_r, int highlight_g, int highlight_b, int blend)
{
    if (scene) return (Tvg_Result) reinterpret_cast<Scene*>(scene)->add(SceneEffect::Tritone, shadow_r, shadow_g, shadow_b, midtone_r, midtone_g, midtone_b, highlight_r, highlight_g, highlight_b, blend);
    return TVG_RESULT_INVALID_ARGUMENT;
}


/************************************************************************/
/* Text API                                                            */
/************************************************************************/

TVG_API Tvg_Paint tvg_text_new()
{
    return (Tvg_Paint)Text::gen();
}


TVG_API Tvg_Result tvg_text_set_font(Tvg_Paint text, const char* name)
{
    if (text) return (Tvg_Result) reinterpret_cast<Text*>(text)->font(name);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_text_set_size(Tvg_Paint text, float size)
{
    if (text) return (Tvg_Result) reinterpret_cast<Text*>(text)->size(size);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_text_set_text(Tvg_Paint text, const char* utf8)
{
    if (text) return (Tvg_Result) reinterpret_cast<Text*>(text)->text(utf8);
    return TVG_RESULT_INVALID_ARGUMENT;
}


 TVG_API const char* tvg_text_get_text(const Tvg_Paint text)
 {
    return text ? reinterpret_cast<Text*>(text)->text() : nullptr;
 }



TVG_API Tvg_Result tvg_text_align(Tvg_Paint text, float x, float y)
{
    if (text) return (Tvg_Result) reinterpret_cast<Text*>(text)->align(x, y);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_text_layout(Tvg_Paint text, float w, float h)
{
    if (text) return (Tvg_Result) reinterpret_cast<Text*>(text)->layout(w, h);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_text_set_outline(Tvg_Paint text, float width, uint8_t r, uint8_t g, uint8_t b)
{
    if (text) return (Tvg_Result) reinterpret_cast<Text*>(text)->outline(width, r, g, b);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_text_set_color(Tvg_Paint text, uint8_t r, uint8_t g, uint8_t b)
{
    if (text) return (Tvg_Result) reinterpret_cast<Text*>(text)->fill(r, g, b);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_text_set_italic(Tvg_Paint text, float shear)
{
    if (text) return (Tvg_Result) reinterpret_cast<Text*>(text)->italic(shear);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_text_set_gradient(Tvg_Paint text, Tvg_Gradient gradient)
{
    if (text) return (Tvg_Result) reinterpret_cast<Text*>(text)->fill((Fill*)(gradient));
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_text_wrap_mode(Tvg_Paint text, Tvg_Text_Wrap mode)
{
    if (text) return (Tvg_Result) reinterpret_cast<Text*>(text)->wrap(TextWrap(mode));
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API uint32_t tvg_text_line_count(Tvg_Paint text)
{
    if (text) return reinterpret_cast<Text*>(text)->lines();
    return 0;
}


TVG_API Tvg_Result tvg_text_spacing(Tvg_Paint text, float letter, float line)
{
    if (text) return (Tvg_Result) reinterpret_cast<Text*>(text)->spacing(letter, line);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_text_get_text_metrics(const Tvg_Paint text, Tvg_Text_Metrics* metrics)
{
    if (text && metrics) return (Tvg_Result) reinterpret_cast<Text*>(text)->metrics(*reinterpret_cast<TextMetrics*>(metrics));
    return TVG_RESULT_INVALID_ARGUMENT;
}

TVG_API Tvg_Result tvg_text_get_glyph_metrics(const Tvg_Paint text, const char* ch, Tvg_Glyph_Metrics* metrics, const char** next)
{
    if (text && metrics) return (Tvg_Result) reinterpret_cast<Text*>(text)->metrics(ch, *reinterpret_cast<GlyphMetrics*>(metrics), next);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_font_load(const char* path)
{
    return (Tvg_Result) Text::load(path);
}


TVG_API Tvg_Result tvg_font_load_data(const char* name, const char* data, uint32_t size, const char *mimetype, bool copy)
{
    return (Tvg_Result) Text::load(name, data, size, mimetype ? mimetype : "", copy);
}


TVG_API Tvg_Result tvg_font_unload(const char* path)
{
    return (Tvg_Result) Text::unload(path);
}


/************************************************************************/
/* Saver API                                                            */
/************************************************************************/

TVG_API Tvg_Saver tvg_saver_new()
{
    return (Tvg_Saver) Saver::gen();
}


TVG_API Tvg_Result tvg_saver_save_paint(Tvg_Saver saver, Tvg_Paint paint, const char* path, uint32_t quality)
{
    if (saver) return (Tvg_Result) reinterpret_cast<Saver*>(saver)->save((Paint*)paint, path, quality);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_saver_save_animation(Tvg_Saver saver, Tvg_Animation animation, const char* path, uint32_t quality, uint32_t fps)
{
    if (saver) return (Tvg_Result) reinterpret_cast<Saver*>(saver)->save((Animation*)animation, path, quality, fps);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_saver_sync(Tvg_Saver saver)
{
    if (saver) return (Tvg_Result) reinterpret_cast<Saver*>(saver)->sync();
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_saver_del(Tvg_Saver saver)
{
    if (saver) {
        delete(reinterpret_cast<Saver*>(saver));
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


/************************************************************************/
/* Animation API                                                        */
/************************************************************************/

TVG_API Tvg_Animation tvg_animation_new()
{
    return (Tvg_Animation) Animation::gen();
}


TVG_API Tvg_Result tvg_animation_set_frame(Tvg_Animation animation, float no)
{
    if (animation) return (Tvg_Result) reinterpret_cast<Animation*>(animation)->frame(no);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_animation_get_frame(Tvg_Animation animation, float* no)
{
    if (animation && no) {
        *no = reinterpret_cast<Animation*>(animation)->curFrame();
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_animation_get_total_frame(Tvg_Animation animation, float* cnt)
{
    if (animation && cnt) {
        *cnt = reinterpret_cast<Animation*>(animation)->totalFrame();
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Paint tvg_animation_get_picture(Tvg_Animation animation)
{
    if (animation) return (Tvg_Paint) reinterpret_cast<Animation*>(animation)->picture();
    return nullptr;
}


TVG_API Tvg_Result tvg_animation_get_duration(Tvg_Animation animation, float* duration)
{
    if (animation && duration) {
        *duration = reinterpret_cast<Animation*>(animation)->duration();
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_animation_set_segment(Tvg_Animation animation, float start, float end)
{
    if (animation) return (Tvg_Result) reinterpret_cast<Animation*>(animation)->segment(start, end);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_animation_get_segment(Tvg_Animation animation, float* start, float* end)
{
    if (animation) return (Tvg_Result) reinterpret_cast<Animation*>(animation)->segment(start, end);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_animation_del(Tvg_Animation animation)
{
    if (animation) {
        delete(reinterpret_cast<Animation*>(animation));
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


/************************************************************************/
/* Accessor API                                                         */
/************************************************************************/

TVG_API Tvg_Accessor tvg_accessor_new()
{
    return (Tvg_Accessor) Accessor::gen();
}


TVG_API Tvg_Result tvg_accessor_del(Tvg_Accessor accessor)
{
    if (accessor) {
        delete(reinterpret_cast<Accessor*>(accessor));
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API Tvg_Result tvg_accessor_set(Tvg_Accessor accessor, Tvg_Paint paint, bool (*func)(Tvg_Paint paint, void* data), void* data)
{
    if (accessor) return (Tvg_Result) reinterpret_cast<Accessor*>(accessor)->set(static_cast<Picture*>(reinterpret_cast<Paint*>(paint)),
                                                [func](const Paint* paint, void* data) { return func((Tvg_Paint) paint, data); }, data);
    return TVG_RESULT_INVALID_ARGUMENT;
}


TVG_API uint32_t tvg_accessor_generate_id(const char* name)
{
    return Accessor::id(name);
}

TVG_API const char* tvg_accessor_get_name(Tvg_Accessor accessor, uint32_t id)
{
    if (accessor) return reinterpret_cast<Accessor*>(accessor)->name(id);
    return nullptr;
}

/************************************************************************/
/* Lottie Animation API                                                 */
/************************************************************************/

TVG_API Tvg_Animation tvg_lottie_animation_new()
{
#ifdef THORVG_LOTTIE_LOADER_SUPPORT
    return (Tvg_Animation) LottieAnimation::gen();
#endif
    return nullptr;
}


TVG_API uint32_t tvg_lottie_animation_gen_slot(Tvg_Animation animation, const char* slot)
{
#ifdef THORVG_LOTTIE_LOADER_SUPPORT
    if (animation) return reinterpret_cast<LottieAnimation*>(animation)->gen(slot);
#endif
    return 0;
}


TVG_API Tvg_Result tvg_lottie_animation_apply_slot(Tvg_Animation animation, uint32_t id)
{
#ifdef THORVG_LOTTIE_LOADER_SUPPORT
    if (animation) return (Tvg_Result) reinterpret_cast<LottieAnimation*>(animation)->apply(id);
    return TVG_RESULT_INVALID_ARGUMENT;
#endif
    return TVG_RESULT_NOT_SUPPORTED;
}


TVG_API Tvg_Result tvg_lottie_animation_del_slot(Tvg_Animation animation, uint32_t id)
{
#ifdef THORVG_LOTTIE_LOADER_SUPPORT
    if (animation) return (Tvg_Result) reinterpret_cast<LottieAnimation*>(animation)->del(id);
    return TVG_RESULT_INVALID_ARGUMENT;
#endif
    return TVG_RESULT_NOT_SUPPORTED;
}


TVG_API Tvg_Result tvg_lottie_animation_set_marker(Tvg_Animation animation, const char* marker)
{
#ifdef THORVG_LOTTIE_LOADER_SUPPORT
    if (animation) return (Tvg_Result) reinterpret_cast<LottieAnimation*>(animation)->segment(marker);
    return TVG_RESULT_INVALID_ARGUMENT;
#endif
    return TVG_RESULT_NOT_SUPPORTED;
}


TVG_API Tvg_Result tvg_lottie_animation_get_markers_cnt(Tvg_Animation animation, uint32_t* cnt)
{
#ifdef THORVG_LOTTIE_LOADER_SUPPORT
    if (animation && cnt) {
        *cnt = reinterpret_cast<LottieAnimation*>(animation)->markersCnt();
        return TVG_RESULT_SUCCESS;
    }
    return TVG_RESULT_INVALID_ARGUMENT;
#endif
    return TVG_RESULT_NOT_SUPPORTED;
}

TVG_API TVG_DEPRECATED Tvg_Result tvg_lottie_animation_get_marker(Tvg_Animation animation, uint32_t idx, const char** name)
{
    if (!name) return TVG_RESULT_INVALID_ARGUMENT;  // for backward compat.
    return tvg_lottie_animation_get_marker_info(animation, idx, name, nullptr, nullptr);
}

TVG_API Tvg_Result tvg_lottie_animation_get_marker_info(Tvg_Animation animation, uint32_t idx, const char** name, float* begin, float* end)
{
#ifdef THORVG_LOTTIE_LOADER_SUPPORT
    if (!animation) return TVG_RESULT_INVALID_ARGUMENT;
    auto ret = reinterpret_cast<LottieAnimation*>(animation)->marker(idx, begin, end);
    if (name) *name = ret;
    if (ret) return TVG_RESULT_SUCCESS;
    auto markerCnt = reinterpret_cast<LottieAnimation*>(animation)->markersCnt();
    if (markerCnt > 0 && idx >= markerCnt) return TVG_RESULT_INVALID_ARGUMENT;
    return TVG_RESULT_INSUFFICIENT_CONDITION;
#endif
    return TVG_RESULT_NOT_SUPPORTED;
}

TVG_API Tvg_Result tvg_lottie_animation_tween(Tvg_Animation animation, float from, float to, float progress)
{
#ifdef THORVG_LOTTIE_LOADER_SUPPORT
    if (animation) return (Tvg_Result) reinterpret_cast<LottieAnimation*>(animation)->tween(from, to, progress);
    return TVG_RESULT_INVALID_ARGUMENT;
#endif
    return TVG_RESULT_NOT_SUPPORTED;
}

TVG_API Tvg_Result tvg_lottie_animation_tween_to(Tvg_Animation animation, float to)
{
#ifdef THORVG_LOTTIE_LOADER_SUPPORT
    if (animation) return (Tvg_Result) reinterpret_cast<LottieAnimation*>(animation)->tweenTo(to);
    return TVG_RESULT_INVALID_ARGUMENT;
#endif
    return TVG_RESULT_NOT_SUPPORTED;
}

TVG_API Tvg_Result tvg_lottie_animation_tween_go(Tvg_Animation animation, float progress)
{
#ifdef THORVG_LOTTIE_LOADER_SUPPORT
    if (animation) return (Tvg_Result) reinterpret_cast<LottieAnimation*>(animation)->tween(progress);
    return TVG_RESULT_INVALID_ARGUMENT;
#endif
    return TVG_RESULT_NOT_SUPPORTED;
}

TVG_API Tvg_Result tvg_lottie_animation_set_quality(Tvg_Animation animation, uint8_t value)
{
#ifdef THORVG_LOTTIE_LOADER_SUPPORT
    if (animation) return (Tvg_Result) reinterpret_cast<LottieAnimation*>(animation)->quality(value);
    return TVG_RESULT_INVALID_ARGUMENT;
#endif
    return TVG_RESULT_NOT_SUPPORTED;
}


TVG_API Tvg_Result tvg_lottie_animation_set_audio_resolver(Tvg_Animation animation, Tvg_Audio_Resolver resolver, void* data)
{
#ifdef THORVG_LOTTIE_LOADER_SUPPORT
    if (!animation) return TVG_RESULT_INVALID_ARGUMENT;
    auto anim = reinterpret_cast<tvg::LottieAnimation*>(animation);
    if (!resolver) return (Tvg_Result) anim->resolver(nullptr, nullptr);
    return (Tvg_Result) anim->resolver([resolver](const tvg::LottieAudioResolver& in, void* data) {
        Tvg_Audio_Info info{in.src, in.mimeType, in.size, in.offset, in.volume, in.active, in.embedded};
        resolver(&info, data);
    }, data);
#endif
    return TVG_RESULT_NOT_SUPPORTED;
}

#ifdef __cplusplus
}
#endif
