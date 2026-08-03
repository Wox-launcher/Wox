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

#include "tvgMath.h"
#include "tvgTaskScheduler.h"
#include "tvgLottieModel.h"
#include "tvgCompressor.h"


/************************************************************************/
/* Internal Class Implementation                                        */
/************************************************************************/

Point LottieTextFollowPath::split(float dLen, float lenSearched, float& angle)
{
    switch (*cmds) {
        case PathCommand::MoveTo: {
            angle = 0.0f;
            break;
        }
        case PathCommand::LineTo: {
            auto dp = *pts - *(pts - 1);
            angle = tvg::atan(dp);
            break;
        }
        case PathCommand::CubicTo: {
            auto bz = Bezier{*(pts - 1), *pts, *(pts + 1), *(pts + 2)};
            float t = bz.at(lenSearched - currentLen, dLen);
            angle = deg2rad(bz.angle(t));
            return bz.at(t);
        }
        case PathCommand::Close: {
            auto dp = *start - *(pts - 1);
            angle = tvg::atan(dp);
            break;
        }
    }
    return {};
}

/************************************************************************/
/* External Class Implementation                                        */
/************************************************************************/

void LottieTextFollowPath::rewind()
{
    pts = path.pts.data;
    cmds = path.cmds.data;
    cmdsCnt = path.cmds.count;
    currentLen = 0.0f;
}

float LottieTextFollowPath::prepare(LottieMask* mask, float frameNo, float scale, LottieTween& tween, LottieExpressions* exps)
{
    this->mask = mask;
    Matrix m{1.0f / scale, 0.0f, 0.0f, 0.0f, 1.0f / scale, 0.0f, 0.0f, 0.0f, 1.0f};
    path.clear();
    mask->pathset(frameNo, path, &m, tween, exps);

    rewind();
    totalLen = tvg::length(cmds, cmdsCnt, pts, path.pts.count);
    start = pts;

    return firstMargin(frameNo, tween, exps) / scale;
}

Point LottieTextFollowPath::position(float lenSearched, float& angle)
{
    //position before the start of the curve
    if (lenSearched <= 0.0f) {
        //shape is closed -> wrapping
        if (path.cmds.last() == PathCommand::Close) {
            while (lenSearched < 0.0f) lenSearched += totalLen;
        //linear interpolation
        } else {
            if (cmds >= path.cmds.data + path.cmds.count - 1) return *start;
            switch (*(cmds + 1)) {
                case PathCommand::LineTo: {
                    auto dp = *(pts + 1) - *pts;
                    angle = tvg::atan(dp);
                    return {pts->x + lenSearched * cos(angle), pts->y + lenSearched * sin(angle)};
                }
                case PathCommand::CubicTo: {
                    angle = deg2rad(Bezier{*pts, *(pts + 1), *(pts + 2), *(pts + 3)}.angle(0.0001f));
                    return {pts->x + lenSearched * cos(angle), pts->y + lenSearched * sin(angle)};
                }
                default:
                    angle = 0.0f;
                return *start;
            }
        }
    }

    auto shift = [&]() -> void {
        switch (*cmds) {
            case PathCommand::MoveTo: start = pts; ++pts; break;
            case PathCommand::LineTo: ++pts; break;
            case PathCommand::CubicTo: pts += 3; break;
            case PathCommand::Close: break;
        }
        ++cmds;
        --cmdsCnt;
    };

    //position beyond the end of the curve
    if (lenSearched >= totalLen) {
        //shape is closed -> wrapping
        if (path.cmds.last() == PathCommand::Close) {
            while (lenSearched > totalLen) lenSearched -= totalLen;
        //linear interpolation
        } else {
            while (cmdsCnt > 1) shift();
            switch (*cmds) {
                case PathCommand::MoveTo:
                    angle = 0.0f;
                    return *pts;
                case PathCommand::LineTo: {
                    auto len = lenSearched - totalLen;
                    auto dp = *pts - *(pts - 1);
                    angle = tvg::atan(dp);
                    return {pts->x + len * cos(angle), pts->y + len * sin(angle)};
                }
                case PathCommand::CubicTo: {
                    auto len = lenSearched - totalLen;
                    angle = deg2rad(Bezier{*(pts - 1), *pts, *(pts + 1), *(pts + 2)}.angle(0.999f));
                    return {(pts + 2)->x + len * cos(angle), (pts + 2)->y + len * sin(angle)};
                }
                case PathCommand::Close: {
                    auto len = lenSearched - totalLen;
                    auto dp = *start - *(pts - 1);
                    angle = tvg::atan(dp);
                    return {(pts - 1)->x + len * cos(angle), (pts - 1)->y + len * sin(angle)};
                }
            }
        }
    }

    //reset required if text partially crosses curve start
    if (lenSearched < currentLen) rewind();

    auto length = [&]() -> float {
        switch (*cmds) {
            case PathCommand::MoveTo: return 0.0f;
            case PathCommand::LineTo: return tvg::length(*(pts - 1), *pts);
            case PathCommand::CubicTo: return Bezier{*(pts - 1), *pts, *(pts + 1), *(pts + 2)}.length();
            case PathCommand::Close: return tvg::length(*(pts - 1), *start);
            default: return 0.0f;
        }
    };

    while (cmdsCnt > 0) {
        auto dLen = length();
        if (currentLen + dLen < lenSearched) {
            shift();
            currentLen += dLen;
            continue;
        }
        return split(dLen, lenSearched, angle);
    }
    return {};
}


