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

#include "tvgStr.h"
#include "tvgCompressor.h"
#include "tvgLottieModel.h"
#include "tvgLottieParser.h"
#include "tvgLottieExpressions.h"


/************************************************************************/
/* Internal Class Implementation                                        */
/************************************************************************/

#define KEY_AS(name) !strcmp(key, name)


static unsigned long _int2str(int num)
{
    char str[20];
    snprintf(str, 20, "%d", num);
    return djb2Encode(str);
}


void LottieParser::getExpression(char* code, LottieComposition* comp, LottieLayer* layer, LottieObject* object, LottieProperty* property)
{
    if (!comp->expressions) comp->expressions = true;

    auto inst = new LottieExpression;
    inst->code = code;
    inst->comp = comp;
    inst->layer = layer;
    inst->object = object;
    inst->property = property;

    property->exp = inst;
}


LottieEffect* LottieParser::getEffect(int type)
{
    switch (type) {
        case LottieEffect::Custom: return new LottieFxCustom;
        case LottieEffect::Tint: return new LottieFxTint;
        case LottieEffect::Fill: return new LottieFxFill;
        case LottieEffect::Stroke: return new LottieFxStroke;
        case LottieEffect::Tritone: return new LottieFxTritone;
        case LottieEffect::DropShadow: return new LottieFxDropShadow;
        case LottieEffect::GaussianBlur: return new LottieFxGaussianBlur;
        default: return nullptr;
    }
}


MaskMethod LottieParser::getMaskMethod(bool inversed)
{
    auto mode = getString();
    if (!mode) return MaskMethod::None;

    switch (mode[0]) {
        case 'a': {
            if (inversed) return MaskMethod::InvAlpha;
            else return MaskMethod::Add;
        }
        case 's': {
            if (inversed) return MaskMethod::Intersect;
            return MaskMethod::Subtract;
        }
        case 'i': {
            if (inversed) return MaskMethod::Difference;
            return MaskMethod::Intersect;
        }
        case 'f': {
            if (inversed) return MaskMethod::Intersect;
            return MaskMethod::Difference;
        }
        case 'l': return MaskMethod::Lighten;
        case 'd': return MaskMethod::Darken;
        default: return MaskMethod::None;
    }
}


RGB32 LottieParser::getColor(const char *str)
{
    RGB32 color = {0, 0, 0};

    if (!str) return color;

    auto len = strlen(str);

    // some resource has empty color string, return a default color for those cases.
    if (len != 7 || str[0] != '#') return color;

    char tmp[3] = {'\0', '\0', '\0'};
    tmp[0] = str[1];
    tmp[1] = str[2];
    color.r = uint8_t(strtol(tmp, nullptr, 16));

    tmp[0] = str[3];
    tmp[1] = str[4];
    color.g = uint8_t(strtol(tmp, nullptr, 16));

    tmp[0] = str[5];
    tmp[1] = str[6];
    color.b = uint8_t(strtol(tmp, nullptr, 16));

    return color;
}


bool LottieParser::getValue(TextDocument& doc)
{
    enterObject();
    while (auto key = nextObjectKey()) {
        if (KEY_AS("s")) doc.size = getFloat() * 0.01f;
        else if (KEY_AS("f")) doc.name = getStringCopy();
        else if (KEY_AS("t")) doc.text = getStringCopy();
        else if (KEY_AS("j"))
        {
            auto val = getInt();
            if (val == 1) doc.justify = 1.0f;        //right align
            else if (val == 2) doc.justify = 0.5f;   //center align
        }
        else if (KEY_AS("ca")) doc.caps = getInt();
        else if (KEY_AS("tr")) doc.tracking = getFloat() * 0.1f;
        else if (KEY_AS("lh")) doc.height = getFloat();
        else if (KEY_AS("ls")) doc.shift = getFloat();
        else if (KEY_AS("fc")) getValue(doc.color);
        else if (KEY_AS("ps")) getValue(doc.bbox.pos);
        else if (KEY_AS("sz")) getValue(doc.bbox.size);
        else if (KEY_AS("sc")) getValue(doc.stroke.color);
        else if (KEY_AS("sw")) doc.stroke.width = getFloat();
        else if (KEY_AS("of")) doc.stroke.below = !getBool();
        else skip();
    }
    return false;
}


bool LottieParser::getValue(PathSet& path)
{
    Array<Point> outs, ins, pts;
    bool closed = false;

    /* The shape object could be wrapped by a array
       if its part of the keyframe object */
    auto arrayWrapper = (peekType() == kArrayType) ? true : false;
    if (arrayWrapper) enterArray();

    enterObject();
    while (auto key = nextObjectKey()) {
        if (KEY_AS("i")) getValue(ins);
        else if (KEY_AS("o")) getValue(outs);
        else if (KEY_AS("v")) getValue(pts);
        else if (KEY_AS("c")) closed = getBool();
        else skip();
    }

    //exit properly from the array
    if (arrayWrapper) nextArrayValue();

    //valid path data?
    if (ins.empty() || outs.empty() || pts.empty()) return false;
    if (ins.count != outs.count || outs.count != pts.count) return false;

    //convert path
    auto out = outs.begin();
    auto in = ins.begin();
    auto pt = pts.begin();

    //Store manipulated results
    RenderPath swap;

    //Reuse the buffers
    swap.pts.data = path.pts;
    swap.pts.reserved = path.ptsCnt;
    swap.cmds.data = path.cmds;
    swap.cmds.reserved = path.cmdsCnt;

    size_t extra = closed ? 3 : 0;
    swap.pts.reserve(pts.count * 3 + 1 + extra);
    swap.cmds.reserve(pts.count + 2);

    swap.moveTo(*pt);

    for (++pt, ++out, ++in; pt < pts.end(); ++pt, ++out, ++in) {
        swap.cubicTo(*(pt - 1) + *(out - 1), *pt + *in, *pt);
    }

    if (closed) {
        swap.cubicTo(pts.last() + outs.last(), pts.first() + ins.first(), pts.first());
        swap.close();
    }

    path.pts = swap.pts.data;
    path.cmds = swap.cmds.data;
    path.ptsCnt = swap.pts.count;
    path.cmdsCnt = swap.cmds.count;

    swap.dismiss();

    return false;
}


bool LottieParser::getValue(ColorStop& color)
{
    if (peekType() == kArrayType) {
        enterArray();
        if (!nextArrayValue()) return true;
    }

    if (!color.input) color.input = new Array<float>(static_cast<LottieGradient*>(context.parent)->colorStops.count * 6);
    else color.input->clear();

    do color.input->push(getFloat());
    while (nextArrayValue());

    return true;
}


bool LottieParser::getValue(Array<Point>& pts)
{
    Point pt{};
    enterArray();
    while (nextArrayValue()) {
        enterArray();
        getValue(pt);
        pts.push(pt);
    }
    return false;
}


bool LottieParser::getValue(int8_t& val)
{
    if (peekType() == kArrayType) {
        enterArray();
        if (nextArrayValue()) val = getInt();
        //discard rest
        while (nextArrayValue()) getInt();
    } else {
        val = (int8_t) getFloat();
    }
    return false;
}


