/*
 * Copyright (c) 2023 - 2026 ThorVG project. All rights reserved.

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

#include <algorithm>
#include "tvgCommon.h"
#include "tvgMath.h"
#include "tvgScene.h"
#include "tvgText.h"
#include "tvgLoader.h"
#include "tvgLottieModel.h"
#include "tvgLottieBuilder.h"
#include "tvgLottieExpressions.h"
#include "tvgLottieTween.h"

/************************************************************************/
/* Internal Class Implementation                                        */
/************************************************************************/

static bool _buildComposition(LottieComposition* comp, LottieLayer* parent);
static bool _draw(LottieGroup* parent, LottieShape* shape, RenderContext* ctx);

static void _dimension3d(LottieTransform* transform, float frameNo, Matrix& m, float angle, LottieTween& tween, LottieExpressions* exps)
{
    auto x = deg2rad(transform->ddd->rx(frameNo, tween, exps));
    auto y = deg2rad(transform->ddd->ry(frameNo, tween, exps));
    auto z = deg2rad(transform->rotation(frameNo, tween, exps)) + angle;

    auto sx = sinf(x), sy = sinf(y), sz = sinf(z);
    auto cx = cosf(x), cy = cosf(y), cz = cosf(z);

    auto ri00 = cy * cz;
    auto ri01 = -cy * sz;
    auto ri10 = sx * sy * cz + cx * sz;
    auto ri11 = -sx * sy * sz + cx * cz;

    auto o = transform->ddd->orient(frameNo, tween, exps);

    // fast-path
    if (o.x == 0.0f && o.y == 0.0f && o.z == 0.0f) {
        m.e11 = ri00;
        m.e12 = ri01;
        m.e21 = ri10;
        m.e22 = ri11;
        return;
    }

    auto ri02 = sy;
    auto ri12 = -sx * cy;

    auto ox = deg2rad(o.x);
    auto oy = deg2rad(o.y);
    auto oz = deg2rad(o.z);

    auto sox = sinf(ox), soy = sinf(oy), soz = sinf(oz);
    auto cox = cosf(ox), coy = cosf(oy), coz = cosf(oz);

    auto ro00 = coy * coz;
    auto ro01 = -coy * soz;
    auto ro10 = sox * soy * coz + cox * soz;
    auto ro11 = -sox * soy * soz + cox * coz;
    auto ro20 = -cox * soy * coz + sox * soz;
    auto ro21 = cox * soy * soz + sox * coz;

    m.e11 = ri00 * ro00 + ri01 * ro10 + ri02 * ro20;
    m.e12 = ri00 * ro01 + ri01 * ro11 + ri02 * ro21;
    m.e21 = ri10 * ro00 + ri11 * ro10 + ri12 * ro20;
    m.e22 = ri10 * ro01 + ri11 * ro11 + ri12 * ro21;
}

static void _rotate(LottieTransform* transform, float frameNo, Matrix& m, float angle, LottieTween& tween, LottieExpressions* exps)
{
    if (transform->ddd) {
        _dimension3d(transform, frameNo, m, angle, tween, exps);
    } else {
        // rotation z
        auto degree = transform->rotation(frameNo, tween, exps) + angle;
        if (degree == 0.0f) return;
        auto radian = deg2rad(degree);
        m.e11 = cosf(radian);
        m.e12 = -sinf(radian);
        m.e21 = sinf(radian);
        m.e22 = cosf(radian);
    }
}


static void _skew(Matrix* m, float angleDeg, float axisDeg)
{
    auto angle = -deg2rad(angleDeg);
    auto tanVal = tanf(angle);

    axisDeg = fmod(axisDeg, 180.0f);
    if (fabsf(axisDeg) < 0.01f || fabsf(axisDeg - 180.0f) < 0.01f || fabsf(axisDeg + 180.0f) < 0.01f) {
        float cosVal = cosf(deg2rad(axisDeg));
        auto B = cosVal * cosVal * tanVal;
        m->e12 += B * m->e11;
        m->e22 += B * m->e21;
        return;
    } else if (fabsf(axisDeg - 90.0f) < 0.01f || fabsf(axisDeg + 90.0f) < 0.01f) {
        float sinVal = -sinf(deg2rad(axisDeg));
        auto C = sinVal * sinVal * tanVal;
        m->e11 -= C * m->e12;
        m->e21 -= C * m->e22;
        return;
    }

    auto axis = -deg2rad(axisDeg);
    auto cosVal = cosf(axis);
    auto sinVal = sinf(axis);
    auto A = sinVal * cosVal * tanVal;
    auto B = cosVal * cosVal * tanVal;
    auto C = sinVal * sinVal * tanVal;

    auto e11 = m->e11;
    auto e21 = m->e21;
    m->e11 = (1.0f - A) * e11 - C * m->e12;
    m->e12 = B * e11 + (1.0f + A) * m->e12;
    m->e21 = (1.0f - A) * e21 - C * m->e22;
    m->e22 = B * e21 + (1.0f + A) * m->e22;
}

static bool _update(LottieTransform* transform, float frameNo, Matrix& matrix, uint8_t& opacity, bool autoOrient, LottieTween& tween, LottieExpressions* exps)
{
    tvg::identity(&matrix);

    if (!transform) {
        opacity = 255;
        return false;
    }

    if (transform->coords) translate(&matrix, {transform->coords->x(frameNo, tween, exps), transform->coords->y(frameNo, tween, exps)});
    else translate(&matrix, transform->position(frameNo, tween, exps));

    auto angle = (autoOrient) ? transform->position.angle(frameNo, tween) : 0.0f;
    _rotate(transform, frameNo, matrix, angle, tween, exps);

    auto skewAngle = transform->skewAngle(frameNo, tween, exps);
    if (skewAngle != 0.0f) {
        // For angles where tangent explodes, the shape degenerates into an infinitely thin line.
        // This is handled by zeroing out the matrix due to finite numerical precision.
        skewAngle = fmod(skewAngle, 180.0f);
        if (fabsf(skewAngle - 90.0f) < 0.01f || fabsf(skewAngle + 90.0f) < 0.01f) return false;
        _skew(&matrix, skewAngle, transform->skewAxis(frameNo, exps));
    }

    auto scale = transform->scale(frameNo, tween, exps);
    scaleR(&matrix, scale * 0.01f);

    //Lottie specific anchor transform.
    translateR(&matrix, -transform->anchor(frameNo, tween, exps));

    //invisible just in case.
    if (scale.x == 0.0f || scale.y == 0.0f) opacity = 0;
    else opacity = transform->opacity(frameNo, tween, exps);

    return true;
}


void LottieBuilder::updateTransform(LottieLayer* layer, float frameNo)
{
    if (!layer) return;

    if (tween.active) {
        layer->cache.frameNo = -1.0f;  // tweening doesn't cache the result; invalidate it
    } else {
        if (tvg::equal(layer->cache.frameNo, frameNo)) return;
        layer->cache.frameNo = frameNo;
    }

    auto transform = layer->transform;
    auto parent = layer->parent;

    if (parent) updateTransform(parent, frameNo);

    auto& matrix = layer->cache.matrix;

    _update(transform, frameNo, matrix, layer->cache.opacity, layer->autoOrient, tween, exps);

    if (parent) layer->cache.matrix = parent->cache.matrix * matrix;
}


void LottieBuilder::updateTransform(LottieGroup* parent, LottieObject** child, float frameNo, TVG_UNUSED Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto transform = static_cast<LottieTransform*>(*child);
    if (!transform) return;

    Matrix m;
    uint8_t opacity;

    if (parent->mergeable()) {
        if (ctx->transform) {
            _update(transform, frameNo, m, opacity, false, tween, exps);
            *ctx->transform *= m;
        } else {
            ctx->transform = new Matrix;
            _update(transform, frameNo, *ctx->transform, opacity, false, tween, exps);
        }
        return;
    }

    ctx->merging = nullptr;

    if (!_update(transform, frameNo, m, opacity, false, tween, exps)) return;

    ctx->propagator->transform(ctx->propagator->transform() * m);
    ctx->propagator->opacity(MULTIPLY(opacity, PAINT(ctx->propagator)->opacity));

    //FIXME: preserve the stroke width. too workaround, need a better design.
    if (to<ShapeImpl>(ctx->propagator)->rs.strokeWidth() > 0.0f) {
        auto denominator = sqrtf(m.e11 * m.e11 + m.e12 * m.e12);
        if (denominator > 1.0f) ctx->propagator->strokeWidth(ctx->propagator->strokeWidth() / denominator);
    }

    //FIXME: compensate gradient fills when the propagator enters a group scope.
    if (ctx->fragment) {
        Matrix im;
        if (inverse(&m, &im)) {
            auto& rs = to<ShapeImpl>(ctx->propagator)->rs;
            if (rs.fill) rs.fill->transform(rs.fill->transform() * im);
            if (rs.stroke && rs.stroke->fill) rs.stroke->fill->transform(rs.stroke->fill->transform() * im);
        }
    }
}