void LottieSlot::reset()
{
    if (!overridden) return;

    ARRAY_FOREACH(pair, pairs) {
        pair->obj->override(pair->prop, true);
        delete(pair->prop);
        pair->prop = nullptr;
    }
    overridden = false;
}


void LottieSlot::apply(LottieProperty* prop, bool byDefault)
{
    auto release = overridden || byDefault;

    //apply slot object to all targets
    ARRAY_FOREACH(pair, pairs) {
        auto backup = pair->obj->override(prop, release);
        if (!release) pair->prop = backup;
    }

    if (!byDefault) overridden = true;
}


float LottieTextRange::factor(float frameNo, float totalLen, float idx)
{
    auto offset = this->offset(frameNo);
    auto start = this->start(frameNo) + offset;
    auto end = this->end(frameNo) + offset;

    if (random > 0) {
        auto range = end - start;
        auto len = (rangeUnit == Unit::Percent) ? 100.0f : totalLen;
        start = static_cast<float>(random % int(len - range));
        end = start + range;
    }

    auto divisor = (rangeUnit == Unit::Percent) ? (100.0f / totalLen) : 1.0f;
    start /= divisor;
    end /= divisor;

    auto f = 0.0f;

    switch (this->shape) {
        case Square: {
            auto smoothness = this->smoothness(frameNo);
            if (tvg::zero(smoothness)) f = idx >= nearbyintf(start) && idx < nearbyintf(end) ? 1.0f : 0.0f;
            else {
                if (idx >= std::floor(start)) {
                    auto diff = idx - start;
                    f = diff < 0.0f ? std::min(end, 1.0f) + diff : end - idx;
                }
                smoothness *= 0.01f;
                f = (f - (1.0f - smoothness) * 0.5f) / smoothness;
            }
            break;
        }
        case RampUp: {
            f = tvg::equal(start, end) ? (idx >= end ? 1.0f : 0.0f) : (0.5f + idx - start) / (end - start);
            break;
        }
        case RampDown: {
            f = tvg::equal(start, end) ? (idx >= end ? 0.0f : 1.0f) : 1.0f - (0.5f + idx - start) / (end - start);
            break;
        }
        case Triangle: {
            f = tvg::equal(start, end) ? 0.0f : 2.0f * (0.5f + idx - start) / (end - start);
            f = f < 1.0f ? f : 2.0f - f;
            break;
        }
        case Round: {
            idx = tvg::clamp(idx + (0.5f - start), 0.0f, end - start);
            auto range = 0.5f * (end - start);
            auto t = idx - range;
            f = tvg::equal(start, end) ? 0.0f : sqrtf(1.0f - t * t / (range * range));
            break;
        }
        case Smooth: {
            idx = tvg::clamp(idx + (0.5f - start), 0.0f, end - start);
            f = tvg::equal(start, end) ? 0.0f : 0.5f * (1.0f + cosf(MATH_PI * (1.0f + 2.0f * idx / (end - start))));
            break;
        }
    }
    f = tvg::clamp(f, 0.0f, 1.0f);

    //apply easing
    auto minEase = tvg::clamp(this->minEase(frameNo), -100.0f, 100.0f);
    auto maxEase = tvg::clamp(this->maxEase(frameNo), -100.0f, 100.0f);
    if (!tvg::zero(minEase) || !tvg::zero(maxEase)) {
        Point in{1.0f, 1.0f};
        Point out{0.0f, 0.0f};

        if (maxEase > 0.0f) in.x = 1.0f - maxEase * 0.01f;
        else in.y = 1.0f + maxEase * 0.01f;
        if (minEase > 0.0f) out.x = minEase * 0.01f;
        else out.y = -minEase * 0.01f;

        interpolator->set(nullptr, in, out);
        f = interpolator->progress(f);
    }
    f = tvg::clamp(f, 0.0f, 1.0f);

    return f * this->maxAmount(frameNo) * 0.01f;
}