bool LottieParser::getValue(uint8_t& val)
{
    if (peekType() == kArrayType) {
        enterArray();
        if (nextArrayValue()) val = (uint8_t)(getFloat() * 2.55f);
        //discard rest
        while (nextArrayValue()) getFloat();
    } else {
        val = (uint8_t)(getFloat() * 2.55f);
    }
    return false;
}


bool LottieParser::getValue(float& val)
{
    if (peekType() == kArrayType) {
        enterArray();
        if (nextArrayValue()) val = getFloat();
        //discard rest
        while (nextArrayValue()) getFloat();
    } else {
        val = getFloat();
    }
    return false;
}


bool LottieParser::getValue(Point& pt)
{
    if (peekType() == kNullType) return false;
    if (peekType() == kArrayType) {
        enterArray();
        if (!nextArrayValue()) return false;
    }

    pt.x = getFloat();
    pt.y = getFloat();

    while (nextArrayValue()) getFloat();  //drop

    return true;
}

bool LottieParser::getValue(Point3& pt)
{
    if (peekType() == kNullType) return false;
    if (peekType() == kArrayType) {
        enterArray();
        if (!nextArrayValue()) return false;
    }

    pt.x = getFloat();
    pt.y = getFloat();
    pt.z = getFloat();

    while (nextArrayValue()) getFloat();  // drop

    return true;
}

bool LottieParser::getValue(RGB32& color)
{
    if (peekType() == kArrayType) {
        enterArray();
        if (!nextArrayValue()) return false;
    }

    color.r = REMAP255(getFloat());
    color.g = REMAP255(getFloat());
    color.b = REMAP255(getFloat());

    while (nextArrayValue()) getFloat(); //drop

    //TODO: color filter?

    return true;
}


void LottieParser::getInterpolatorPoint(Point& pt)
{
    enterObject();
    while (auto key = nextObjectKey()) {
        if (KEY_AS("x")) getValue(pt.x);
        else if (KEY_AS("y")) getValue(pt.y);
    }
}


template<typename T>
void LottieParser::parseSlotProperty(T& prop)
{
    while (auto key = nextObjectKey()) {
        if (KEY_AS("p")) parseProperty(prop);
        else skip();
    }
}


template<typename T>
bool LottieParser::parseTangent(const char *key, LottieVectorFrame<T>& value)
{
    if (KEY_AS("ti") && getValue(value.inTangent)) ;
    else if (KEY_AS("to") && getValue(value.outTangent)) ;       
    else return false;

    value.hasTangent = true;
    return true;
}


template<typename T>
bool LottieParser::parseTangent(const char *key, LottieScalarFrame<T>& value)
{
    return false;
}


LottieInterpolator* LottieParser::getInterpolator(const char* key, Point& in, Point& out)
{
    char buf[20];

    if (!key) {
        snprintf(buf, sizeof(buf), "%.2f_%.2f_%.2f_%.2f", (double)in.x, (double)in.y, (double)out.x, (double)out.y);
        key = buf;
    }

    LottieInterpolator* interpolator = nullptr;

    //get a cached interpolator if it has any.
    ARRAY_FOREACH(p, comp->interpolators) {
        if (!strncmp((*p)->key, key, sizeof(buf))) interpolator = *p;
    }

    //new interpolator
    if (!interpolator) {
        interpolator = tvg::malloc<LottieInterpolator>(sizeof(LottieInterpolator));
        interpolator->set(key, in, out);
        comp->interpolators.push(interpolator);
    }

    return interpolator;
}


template<typename T>
void LottieParser::parseKeyFrame(T& prop)
{
    Point inTangent, outTangent;
    const char* interpolatorKey = nullptr;
    auto& frame = prop.newFrame();
    auto interpolator = false;

    enterObject();

    while (auto key = nextObjectKey()) {
        if (KEY_AS("i")) {
            interpolator = true;
            getInterpolatorPoint(inTangent);
        } else if (KEY_AS("o")) {
            getInterpolatorPoint(outTangent);
        } else if (KEY_AS("n")) {
            if (peekType() == kStringType) {
                interpolatorKey = getString();
            } else {
                enterArray();
                while (nextArrayValue()) {
                    if (!interpolatorKey) interpolatorKey = getString();
                    else skip();
                }
            }
        } else if (KEY_AS("t")) {
            frame.no = getFloat();
        } else if (KEY_AS("s")) {
            getValue(frame.value);
        } else if (KEY_AS("e")) {
            //current end frame and the next start frame is duplicated,
            //We propagate the end value to the next frame to avoid having duplicated values.
            auto& frame2 = prop.nextFrame();
            getValue(frame2.value);
        } else if (parseTangent(key, frame)) {
            continue;
        } else if (KEY_AS("h")) {
            frame.hold = getInt();
        } else skip();
    }

    if (interpolator) {
        frame.interpolator = getInterpolator(interpolatorKey, inTangent, outTangent);
    }
}

template<typename T>
void LottieParser::parsePropertyInternal(T& prop)
{
    //single value property
    if (peekType() == kNumberType) {
        getValue(prop.value);
    //multi value property
    } else {
        enterArray();
        while (nextArrayValue()) {
            if (peekType() == kObjectType) parseKeyFrame(prop);  //keyframes value
            else if (getValue(prop.value)) break; //multi value property with no keyframes
        }
        prop.prepare();
    }
}


void LottieParser::registerSlot(LottieObject* obj, const char* sid, LottieProperty& prop)
{
    auto val = djb2Encode(sid);

    //append object if the slot already exists.
    ARRAY_FOREACH(p, comp->slots) {
        if ((*p)->sid != val) continue;
        (*p)->pairs.push({obj});
        prop.sid = val;
        return;
    }
    comp->slots.push(new LottieSlot(context.layer, context.parent, val, obj, prop.type));
    prop.sid = val;
}


template<typename T>
void LottieParser::parseProperty(T& prop, LottieObject* obj)
{
    enterObject();
    while (auto key = nextObjectKey()) {
        if (KEY_AS("k")) parsePropertyInternal(prop);
        else if (parseCommon(obj, prop, key)) continue;
        else skip();
    }
}


bool LottieParser::parseCommon(LottieObject* obj, const char* key)
{
    if (KEY_AS("nm")) {
        obj->id = djb2Encode(getString());
    } else if (KEY_AS("hd")) {
        obj->hidden = getBool();
    } else return false;
    return true;
}


bool LottieParser::parseCommon(LottieObject* obj, LottieProperty& prop, const char* key)
{
    if (KEY_AS("ix")) {
        prop.ix = getInt();
    } else if (KEY_AS("x") && expressions) {
        getExpression(getStringCopy(), comp, context.layer, context.parent, &prop);
    } else if (KEY_AS("sid")) {
        registerSlot(obj, getString(), prop);
    } else return false;
    return true;
}


bool LottieParser::parseDirection(LottieShape* shape, const char* key)
{
    if (KEY_AS("d")) {
        if (getInt() == 3) shape->clockwise = false;       //default is true
        return true;
    }
    return false;
}


LottieRect* LottieParser::parseRect()
{
    auto rect = new LottieRect;

    context.parent = rect;

    while (auto key = nextObjectKey()) {
        if (parseCommon(rect, key)) continue;
        else if (KEY_AS("s")) parseProperty(rect->size);
        else if (KEY_AS("p")) parseProperty(rect->position);
        else if (KEY_AS("r")) parseProperty(rect->radius);
        else if (parseDirection(rect, key)) continue;
        else skip();
    }
    return rect;
}