void LottieBuilder::updateGroup(LottieGroup* parent, LottieObject** child, float frameNo, TVG_UNUSED Inlist<RenderContext>& pcontexts, RenderContext* ctx)
{
    auto group = static_cast<LottieGroup*>(*child);

    if (!group->visible) return;

    // prepare render data

    // special tune: sharing the context if the blending is compatible
    // propagate the blending to its parent(layer) if possible. this potentially helps performance if the layer has mattes/maskings.
    if (group->blendMethod == BlendMethod::Normal || group->blendMethod == parent->blendMethod) {
        group->scene = parent->scene;
    } else if (parent->blendMethod == BlendMethod::Normal && parent->children.count == 1) {
        group->scene = parent->scene;
        group->scene->blend(group->blendMethod);
    } else {
        group->scene = tvg::Scene::gen();
        parent->scene->add(group->scene);
        group->scene->blend(group->blendMethod);
    }

    group->reqFragment |= ctx->reqFragment;

    //generate a merging shape to consolidate partial shapes into a single entity
    if (group->mergeable()) _draw(group, nullptr, ctx);

    Inlist<RenderContext> contexts;
    auto propagator = group->mergeable() ? ctx->propagator : static_cast<Shape*>(PAINT(ctx->propagator)->duplicate(group->pooling()));
    contexts.back(new RenderContext(*ctx, propagator, group->mergeable()));

    updateChildren(group, frameNo, contexts);
}

static void _update(LottieStroke* stroke, float frameNo, RenderContext* ctx, LottieTween& tween, LottieExpressions* exps)
{
    ctx->propagator->strokeWidth(stroke->width(frameNo, tween, exps));
    ctx->propagator->strokeCap(stroke->cap);
    ctx->propagator->strokeJoin(stroke->join);
    ctx->propagator->strokeMiterlimit(stroke->miterLimit);

    if (stroke->dashattr) {
        auto dashes = tvg::malloc<float>(stroke->dashattr->size * sizeof(float));
        for (uint8_t i = 0; i < stroke->dashattr->size; ++i) dashes[i] = stroke->dashattr->values[i](frameNo, tween, exps);
        ctx->propagator->strokeDash(dashes, stroke->dashattr->size, stroke->dashattr->offset(frameNo, tween, exps));
        tvg::free(dashes);
    } else {
        ctx->propagator->strokeDash(nullptr, 0);
    }
}


bool LottieBuilder::fragmented(LottieGroup* parent, LottieObject** child, Inlist<RenderContext>& contexts, RenderContext* ctx, RenderFragment fragment)
{
    if (ctx->fragment) return true;
    if (!ctx->reqFragment) return false;

    contexts.back(new RenderContext(*ctx, (Shape*)(PAINT(ctx->propagator)->duplicate(parent->pooling()))));

    contexts.tail->begin = child - 1;
    ctx->fragment = fragment;

    return false;
}


bool LottieBuilder::updateSolidStroke(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto stroke = static_cast<LottieSolidStroke*>(*child);
    if (fragmented(parent, child, contexts, ctx, RenderFragment::ByStroke)) return false;

    auto opacity = stroke->opacity(frameNo, tween, exps);
    if (opacity == 0) return false;

    ctx->merging = nullptr;
    auto color = stroke->color(frameNo, tween, exps);
    ctx->propagator->strokeFill(color.r, color.g, color.b, opacity);
    _update(static_cast<LottieStroke*>(stroke), frameNo, ctx, tween, exps);

    return false;
}


bool LottieBuilder::updateGradientStroke(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto stroke = static_cast<LottieGradientStroke*>(*child);
    if (fragmented(parent, child, contexts, ctx, RenderFragment::ByStroke)) return false;

    auto opacity = stroke->opacity(frameNo, tween, exps);
    if (opacity == 0 && !stroke->opaque) return false;

    ctx->merging = nullptr;
    if (auto val = stroke->fill(frameNo, opacity, tween, exps)) ctx->propagator->strokeFill(val);
    _update(static_cast<LottieStroke*>(stroke), frameNo, ctx, tween, exps);

    return false;
}


bool LottieBuilder::updateSolidFill(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto fill = static_cast<LottieSolidFill*>(*child);
    auto opacity = fill->opacity(frameNo, tween, exps);

    //interrupted by fully opaque, stop the current rendering
    if (ctx->fragment == RenderFragment::ByFill && opacity == 255) return true;
    if (opacity == 0) return false;

    if (fragmented(parent, child, contexts, ctx, RenderFragment::ByFill)) return false;

    ctx->merging = nullptr;
    auto color = fill->color(frameNo, tween, exps);
    ctx->propagator->fill(color.r, color.g, color.b, opacity);
    ctx->propagator->fillRule(fill->rule);

    if (ctx->propagator->strokeWidth() > 0) ctx->propagator->order(true);

    return false;
}


bool LottieBuilder::updateGradientFill(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto fill = static_cast<LottieGradientFill*>(*child);
    auto opacity = fill->opacity(frameNo, tween, exps);

    //interrupted by fully opaque, stop the current rendering
    if (ctx->fragment == RenderFragment::ByFill && fill->opaque && opacity == 255) return true;

    if (fragmented(parent, child, contexts, ctx, RenderFragment::ByFill)) return false;

    ctx->merging = nullptr;

    if (auto val = fill->fill(frameNo, opacity, tween, exps)) ctx->propagator->fill(val);
    ctx->propagator->fillRule(fill->rule);

    if (ctx->propagator->strokeWidth() > 0) ctx->propagator->order(true);

    return false;
}


static bool _draw(LottieGroup* parent, LottieShape* shape, RenderContext* ctx)
{
    if (ctx->merging) return false;

    if (shape) {
        ctx->merging = shape->pooling();
        PAINT(ctx->propagator)->duplicate(ctx->merging);
    } else {
        ctx->merging = static_cast<Shape*>(ctx->propagator->duplicate());
    }

    parent->scene->add(ctx->merging);

    return true;
}


static void _repeat(LottieGroup* parent, Shape* path, LottieRenderPooler<Shape>* pooler, RenderContext* ctx)
{
    path->ref();  //prevent pooler returns the same path.

    Array<Shape*> propagators;
    propagators.push(ctx->propagator);
    Array<Shape*> shapes;

    ARRAY_REVERSE_FOREACH(repeater, ctx->repeaters) {
        shapes.reserve(repeater->cnt);

        for (int i = 0; i < repeater->cnt; ++i) {
            auto multiplier = repeater->offset + static_cast<float>(i);
            ARRAY_FOREACH(p, propagators) {
                auto shape = pooler->pooling();
                shape->ref();   //prevent pooler returns the same shape
                PAINT((*p))->duplicate(shape);
                to<ShapeImpl>(shape)->rs.path = to<ShapeImpl>(path)->rs.path;
                auto opacity = tvg::lerp<uint8_t>(repeater->startOpacity, repeater->endOpacity, static_cast<float>(i + 1) / repeater->cnt);
                shape->opacity(MULTIPLY(shape->opacity(), opacity));

                auto m = tvg::identity();
                translate(&m, repeater->position * multiplier + repeater->anchor);
                scale(&m, {powf(repeater->scale.x * 0.01f, multiplier), powf(repeater->scale.y * 0.01f, multiplier)});
                rotate(&m, repeater->rotation * multiplier);
                translateR(&m, -repeater->anchor);

                Matrix inv;
                inverse(&repeater->transform, &inv);
                shape->transform((repeater->transform * m) * (inv * shape->transform()));
                shapes.push(shape);
            }
        }

        propagators.clear();
        propagators.reserve(shapes.count);

        //push repeat shapes in order.
        if (repeater->inorder) {
            ARRAY_FOREACH(p, shapes) {
                parent->scene->add(*p);
                (*p)->unref();
                propagators.push(*p);
            }
        } else if (!shapes.empty()) {
            ARRAY_REVERSE_FOREACH(p, shapes) {
                parent->scene->add(*p);
                (*p)->unref();
                propagators.push(*p);
            }
        }
        shapes.clear();
    }
    path->unref();
}