void LottieFont::prepare()
{
    if (b64src) Text::load(name, b64src, size, mime, false);
}


void LottieImage::prepare(bool external)
{
    LottieObject::type = LottieObject::Image;

    //Prepare the Picture image
    auto result = Result::Unknown;
    auto picture = Picture::gen();
    if (bitmap.size > 0) result = picture->load((const char*)bitmap.data, bitmap.size, bitmap.mimeType);
    else if (external) result = picture->load(bitmap.path);
    if (result == Result::Success) resolved = true;
    picture->size(bitmap.width, bitmap.height);
    bitmap.picture = picture;
    picture->ref();
}

void LottieTrimpath::segment(float frameNo, float& start, float& end, LottieTween& tween, LottieExpressions* exps)
{
    start = tvg::clamp(this->start(frameNo, tween, exps) * 0.01f, 0.0f, 1.0f);
    end = tvg::clamp(this->end(frameNo, tween, exps) * 0.01f, 0.0f, 1.0f);

    auto diff = fabs(start - end);
    if (tvg::zero(diff)) {
        start = 0.0f;
        end = 0.0f;
        return;
    }

    //Even if the start and end values do not cause trimming, an offset > 0 can still affect dashing starting point
    auto o = fmodf(this->offset(frameNo, tween, exps), 360.0f) / 360.0f;  //0 ~ 1
    if (tvg::zero(o) && diff >= 1.0f) {
        start = 0.0f;
        end = 1.0f;
        return;
    }

    if (start > end) std::swap(start, end);
    start += o;
    end += o;
}


uint32_t LottieGradient::populate(ColorStop& color, size_t count)
{
    if (!color.input) return 0;

    uint32_t alphaCnt = (color.input->count - (count * 4)) / 2;
    Array<Fill::ColorStop> output(count + alphaCnt);
    uint32_t cidx = 0;               //color count
    uint32_t clast = count * 4;
    if (clast > color.input->count) clast = color.input->count;
    uint32_t aidx = clast;           //alpha count
    Fill::ColorStop cs;

    //merge color stops.
    for (uint32_t i = 0; i < color.input->count; ++i) {
        if (cidx == clast || aidx == color.input->count) break;
        if ((*color.input)[cidx] == (*color.input)[aidx]) {
            cs.offset = (*color.input)[cidx];
            cs.r = (uint8_t)nearbyint((*color.input)[cidx + 1] * 255.0f);
            cs.g = (uint8_t)nearbyint((*color.input)[cidx + 2] * 255.0f);
            cs.b = (uint8_t)nearbyint((*color.input)[cidx + 3] * 255.0f);
            cs.a = (uint8_t)nearbyint((*color.input)[aidx + 1] * 255.0f);
            cidx += 4;
            aidx += 2;
        } else if ((*color.input)[cidx] < (*color.input)[aidx]) {
            cs.offset = (*color.input)[cidx];
            cs.r = (uint8_t)nearbyint((*color.input)[cidx + 1] * 255.0f);
            cs.g = (uint8_t)nearbyint((*color.input)[cidx + 2] * 255.0f);
            cs.b = (uint8_t)nearbyint((*color.input)[cidx + 3] * 255.0f);
            //generate alpha value
            if (output.count > 0) {
                auto p = ((*color.input)[cidx] - output.last().offset) / ((*color.input)[aidx] - output.last().offset);
                cs.a = tvg::lerp<uint8_t>(output.last().a, (uint8_t)nearbyint((*color.input)[aidx + 1] * 255.0f), p);
            } else cs.a = (uint8_t)nearbyint((*color.input)[aidx + 1] * 255.0f);
            cidx += 4;
        } else {
            cs.offset = (*color.input)[aidx];
            cs.a = (uint8_t)nearbyint((*color.input)[aidx + 1] * 255.0f);
            //generate color value
            if (output.count > 0) {
                auto p = ((*color.input)[aidx] - output.last().offset) / ((*color.input)[cidx] - output.last().offset);
                cs.r = tvg::lerp<uint8_t>(output.last().r, (uint8_t)nearbyint((*color.input)[cidx + 1] * 255.0f), p);
                cs.g = tvg::lerp<uint8_t>(output.last().g, (uint8_t)nearbyint((*color.input)[cidx + 2] * 255.0f), p);
                cs.b = tvg::lerp<uint8_t>(output.last().b, (uint8_t)nearbyint((*color.input)[cidx + 3] * 255.0f), p);
            } else {
                cs.r = (uint8_t)nearbyint((*color.input)[cidx + 1] * 255.0f);
                cs.g = (uint8_t)nearbyint((*color.input)[cidx + 2] * 255.0f);
                cs.b = (uint8_t)nearbyint((*color.input)[cidx + 3] * 255.0f);
            }
            aidx += 2;
        }
        if (cs.a < 255) opaque = false;
        output.push(cs);
    }

    //color remains
    while (cidx + 3 < clast) {
        cs.offset = (*color.input)[cidx];
        cs.r = (uint8_t)nearbyint((*color.input)[cidx + 1] * 255.0f);
        cs.g = (uint8_t)nearbyint((*color.input)[cidx + 2] * 255.0f);
        cs.b = (uint8_t)nearbyint((*color.input)[cidx + 3] * 255.0f);
        cs.a = (output.count > 0) ? output.last().a : 255;
        if (cs.a < 255) opaque = false;
        output.push(cs);
        cidx += 4;
    }

    //alpha remains
    while (aidx < color.input->count) {
        cs.offset = (*color.input)[aidx];
        cs.a = (uint8_t)nearbyint((*color.input)[aidx + 1] * 255.0f);
        if (cs.a < 255) opaque = false;
        if (output.count > 0) {
            cs.r = output.last().r;
            cs.g = output.last().g;
            cs.b = output.last().b;
        } else cs.r = cs.g = cs.b = 255;
        output.push(cs);
        aidx += 2;
    }

    color.data = output.data;
    output.data = nullptr;

    color.input->reset();
    delete(color.input);
    color.input = nullptr;

    return output.count;
}