LottieEllipse* LottieParser::parseEllipse()
{
    auto ellipse = new LottieEllipse;

    context.parent = ellipse;

    while (auto key = nextObjectKey()) {
        if (parseCommon(ellipse, key)) continue;
        else if (KEY_AS("p")) parseProperty(ellipse->position);
        else if (KEY_AS("s")) parseProperty(ellipse->size);
        else if (parseDirection(ellipse, key)) continue;
        else skip();
    }
    return ellipse;
}

LottieTransform* LottieParser::parseTransform(bool ddd)
{
    auto transform = new LottieTransform;

    context.parent = transform;

    if (ddd) {
        transform->ddd = new LottieTransform::Dimension3;
        TVGLOG("LOTTIE", "3D transform(ddd) is not totally compatible.");
    }

    while (auto key = nextObjectKey()) {
        if (parseCommon(transform, key)) continue;
        else if (KEY_AS("p"))
        {
            enterObject();
            while (auto key = nextObjectKey()) {
                if (KEY_AS("k")) parsePropertyInternal(transform->position);
                else if (KEY_AS("x")) //must be prior to parseCommon()
                {
                    //check separateCoord to figure out whether "x(expression)" / "x(coord)"
                    if (peekType() == kStringType) {
                        if (expressions) getExpression(getStringCopy(), comp, context.layer, context.parent, &transform->position);
                        else skip();
                    } else parseProperty(transform->separateCoord()->x);
                }
                else if (KEY_AS("y")) parseProperty(transform->separateCoord()->y);
                else if (parseCommon(transform, transform->position, key)) continue;
                else skip();
            }
        }
        else if (KEY_AS("a")) parseProperty(transform->anchor);
        else if (KEY_AS("s")) parseProperty(transform->scale, transform);
        else if (KEY_AS("r")) parseProperty(transform->rotation, transform);
        else if (KEY_AS("o")) parseProperty(transform->opacity, transform);
        else if (transform->ddd && KEY_AS("rx")) parseProperty(transform->ddd->rx);
        else if (transform->ddd && KEY_AS("ry")) parseProperty(transform->ddd->ry);
        else if (transform->ddd && KEY_AS("rz")) parseProperty(transform->rotation);
        else if (transform->ddd && KEY_AS("or")) parseProperty(transform->ddd->orient);
        else if (KEY_AS("sk")) parseProperty(transform->skewAngle, transform);
        else if (KEY_AS("sa")) parseProperty(transform->skewAxis, transform);
        else skip();
    }
    return transform;
}


LottieSolidFill* LottieParser::parseSolidFill()
{
    auto fill = new LottieSolidFill;

    context.parent = fill;

    while (auto key = nextObjectKey()) {
        if (parseCommon(fill, key)) continue;
        else if (KEY_AS("c")) parseProperty(fill->color, fill);
        else if (KEY_AS("o")) parseProperty(fill->opacity, fill);
        else if (KEY_AS("fillEnabled")) fill->hidden |= !getBool();
        else if (KEY_AS("r")) fill->rule = (getInt() == 1) ? FillRule::NonZero : FillRule::EvenOdd;
        else skip();
    }
    return fill;
}


void LottieParser::parseStrokeDash(LottieStroke* stroke)
{
    enterArray();
    while (nextArrayValue()) {
        enterObject();
        const char* style = nullptr;
        while (auto key = nextObjectKey()) {
            if (KEY_AS("n")) style = getString();
            else if (KEY_AS("v")) {
                if (style && !strcmp("o", style)) parseProperty(stroke->dashOffset());
                else parseProperty(stroke->dashValue());
            } else skip();
        }
    }
}


LottieSolidStroke* LottieParser::parseSolidStroke()
{
    auto stroke = new LottieSolidStroke;

    context.parent = stroke;

    while (auto key = nextObjectKey()) {
        if (parseCommon(stroke, key)) continue;
        else if (KEY_AS("c")) parseProperty(stroke->color, stroke);
        else if (KEY_AS("o")) parseProperty(stroke->opacity, stroke);
        else if (KEY_AS("w")) parseProperty(stroke->width, stroke);
        else if (KEY_AS("lc")) stroke->cap = (StrokeCap) (getInt() - 1);
        else if (KEY_AS("lj")) stroke->join = (StrokeJoin) (getInt() - 1);
        else if (KEY_AS("ml")) stroke->miterLimit = getFloat();
        else if (KEY_AS("fillEnabled")) stroke->hidden |= !getBool();
        else if (KEY_AS("d")) parseStrokeDash(stroke);
        else skip();
    }
    return stroke;
}


void LottieParser::getPathSet(LottiePath* obj, LottiePathSet& path)
{
    enterObject();
    while (auto key = nextObjectKey()) {
        if (KEY_AS("k"))
        {
            if (peekType() == kArrayType) {
                enterArray();
                while (nextArrayValue()) parseKeyFrame(path);
            } else {
                getValue(path.value);
            }
        }
        else if (parseCommon(obj, path, key)) continue;
        else skip();
    }
}


LottiePath* LottieParser::parsePath()
{
    auto path = new LottiePath;

    while (auto key = nextObjectKey()) {
        if (parseCommon(path, key)) continue;
        else if (KEY_AS("ks")) getPathSet(path, path->pathset);
        else if (parseDirection(path, key)) continue;
        else skip();
    }
    return path;
}


LottiePolyStar* LottieParser::parsePolyStar()
{
    auto star = new LottiePolyStar;

    context.parent = star;

    while (auto key = nextObjectKey()) {
        if (parseCommon(star, key)) continue;
        else if (KEY_AS("p")) parseProperty(star->position);
        else if (KEY_AS("pt")) parseProperty(star->ptsCnt);
        else if (KEY_AS("ir")) parseProperty(star->innerRadius);
        else if (KEY_AS("is")) parseProperty(star->innerRoundness);
        else if (KEY_AS("or")) parseProperty(star->outerRadius);
        else if (KEY_AS("os")) parseProperty(star->outerRoundness);
        else if (KEY_AS("r")) parseProperty(star->rotation);
        else if (KEY_AS("sy")) star->type = (LottiePolyStar::Type) getInt();
        else if (parseDirection(star, key)) continue;
        else skip();
    }
    return star;
}


LottieRoundedCorner* LottieParser::parseRoundedCorner()
{
    auto corner = new LottieRoundedCorner;

    context.parent = corner;

    while (auto key = nextObjectKey()) {
        if (parseCommon(corner, key)) continue;
        else if (KEY_AS("r")) parseProperty(corner->radius);
        else skip();
    }
    return corner;
}


void LottieParser::parseColorStop(LottieGradient* gradient)
{
    enterObject();
    while (auto key = nextObjectKey()) {
        if (KEY_AS("p")) gradient->colorStops.count = getInt();
        else if (KEY_AS("k")) parseProperty(gradient->colorStops, gradient);
        else if (parseCommon(gradient, gradient->colorStops, key)) continue;
        else skip();
    }
}