void LottieBuilder::appendRect(LottieRect* rect, Shape* shape, Point& pos, Point& size, float r, bool clockwise, RenderContext* ctx)
{
    auto& path = to<ShapeImpl>(shape)->rs.path;
    auto cnt = path.pts.count;

    if (ctx->modifiers) {
        auto temp = rect->pooling();
        temp->reset();
        temp->appendRect(pos.x, pos.y, size.x, size.y, r, r, clockwise);
        ctx->modifiers->rect(to<ShapeImpl>(temp)->rs.path, to<ShapeImpl>(shape)->rs.path, pos, size, r, clockwise);
    } else shape->appendRect(pos.x, pos.y, size.x, size.y, r, r, clockwise);

    if (ctx->transform) {
        for (auto i = cnt; i < path.pts.count; ++i) {
            path.pts[i] *= *ctx->transform;
        }
    }
}


void LottieBuilder::updateRect(LottieGroup* parent, LottieObject** child, float frameNo, TVG_UNUSED Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto rect = static_cast<LottieRect*>(*child);
    auto size = rect->size(frameNo, tween, exps);
    auto pos = rect->position(frameNo, tween, exps) - size * 0.5f;
    auto r = std::min({rect->radius(frameNo, tween, exps), size.x * 0.5f, size.y * 0.5f});

    if (ctx->repeaters.empty()) {
        _draw(parent, rect, ctx);
        appendRect(rect, ctx->merging, pos, size, r, rect->clockwise, ctx);
    } else {
        auto shape = rect->pooling();
        shape->reset();
        appendRect(rect, shape, pos, size, r, rect->clockwise, ctx);
        _repeat(parent, shape, rect, ctx);
    }
}

void LottieBuilder::appendCircle(LottieEllipse* ellipse, Shape* shape, Point& center, Point& radius, bool clockwise, RenderContext* ctx)
{
    auto& path = to<ShapeImpl>(shape)->rs.path;
    auto cnt = path.pts.count;

    if (ctx->modifiers) {
        auto temp = ellipse->pooling();
        temp->reset();
        temp->appendCircle(center.x, center.y, radius.x, radius.y, clockwise);
        ctx->modifiers->ellipse(to<ShapeImpl>(temp)->rs.path, to<ShapeImpl>(shape)->rs.path, center, radius, clockwise);
    } else shape->appendCircle(center.x, center.y, radius.x, radius.y, clockwise);

    if (ctx->transform) {
        for (auto i = cnt; i < to<ShapeImpl>(shape)->rs.path.pts.count; ++i) {
            to<ShapeImpl>(shape)->rs.path.pts[i] *= *ctx->transform;
        }
    }
}


void LottieBuilder::updateEllipse(LottieGroup* parent, LottieObject** child, float frameNo, TVG_UNUSED Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto ellipse = static_cast<LottieEllipse*>(*child);
    auto pos = ellipse->position(frameNo, tween, exps);
    auto size = ellipse->size(frameNo, tween, exps) * 0.5f;

    if (ctx->repeaters.empty()) {
        _draw(parent, ellipse, ctx);
        appendCircle(ellipse, ctx->merging, pos, size, ellipse->clockwise, ctx);
    } else {
        auto shape = ellipse->pooling();
        shape->reset();
        appendCircle(ellipse, shape, pos, size, ellipse->clockwise, ctx);
        _repeat(parent, shape, ellipse, ctx);
    }
}


void LottieBuilder::updatePath(LottieGroup* parent, LottieObject** child, float frameNo, TVG_UNUSED Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto path = static_cast<LottiePath*>(*child);

    if (ctx->repeaters.empty()) {
        _draw(parent, path, ctx);
        path->pathset(frameNo, to<ShapeImpl>(ctx->merging)->rs.path, ctx->transform, tween, exps, ctx->modifiers);
        PAINT(ctx->merging)->mark(RenderUpdateFlag::Path);
    } else {
        auto shape = path->pooling();
        shape->reset();
        path->pathset(frameNo, to<ShapeImpl>(shape)->rs.path, ctx->transform, tween, exps, ctx->modifiers);
        _repeat(parent, shape, path, ctx);
    }
}


static void _close(Array<Point>& pts, const Point& p, bool round)
{
    if (round && tvg::zero(pts.last() - pts[pts.count - 2])) pts[pts.count - 2] = p;
    pts.last() = p;
}

void LottieBuilder::updateStar(LottiePolyStar* star, float frameNo, Matrix* transform, Shape* merging, RenderContext* ctx, LottieTween& tween, LottieExpressions* exps)
{
    static constexpr auto POLYSTAR_MAGIC_NUMBER = 0.47829f / 0.28f;

    auto ptsCnt = star->ptsCnt(frameNo, tween, exps);
    auto innerRadius = star->innerRadius(frameNo, tween, exps);
    auto outerRadius = star->outerRadius(frameNo, tween, exps);
    auto innerRoundness = star->innerRoundness(frameNo, tween, exps) * 0.01f;
    auto outerRoundness = star->outerRoundness(frameNo, tween, exps) * 0.01f;

    auto angle = deg2rad(-90.0f);
    auto partialPointRadius = 0.0f;
    auto anglePerPoint = (2.0f * MATH_PI / ptsCnt);
    auto halfAnglePerPoint = anglePerPoint * 0.5f;
    auto partialPointAmount = ptsCnt - floorf(ptsCnt);
    auto longSegment = false;
    auto numPoints = size_t(ceilf(ptsCnt) * 2);
    auto direction = star->clockwise ? 1.0f : -1.0f;
    auto hasRoundness = false;

    Shape* shape;
    if (ctx->modifiers) {
        shape = star->pooling();
        shape->reset();
    } else {
        shape = merging;
    }

    float x, y;

    if (!tvg::zero(partialPointAmount)) {
        angle += halfAnglePerPoint * (1.0f - partialPointAmount) * direction;
    }

    if (!tvg::zero(partialPointAmount)) {
        partialPointRadius = innerRadius + partialPointAmount * (outerRadius - innerRadius);
        x = partialPointRadius * cosf(angle);
        y = partialPointRadius * sinf(angle);
        angle += anglePerPoint * partialPointAmount * 0.5f * direction;
    } else {
        x = outerRadius * cosf(angle);
        y = outerRadius * sinf(angle);
        angle += halfAnglePerPoint * direction;
    }

    if (tvg::zero(innerRoundness) && tvg::zero(outerRoundness)) {
        to<ShapeImpl>(shape)->rs.path.pts.reserve(numPoints + 2);
        to<ShapeImpl>(shape)->rs.path.cmds.reserve(numPoints + 3);
    } else {
        to<ShapeImpl>(shape)->rs.path.pts.reserve(numPoints * 3 + 2);
        to<ShapeImpl>(shape)->rs.path.cmds.reserve(numPoints + 3);
        hasRoundness = true;
    }

    auto in = Point{x, y} * transform;
    shape->moveTo(in.x, in.y);

    for (size_t i = 0; i < numPoints; i++) {
        auto radius = longSegment ? outerRadius : innerRadius;
        auto dTheta = halfAnglePerPoint;
        if (!tvg::zero(partialPointRadius) && i == numPoints - 2) {
            dTheta = anglePerPoint * partialPointAmount * 0.5f;
        }
        if (!tvg::zero(partialPointRadius) && i == numPoints - 1) {
            radius = partialPointRadius;
        }
        auto previousX = x;
        auto previousY = y;
        x = radius * cosf(angle);
        y = radius * sinf(angle);

        if (hasRoundness) {
            auto cp1Theta = (tvg::atan2(previousY, previousX) - MATH_PI2 * direction);
            auto cp1Dx = cosf(cp1Theta);
            auto cp1Dy = sinf(cp1Theta);
            auto cp2Theta = (tvg::atan2(y, x) - MATH_PI2 * direction);
            auto cp2Dx = cosf(cp2Theta);
            auto cp2Dy = sinf(cp2Theta);

            auto cp1Roundness = longSegment ? innerRoundness : outerRoundness;
            auto cp2Roundness = longSegment ? outerRoundness : innerRoundness;
            auto cp1Radius = longSegment ? innerRadius : outerRadius;
            auto cp2Radius = longSegment ? outerRadius : innerRadius;

            auto cp1x = cp1Radius * cp1Roundness * POLYSTAR_MAGIC_NUMBER * cp1Dx / ptsCnt;
            auto cp1y = cp1Radius * cp1Roundness * POLYSTAR_MAGIC_NUMBER * cp1Dy / ptsCnt;
            auto cp2x = cp2Radius * cp2Roundness * POLYSTAR_MAGIC_NUMBER * cp2Dx / ptsCnt;
            auto cp2y = cp2Radius * cp2Roundness * POLYSTAR_MAGIC_NUMBER * cp2Dy / ptsCnt;

            if (!tvg::zero(partialPointAmount) && ((i == 0) || (i == numPoints - 1))) {
                cp1x *= partialPointAmount;
                cp1y *= partialPointAmount;
                cp2x *= partialPointAmount;
                cp2y *= partialPointAmount;
            }
            auto in2 = Point{previousX - cp1x, previousY - cp1y} * transform;
            auto in3 = Point{x + cp2x, y + cp2y} * transform;
            auto in4 = Point{x, y} * transform;
            shape->cubicTo(in2.x, in2.y, in3.x, in3.y, in4.x, in4.y);
        } else {
            auto in = Point{x, y} * transform;
            shape->lineTo(in.x, in.y);
        }
        angle += dTheta * direction;
        longSegment = !longSegment;
    }
    //ensure proper shape closure - important for modifiers that behave differently for degenerate (linear) vs curved cubics
    _close(to<ShapeImpl>(shape)->rs.path.pts, in, hasRoundness);
    shape->close();

    if (ctx->modifiers) ctx->modifiers->polystar(to<ShapeImpl>(shape)->rs.path, to<ShapeImpl>(merging)->rs.path, outerRoundness, hasRoundness);
}