Fill* LottieGradient::fill(float frameNo, uint8_t opacity, LottieTween& tween, LottieExpressions* exps)
{
    if (opacity == 0) return nullptr;

    Fill* fill;
    auto s = start(frameNo, tween, exps);
    auto e = end(frameNo, tween, exps);

    //Linear Graident
    if (id == 1) {
        fill = LinearGradient::gen();
        static_cast<LinearGradient*>(fill)->linear(s.x, s.y, e.x, e.y);
    //Radial Gradient
    } else {
        fill = RadialGradient::gen();
        auto w = fabsf(e.x - s.x);
        auto h = fabsf(e.y - s.y);
        auto r = (w > h) ? (w + 0.375f * h) : (h + 0.375f * w);
        auto progress = this->height(frameNo, tween, exps) * 0.01f;

        if (tvg::zero(progress)) {
            static_cast<RadialGradient*>(fill)->radial(s.x, s.y, r, s.x, s.y, 0.0f);
        } else {
            auto startAngle = rad2deg(tvg::atan(e - s));
            auto angle = deg2rad((startAngle + this->angle(frameNo, tween, exps)));
            auto fx = s.x + cos(angle) * progress * r;
            auto fy = s.y + sin(angle) * progress * r;
            // Lottie doesn't have any focal radius concept
            static_cast<RadialGradient*>(fill)->radial(s.x, s.y, r, fx, fy, 0.0f);
        }
    }

    colorStops(frameNo, fill, tween, exps);

    //multiply the current opacity with the fill
    if (opacity < 255) {
        const Fill::ColorStop* colorStops;
        auto cnt = fill->colorStops(&colorStops);
        for (uint32_t i = 0; i < cnt; ++i) {
            const_cast<Fill::ColorStop*>(&colorStops[i])->a = MULTIPLY(colorStops[i].a, opacity);
        }
    }

    return fill;
}


LottieGroup::LottieGroup()
{
    reqFragment = false;
    buildDone = false;
    trimpath = false;
    visible = false;
    allowMerge = true;
}


LottieProperty* LottieGroup::property(uint16_t ix)
{
    ARRAY_FOREACH(p, children) {
        auto child = static_cast<LottieObject*>(*p);
        if (auto property = child->property(ix)) return property;
    }

    return nullptr;
}