void LottieParser::parseGradient(LottieGradient* gradient, const char* key)
{
    if (KEY_AS("t")) gradient->id = getInt();
    else if (KEY_AS("o")) parseProperty(gradient->opacity, gradient);
    else if (KEY_AS("g")) parseColorStop(gradient);
    else if (KEY_AS("s")) parseProperty(gradient->start, gradient);
    else if (KEY_AS("e")) parseProperty(gradient->end, gradient);
    else if (KEY_AS("h")) parseProperty(gradient->height, gradient);
    else if (KEY_AS("a")) parseProperty(gradient->angle, gradient);
    else skip();
}


LottieGradientFill* LottieParser::parseGradientFill()
{
    auto fill = new LottieGradientFill;

    context.parent = fill;

    while (auto key = nextObjectKey()) {
        if (parseCommon(fill, key)) continue;
        else if (KEY_AS("r")) fill->rule = (getInt() == 1) ? FillRule::NonZero : FillRule::EvenOdd;
        else parseGradient(fill, key);
    }

    fill->prepare();

    return fill;
}


LottieGradientStroke* LottieParser::parseGradientStroke()
{
    auto stroke = new LottieGradientStroke;

    context.parent = stroke;

    while (auto key = nextObjectKey()) {
        if (parseCommon(stroke, key)) continue;
        else if (KEY_AS("lc")) stroke->cap = (StrokeCap) (getInt() - 1);
        else if (KEY_AS("lj")) stroke->join = (StrokeJoin) (getInt() - 1);
        else if (KEY_AS("ml")) stroke->miterLimit = getFloat();
        else if (KEY_AS("w")) parseProperty(stroke->width);
        else if (KEY_AS("d")) parseStrokeDash(stroke);
        else parseGradient(stroke, key);
    }
    stroke->prepare();

    return stroke;
}


LottieTrimpath* LottieParser::parseTrimpath()
{
    auto trim = new LottieTrimpath;

    context.parent = trim;

    while (auto key = nextObjectKey()) {
        if (parseCommon(trim, key)) continue;
        else if (KEY_AS("s")) parseProperty(trim->start);
        else if (KEY_AS("e")) parseProperty(trim->end);
        else if (KEY_AS("o")) parseProperty(trim->offset);
        else if (KEY_AS("m")) trim->type = static_cast<LottieTrimpath::Type>(getInt());
        else skip();
    }
    return trim;
}


LottieRepeater* LottieParser::parseRepeater()
{
    auto repeater = new LottieRepeater;

    context.parent = repeater;

    while (auto key = nextObjectKey()) {
        if (parseCommon(repeater, key)) continue;
        else if (KEY_AS("c")) parseProperty(repeater->copies);
        else if (KEY_AS("o")) parseProperty(repeater->offset);
        else if (KEY_AS("m")) repeater->inorder = getInt() == 2;
        else if (KEY_AS("tr"))
        {
            enterObject();
            while (auto key = nextObjectKey()) {
                if (KEY_AS("a")) parseProperty(repeater->anchor);
                else if (KEY_AS("p")) parseProperty(repeater->position);
                else if (KEY_AS("r")) parseProperty(repeater->rotation);
                else if (KEY_AS("s")) parseProperty(repeater->scale);
                else if (KEY_AS("so")) parseProperty(repeater->startOpacity);
                else if (KEY_AS("eo")) parseProperty(repeater->endOpacity);
                else skip();
            }
        }
        else skip();
    }
    return repeater;
}


LottieOffsetPath* LottieParser::parseOffsetPath()
{
    auto offsetPath = new LottieOffsetPath;

    context.parent = offsetPath;

    while (auto key = nextObjectKey()) {
        if (parseCommon(offsetPath, key)) continue;
        else if (KEY_AS("a")) parseProperty(offsetPath->offset);
        else if (KEY_AS("lj")) offsetPath->join = (StrokeJoin) (getInt() - 1);
        else if (KEY_AS("ml")) parseProperty(offsetPath->miterLimit);
        else skip();
    }
    return offsetPath;
}

LottiePuckerBloat* LottieParser::parsePuckerBloat()
{
    auto puckerBloat = new LottiePuckerBloat;

    context.parent = puckerBloat;

    while (auto key = nextObjectKey()) {
        if (parseCommon(puckerBloat, key)) continue;
        else if (KEY_AS("a")) parseProperty(puckerBloat->amount);
        else skip();
    }
    return puckerBloat;
}

LottieZigZag* LottieParser::parseZigZag()
{
    auto zigzag = new LottieZigZag;

    context.parent = zigzag;

    while (auto key = nextObjectKey()) {
        if (parseCommon(zigzag, key)) continue;
        else if (KEY_AS("s")) parseProperty(zigzag->amplitude);
        else if (KEY_AS("r")) parseProperty(zigzag->frequency);
        else if (KEY_AS("pt")) parseProperty(zigzag->point);
        else skip();
    }
    return zigzag;
}

LottieObject* LottieParser::parseObject(const char* type)
{
    if (!strcmp(type, "gr")) return parseGroup();
    else if (!strcmp(type, "rc")) return parseRect();
    else if (!strcmp(type, "el")) return parseEllipse();
    else if (!strcmp(type, "tr")) return parseTransform();
    else if (!strcmp(type, "fl")) return parseSolidFill();
    else if (!strcmp(type, "st")) return parseSolidStroke();
    else if (!strcmp(type, "sh")) return parsePath();
    else if (!strcmp(type, "sr")) return parsePolyStar();
    else if (!strcmp(type, "rd")) return parseRoundedCorner();
    else if (!strcmp(type, "gf")) return parseGradientFill();
    else if (!strcmp(type, "gs")) return parseGradientStroke();
    else if (!strcmp(type, "tm")) return parseTrimpath();
    else if (!strcmp(type, "rp")) return parseRepeater();
    else if (!strcmp(type, "pb")) return parsePuckerBloat();
    else if (!strcmp(type, "op")) return parseOffsetPath();
    else if (!strcmp(type, "zz")) return parseZigZag();
    else if (!strcmp(type, "mm")) TVGLOG("LOTTIE", "MergePath(mm) is not supported yet");
    else if (!strcmp(type, "tw")) TVGLOG("LOTTIE", "Twist(tw) is not supported yet");
    return nullptr;
}


//capture the type name if the upcoming type is irregularly addressed after the actual properties.
char* LottieParser::captureType()
{
    if (!isPrimitive() || !strcmp(val.GetString(), "ty")) return nullptr;

    auto level = 0;
    for (auto p = getPos(); *p != '\0'; ++p) {
        if (*p == '{') level++;
        else if (*p == '}') {
            if (--level < 0) break;
        } else if (level == 0) {
            if (!strncmp(p, "\"ty\"", 4)) {
                p += 4;
                while (*p != '\0' && (isspace(*p) || *p == '\n')) ++p;
                if (*p++ != ':') return nullptr;
                while (*p != '\0' && (isspace(*p) || *p == '\n')) ++p;
                if (*p++ != '\"') return nullptr;
                const char* start = p;
                while (*p != '\0' && *p != '\"') ++p;
                if (*p == '\"') return tvg::duplicate(start, p - start);
                return nullptr;
            }
        }
    }
    return nullptr;
}