void LottieBuilder::updatePolygon(LottieGroup* parent, LottiePolyStar* star, float frameNo, Matrix* transform, Shape* merging, RenderContext* ctx, LottieTween& tween, LottieExpressions* exps)
{
    static constexpr auto POLYGON_MAGIC_NUMBER = 0.25f;

    auto ptsCnt = size_t(floor(star->ptsCnt(frameNo, tween, exps)));
    auto radius = star->outerRadius(frameNo, tween, exps);
    auto outerRoundness = star->outerRoundness(frameNo, tween, exps) * 0.01f;

    auto angle = -MATH_PI2;
    auto anglePerPoint = 2.0f * MATH_PI / float(ptsCnt);
    auto direction = star->clockwise ? 1.0f : -1.0f;
    auto hasRoundness = !tvg::zero(outerRoundness);
    auto x = radius * cosf(angle);
    auto y = radius * sinf(angle);

    angle += anglePerPoint * direction;

    Shape* shape;
    if (ctx->modifiers) {
        shape = star->pooling();
        shape->reset();
    } else {
        shape = merging;
        if (hasRoundness) {
            to<ShapeImpl>(shape)->rs.path.pts.reserve(ptsCnt * 3 + 2);
            to<ShapeImpl>(shape)->rs.path.cmds.reserve(ptsCnt + 3);
        } else {
            to<ShapeImpl>(shape)->rs.path.pts.reserve(ptsCnt + 2);
            to<ShapeImpl>(shape)->rs.path.cmds.reserve(ptsCnt + 3);
        }
    }

    auto in = Point{x, y} * transform;
    shape->moveTo(in.x, in.y);

    auto coeff = anglePerPoint * radius * outerRoundness * POLYGON_MAGIC_NUMBER;
    for (size_t i = 0; i < ptsCnt; i++) {
        auto previousX = x;
        auto previousY = y;
        x = (radius * cosf(angle));
        y = (radius * sinf(angle));

        if (hasRoundness) {
            auto cp1Theta = tvg::atan2(previousY, previousX) - MATH_PI2 * direction;
            auto cp1x = coeff * cosf(cp1Theta);
            auto cp1y = coeff * sinf(cp1Theta);
            auto cp2Theta = tvg::atan2(y, x) - MATH_PI2 * direction;
            auto cp2x = coeff * cosf(cp2Theta);
            auto cp2y = coeff * sinf(cp2Theta);

            auto in2 = Point{previousX - cp1x, previousY - cp1y} * transform;
            auto in3 = Point{x + cp2x, y + cp2y} * transform;
            auto in4 = Point{x, y} * transform;
            shape->cubicTo(in2.x, in2.y, in3.x, in3.y, in4.x, in4.y);
        } else {
            Point in = {x, y};
            if (transform) in *= *transform;
            shape->lineTo(in.x, in.y);
        }
        angle += anglePerPoint * direction;
    }
    //ensure proper shape closure - important for modifiers that behave differently for degenerate (linear) vs curved cubics
    _close(to<ShapeImpl>(shape)->rs.path.pts, in, hasRoundness);
    shape->close();

    if (ctx->modifiers) ctx->modifiers->polystar(to<ShapeImpl>(shape)->rs.path, to<ShapeImpl>(merging)->rs.path, 0.0f, false);
}


void LottieBuilder::updatePolystar(LottieGroup* parent, LottieObject** child, float frameNo, TVG_UNUSED Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto star = static_cast<LottiePolyStar*>(*child);

    //Optimize: Can we skip the individual coords transform?
    auto matrix = tvg::identity();
    translate(&matrix, star->position(frameNo, tween, exps));
    rotate(&matrix, star->rotation(frameNo, tween, exps));

    if (ctx->transform) matrix = *ctx->transform * matrix;

    auto identity = tvg::identity((const Matrix*)&matrix);

    if (ctx->repeaters.empty()) {
        _draw(parent, star, ctx);
        if (star->type == LottiePolyStar::Star) updateStar(star, frameNo, (identity ? nullptr : &matrix), ctx->merging, ctx, tween, exps);
        else updatePolygon(parent, star, frameNo, (identity  ? nullptr : &matrix), ctx->merging, ctx, tween, exps);
        PAINT(ctx->merging)->mark(RenderUpdateFlag::Path);
    } else {
        auto shape = star->pooling();
        shape->reset();
        if (star->type == LottiePolyStar::Star) updateStar(star, frameNo, (identity ? nullptr : &matrix), shape, ctx, tween, exps);
        else updatePolygon(parent, star, frameNo, (identity  ? nullptr : &matrix), shape, ctx, tween, exps);
        _repeat(parent, shape, star, ctx);
    }
}


void LottieBuilder::updateRoundedCorner(TVG_UNUSED LottieGroup* parent, LottieObject** child, float frameNo, TVG_UNUSED Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto roundedCorner = static_cast<LottieRoundedCorner*>(*child);
    auto r = roundedCorner->radius(frameNo, tween, exps);
    if (r < LottieRoundnessModifier::ROUNDNESS_EPSILON) return;
    ctx->update(new LottieRoundnessModifier(r));
}


void LottieBuilder::updateOffsetPath(TVG_UNUSED LottieGroup* parent, LottieObject** child, float frameNo, TVG_UNUSED Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto offset = static_cast<LottieOffsetPath*>(*child);
    ctx->update(new LottieOffsetModifier(offset->offset(frameNo, tween, exps), offset->miterLimit(frameNo, tween, exps), offset->join));
}

void LottieBuilder::updatePuckerBloat(TVG_UNUSED LottieGroup* parent, LottieObject** child, float frameNo, TVG_UNUSED Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto puckerBloat = static_cast<LottiePuckerBloat*>(*child);
    ctx->update(new LottiePuckerBloatModifier(puckerBloat->amount(frameNo, tween, exps)));
}

void LottieBuilder::updateZigZag(TVG_UNUSED LottieGroup* parent, LottieObject** child, float frameNo, TVG_UNUSED Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto zigzag = static_cast<LottieZigZag*>(*child);
    auto amplitude = zigzag->amplitude(frameNo, tween, exps);
    if (tvg::zero(amplitude)) return;
    auto frequency = zigzag->frequency(frameNo, tween, exps);
    auto point = (LottieZigZagModifier::PointType)zigzag->point(frameNo, tween, exps);
    ctx->update(new LottieZigZagModifier(amplitude, frequency, point));
}

void LottieBuilder::updateRepeater(TVG_UNUSED LottieGroup* parent, LottieObject** child, float frameNo, TVG_UNUSED Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto repeater = static_cast<LottieRepeater*>(*child);

    RenderRepeater r;
    r.cnt = static_cast<int>(repeater->copies(frameNo, tween, exps));
    r.transform = ctx->propagator->transform();
    r.offset = repeater->offset(frameNo, tween, exps);
    r.position = repeater->position(frameNo, tween, exps);
    r.anchor = repeater->anchor(frameNo, tween, exps);
    r.scale = repeater->scale(frameNo, tween, exps);
    r.rotation = repeater->rotation(frameNo, tween, exps);
    r.startOpacity = repeater->startOpacity(frameNo, tween, exps);
    r.endOpacity = repeater->endOpacity(frameNo, tween, exps);
    r.inorder = repeater->inorder;
    ctx->repeaters.push(r);

    ctx->merging = nullptr;
}