void LottieGroup::prepare(LottieObject::Type type)
{
    LottieObject::type = type;

    if (children.count == 0) return;

    size_t strokeCnt = 0;
    size_t fillCnt = 0;

    ARRAY_REVERSE_FOREACH(c, children) {
        auto child = static_cast<LottieObject*>(*c);

        if (child->type == LottieObject::Type::Trimpath) trimpath = true;

        /* Figure out if this group is a simple path drawing.
           In that case, the rendering context can be sharable with the parent's. */
        if (allowMerge && !child->mergeable()) allowMerge = false;

        //Figure out this group has visible contents
        switch (child->type) {
            case LottieObject::Group: {
                visible |= static_cast<LottieGroup*>(child)->visible;
                break;
            }
            case LottieObject::Rect:
            case LottieObject::Ellipse:
            case LottieObject::Path:
            case LottieObject::Polystar:
            case LottieObject::Image:
            case LottieObject::Text: {
                visible = true;
                break;
            }
            default: break;
        }

        if (reqFragment) continue;

        /* Figure out if the rendering context should be fragmented.
           Multiple stroking or grouping with a stroking would occur this.
           This fragment resolves the overlapped stroke outlines. */
        if (child->type == LottieObject::Group && !child->mergeable()) {
            if (strokeCnt > 0 || fillCnt > 0) reqFragment = true;
        } else if (child->type == LottieObject::SolidStroke || child->type == LottieObject::GradientStroke) {
            if (strokeCnt > 0) reqFragment = true;
            else ++strokeCnt;
        } else if (child->type == LottieObject::SolidFill || child->type == LottieObject::GradientFill) {
            if (fillCnt > 0) reqFragment = true;
            else ++fillCnt;
        }
    }

    //Reverse the drawing order if this group has a trimpath.
    if (!trimpath) return;

    for (uint32_t i = 0; i < children.count - 1; ) {
        auto child2 = children[i + 1];
        if (!child2->mergeable() || child2->type == LottieObject::Transform) {
            i += 2;
            continue;
        }
        auto child = children[i];
        if (!child->mergeable() || child->type == LottieObject::Transform) {
            i++;
            continue;
        }
        children[i] = child2;
        children[i + 1] = child;
        i++;
    }
}

LottieLayer::LottieLayer()
{
    autoOrient = false;
    matteSrc = false;
    effect = false;
}

LottieLayer::~LottieLayer()
{
    //No need to free assets children because the Composition owns them.
    if (rid) children.clear();

    ARRAY_FOREACH(p, masks) delete(*p);
    ARRAY_FOREACH(p, effects) delete(*p);

    delete(transform);
    delete(audioCtrl);
    tvg::free(name);
}


LottieProperty* LottieLayer::property(uint16_t ix)
{
    if (transform) {
        if (auto property = transform->property(ix)) return property;
    }

    return LottieGroup::property(ix);
}


void LottieLayer::prepare(RGB32* color)
{
    /* if layer is hidden, only useful data is its transform matrix.
       so force it to be a Null Layer and release all resource. */
    if (hidden) {
        type = LottieLayer::Null;
        ARRAY_FOREACH(p, children) delete(*p);
        children.reset();
        return;
    }

    // prepare the viewport clipper or a solid fill in advance if it is a layer type.
    if (type == LottieLayer::Precomp || (color && type == LottieLayer::Solid)) {
        auto obj = Shape::gen();
        obj->appendRect(0.0f, 0.0f, w, h);
        obj->ref();
        if (color && type == LottieLayer::Solid) obj->fill(color->r, color->g, color->b);
        statical.pooler.push(obj);
    }

    LottieGroup::prepare(LottieObject::Layer);
}


float LottieLayer::remap(LottieComposition* comp, float frameNo, LottieExpressions* exp)
{
    if (timeRemap.frames || timeRemap.value >= 0.0f) {
        return comp->frameAtTime(timeRemap(frameNo, exp));
    }
    return (frameNo - startFrame) / timeStretch;
}

LottieComposition::~LottieComposition()
{
    if (!initiated && root) Paint::rel(root->scene);

    delete(root);
    tvg::free(version);
    tvg::free(name);

    ARRAY_FOREACH(p, interpolators) {
        tvg::free((*p)->key);
        tvg::free(*p);
    }

    ARRAY_FOREACH(p, assets) delete(*p);
    ARRAY_FOREACH(p, fonts) delete(*p);
    ARRAY_FOREACH(p, slots) delete(*p);
    ARRAY_FOREACH(p, markers) delete(*p);
}