void LottieParser::parseObject(Array<LottieObject*>& parent)
{
    enterObject();

    auto type = captureType();
    auto freeType = false;

    if (!type) {
        if (auto key = nextObjectKey()) {
            if (KEY_AS("ty")) type = (char*)getString();
            else skip();
        }
    } else freeType = true;

    if (type) {
        if (auto child = parseObject(type)) {
            if (child->hidden) delete(child);
            else parent.push(child);
        }
    }

    if (freeType) tvg::free(type);

    while(nextObjectKey()) skip();
}


void LottieParser::parseVolume(LottieLayer* layer)
{
    enterObject();
    while (auto key = nextObjectKey()) {
        if (KEY_AS("lv")) parseProperty(layer->audio()->volume);
        else skip();
    }
}


void LottieParser::parseAudio(LottieAudio* audio, const char* data, const char* subPath, bool embedded)
{
    auto dlen = strlen(data);
    if (dlen == 0) return;

    if (embedded && !strncmp(data, "data:audio/", 11)) {
        auto mime = data + 11;
        auto semi = strstr(mime, ";");
        if (!semi) return;
        audio->mimeType = duplicate(mime, semi - mime);
        auto b64 = strstr(semi, ",");
        if (!b64) return;
        ++b64;
        audio->size = b64Decode(b64, dlen - (b64 - data), &audio->data);
    } else if (!strncmp(data, "https://", 8) || !strncmp(data, "http://", 7)) {
        audio->path = duplicate(data);
    } else {
        auto subPathLen = subPath ? strlen(subPath) : 0;
        auto len = strlen(dirName) + subPathLen + dlen + 2;
        audio->path = tvg::malloc<char>(len);
        snprintf(audio->path, len, "%s/%s%s", dirName, subPath ? subPath : "", data);
    }
}


void LottieParser::parseImage(LottieImage* image, const char* data, const char* subPath, bool embedded, float width, float height)
{
    auto dlen = strlen(data);
    if (dlen == 0) return;

    auto external = false;

    //embedded image resource. should start with "data:"
    //header look like "data:image/png;base64," so need to skip till ','.
    if (embedded && !strncmp(data, "data:image/", 11)) {
        //figure out the mimetype
        auto mimeType = data + 11;
        auto needle = strstr(mimeType, ";");
        if (!needle) return;
        image->bitmap.mimeType = duplicate(mimeType, needle - mimeType);
        //b64 data
        auto b64Data = strstr(needle, ",");
        if (!b64Data) return;
        ++b64Data;
        size_t length = dlen - (b64Data - data);
        image->bitmap.size = b64Decode(b64Data, length, &image->bitmap.data);
    //remote image resource (https:// or http://)
    } else if (!strncmp(data, "https://", 8) || !strncmp(data, "http://", 7)) {
        image->bitmap.path = duplicate(data);
    //external image resource
    } else {
        auto subPathLen = subPath ? strlen(subPath) : 0;
        auto len = strlen(dirName) + subPathLen + dlen + 2;
        image->bitmap.path = tvg::malloc<char>(len);
        snprintf(image->bitmap.path, len, "%s/%s%s", dirName, subPath ? subPath : "", data);
        external = true;
    }
    image->bitmap.width = width;
    image->bitmap.height = height;
    image->prepare(external);
}


LottieObject* LottieParser::parseAsset()
{
    enterObject();

    LottieObject* obj = nullptr;
    unsigned long id = 0;

    //Used for Image Asset
    const char* sid = nullptr;
    const char* data = nullptr;
    const char* subPath = nullptr;
    float width = 0.0f;
    float height = 0.0f;
    auto embedded = false;

    while (auto key = nextObjectKey()) {
        if (KEY_AS("id"))
        {
            if (peekType() == kStringType) {
                id = djb2Encode(getString());
            } else {
                id = _int2str(getInt());
            }
        }
        else if (KEY_AS("layers")) obj = parseLayers(comp->root);
        else if (KEY_AS("u")) subPath = getString();
        else if (KEY_AS("p")) data = getString();
        else if (KEY_AS("w")) width = getFloat();
        else if (KEY_AS("h")) height = getFloat();
        else if (KEY_AS("e")) embedded = getInt();
        else if (KEY_AS("sid")) sid = getString();
        else skip();
    }
    if (data) {
        if (!strncmp(data, "data:image/", 11) || width != 0.0f || height != 0.0f) {
            auto asset = new LottieImage;
            parseImage(asset, data, subPath, embedded, width, height);
            if (sid) registerSlot(asset, sid, asset->bitmap);
            obj = asset;
        } else if (!strncmp(data, "data:audio/", 11) || !embedded) {
            auto asset = new LottieAudio;
            parseAudio(asset, data, subPath, embedded);
            obj = asset;
        } else TVGLOG("LOTTIE", "Unexpected data type");
    }
    if (obj) obj->id = id;
    return obj;
}


void LottieParser::parseFontData(LottieFont* font, const char* data)
{
    static const char* TTF = "ttf";
    static const char* OTF = "otf";

    if (!data) return;

    //handle base64 font data
    if (!strncmp(data, "data:font/", sizeof("data:font/") - 1)) {
        data += sizeof("data:font/") - 1;
        if (!strncasecmp(data, TTF, 3)) {
            font->mime = strdup(TTF);
            data += 3;
        } else if (!strncasecmp(data, OTF, 3)) {
            font->mime = strdup(OTF);
            data += 3;
        } else {
            TVGLOG("LOTTIE", "Not support the current font type!");
            return;
        }
        auto b64Data = strstr(data, ",");
        if (!b64Data) return;
        data = b64Data + 1;
        font->size = b64Decode(data, strlen(data), &font->b64src);
    //external font resource
    } else {
        font->path = duplicate(data);
    }
}


LottieFont* LottieParser::parseFont()
{
    enterObject();

    auto font = new LottieFont;

    while (auto key = nextObjectKey()) {
        if (KEY_AS("fName")) font->name = getStringCopy();
        else if (KEY_AS("fFamily")) font->family = getStringCopy();
        else if (KEY_AS("fStyle")) font->style = getStringCopy();
        else if (KEY_AS("fPath")) parseFontData(font, getString());
        else if (KEY_AS("ascent")) font->ascent = getFloat();
        else if (KEY_AS("origin")) font->origin = (LottieFont::Origin) getInt();
        else skip();
    }

    font->prepare();
    return font;
}


void LottieParser::parseAssets()
{
    enterArray();
    while (nextArrayValue()) {
        auto asset = parseAsset();
        if (asset) comp->assets.push(asset);
        else TVGERR("LOTTIE", "Invalid Asset!");
    }
}

LottieMarker* LottieParser::parseMarker()
{
    enterObject();
    
    auto marker = new LottieMarker;
    
    while (auto key = nextObjectKey()) {
        if (KEY_AS("cm")) marker->name = getStringCopy();
        else if (KEY_AS("tm")) marker->time = getFloat();
        else if (KEY_AS("dr")) marker->duration = getFloat();
        else skip();
    }
    
    return marker;
}

void LottieParser::parseMarkers()
{
    enterArray();
    while (nextArrayValue()) {
        comp->markers.push(parseMarker());
    }
}