void LottieBuilder::updateTrimpath(TVG_UNUSED LottieGroup* parent, LottieObject** child, float frameNo, TVG_UNUSED Inlist<RenderContext>& contexts, RenderContext* ctx)
{
    auto trimpath = static_cast<LottieTrimpath*>(*child);

    float begin, end;
    trimpath->segment(frameNo, begin, end, tween, exps);

    if (to<ShapeImpl>(ctx->propagator)->rs.stroke) {
        auto length = fabsf(begin - end);
        auto tmp = begin;
        begin = (length * to<ShapeImpl>(ctx->propagator)->rs.stroke->trim.begin) + tmp;
        end = (length * to<ShapeImpl>(ctx->propagator)->rs.stroke->trim.end) + tmp;
    }

    ctx->propagator->trimpath(begin, end, trimpath->type == LottieTrimpath::Type::Simultaneous);
    ctx->merging = nullptr;
}


void LottieBuilder::updateChildren(LottieGroup* parent, float frameNo, Inlist<RenderContext>& contexts)
{
    contexts.head->begin = parent->children.end() - 1;

    while (!contexts.empty()) {
        auto ctx = contexts.front();
        ctx->reqFragment = parent->reqFragment;
        auto stop = false;
        for (auto child = ctx->begin; child >= parent->children.data; --child) {
            //Here switch-case statements are more performant than virtual methods.
            switch ((*child)->type) {
                case LottieObject::Group: {
                    updateGroup(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::Transform: {
                    updateTransform(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::SolidFill: {
                    stop = updateSolidFill(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::SolidStroke: {
                    stop = updateSolidStroke(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::GradientFill: {
                    stop = updateGradientFill(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::GradientStroke: {
                    stop = updateGradientStroke(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::Rect: {
                    updateRect(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::Ellipse: {
                    updateEllipse(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::Path: {
                    updatePath(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::Polystar: {
                    updatePolystar(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::Trimpath: {
                    updateTrimpath(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::Repeater: {
                    updateRepeater(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::RoundedCorner: {
                    updateRoundedCorner(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::OffsetPath: {
                    updateOffsetPath(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::PuckerBloat: {
                    updatePuckerBloat(parent, child, frameNo, contexts, ctx);
                    break;
                }
                case LottieObject::ZigZag: {
                    updateZigZag(parent, child, frameNo, contexts, ctx);
                    break;
                }
                default: break;
            }

            //stop processing for those invisible contents
            if (stop || ctx->propagator->opacity() == 0) break;
        }
        delete(ctx);
    }
}


void LottieBuilder::updatePrecomp(LottieComposition* comp, LottieLayer* precomp, float frameNo)
{
    if (precomp->children.empty()) return;

    frameNo = precomp->remap(comp, frameNo, exps);

    ARRAY_REVERSE_FOREACH(c, precomp->children) {
        auto child = static_cast<LottieLayer*>(*c);
        if (!child->matteSrc) updateLayer(comp, precomp->scene, child, frameNo);
    }

    //clip the layer viewport
    auto clipper = precomp->statical.pooling(true);
    clipper->transform(precomp->cache.matrix);
    precomp->scene->clip(clipper);
}

void LottieBuilder::updatePrecomp(LottieComposition* comp, LottieLayer* precomp, float frameNo, LottieTween& tween)
{
    //record & recover the tweening frame number before remapping
    auto record = tween.to;
    tween.to = precomp->remap(comp, record, exps);

    updatePrecomp(comp, precomp, frameNo);

    tween.to = record;
}


void LottieBuilder::updateSolid(LottieLayer* layer)
{
    auto solidFill = layer->statical.pooling(true);
    solidFill->opacity(layer->cache.opacity);
    layer->scene->add(solidFill);
}


void LottieBuilder::updateImage(LottieGroup* layer)
{
    if (layer->children.empty()) return;

    auto image = static_cast<LottieImage*>(layer->children.first());
    if (image->type != LottieObject::Type::Image) {
        TVGERR("LOTTIE", "Expected image data.");
        return;
    }

    auto picture = image->bitmap.picture;
    if (!picture) return;

    //resolve an image asset if need
    if (resolver && !image->resolved) {
        resolver->func(picture, image->bitmap.path, resolver->data);
        picture->size(image->bitmap.width, image->bitmap.height);
        image->resolved = true;
    }

    //LottieImage can be shared among other layers
    layer->scene->add(picture->refCnt() == 1 ? picture : picture->duplicate());
}


void LottieBuilder::updateURLFont(LottieLayer* layer, float frameNo, LottieText* text, const TextDocument& doc)
{
    //text load
    //TODO: cache the text instance, don't need to reload every frame.
    auto paint = Text::gen();
    if (paint->font(doc.name) != Result::Success) {
        char* src;
        bool free = false;
        if (text->font && text->font->path) src = text->font->path;
        else {
            auto len = (strlen(doc.name) + 6);
            src = tvg::malloc<char>(sizeof(char) * len);
            snprintf(src, len, "name:%s", doc.name);
            free = true;
        }
        if (!resolver || !resolver->func(paint, src, resolver->data)) {
            paint->font(nullptr);  //fallback to any available font
        }
        if (free) tvg::free(src);
    }

    //text build
    auto len = strlen(doc.text);
    auto buf = tvg::malloc<char>(len + 1);

    // preprocess text: convert '\r' and '\3' to '\n' only when they appear within the string
    // this is for the modern-system compatibility
    auto p = buf;
    auto feed = false;
    for (size_t i = 0; i < len; ++i) {
        // replace the carriage return and end of text with line feed.
        if (doc.text[i] == '\r' || doc.text[i] == '\3') {
            if (i == len - 1) break;  // ignore trailing occurrences
            if (!feed) *p = '\n';
            else continue;
        } else *p = doc.text[i];
        feed = (*p == '\n') ? true : false;
        ++p;
    }
    *p = '\0';

    auto color = doc.color;
    paint->fill(color.r, color.g, color.b);
    paint->size(doc.size * 75.0f); //1 pt = 1/72; 1 in = 96 px; -> 72/96 = 0.75
    if (text->font && text->font->style && strstr(text->font->style, "Italic")) paint->italic();
    paint->text(buf);
    paint->layout(doc.bbox.size.x, doc.bbox.size.y);
    if (doc.bbox.size.x > 0.0f) paint->wrap(TextWrap::Smart);

    //align the text to the base line, or top within the box
    TextMetrics metrics;
    paint->metrics(metrics);
    auto vpos = doc.bbox.pos.y - (doc.bbox.size.y > 0.0f ? 0.0f : metrics.ascent);
    paint->translate(doc.bbox.pos.x, vpos);
    paint->align(doc.justify, 0.0f);

    //apply spacing
    auto hspacing = 1.0f + doc.tracking * doc.size / metrics.ascent;
    auto vspacing = (doc.height > 0.0f && paint->lines() > 1) ? (doc.height / metrics.advance) : 1.0f;
    paint->spacing(hspacing, vspacing);

    layer->scene->add(paint);

    //outline
    auto strkColor = doc.stroke.color;
    if (doc.stroke.width > 0.0f) {
        paint->outline(doc.stroke.width, strkColor.r, strkColor.g, strkColor.b);
        to<TextImpl>(paint)->shape->order(doc.stroke.below);
    }

    tvg::free(buf);

    //text range
    if (text->ranges.empty()) return;

    //FIXME: it only considers a single text-range.
    ARRAY_FOREACH(p, text->ranges) {
        auto range = *p;
        auto f = range->factor(frameNo, float(len), 0.0f);
        if (tvg::zero(f)) continue;

        //fill & opacity
        range->color(frameNo, color, strkColor, f, tween, exps);
        paint->fill(color.r, color.g, color.b);
        paint->opacity(range->style.opacity(frameNo, tween, exps));

        //stroke
        if (range->style.flags.strokeWidth) {
            paint->outline(f * range->style.strokeWidth(frameNo, tween, exps), strkColor.r, strkColor.g, strkColor.b);
        }
    }
}


Shape* LottieBuilder::textShape(LottieText* text, float frameNo, const TextDocument& doc, LottieGlyph* glyph, const RenderText& ctx)
{
    auto& transform = ctx.lineScene->transform();
    auto shape = text->pooling();
    shape->reset();

    ARRAY_FOREACH(p, glyph->children) {
        auto group = static_cast<LottieGroup*>(*p);
        ARRAY_FOREACH(p, group->children) {
            static_cast<LottiePath*>(*p)->pathset(frameNo, to<ShapeImpl>(shape)->rs.path, nullptr, tween, exps);
            PAINT(shape)->mark(RenderUpdateFlag::Path);
        }
    }
    shape->fill(doc.color.r, doc.color.g, doc.color.b);
    shape->translate(ctx.cursor.x - transform.e13, ctx.cursor.y - transform.e23);
    shape->opacity(255);

    if (doc.stroke.width > 0.0f) {
        shape->strokeJoin(StrokeJoin::Round);
        shape->strokeWidth(doc.stroke.width / ctx.scale);
        shape->strokeFill(doc.stroke.color.r, doc.stroke.color.g, doc.stroke.color.b);
        shape->order(doc.stroke.below);
    }
    return shape;
}


bool LottieBuilder::updateTextRange(LottieText* text, float frameNo, Shape* shape, const TextDocument& doc, RenderText& ctx)
{
    if (text->ranges.empty()) return false;

    auto& transform = ctx.lineScene->transform();
    Point scaling = {1.0f, 1.0f};
    Point translation = {};
    auto rotation = 0.0f;
    auto color = doc.color;
    auto strokeColor = doc.stroke.color;
    uint8_t opacity = 255, fillOpacity = 255, strokeOpacity = 255;
    auto needGroup = false;

    ARRAY_FOREACH(p, text->ranges) {
        auto range = *p;
        auto basedIdx = ctx.idx;

        if (range->based == LottieTextRange::Based::CharsExcludingSpaces) basedIdx = ctx.idx - ctx.space;
        else if (range->based == LottieTextRange::Based::Words) basedIdx = ctx.line + ctx.space;
        else if (range->based == LottieTextRange::Based::Lines) basedIdx = ctx.line;

        auto f = range->factor(frameNo, float(ctx.nChars), (float)basedIdx);
        if (tvg::zero(f)) continue;

        needGroup = true;

        //transform
        translation = translation + f * range->style.position(frameNo, tween, exps);
        scaling *= (f * (range->style.scale(frameNo, tween, exps) * 0.01f - Point{1.0f, 1.0f}) + Point{1.0f, 1.0f});
        rotation += f * range->style.rotation(frameNo, tween, exps);

        //fill & opacity
        opacity = (uint8_t)(opacity - f * (opacity - range->style.opacity(frameNo, tween, exps)));
        shape->opacity(opacity);

        range->color(frameNo, color, strokeColor, f, tween, exps);
        fillOpacity = (uint8_t)(fillOpacity - f * (fillOpacity - range->style.fillOpacity(frameNo, tween, exps)));
        shape->fill(color.r, color.g, color.b, fillOpacity);

        //stroke
        if (range->style.flags.strokeWidth) shape->strokeWidth(f * range->style.strokeWidth(frameNo, tween, exps) / ctx.scale);
        if (shape->strokeWidth() > 0.0f) {
            strokeOpacity = (uint8_t)(strokeOpacity - f * (strokeOpacity - range->style.strokeOpacity(frameNo, tween, exps)));
            shape->strokeFill(strokeColor.r, strokeColor.g, strokeColor.b, strokeOpacity);
            shape->order(doc.stroke.below);
        }
        ctx.cursor.x += f * range->style.letterSpace(frameNo, tween, exps);
        auto spacing = f * range->style.lineSpace(frameNo, tween, exps);
        if (spacing > ctx.lineSpace) ctx.lineSpace = spacing;
    }
    // Apply line group transformation just once
    if (ctx.lineScene->paints().empty() && needGroup) {
        tvg::identity(&transform);
        translate(&transform, ctx.cursor);

        // center pivoting
        auto align = text->alignOp.anchor(frameNo, tween, exps);
        transform.e13 += align.x;
        transform.e23 += align.y;
        rotate(&transform, rotation);

        //center pivoting
        auto pivot = align * -1;
        transform.e13 += (pivot.x * transform.e11 + pivot.x * transform.e12);
        transform.e23 += (pivot.y * transform.e21 + pivot.y * transform.e22);

        //world space translation
        transform.e13 += translation.x / ctx.scale;
        transform.e23 += translation.y / ctx.scale;

        ctx.lineScene->transform(transform);
    }
    auto& matrix = shape->transform();
    tvg::identity(&matrix);
    translate(&matrix, (translation / ctx.scale + ctx.cursor) - Point{transform.e13, transform.e23});
    tvg::scale(&matrix, scaling * ctx.capScale);
    shape->transform(matrix);

    if (needGroup) ctx.lineScene->add(shape);

    return needGroup;
}


static void _commit(LottieGlyph* glyph, Shape* shape, const RenderText& ctx)
{
    auto& matrix = shape->transform();

    if (ctx.follow) {
        tvg::identity(&matrix);
        auto angle = 0.0f;
        auto width = glyph->width * 0.5f;
        auto pos = ctx.follow->position(ctx.cursor.x + width + ctx.firstMargin, angle);
        matrix.e11 = matrix.e22 = ctx.capScale;
        matrix.e13 = pos.x - width * matrix.e11;
        matrix.e23 = pos.y - width * matrix.e21;
    } else {
        matrix.e11 = matrix.e22 = ctx.capScale;
        matrix.e13 = ctx.cursor.x;
        matrix.e23 = ctx.cursor.y;
    }
    shape->transform(matrix);
    ctx.textScene->add(shape);
}

static LottieGlyph* _searchGlyph(const LottieFont* font, const char* p, const TextDocument& doc, float& capScale)
{
    capScale = 1.0f;
    auto outCode = p;

    // SmallCaps: lowercase letters are converted to uppercase with 70% scale
    if ((unsigned char)*p < 0x80 && doc.caps) {
        if (*p >= 'a' && *p <= 'z') {
            char capCode = *p + 'A' - 'a';
            outCode = &capCode;
            if (doc.caps == 2) capScale = 0.7f;
        }
    }

    // search for matching glyph in font
    ARRAY_FOREACH(g, font->chars)
    {
        auto glyph = *g;
        if (!strncmp(glyph->code, outCode, glyph->len)) return glyph;
    }
    return nullptr;
}

static float _nextWordWidth(const LottieText* text, const TextDocument& doc, const char* p)
{
    float w = 0.0f;
    // accumulate width until space, carriage return (13), or end-of-text (3)
    while (*p && *p != ' ' && (unsigned char)*p != 13 && (unsigned char)*p != 3) {
        float capScale;
        auto glyph = _searchGlyph(text->font, p, doc, capScale);
        if (glyph) {
            w += (glyph->width + doc.tracking) * capScale;
            p += glyph->len;
        } else ++p;
    }
    return w;
}

void LottieBuilder::updateLocalFont(LottieLayer* layer, float frameNo, LottieText* text, const TextDocument& doc)
{
    RenderText ctx(text, doc);
    ctx.follow = (text->follow && ((uint32_t)text->follow->maskIdx < layer->masks.count)) ? text->follow : nullptr;
    ctx.firstMargin = ctx.follow ? ctx.follow->prepare(layer->masks[ctx.follow->maskIdx], frameNo, ctx.scale, tween, exps) : 0.0f;
    auto lineWrapped = false;

    //text string
    while (true) {
        //new line of the cursor position
        if (lineWrapped || *ctx.p == 13 || *ctx.p == 3 || *ctx.p == '\0') {
            //text layout position
            auto ascent = text->font->ascent * ctx.scale;
            if (ascent > doc.bbox.size.y) ascent = doc.bbox.size.y;

            //horizontal alignment
            Point layout = {doc.bbox.pos.x, doc.bbox.pos.y + ascent - doc.shift};
            layout.x += doc.justify * (doc.bbox.size.x - ctx.cursor.x * ctx.scale);

            //new text group, single scene based on text-grouping
            ctx.textScene->add(ctx.lineScene);
            ctx.textScene->translate(layout.x, layout.y);
            ctx.textScene->scale(ctx.scale);
            layer->scene->add(ctx.textScene);

            ctx.lineScene = Scene::gen();
            ctx.lineScene->translate(ctx.cursor.x, ctx.cursor.y);

            if (*ctx.p == '\0') {
                ctx.textScene = nullptr;
                break;
            }
            if (!lineWrapped) ++ctx.p;

            ctx.totalLineSpace += ctx.lineSpace;
            ctx.lineSpace = 0.0f;
            lineWrapped = false;

            //new text group, single scene for each line
            ctx.textScene = Scene::gen();
            ctx.cursor = {0.0f, (++ctx.line * doc.height + ctx.totalLineSpace) / ctx.scale};
            continue;
        }
        /* all lowercase letters are converted to uppercase in the "t" text field, making the "ca" value irrelevant, thus AllCaps is nothing to do.
           So only convert lowercase letters to uppercase (for 'SmallCaps' an extra scaling factor applied) */
        auto glyph = _searchGlyph(text->font, ctx.p, doc, ctx.capScale);
        auto advance = glyph ? (glyph->width + doc.tracking) * ctx.capScale : 0.0f;

        if (*ctx.p == ' ') {
            // if next word overflows the box, break at this space
            if (doc.bbox.size.x > 0.0f && (ctx.cursor.x + advance + _nextWordWidth(text, doc, ctx.p + 1)) * ctx.scale >= doc.bbox.size.x) {
                ++ctx.p;
                lineWrapped = true;
                continue;
            }
            ++ctx.space;
            // new text group, single scene for each word
            if (text->alignOp.group == LottieText::AlignOption::Group::Word) {
                ctx.textScene->add(ctx.lineScene);
                ctx.lineScene = Scene::gen();
                ctx.lineScene->translate(ctx.cursor.x, ctx.cursor.y);
            }
        }

        // draw matched glyphs
        if (glyph) {
            if (doc.bbox.size.x > 0.0f && ctx.cursor.x > 0.0f && (ctx.cursor.x + advance) * ctx.scale > doc.bbox.size.x) {
                lineWrapped = true;
                continue;
            }
            if (text->alignOp.group == LottieText::AlignOption::Group::Chars || text->alignOp.group == LottieText::AlignOption::Group::All) {
                ctx.textScene->add(ctx.lineScene);
                ctx.lineScene = Scene::gen();
                ctx.lineScene->translate(ctx.cursor.x, ctx.cursor.y);
            }
            auto shape = textShape(text, frameNo, doc, glyph, ctx);
            if (!updateTextRange(text, frameNo, shape, doc, ctx)) _commit(glyph, shape, ctx);
            ctx.cursor.x += advance;
            ctx.p += glyph->len;
            ctx.idx += glyph->len;
        } else {
            ++ctx.p;
            ++ctx.idx;
        }
    }
}

void LottieBuilder::updateText(LottieLayer* layer, float frameNo)
{
    if (layer->children.empty()) return;

    auto text = static_cast<LottieText*>(layer->children.first());
    auto& doc = text->doc(frameNo, exps);
    if (!doc.text) return;

    if (text->font && text->font->origin == LottieFont::Origin::Local && !text->font->chars.empty()) {
        updateLocalFont(layer, frameNo, text, doc);
    } else {
        updateURLFont(layer, frameNo, text, doc);
    }
}


void LottieBuilder::updateMasks(LottieLayer* layer, float frameNo)
{
    if (layer->masks.count == 0) return;

    //Introduce an intermediate scene for embracing matte + masking or precomp clipping + masking replaced by clipping
    if (layer->matteTarget || layer->type == LottieLayer::Precomp) {
        auto scene = Scene::gen();
        scene->add(layer->scene);
        layer->scene = scene;
    }

    Shape* pShape = nullptr;
    MaskMethod pMethod;
    uint8_t pOpacity;

    ARRAY_FOREACH(p, layer->masks) {
        auto mask = *p;
        if (mask->method == MaskMethod::None) continue;

        auto method = mask->method;
        auto opacity = mask->opacity(frameNo);
        auto expand = mask->expand(frameNo);

        //the first mask
        if (!pShape) {
            pShape = layer->pooling();
            to<ShapeImpl>(pShape)->reset();
            auto compMethod = (method == MaskMethod::Subtract || method == MaskMethod::InvAlpha) ? MaskMethod::InvAlpha : MaskMethod::Alpha;
            //Cheaper. Replace the masking with a clipper
            if (!layer->effect && layer->masks.count == 1 && compMethod == MaskMethod::Alpha) {
                layer->scene->opacity(MULTIPLY(layer->scene->opacity(), opacity));
                layer->scene->clip(pShape);
            } else {
                layer->scene->mask(pShape, compMethod);
            }
        //Chain mask composition
        } else if (pMethod != method || pOpacity != opacity || (method != MaskMethod::Subtract && method != MaskMethod::Difference)) {
            auto shape = layer->pooling();
            to<ShapeImpl>(shape)->reset();
            pShape->mask(shape, method);
            pShape = shape;
        }

        pShape->fill(255, 255, 255, opacity);
        pShape->transform(layer->cache.matrix);

        //Default Masking
        if (expand == 0.0f) {
            mask->pathset(frameNo, to<ShapeImpl>(pShape)->rs.path, nullptr, tween, exps);
        //Masking with Expansion (Offset)
        } else {
            //TODO: Once path direction support is implemented, ensure that the direction is ignored here
            auto offset = LottieOffsetModifier(expand);
            mask->pathset(frameNo, to<ShapeImpl>(pShape)->rs.path, nullptr, tween, exps, &offset);
        }
        pOpacity = opacity;
        pMethod = method;
    }
}


bool LottieBuilder::updateMatte(LottieComposition* comp, float frameNo, Scene* scene, LottieLayer* layer)
{
    auto target = layer->matteTarget;
    if (!target || target->type == LottieLayer::Null) return true;

    updateLayer(comp, scene, target, frameNo);

    if (target->scene) {
        layer->scene->mask(target->scene, layer->matteType);
    } else if (layer->matteType == MaskMethod::Alpha || layer->matteType == MaskMethod::Luma) {
        //matte target is not exist. alpha blending definitely bring an invisible result
        Paint::rel(layer->scene);
        layer->scene = nullptr;
        return false;
    }
    return true;
}


void LottieBuilder::updateStrokeEffect(LottieLayer* layer, LottieFxStroke* effect, float frameNo)
{
    if (layer->masks.count == 0) return;

    auto shape = layer->pooling();
    shape->reset();

    //FIXME: all mask
    if (effect->allMask(frameNo)) {
        ARRAY_FOREACH(p, layer->masks) {
            (*p)->pathset(frameNo, to<ShapeImpl>(shape)->rs.path, nullptr, tween, exps);
        }
    //A specific mask
    } else {
        auto idx = static_cast<uint32_t>(effect->mask(frameNo) - 1);
        if (idx < 0 || idx >= layer->masks.count) return;
        layer->masks[idx]->pathset(frameNo, to<ShapeImpl>(shape)->rs.path, nullptr, tween, exps);
    }

    shape->transform(layer->cache.matrix);
    shape->trimpath(effect->begin(frameNo) * 0.01f, effect->end(frameNo) * 0.01f);
    shape->strokeFill(255, 255, 255, (int)(effect->opacity(frameNo) * 255.0f));
    shape->strokeJoin(StrokeJoin::Round);
    shape->strokeCap(StrokeCap::Round);

    auto size = effect->size(frameNo) * 2.0f;
    shape->strokeWidth(size);

    //fill the color to the layer shapes if any
    auto color = effect->color(frameNo);
    if (color.r != 255 || color.g != 255 || color.b != 255) {
        auto accessor = tvg::Accessor::gen();
        auto stroke = (layer->type == LottieLayer::Type::Shape) ? true : false;
        auto f = [color, size, stroke](const tvg::Paint* paint, void* data) -> bool {
            if (paint->type() == tvg::Type::Shape) {
                auto shape = (tvg::Shape*) paint;
                //expand shape to fill the stroke region
                if (stroke) {
                    shape->strokeWidth(size);
                    shape->strokeFill(color.r, color.g, color.b, 255);
                }
                shape->fill(color.r, color.g, color.b, 255);
            }
            return true;
        };
        accessor->set(layer->scene, f, nullptr);
        delete(accessor);
    }

    layer->scene->mask(shape, MaskMethod::Alpha);
}


void LottieBuilder::updateEffect(LottieLayer* layer, float frameNo, uint8_t quality)
{
    constexpr float BLUR_TO_SIGMA = 0.3f;

    if (layer->effects.count == 0) return;

    ARRAY_FOREACH(p, layer->effects) {
        if (!(*p)->enable) continue;
        switch ((*p)->type) {
            case LottieEffect::Tint: {
                auto effect = static_cast<LottieFxTint*>(*p);
                auto black = effect->black(frameNo);
                auto white = effect->white(frameNo);
                layer->scene->add(SceneEffect::Tint, black.r, black.g, black.b, white.r, white.g, white.b, (double)effect->intensity(frameNo));
                break;
            }
            case LottieEffect::Fill: {
                auto effect = static_cast<LottieFxFill*>(*p);
                auto color = effect->color(frameNo);
                layer->scene->add(SceneEffect::Fill, color.r, color.g, color.b, (int)(255.0f * effect->opacity(frameNo)));
                break;
            }
            case LottieEffect::Stroke: {
                auto effect = static_cast<LottieFxStroke*>(*p);
                updateStrokeEffect(layer, effect, frameNo);
                break;
            }
            case LottieEffect::Tritone: {
                auto effect = static_cast<LottieFxTritone*>(*p);
                auto dark = effect->dark(frameNo);
                auto midtone = effect->midtone(frameNo);
                auto bright = effect->bright(frameNo);
                layer->scene->add(SceneEffect::Tritone, dark.r, dark.g, dark.b, midtone.r, midtone.g, midtone.b, bright.r, bright.g, bright.b, (int)effect->blend(frameNo));
                break;
            }
            case LottieEffect::DropShadow: {
                auto effect = static_cast<LottieFxDropShadow*>(*p);
                auto color = effect->color(frameNo);
                //seems the opacity range in dropshadow is 0 ~ 256
                layer->scene->add(SceneEffect::DropShadow, color.r, color.g, color.b, std::min(255, (int)effect->opacity(frameNo)), (double)effect->angle(frameNo), double(effect->distance(frameNo)), (double)(effect->blurness(frameNo) * BLUR_TO_SIGMA), quality);
                break;
            }
            case LottieEffect::GaussianBlur: {
                auto effect = static_cast<LottieFxGaussianBlur*>(*p);
                layer->scene->add(SceneEffect::GaussianBlur, (double)(effect->blurness(frameNo) * BLUR_TO_SIGMA), effect->direction(frameNo) - 1, effect->wrap(frameNo), quality);
                break;
            }
            default: break;
        }
    }
}


void LottieBuilder::updateLayer(LottieComposition* comp, Scene* scene, LottieLayer* layer, float frameNo)
{
    if (layer->type == LottieLayer::Audio) {
        if (audioResolver.func) updateAudio(comp, layer, frameNo);
        return;
    }

    layer->scene = nullptr;

    //visibility
    if (frameNo < layer->inFrame || frameNo >= layer->outFrame) return;

    updateTransform(layer, frameNo);

    //full transparent scene. no need to perform
    if (layer->type != LottieLayer::Null && layer->cache.opacity == 0) return;

    //Prepare render data
    layer->scene = Scene::gen();
    layer->scene->id = layer->id;

    //ignore opacity when Null layer?
    if (layer->type != LottieLayer::Null) layer->scene->opacity(layer->cache.opacity);

    layer->scene->transform(layer->cache.matrix);

    if (!layer->matteSrc && !updateMatte(comp, frameNo, scene, layer)) return;

    layer->scene->blend(layer->blendMethod);

    switch (layer->type) {
        case LottieLayer::Precomp: {
            if (!tween.active) updatePrecomp(comp, layer, frameNo);
            else updatePrecomp(comp, layer, frameNo, tween);
            break;
        }
        case LottieLayer::Solid: {
            updateSolid(layer);
            break;
        }
        case LottieLayer::Image: {
            updateImage(layer);
            break;
        }
        case LottieLayer::Text: {
            updateText(layer, frameNo);
            break;
        }
        default: {
            if (!layer->children.empty()) {
                Inlist<RenderContext> contexts;
                contexts.back(new RenderContext(layer->pooling()));
                updateChildren(layer, frameNo, contexts);
                contexts.free();
            }
            break;
        }
    }

    updateMasks(layer, frameNo);

    updateEffect(layer, frameNo, comp->quality);

    if (!layer->matteSrc) scene->add(layer->scene);
}


static void _buildReference(LottieComposition* comp, LottieLayer* layer)
{
    ARRAY_FOREACH(p, comp->assets) {
        if (layer->rid != (*p)->id) continue;
        if (layer->type == LottieLayer::Precomp) {
            auto asset = static_cast<LottieLayer*>(*p);
            if (_buildComposition(comp, asset)) {
                layer->children = asset->children;
                layer->reqFragment = asset->reqFragment;
                layer->effect |= asset->effect;
            }
        } else if (layer->type == LottieLayer::Image || layer->type == LottieLayer::Audio) {
            layer->children.push(*p);
        }
        break;
    }
}


static void _buildHierarchy(LottieGroup* parent, LottieLayer* child)
{
    if (child->pix == -1) return;

    if (child->matteTarget && child->pix == child->matteTarget->ix) {
        child->parent = child->matteTarget;
        return;
    }

    ARRAY_FOREACH(p, parent->children) {
        auto parent = static_cast<LottieLayer*>(*p);
        if (child == parent) continue;
        if (child->pix == parent->ix) {
            child->parent = parent;
            break;
        }
        if (parent->matteTarget && parent->matteTarget->ix == child->pix) {
            child->parent = parent->matteTarget;
            break;
        }
    }
}


static void _attachFont(LottieComposition* comp, LottieLayer* parent)
{
    //TODO: Consider to migrate this attachment to the frame update time.
    ARRAY_FOREACH(p, parent->children) {
        auto text = static_cast<LottieText*>(*p);
        auto& doc = text->doc(0);
        if (!doc.name) continue;
        auto len = strlen(doc.name);
        for (uint32_t i = 0; i < comp->fonts.count; ++i) {
            auto font = comp->fonts[i];
            auto len2 = strlen(font->name);
            if (len == len2 && !strcmp(font->name, doc.name)) {
                text->font = font;
                break;
            }
        }
    }
}


static bool _buildComposition(LottieComposition* comp, LottieLayer* parent)
{
    if (parent->children.count == 0) return false;
    if (parent->buildDone) return true;
    parent->buildDone = true;

    ARRAY_FOREACH(p, parent->children) {
        auto child = static_cast<LottieLayer*>(*p);

        //attach the precomp layer.
        if (child->rid) _buildReference(comp, child);

        if (child->matteType != MaskMethod::None) {
            //no index of the matte layer is provided: the layer above is used as the matte source
            if (child->mix == -1) {
                if (p > parent->children.begin()) {
                    child->matteTarget = static_cast<LottieLayer*>(*(p - 1));
                }
            //matte layer is specified by an index.
            } else child->matteTarget = parent->layerByIdx(child->mix);
        }

        if (child->matteTarget) {
            child->matteTarget->matteSrc = true;
            //parenting
            _buildHierarchy(parent, child->matteTarget);
            //precomp referencing
            if (child->matteTarget->rid) _buildReference(comp, child->matteTarget);
        }
        _buildHierarchy(parent, child);

        //attach the necessary font data
        if (child->type == LottieLayer::Text) _attachFont(comp, child);
    }
    return true;
}


/************************************************************************/
/* External Class Implementation                                        */
/************************************************************************/

void LottieBuilder::updateAudio(LottieComposition* comp, LottieLayer* layer, float frameNo)
{
    if (layer->children.empty()) return;

    auto ctrl = layer->audio();
    auto active = frameNo >= layer->inFrame && frameNo < layer->outFrame;

    auto volume = 100.0f;
    if (active) volume = tvg::clamp(ctrl->volume(frameNo, tween, exps), 0.0f, 100.0f);

    auto changed = (active != ctrl->prevActive) ||
                   (active && !tvg::equal(volume, ctrl->prevVolume));

    if (changed) {
        auto asset = static_cast<LottieAudio*>(layer->children.first());

        LottieAudioResolver info{};
        info.src      = asset->data;          //identifies the source on both activation and deactivation
        info.mimeType = asset->mimeType;
        info.size     = asset->size;
        info.embedded = (asset->size > 0);
        info.volume   = volume;
        info.active   = active;
        if (active) info.offset = (layer->remap(comp, frameNo, exps) - layer->remap(comp, layer->inFrame, exps)) / comp->frameRate;

        audioResolver.func(info, audioResolver.data);
    }

    ctrl->prevActive = active;
    ctrl->prevVolume = volume;
}


bool LottieBuilder::update(LottieComposition* comp, float frameNo)
{
    if (comp->root->children.empty()) return false;

    comp->clamp(frameNo);

    if (tween.active) comp->clamp(tween.to);

    if (exps && comp->expressions) exps->update(comp->timeAtFrame(frameNo));

    //update children layers
    ARRAY_REVERSE_FOREACH(child, comp->root->children) {
        auto layer = static_cast<LottieLayer*>(*child);
        if (!layer->matteSrc) updateLayer(comp, comp->root->scene, layer, frameNo);
    }

    return true;
}


void LottieBuilder::build(LottieComposition* comp)
{
    if (!comp) return;

    comp->root->scene = Scene::gen();

    _buildComposition(comp, comp->root);

    //viewport clip
    auto clip = Shape::gen();
    clip->appendRect(0, 0, comp->w, comp->h);
    comp->root->scene->clip(clip);

    //turn off partial rendering for children
    to<SceneImpl>(comp->root->scene)->size({comp->w, comp->h});
}