void LottieParser::parseChars(Array<LottieGlyph*>& glyphs)
{
    enterArray();
    while (nextArrayValue()) {
        enterObject();
        //a new glyph
        auto glyph = new LottieGlyph;
        while (auto key = nextObjectKey()) {
            if (KEY_AS("ch")) glyph->code = getStringCopy();
            else if (KEY_AS("size")) glyph->size = static_cast<uint16_t>(getFloat());
            else if (KEY_AS("style")) glyph->style = getStringCopy();
            else if (KEY_AS("w")) glyph->width = getFloat();
            else if (KEY_AS("fFamily")) glyph->family = getStringCopy();
            else if (KEY_AS("data"))
            {   //glyph shapes
                enterObject();
                while (auto key = nextObjectKey()) {
                    if (KEY_AS("shapes")) parseShapes(glyph->children);
                }
            } else skip();
        }
        if (glyph->prepare()) glyphs.push(glyph);
    }
}

void LottieParser::parseFonts()
{
    enterObject();
    while (auto key = nextObjectKey()) {
        if (KEY_AS("list")) {
            enterArray();
            while (nextArrayValue()) {
                comp->fonts.push(parseFont());
            }
        } else skip();
    }
}


LottieObject* LottieParser::parseGroup()
{
    auto group = new LottieGroup;

    while (auto key = nextObjectKey()) {
        if (parseCommon(group, key)) continue;
        else if (KEY_AS("it")) {
            enterArray();
            while (nextArrayValue()) parseObject(group->children);
        } else if (KEY_AS("bm")) {
            group->blendMethod = (BlendMethod) getInt();
        } else skip();
    }
    group->prepare();

    return group;
}


void LottieParser::parseTimeRemap(LottieLayer* layer)
{
    parseProperty(layer->timeRemap);
}


void LottieParser::parseShapes(Array<LottieObject*>& parent)
{
    enterArray();
    while (nextArrayValue()) parseObject(parent);
}


void LottieParser::parseTextAlignmentOption(LottieText* text)
{
    enterObject();
    while (auto key = nextObjectKey()) {
        if (KEY_AS("g")) text->alignOp.group = (LottieText::AlignOption::Group) getInt();
        else if (KEY_AS("a")) parseProperty(text->alignOp.anchor);
        else skip();
    }
}


void LottieParser::parseTextRange(LottieText* text)
{
    enterArray();
    while (nextArrayValue()) {
        enterObject();

        auto selector = new LottieTextRange;
        context.parent = selector;

        while (auto key = nextObjectKey()) {
            if (KEY_AS("s")) { // text range selector
                enterObject();
                while (auto key = nextObjectKey()) {
                    if (KEY_AS("t")) selector->expressible = (bool) getInt();
                    else if (KEY_AS("xe"))
                    {
                        parseProperty(selector->maxEase);
                        selector->interpolator = tvg::malloc<LottieInterpolator>(sizeof(LottieInterpolator));
                    }
                    else if (KEY_AS("ne")) parseProperty(selector->minEase);
                    else if (KEY_AS("a")) parseProperty(selector->maxAmount);
                    else if (KEY_AS("b")) selector->based = (LottieTextRange::Based) getInt();
                    else if (KEY_AS("rn")) selector->random = getInt() ? rand() : 0;
                    else if (KEY_AS("sh")) selector->shape = (LottieTextRange::Shape) getInt();
                    else if (KEY_AS("o")) parseProperty(selector->offset);
                    else if (KEY_AS("r")) selector->rangeUnit = (LottieTextRange::Unit) getInt();
                    else if (KEY_AS("sm")) parseProperty(selector->smoothness);
                    else if (KEY_AS("s")) parseProperty(selector->start);
                    else if (KEY_AS("e")) parseProperty(selector->end);
                    else skip();
                }
            } else if (KEY_AS("a")) { // text style
                enterObject();
                while (auto key = nextObjectKey()) {
                    if (KEY_AS("t")) parseProperty(selector->style.letterSpace, selector);
                    else if (KEY_AS("ls")) parseProperty(selector->style.lineSpace, selector);
                    else if (KEY_AS("fc"))
                    {
                        parseProperty(selector->style.fillColor, selector);
                        selector->style.flags.fillColor = true;
                    }
                    else if (KEY_AS("fo")) parseProperty(selector->style.fillOpacity, selector);
                    else if (KEY_AS("sw"))
                    {
                        parseProperty(selector->style.strokeWidth, selector);
                        selector->style.flags.strokeWidth = true;
                    }
                    else if (KEY_AS("sc"))
                    {
                        parseProperty(selector->style.strokeColor, selector);
                        selector->style.flags.strokeColor = true;
                    }
                    else if (KEY_AS("so")) parseProperty(selector->style.strokeOpacity, selector);
                    else if (KEY_AS("o")) parseProperty(selector->style.opacity, selector);
                    else if (KEY_AS("p")) parseProperty(selector->style.position, selector);
                    else if (KEY_AS("s")) parseProperty(selector->style.scale, selector);
                    else if (KEY_AS("r")) parseProperty(selector->style.rotation, selector);
                    else skip();
                }
            } else skip();
        }

        text->ranges.push(selector);
    }
}


void LottieParser::parseTextFollowPath(LottieText* text)
{
    enterObject();
    auto key = nextObjectKey();
    if (!key) return;
    if (!text->follow) text->follow = new LottieTextFollowPath;
    do {
        if (KEY_AS("m")) text->follow->maskIdx = getInt();
        else if (KEY_AS("f")) parseProperty(text->follow->firstMargin);
        else skip();
    } while ((key = nextObjectKey()));
}


void LottieParser::parseText(Array<LottieObject*>& parent)
{
    enterObject();

    auto text = new LottieText;

    while (auto key = nextObjectKey()) {
        if (KEY_AS("d")) parseProperty(text->doc, text);
        else if (KEY_AS("a")) parseTextRange(text);
        else if (KEY_AS("m")) parseTextAlignmentOption(text);
        else if (KEY_AS("p")) parseTextFollowPath(text);
        else skip();
    }
    parent.push(text);
}


void LottieParser::getLayerSize(float& val)
{
    if (val == 0.0f) {
        val = getFloat();
    } else {
        //layer might have both w(width) & sw(solid color width)
        //override one if the a new size is smaller.
        auto w = getFloat();
        if (w < val) val = w;
    }
}

LottieMask* LottieParser::parseMask()
{
    auto mask = new LottieMask;
    enterObject();
    while (auto key = nextObjectKey()) {
        if (KEY_AS("inv")) mask->inverse = getBool();
        else if (KEY_AS("mode")) mask->method = getMaskMethod(mask->inverse);
        else if (KEY_AS("pt")) getPathSet(nullptr, mask->pathset);
        else if (KEY_AS("o")) parseProperty(mask->opacity);
        else if (KEY_AS("x")) parseProperty(mask->expand);
        else skip();
    }
    return mask;
}


void LottieParser::parseMasks(LottieLayer* layer)
{
    enterArray();
    while (nextArrayValue()) {
        if (auto mask = parseMask()) {
            layer->masks.push(mask);
        }
    }
}


bool LottieParser::parseEffect(LottieEffect* effect, void(LottieParser::*func)(LottieEffect*, int))
{
    //custom effect expects dynamic property allocations
    auto custom = (effect->type == LottieEffect::Custom) ? true : false;
    enterArray();
    int idx = 0;
    while (nextArrayValue()) {
        enterObject();
        LottieFxCustom::Property* property = nullptr;
        while (auto key = nextObjectKey()) {
            if (custom && KEY_AS("ty")) property = static_cast<LottieFxCustom*>(effect)->property(getInt());
            else if (KEY_AS("v") && (!custom || property)) {
                if (peekType() == kObjectType) {
                    enterObject();
                    while (auto key = nextObjectKey()) {
                        if (KEY_AS("k")) (this->*func)(effect, idx++);
                        else skip();
                    }
                } else (this->*func)(effect, idx++);
            }
            else if (property && KEY_AS("nm")) property->nm = djb2Encode(getString());
            else if (property && KEY_AS("mn")) property->mn = djb2Encode(getString());
            else skip();
        }
    }
    return true;
}


void LottieParser::parseCustom(LottieEffect* effect, int idx)
{
    if ((uint32_t)idx >= static_cast<LottieFxCustom*>(effect)->props.count) {
        TVGERR("LOTTIE", "Parsing error in Custom effect!");
        skip();
        return;
    }

    auto prop = static_cast<LottieFxCustom*>(effect)->props[idx].property;

    switch (prop->type) {
        case LottieProperty::Type::Integer: {
            parsePropertyInternal(*static_cast<LottieInteger*>(prop)); break;
        }
        case LottieProperty::Type::Float: {
            parsePropertyInternal(*static_cast<LottieFloat*>(prop)); break;
        }
        case LottieProperty::Type::Vector: {
            parsePropertyInternal(*static_cast<LottieVector*>(prop)); break;
        }
        case LottieProperty::Type::Color: {
            parsePropertyInternal(*static_cast<LottieColor*>(prop)); break;
        }
        default: {
            TVGLOG("LOTTIE", "Missing Property Type? = %d", (int) prop->type);
            skip();
        }
    }
}


void LottieParser::parseTint(LottieEffect* effect, int idx)
{
    auto tint = static_cast<LottieFxTint*>(effect);

    if (idx == 0) parsePropertyInternal(tint->black);
    else if (idx == 1) parsePropertyInternal(tint->white);
    else if (idx == 2) parsePropertyInternal(tint->intensity);
    else skip();
}


void LottieParser::parseTritone(LottieEffect* effect, int idx)
{
    auto tritone = static_cast<LottieFxTritone*>(effect);

    if (idx == 0) parsePropertyInternal(tritone->bright);
    else if (idx == 1) parsePropertyInternal(tritone->midtone);
    else if (idx == 2) parsePropertyInternal(tritone->dark);
    else if (idx == 3) parsePropertyInternal(tritone->blend);
    else skip();
}


void LottieParser::parseFill(LottieEffect* effect, int idx)
{
    auto fill = static_cast<LottieFxFill*>(effect);

    if (idx == 2) parsePropertyInternal(fill->color);
    else if (idx == 6) parsePropertyInternal(fill->opacity);
    else skip();
}


void LottieParser::parseGaussianBlur(LottieEffect* effect, int idx)
{
    auto blur = static_cast<LottieFxGaussianBlur*>(effect);

    if (idx == 0) parsePropertyInternal(blur->blurness);
    else if (idx == 1) parsePropertyInternal(blur->direction);
    else if (idx == 2) parsePropertyInternal(blur->wrap);
    else skip();
}


void LottieParser::parseDropShadow(LottieEffect* effect, int idx)
{
    auto shadow = static_cast<LottieFxDropShadow*>(effect);

    if (idx == 0) parsePropertyInternal(shadow->color);
    else if (idx == 1) parsePropertyInternal(shadow->opacity);
    else if (idx == 2) parsePropertyInternal(shadow->angle);
    else if (idx == 3) parsePropertyInternal(shadow->distance);
    else if (idx == 4) parsePropertyInternal(shadow->blurness);
    else skip();
}


void LottieParser::parseStroke(LottieEffect* effect, int idx)
{
    auto stroke = static_cast<LottieFxStroke*>(effect);

    if (idx == 0) parsePropertyInternal(stroke->mask);
    else if (idx == 1) parsePropertyInternal(stroke->allMask);
    else if (idx == 3) parsePropertyInternal(stroke->color);
    else if (idx == 4) parsePropertyInternal(stroke->size);
    else if (idx == 6) parsePropertyInternal(stroke->opacity);
    else if (idx == 7) parsePropertyInternal(stroke->begin);
    else if (idx == 8) parsePropertyInternal(stroke->end);
    else skip();
}


bool LottieParser::parseEffect(LottieEffect* effect)
{
    switch (effect->type) {
        case LottieEffect::Custom: return parseEffect(effect, &LottieParser::parseCustom);
        case LottieEffect::Tint: return parseEffect(effect, &LottieParser::parseTint);
        case LottieEffect::Fill: return parseEffect(effect, &LottieParser::parseFill);
        case LottieEffect::Stroke: return parseEffect(effect, &LottieParser::parseStroke);
        case LottieEffect::Tritone: return parseEffect(effect, &LottieParser::parseTritone);
        case LottieEffect::DropShadow: return parseEffect(effect, &LottieParser::parseDropShadow);
        case LottieEffect::GaussianBlur: return parseEffect(effect, &LottieParser::parseGaussianBlur);
        default: return false;
    }
}


void LottieParser::parseEffects(LottieLayer* layer)
{
    enterArray();
    while (nextArrayValue()) {
        LottieEffect* effect = nullptr;
        auto invalid = true;
        enterObject();
        while (auto key = nextObjectKey()) {
            //type must be prioritized.
            if (KEY_AS("ty"))
            {
                effect = getEffect(getInt());
                if (!effect) break;
                else invalid = false;
            }
            else if (effect && KEY_AS("nm")) effect->nm = djb2Encode(getString());
            else if (effect && KEY_AS("mn")) effect->mn = djb2Encode(getString());
            else if (effect && KEY_AS("ix")) effect->ix = getInt();
            else if (effect && KEY_AS("en")) effect->enable = getInt();
            else if (effect && KEY_AS("ef")) parseEffect(effect);
            else skip();
        }
        //TODO: remove when all effects were guaranteed.
        if (invalid) {
            TVGLOG("LOTTIE", "Not supported Layer Effect = %d", effect ? (int)effect->type : -1);
            while (nextObjectKey()) skip();
        } else layer->effects.push(effect);
    }
}


LottieLayer* LottieParser::parseLayer(LottieLayer* precomp)
{
    auto layer = new LottieLayer;

    layer->comp = precomp;
    context.layer = layer;

    auto ddd = false;
    RGB32 color;

    enterObject();

    while (auto key = nextObjectKey()) {
        if (KEY_AS("nm"))
        {
            layer->name = getStringCopy();
            layer->id = djb2Encode(layer->name);
        }
        else if (KEY_AS("ddd")) ddd = getInt();  //3d layer
        else if (KEY_AS("ind")) layer->ix = getInt();
        else if (KEY_AS("ty")) layer->type = (LottieLayer::Type) getInt();
        else if (KEY_AS("sr")) layer->timeStretch = getFloat();
        else if (KEY_AS("ks"))
        {
            enterObject();
            layer->transform = parseTransform(ddd);
        }
        else if (KEY_AS("ao")) layer->autoOrient = getInt();
        else if (KEY_AS("shapes")) parseShapes(layer->children);
        else if (KEY_AS("ip")) layer->inFrame = getFloat();
        else if (KEY_AS("op")) layer->outFrame = getFloat();
        else if (KEY_AS("st")) layer->startFrame = getFloat();
        else if (KEY_AS("bm")) layer->blendMethod = (BlendMethod) getInt();
        else if (KEY_AS("parent")) layer->pix = getInt();
        else if (KEY_AS("tm")) parseTimeRemap(layer);
        else if (KEY_AS("w") || KEY_AS("sw")) getLayerSize(layer->w);
        else if (KEY_AS("h") || KEY_AS("sh")) getLayerSize(layer->h);
        else if (KEY_AS("sc")) color = getColor(getString());
        else if (KEY_AS("tt")) layer->matteType = (MaskMethod) getInt();
        else if (KEY_AS("tp")) layer->mix = getInt();
        else if (KEY_AS("masksProperties")) parseMasks(layer);
        else if (KEY_AS("hd")) layer->hidden = getBool();
        else if (KEY_AS("refId")) layer->rid = djb2Encode(getString());
        else if (KEY_AS("td")) layer->matteSrc = getInt();      //used for matte layer
        else if (KEY_AS("t")) parseText(layer->children);
        else if (KEY_AS("ef")) parseEffects(layer);
        else if (KEY_AS("au")) parseVolume(layer);
        else skip();
    }

    layer->prepare(&color);

    layer->effect = !layer->effects.empty();
    precomp->effect |= layer->effect;

    return layer;
}


LottieLayer* LottieParser::parseLayers(LottieLayer* root)
{
    auto precomp = new LottieLayer;

    precomp->type = LottieLayer::Precomp;
    precomp->comp = root;

    enterArray();
    while (nextArrayValue()) {
        precomp->children.push(parseLayer(precomp));
    }

    precomp->prepare();
    return precomp;
}


void LottieParser::postProcess(Array<LottieGlyph*>& glyphs)
{
    //aggregate font characters
    ARRAY_FOREACH(g, glyphs) {
        auto glyph = *g;
        ARRAY_FOREACH(f, comp->fonts) {
            auto font = *f;
            if (!strcmp(font->family, glyph->family) && !strcmp(font->style, glyph->style)) {
                font->chars.push(glyph);
                free(glyph->family);
                free(glyph->style);
                glyph->family = glyph->style = nullptr;
                glyph = nullptr;
                break;
            }
        }
        delete(glyph);
    }
}


/************************************************************************/
/* External Class Implementation                                        */
/************************************************************************/

const char* LottieParser::sid(bool first)
{
    if (first) {
        //verify json
        if (!parseNext()) return nullptr;
        enterObject();
    }
    return nextObjectKey();
}


LottieProperty* LottieParser::parse(LottieSlot* slot)
{
    enterObject();

    LottieProperty* prop = nullptr;
    context = {slot->context.layer, slot->context.parent};

    switch (slot->type) {
        case LottieProperty::Type::Float: {
            prop = new LottieFloat;
            parseSlotProperty(*static_cast<LottieFloat*>(prop));
            break;
        }
        case LottieProperty::Type::Scalar: {
            prop = new LottieScalar;
            parseSlotProperty(*static_cast<LottieScalar*>(prop));
            break;
        }
        case LottieProperty::Type::Vector: {
            prop = new LottieVector;
            parseSlotProperty(*static_cast<LottieVector*>(prop));
            break;
        }
        case LottieProperty::Type::Opacity: {
            prop = new LottieOpacity;
            parseSlotProperty(*static_cast<LottieOpacity*>(prop));
            break;
        }
        case LottieProperty::Type::Color: {
            prop = new LottieColor;
            parseSlotProperty(*static_cast<LottieColor*>(prop));
            break;
        }
        case LottieProperty::Type::ColorStop: {
            auto obj = new LottieGradient;
            while (auto key = nextObjectKey()) {
                if (KEY_AS("p")) parseColorStop(obj);
                else skip();
            }
            obj->prepare();
            prop = new LottieColorStop(obj->colorStops);
            delete(obj);
            break;
        }
        case LottieProperty::Type::TextDoc: {
            prop = new LottieTextDoc;
            parseSlotProperty(*static_cast<LottieTextDoc*>(prop));
            break;
        }
        case LottieProperty::Type::Image: {
            LottieObject* obj = nullptr;
            while (auto key = nextObjectKey()) {
                if (KEY_AS("p")) obj = parseAsset();
                else skip();
            }
            if (!obj) return nullptr;
            prop = new LottieBitmap(static_cast<LottieImage*>(obj)->bitmap);
            delete(obj);
            break;
        }
        default: break;
    }
    prop->sid = slot->sid;
    return prop;
}


void LottieParser::captureSlots(const char* key)
{
    tvg::free(slots);

    // TODO: Replace with immediate parsing, once the slot spec is confirmed by the LAC

    auto begin = getPos();
    auto end = getPos();
    auto depth = 1;
    auto invalid = true;

    //get slots string
    while (++end) {
        if (*end == '}') {
            --depth;
            if (depth == 0) {
                invalid = false;
                break;
            }
        } else if (*end == '{') {
            ++depth;
        }
    }

    if (invalid) {
        TVGERR("LOTTIE", "Invalid Slots!");
        skip();
        return;
    }

    //composite '{' + slots + '}'
    auto len = (end - begin + 2);
    slots = tvg::malloc<char>(sizeof(char) * len + 1);
    slots[0] = '{';
    memcpy(slots + 1, begin, len);
    slots[len] = '\0';

    skip();
}


bool LottieParser::parse()
{
    //verify json.
    if (!parseNext()) return false;

    enterObject();

    if (comp) delete(comp);
    comp = new LottieComposition;

    Array<LottieGlyph*> glyphs;

    auto startFrame = 0.0f;
    auto endFrame = 0.0f;

    while (auto key = nextObjectKey()) {
        if (KEY_AS("v")) comp->version = getStringCopy();
        else if (KEY_AS("fr")) comp->frameRate = getFloat();
        else if (KEY_AS("ip")) startFrame = getFloat();
        else if (KEY_AS("op")) endFrame = getFloat();
        else if (KEY_AS("w")) comp->w = getFloat();
        else if (KEY_AS("h")) comp->h = getFloat();
        else if (KEY_AS("nm")) comp->name = getStringCopy();
        else if (KEY_AS("assets")) parseAssets();
        else if (KEY_AS("layers")) comp->root = parseLayers(comp->root);
        else if (KEY_AS("fonts")) parseFonts();
        else if (KEY_AS("chars")) parseChars(glyphs);
        else if (KEY_AS("markers")) parseMarkers();
        else if (KEY_AS("slots")) captureSlots(key);
        else skip();
    }

    if (Invalid() || !comp->root) {
        delete(comp);
        return false;
    }

    comp->root->inFrame = startFrame;
    comp->root->outFrame = endFrame;

    postProcess(glyphs);

    return true;
}
