/*
 * Copyright (c) 2024 - 2026 ThorVG project. All rights reserved.

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
#include "tvgCompressor.h"
#include "tvgLottieModel.h"
#include "tvgLottieExpressions.h"
#include "tvgLock.h"

#ifdef THORVG_LOTTIE_EXPRESSIONS_SUPPORT

#ifdef THORVG_THREAD_SUPPORT
    #include "jerryscript-port.h"
#endif

/************************************************************************/
/* Internal Class Implementation                                        */
/************************************************************************/

struct ExpContent
{
    LottieExpression* exp;
    union {
        LottieObject* obj;
        LottieEffect* effect;
        LottieProperty* property;
        void *data;
    };
    float frameNo;
    size_t refCnt;
};

static jerry_value_t _content(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt);

//reserved expressions specifiers
static const char* EXP_NAME = "name";
static const char* EXP_CONTENT = "content";
static const char* EXP_WIDTH = "width";
static const char* EXP_HEIGHT = "height";
static const char* EXP_CYCLE = "cycle";
static const char* EXP_PINGPONG = "pingpong";
static const char* EXP_OFFSET = "offset";
static const char* EXP_CONTINUE = "continue";
static const char* EXP_TIME = "time";
static const char* EXP_VALUE = "value";
static const char* EXP_INDEX = "index";
static const char* EXP_EFFECT= "effect";
static const char* EXP_SIZE = "size";
static const char* EXP_POSITION = "position";

static LottieExpressions* _exps = nullptr;
static uint32_t _refCnt = 0;
static Key _lockKey;

/**
 * external magic strings for the per-frame hot path (buildProperty, buildLayer, buildTransform, bm_rt).
 * magic pool: sorted by length, then lexicographically. packed in a single static blob,
 *             because 'jerry_register_magic_strings' does not copy the string data.
 * lengths/count are generated from a single list at compile time; pointers are built once at init.
 */
#define TVG_LOTTIE_MAGIC_STRING_LIST(X) \
    X("key") \
    X("comp") X("name") X("time") \
    X("index") X("layer") X("scale") X("speed") X("value") X("width") \
    X("$bm_rt") X("effect") X("height") X("parent") X("toComp") X("wiggle") \
    X("content") X("enabled") X("inPoint") X("numKeys") X("opacity") \
    X("duration") X("hasAudio") X("hasVideo") X("outPoint") X("position") X("rotation") X("thisComp") X("velocity") \
    X("hasParent") X("numLayers") X("startTime") X("thisLayer") X("timeRemap") X("transform") \
    X("nearestKey") \
    X("anchorPoint") X("audioActive") X("speedAtTime") X("valueAtTime") \
    X("thisProperty") \
    X("frameDuration") X("propertyIndex") \
    X("velocityAtTime")

#define TVG_LOTTIE_MAGIC_POOL_ENTRY(str) str
static constexpr const char _magicPool[] =
    TVG_LOTTIE_MAGIC_STRING_LIST(TVG_LOTTIE_MAGIC_POOL_ENTRY);
#undef TVG_LOTTIE_MAGIC_POOL_ENTRY

#define TVG_LOTTIE_MAGIC_LENGTH_ENTRY(str) (jerry_length_t)(sizeof(str) - 1),
static constexpr jerry_length_t _magicLengths[] = {
    TVG_LOTTIE_MAGIC_STRING_LIST(TVG_LOTTIE_MAGIC_LENGTH_ENTRY)
};
#undef TVG_LOTTIE_MAGIC_LENGTH_ENTRY

#define MAGIC_STRING_COUNT (sizeof(_magicLengths) / sizeof(_magicLengths[0]))
static const jerry_char_t* _magicStrings[MAGIC_STRING_COUNT];

#undef TVG_LOTTIE_MAGIC_STRING_LIST


static ExpContent* _expcontent(LottieExpression* exp, float frameNo, void* data, size_t refCnt = 1)
{
    auto ret = tvg::malloc<ExpContent>(sizeof(ExpContent));
    ret->exp = exp;
    ret->frameNo = frameNo;
    ret->data = data;
    ret->refCnt = refCnt;
    return ret;
}


static inline ExpContent* _expcontent(ExpContent* data)
{
    ++data->refCnt;
    return data;
}


static float _rand()
{
    return (float)(rand() % 10000001) * 0.0000001f;
}


static jerry_value_t _number(float value)
{
    //OPTIMIZE: used a typed buffer instead of a object hash
    auto obj = jerry_object();
    auto val = jerry_number(value);
    jerry_object_set_index(obj, 0, val);
    jerry_object_set_sz(obj, EXP_VALUE, val);
    jerry_value_free(val);

    return obj;
}


static jerry_value_t _point2d(const Point& pt)
{
    //OPTIMIZE: used a typed buffer instead of a object hash
    auto obj = jerry_object();
    auto v1 = jerry_number(pt.x);
    auto v2 = jerry_number(pt.y);
    jerry_object_set_index(obj, 0, v1);
    jerry_object_set_index(obj, 1, v2);
    jerry_value_free(v1);
    jerry_value_free(v2);
    return obj;
}


static jerry_value_t _color(RGB32 rgb)
{
    //OPTIMIZE: used a typed buffer instead of a object hash
    auto value = jerry_object();
    auto r = jerry_number((float)rgb.r);
    auto g = jerry_number((float)rgb.g);
    auto b = jerry_number((float)rgb.b);
    jerry_object_set_index(value, 0, r);
    jerry_object_set_index(value, 1, g);
    jerry_object_set_index(value, 2, b);
    jerry_value_free(r);
    jerry_value_free(g);
    jerry_value_free(b);
    return value;
}

// return true if the number is 1d otherwise 2d, return false
static bool _number(jerry_value_t obj, Point& out)
{
    if (jerry_value_is_number(obj)) {
        out.x = jerry_value_as_number(obj);
        return true;
    }

    // 1d or 2d object
    auto v1 = jerry_object_get_index(obj, 0);
    auto v2 = jerry_object_get_index(obj, 1);

    out.x = jerry_value_as_number(v1);
    jerry_value_free(v1);

    if (jerry_value_is_undefined(v2)) return true;
    out.y = jerry_value_as_number(v2);
    jerry_value_free(v2);

    return false;
}

// TODO: may need to replace with _number(jerry_value_t obj, Point& out)
static float _number(jerry_value_t obj)
{
    if (jerry_value_is_number(obj)) return jerry_value_as_number(obj);

    auto val = jerry_object_get_index(obj, 0);
    auto ret = jerry_value_as_number(val);
    jerry_value_free(val);
    return ret;
}


static Point _point2d(jerry_value_t obj)
{
    auto v1 = jerry_object_get_index(obj, 0);
    auto v2 = jerry_object_get_index(obj, 1);
    Point pt = {jerry_value_as_number(v1), jerry_value_as_number(v2)};
    jerry_value_free(v1);
    jerry_value_free(v2);
    return pt;
}


static RGB32 _color(jerry_value_t obj)
{
    RGB32 out;
    auto r = jerry_object_get_index(obj, 0);
    auto g = jerry_object_get_index(obj, 1);
    auto b = jerry_object_get_index(obj, 2);
    out = {jerry_value_as_int32(r), jerry_value_as_int32(g), jerry_value_as_int32(b)};
    jerry_value_free(r);
    jerry_value_free(g);
    jerry_value_free(b);
    return out;
}


static void contentFree(void *native_p, struct jerry_object_native_info_t *info_p)
{
    if (--static_cast<ExpContent*>(native_p)->refCnt == 0) {
        tvg::free(native_p);
    }
}


static jerry_object_native_info_t freeCb {contentFree, 0, 0};


static char* _name(jerry_value_t args)
{
    auto arg0 = jerry_value_to_string(args);
    auto len = jerry_string_length(arg0);
    auto name = tvg::malloc<jerry_char_t>(len * sizeof(jerry_char_t) + 1);
    jerry_string_to_buffer(arg0, JERRY_ENCODING_UTF8, name, len);
    name[len] = '\0';
    jerry_value_free(arg0);
    return (char*) name;
}


static unsigned long _idByName(jerry_value_t args)
{
    auto name = _name(args);
    auto id = djb2Encode(name);
    tvg::free(name);
    return id;
}


static jerry_value_t _toComp(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto layer = static_cast<LottieLayer*>(jerry_object_get_native_ptr(info->function, nullptr));
    return _point2d(_point2d(args[0]) * layer->cache.matrix);
}


static jerry_value_t _point(const Point& v)
{
    auto obj = _point2d(v);
    jerry_object_set_sz(obj, EXP_VALUE, obj);
    return obj;
}


static jerry_value_t _buildValue(float frameNo, LottieProperty* property)
{
    switch (property->type) {
        case LottieProperty::Type::Integer: return _number((float)(*static_cast<LottieInteger*>(property))(frameNo));
        case LottieProperty::Type::Float: return _number((*static_cast<LottieFloat*>(property))(frameNo));
        case LottieProperty::Type::Scalar: return _point((*static_cast<LottieScalar*>(property))(frameNo));
        case LottieProperty::Type::Vector: return _point((*static_cast<LottieVector*>(property))(frameNo));
        case LottieProperty::Type::PathSet:
        {
            auto obj = jerry_object();
            jerry_object_set_native_ptr(obj, nullptr, property);
            return obj;
        }
        case LottieProperty::Type::Color: return _color((*static_cast<LottieColor*>(property))(frameNo));
        case LottieProperty::Type::Opacity: return jerry_number((*static_cast<LottieOpacity*>(property))(frameNo) / 2.55f);
        case LottieProperty::Type::TextDoc: {
            const auto& doc = (*static_cast<LottieTextDoc*>(property))(frameNo);
            return doc.text ? jerry_string_sz(doc.text) : jerry_string_sz("");
        }
        default: TVGERR("LOTTIE", "Non supported type for value? = %d", (int) property->type);
    }
    return jerry_undefined();
}


static void _buildTransform(jerry_value_t context, float frameNo, LottieTransform* transform)
{
    if (!transform) return;

    auto obj = jerry_object();
    jerry_object_set_sz(context, "transform", obj);

    auto anchorPoint = _point(transform->anchor(frameNo));
    jerry_object_set_sz(obj, "anchorPoint", anchorPoint);
    jerry_value_free(anchorPoint);

    auto position = _point(transform->position(frameNo));
    jerry_object_set_sz(obj, "position", position);
    jerry_value_free(position);

    auto scale = _point(transform->scale(frameNo));
    jerry_object_set_sz(obj, "scale", scale);
    jerry_value_free(scale);

    auto rotation = jerry_number(transform->rotation(frameNo));
    jerry_object_set_sz(obj, "rotation", rotation);
    jerry_value_free(rotation);

    auto opacity = jerry_number(transform->opacity(frameNo) / 2.55f);
    jerry_object_set_sz(obj, "opacity", opacity);
    jerry_value_free(opacity);

    jerry_value_free(obj);
}


static jerry_value_t _buildGroup(LottieGroup* group, float frameNo)
{
    auto obj = jerry_function_external(_content);

    //attach a transform
    ARRAY_FOREACH(p, group->children) {
        if ((*p)->type == LottieObject::Type::Transform) {
            _buildTransform(obj, frameNo, static_cast<LottieTransform*>(*p));
            break;
        }
    }
    jerry_object_set_native_ptr(obj, &freeCb, _expcontent(nullptr, frameNo, group));
    jerry_object_set_sz(obj, EXP_CONTENT, obj);
    return obj;
}


static jerry_value_t _buildPolystar(LottiePolyStar* polystar, float frameNo)
{
    auto obj = jerry_object();
    auto position = jerry_object();
    jerry_object_set_native_ptr(position, nullptr, &polystar->position);
    jerry_object_set_sz(obj, "position", position);
    jerry_value_free(position);
    auto innerRadius = jerry_number(polystar->innerRadius(frameNo));
    jerry_object_set_sz(obj, "innerRadius", innerRadius);
    jerry_value_free(innerRadius);
    auto outerRadius = jerry_number(polystar->outerRadius(frameNo));
    jerry_object_set_sz(obj, "outerRadius", outerRadius);
    jerry_value_free(outerRadius);
    auto innerRoundness = jerry_number(polystar->innerRoundness(frameNo));
    jerry_object_set_sz(obj, "innerRoundness", innerRoundness);
    jerry_value_free(innerRoundness);
    auto outerRoundness = jerry_number(polystar->outerRoundness(frameNo));
    jerry_object_set_sz(obj, "outerRoundness", outerRoundness);
    jerry_value_free(outerRoundness);
    auto rotation = jerry_number(polystar->rotation(frameNo));
    jerry_object_set_sz(obj, "rotation", rotation);
    jerry_value_free(rotation);
    auto ptsCnt = jerry_number(polystar->ptsCnt(frameNo));
    jerry_object_set_sz(obj, "points", ptsCnt);
    jerry_value_free(ptsCnt);

    return obj;
}


static jerry_value_t _buildRect(LottieRect* rect, float frameNo)
{
    auto obj = jerry_object();
    auto size = _buildValue(frameNo, &rect->size);
    jerry_object_set_sz(obj, EXP_SIZE, size);
    jerry_value_free(size);
    auto position = _buildValue(frameNo, &rect->position);
    jerry_object_set_sz(obj, EXP_POSITION, position);
    jerry_value_free(position);
    auto roundness = jerry_number(rect->radius(frameNo));
    jerry_object_set_sz(obj, "roundness", roundness);
    jerry_value_free(roundness);

    return obj;
}


static jerry_value_t _buildEllipse(LottieEllipse* ellipse, float frameNo)
{
    auto obj = jerry_object();
    auto size = _buildValue(frameNo, &ellipse->size);
    jerry_object_set_sz(obj, EXP_SIZE, size);
    jerry_value_free(size);
    auto position = _buildValue(frameNo, &ellipse->position);
    jerry_object_set_sz(obj, EXP_POSITION, position);
    jerry_value_free(position);

    return obj;
}


static jerry_value_t _buildTrimpath(LottieTrimpath* trimpath, float frameNo)
{
    jerry_value_t obj = jerry_object();
    auto start = jerry_number(trimpath->start(frameNo));
    jerry_object_set_sz(obj, "start", start);
    jerry_value_free(start);
    auto end = jerry_number(trimpath->end(frameNo));
    jerry_object_set_sz(obj, "end", end);
    jerry_value_free(end);
    auto offset = jerry_number(trimpath->offset(frameNo));
    jerry_object_set_sz(obj, EXP_OFFSET, offset);
    jerry_value_free(offset);

    return obj;
}


static jerry_value_t _effectProperty(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto name = _name(args[0]);
    auto property = static_cast<LottieFxCustom*>(data->effect)->property(name);
    tvg::free(name);

    if (!property) return jerry_undefined();
    return _buildValue(data->frameNo, property);
}


static jerry_value_t _effect(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto layer = static_cast<LottieLayer*>(data->obj);
    LottieEffect* effect = nullptr;

    //either name or index
    if (jerry_value_is_string(args[0])) {
        effect = layer->effectById(_idByName(args[0]));
    } else {
        effect = layer->effectByIdx((int16_t)jerry_value_as_int32(args[0]));
    }

    if (!effect) return jerry_undefined();

    //find a effect property
    auto obj = jerry_function_external(_effectProperty);
    jerry_object_set_native_ptr(obj, &freeCb, _expcontent(data->exp, data->frameNo, effect));
    return obj;
}


static void _buildLayer(jerry_value_t context, float frameNo, LottieLayer* layer, LottieLayer* comp, LottieExpression* exp)
{
    auto width = jerry_number(layer->w);
    jerry_object_set_sz(context, EXP_WIDTH, width);
    jerry_value_free(width);

    auto height = jerry_number(layer->h);
    jerry_object_set_sz(context, EXP_HEIGHT, height);
    jerry_value_free(height);

    auto index = jerry_number(layer->ix);
    jerry_object_set_sz(context, EXP_INDEX, index);
    jerry_value_free(index);

    auto parent = jerry_object();
    jerry_object_set_native_ptr(parent, nullptr, layer->parent);
    jerry_object_set_sz(context, "parent", parent);
    jerry_value_free(parent);

    auto hasParent = jerry_boolean(layer->parent ? true : false);
    jerry_object_set_sz(context, "hasParent", hasParent);
    jerry_value_free(hasParent);

    auto inPoint = jerry_number(layer->inFrame / exp->comp->frameRate);
    jerry_object_set_sz(context, "inPoint", inPoint);
    jerry_value_free(inPoint);

    auto outPoint = jerry_number(layer->outFrame / exp->comp->frameRate);
    jerry_object_set_sz(context, "outPoint", outPoint);
    jerry_value_free(outPoint);

    //TODO: Confirm exp->layer->comp->timeAtFrame() ?
    auto startTime = jerry_number(exp->comp->timeAtFrame(layer->startFrame));
    jerry_object_set_sz(context, "startTime", startTime);
    jerry_value_free(startTime);

    auto hasVideo = jerry_boolean(false);
    jerry_object_set_sz(context, "hasVideo", hasVideo);
    jerry_value_free(hasVideo);

    auto hasAudio = jerry_boolean(false);
    jerry_object_set_sz(context, "hasAudio", hasAudio);
    jerry_value_free(hasAudio);

    //active, #current in the animation range?

    auto enabled = jerry_boolean(!layer->hidden);
    jerry_object_set_sz(context, "enabled", enabled);
    jerry_value_free(enabled);

    auto audioActive = jerry_boolean(false);
    jerry_object_set_sz(context, "audioActive", audioActive);
    jerry_value_free(audioActive);

    //sampleImage(point, radius = [.5, .5], postEffect=true, t=time)

    _buildTransform(context, frameNo, layer->transform);

    //audioLevels, #the value of the Audio Levels property of the layer in decibels

    auto timeRemap = jerry_object();
    jerry_object_set_native_ptr(timeRemap, nullptr, &layer->timeRemap);
    jerry_object_set_sz(context, "timeRemap", timeRemap);
    jerry_value_free(timeRemap);

    //marker.key(index)
    //marker.key(name)
    //marker.nearestKey(t)
    //marker.numKeys

    //FIXME: This name conflicts with the function object in _layer(). No idea. Maybe jerryscript issue.
#if 0
    if (layer->name) {
        auto name = jerry_string_sz(layer->name);
        jerry_object_set_sz(context, EXP_NAME, name);
        jerry_value_free(name);
    }
#endif

    auto toComp = jerry_function_external(_toComp);
    jerry_object_set_sz(context, "toComp", toComp);
    jerry_object_set_native_ptr(toComp, nullptr, layer);
    jerry_value_free(toComp);

    //content("name"), #look for the named property from a layer
    auto data = _expcontent(exp, frameNo, layer, 2);

    auto content = jerry_function_external(_content);
    jerry_object_set_sz(context, EXP_CONTENT, content);
    jerry_object_set_native_ptr(content, &freeCb, data);
    jerry_value_free(content);

    auto effect = jerry_function_external(_effect);
    jerry_object_set_sz(context, EXP_EFFECT, effect);
    jerry_object_set_native_ptr(effect, &freeCb, data);
    jerry_value_free(effect);
}


static jerry_value_t _addsub(const jerry_value_t args[], float addsub)
{
    //string + string
    if (jerry_value_is_string(args[0]) || jerry_value_is_string(args[1])) {
        auto a = _name(args[0]);
        auto b = _name(args[1]);
        auto ret = tvg::concat(a, b);
        auto val = jerry_string_sz(ret);
        tvg::free(ret);
        tvg::free(a);
        tvg::free(b);
        return val;
    }

    Point v1{}, v2{};
    auto n1 = _number(args[0], v1);
    auto n2 = _number(args[1], v2);

    //1d + 1d
    if (n1 && n2) return jerry_number(v1.x + (addsub * v2.x));

    //2d + 2d
    if (!n1 && !n2) return _point2d(v1 + (addsub * v2));

    // 2d + 1d?
    if (n1) {
        v2.x = v1.x + (addsub * v2.x);
        return _point2d(v2);
    } else {
        v1.x = v1.x + (addsub * v2.x);
        return _point2d(v1);
    }
}

static jerry_value_t _muldiv(const jerry_value_t arg1, float arg2)
{
    //1d
    Point v1;
    auto n1 = _number(arg1, v1);
    if (n1) return jerry_number(v1.x * arg2);

    //2d
    return _point2d(v1 * arg2);
}


static jerry_value_t _add(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    return _addsub(args, 1.0f);
}


static jerry_value_t _sub(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    return _addsub(args, -1.0f);
}


static jerry_value_t _mul(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    return _muldiv(args[0], _number(args[1]));
}


static jerry_value_t _div(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    return _muldiv(args[0], 1.0f / _number(args[1]));
}


static jerry_value_t _mod(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    return jerry_number(fmod(_number(args[0]), _number(args[1])));
}


static jerry_value_t _interp(float t, const jerry_value_t args[], int argsCnt)
{
    auto tMin = 0.0f;
    auto tMax = 1.0f;
    int idx = 0;

    tMin = _number(args[1]);
    tMax = _number(args[2]);
    idx += 2;

    t = (t - tMin) / (tMax - tMin);
    if (t < 0) t = 0.0f;
    else if (t > 1) t = 1.0f;

    //2d
    if (jerry_value_is_object(args[idx + 1]) && jerry_value_is_object(args[idx + 2])) {
        return _point2d(tvg::lerp(_point2d(args[idx + 1]), _point2d(args[idx + 2]), t));
    }

    //1d
    return jerry_number(tvg::lerp(_number(args[idx + 1]), _number(args[idx + 2]), t));
}


static jerry_value_t _linear(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto t = _number(args[0]);
    return _interp(t, args, jerry_value_as_uint32(argsCnt));
}


static jerry_value_t _ease(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto t = _number(args[0]);
    t = (t < 0.5f) ? (4 * t * t * t) : (1.0f - powf(-2.0f * t + 2.0f, 3) * 0.5f);
    return _interp(t, args, jerry_value_as_uint32(argsCnt));
}



static jerry_value_t _easeIn(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto t = _number(args[0]);
    t = t * t * t;
    return _interp(t, args, jerry_value_as_uint32(argsCnt));
}


static jerry_value_t _easeOut(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto t = _number(args[0]);
    t = 1.0f - powf(1.0f - t, 3);
    return _interp(t, args, jerry_value_as_uint32(argsCnt));
}


static jerry_value_t _clamp(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto num = _number(args[0]);
    auto limit1 = _number(args[1]);
    auto limit2 = _number(args[2]);

    //clamping
    if (num < limit1) num = limit1;
    if (num > limit2) num = limit2;

    return jerry_number(num);
}


static jerry_value_t _dot(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    return jerry_number(tvg::dot(_point2d(args[0]), _point2d(args[1])));
}


static jerry_value_t _cross(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    return jerry_number(tvg::cross(_point2d(args[0]), _point2d(args[1])));
}


static jerry_value_t _normalize(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto pt = _point2d(args[0]);
    return _point2d(pt / tvg::length(pt));
}


static jerry_value_t _length(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    return jerry_number(tvg::length(_point2d(args[0])));
}


static jerry_value_t _random(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    return jerry_number(_rand());
}


static jerry_value_t _deg2rad(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    return jerry_number(deg2rad(_number(args[0])));
}


static jerry_value_t _rad2deg(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    return jerry_number(rad2deg(_number(args[0])));
}


static jerry_value_t _content(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto group = static_cast<LottieGroup*>(data->obj);
    auto target = group->content(_idByName(args[0]));
    if (!target) return jerry_undefined();

    //find the a path property(sh) in the group layer?
    switch (target->type) {
        case LottieObject::Group: return _buildGroup(static_cast<LottieGroup*>(target), data->frameNo);
        case LottieObject::Path: {
            auto obj = jerry_object();
            jerry_object_set_native_ptr(obj, nullptr, &static_cast<LottiePath*>(target)->pathset);
            jerry_object_set_sz(obj, "path", obj);
            return obj;
        }
        case LottieObject::Rect: return _buildRect(static_cast<LottieRect*>(target), data->frameNo);
        case LottieObject::Ellipse: return _buildEllipse(static_cast<LottieEllipse*>(target), data->frameNo);
        case LottieObject::Polystar: return _buildPolystar(static_cast<LottiePolyStar*>(target), data->frameNo);
        case LottieObject::Trimpath: return _buildTrimpath(static_cast<LottieTrimpath*>(target), data->frameNo);
        default: break;
    }
    return jerry_undefined();
}


static jerry_value_t _points(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    /* TODO: ThorVG prebuilds the path data for performance.
       It actually need to constructs the Array<Point> for points, inTangents, outTangents and then return here... */
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));

    auto obj = jerry_object();
    jerry_object_set_native_ptr(obj, nullptr, data->property);
    return obj;
}


static jerry_value_t _pointOnPath(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto pathset = static_cast<LottiePathSet*>(data->property);
    auto progress = _number(args[0]);
    auto& out = RenderPath::scratch();
    (*pathset)(data->frameNo, out, nullptr, nullptr);
    return _point2d(out.point(progress));
}

static jerry_value_t _tangentOnPath(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto pathset = static_cast<LottiePathSet*>(data->property);
    auto progress = _number(args[0]);
    auto& out = RenderPath::scratch();
    (*pathset)(data->frameNo, out, nullptr, nullptr);

    auto a = out.point(std::max(0.0f, progress - 0.001f));
    auto b = out.point(std::min(1.0f, progress + 0.001f));
    Point t = {b.x - a.x, b.y - a.y};
    auto len = tvg::length(t);
    if (len > 0.0f) {
        t.x /= len;
        t.y /= len;
    }
    return _point2d(t);
}

static void _buildPath(jerry_value_t context, float frameNo, LottieProperty* pathset)
{
    auto data = _expcontent(nullptr, frameNo, pathset, 3);

    //Trick for fast building path.
    auto points = jerry_function_external(_points);
    jerry_object_set_native_ptr(points, &freeCb, data);
    jerry_object_set_sz(context, "points", points);
    jerry_value_free(points);

    auto pointOnPath = jerry_function_external(_pointOnPath);
    jerry_object_set_native_ptr(pointOnPath, &freeCb, data);
    jerry_object_set_sz(context, "pointOnPath", pointOnPath);
    jerry_value_free(pointOnPath);

    auto tangentOnPath = jerry_function_external(_tangentOnPath);
    jerry_object_set_native_ptr(tangentOnPath, &freeCb, data);
    jerry_object_set_sz(context, "tangentOnPath", tangentOnPath);
    jerry_value_free(tangentOnPath);

    //inTangents
    //outTangents
    //isClosed
}


static jerry_value_t _layerChild(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    jerry_value_t obj = jerry_undefined();

    //find a member by index
    if (jerry_value_is_number(args[0])) {
        auto idx = (uint32_t)jerry_value_as_int32(args[0]) - 1;
        auto children = static_cast<Array<LottieObject*>*>(data->data);
        if (idx < children->count) {
            obj = jerry_function_external(_layerChild);
            jerry_object_set_native_ptr(obj, &freeCb, _expcontent(data->exp, data->frameNo, (*children)[idx]));
        }
    //find a member by name
    } else {
        auto name = _name(args[0]);
        if (name) {
            //for backward compatibility: reserved ADOBE keyword
            if (!strcmp(name, "ADBE Root Vectors Group") || !strcmp(name, "ADBE Vectors Group")) {
                auto group = static_cast<LottieGroup*>(data->obj);
                if (group->type == LottieObject::Type::Group || group->type == LottieObject::Type::Layer) {
                    obj = jerry_function_external(_layerChild);
                    jerry_object_set_native_ptr(obj, &freeCb, _expcontent(data->exp, data->frameNo, &group->children));
                }
            } else if (!strcmp(name, "ADBE Vector Shape")) {
                obj = jerry_object();
                _buildPath(obj, data->frameNo, &static_cast<LottiePath*>(data->obj)->pathset);
            }
            tvg::free(name);
        }
    }
    return obj;
}


static jerry_value_t _layer(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto comp = static_cast<LottieLayer*>(data->obj);

    //either index or name
    auto layer = jerry_value_is_number(args[0]) ? comp->layerByIdx((uint16_t)jerry_value_as_int32(args[0])) : comp->layerById(_idByName(args[0]));
    if (!layer) return jerry_undefined();

    auto obj = jerry_function_external(_layerChild);
    jerry_object_set_native_ptr(obj, &freeCb, _expcontent(data->exp, data->frameNo, layer));
    _buildLayer(obj, data->frameNo, layer, comp, data->exp);

    return obj;
}


static jerry_value_t _nearestKey(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto exp = static_cast<LottieExpression*>(jerry_object_get_native_ptr(info->function, nullptr));
    auto time = _number(args[0]);
    auto frameNo = exp->comp->frameAtTime(time);
    auto index = jerry_number((float)exp->property->nearest(frameNo));

    auto obj = jerry_object();
    jerry_object_set_sz(obj, EXP_INDEX, index);
    jerry_value_free(index);

    return obj;
}


static jerry_value_t _property(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto property = data->obj->property(jerry_value_as_int32(args[0]));
    if (!property) return jerry_undefined();
    return _buildValue(data->frameNo, property);
}


static jerry_value_t _propertyGroup(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto level = jerry_value_as_int32(args[0]);

    //intermediate group
    if (level == 1) {
        auto group = jerry_function_external(_property);
        jerry_object_set_native_ptr(group, &freeCb, _expcontent(data));
        return group;
    }

    TVGLOG("LOTTIE", "propertyGroup(%d)?", level);

    return jerry_undefined();
}


static jerry_value_t _valueAtTime(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto exp = static_cast<LottieExpression*>(jerry_object_get_native_ptr(info->function, nullptr));
    auto time = _number(args[0]);
    auto frameNo = exp->comp->frameAtTime(time);
    return _buildValue(frameNo, exp->property);
}


static jerry_value_t _velocity(Point& prv, Point& cur, float elapsed)
{
    return _point2d({(cur.x - prv.x) / elapsed, (cur.y - prv.y) / elapsed});
}


static jerry_value_t _velocityAtTime(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto exp = static_cast<LottieExpression*>(jerry_object_get_native_ptr(info->function, nullptr));
    auto key = exp->property->nearest(exp->comp->frameAtTime(_number(args[0])));
    auto pframe = exp->property->frameNo(key - 1);
    auto cframe = exp->property->frameNo(key);
    auto elapsed = (cframe - pframe) / (exp->comp->frameRate);

    //compute the velocity
    switch (exp->property->type) {
        case LottieProperty::Type::Float: {
            auto prv = (*static_cast<LottieFloat*>(exp->property))(pframe);
            auto cur = (*static_cast<LottieFloat*>(exp->property))(cframe);
            return jerry_number((cur - prv) / elapsed);
        }
        case LottieProperty::Type::Scalar: {
            auto prv = (*static_cast<LottieScalar*>(exp->property))(pframe);
            auto cur = (*static_cast<LottieScalar*>(exp->property))(cframe);
            return _velocity(prv, cur, elapsed);
        }
        case LottieProperty::Type::Vector: {
            auto prv = (*static_cast<LottieVector*>(exp->property))(pframe);
            auto cur = (*static_cast<LottieVector*>(exp->property))(cframe);
            return _velocity(prv, cur, elapsed);
        }
        default: TVGLOG("LOTTIE", "Non supported type for velocityAtTime?");
    }
    return jerry_undefined();
}


static jerry_value_t _speedAtTime(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto exp = static_cast<LottieExpression*>(jerry_object_get_native_ptr(info->function, nullptr));
    auto key = exp->property->nearest(exp->comp->frameAtTime(_number(args[0])));
    auto pframe = exp->property->frameNo(key - 1);
    auto cframe = exp->property->frameNo(key);

    Point cur, prv;

    //compute the velocity
    switch (exp->property->type) {
        case LottieProperty::Type::Scalar: {
            prv = (*static_cast<LottieScalar*>(exp->property))(pframe);
            cur = (*static_cast<LottieScalar*>(exp->property))(cframe);
            break;
        }
        case LottieProperty::Type::Vector: {
            prv = (*static_cast<LottieVector*>(exp->property))(pframe);
            cur = (*static_cast<LottieVector*>(exp->property))(cframe);
            break;
        }
        default: {
            TVGLOG("LOTTIE", "Non supported type for speedAtTime?");
            return jerry_undefined();
        }
    }

    auto elapsed = (cframe - pframe) / (exp->comp->frameRate);
    auto speed = sqrtf(pow(cur.x - prv.x, 2) + pow(cur.y - prv.y, 2)) / elapsed;
    return jerry_number(speed);
}


static jerry_value_t _wiggle(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto freq = _number(args[0]);
    auto amp = _number(args[1]);
    auto octaves = (argsCnt > 2) ? jerry_value_as_int32(args[2]) : 1;
    auto ampm = (argsCnt > 3) ? _number(args[3]) : 0.5f;
    auto time = (argsCnt > 4) ? _number(args[4]) : data->exp->comp->timeAtFrame(data->frameNo);

    Point result = {0.0f, 0.0f};
    auto property = data->exp->property;

    if (property->type == LottieProperty::Type::Vector) {
        result = (*static_cast<LottieVector*>(property))(data->frameNo);
    } else if (property->type == LottieProperty::Type::Scalar) {
        result = (*static_cast<LottieScalar*>(property))(data->frameNo);
    }

    auto perlin1D = [](float x, int seed) {
        auto x0 = (int)floorf(x);
        auto x1 = x0 + 1;
        auto fx = x - (float)x0;

        // Apply quintic fade curve for smooth interpolation
        // Quintic fade curve for smooth interpolation (6t^5 - 15t^4 + 10t^3)
        // Used in Perlin noise for smoother transitions than linear interpolation
        auto u = fx * fx * fx * (fx * (fx * 6.0f - 15.0f) + 10.0f);

        // Simplified gradient function for 1D Perlin noise
        // Deterministic random generator using glibc's LCG algorithm
        auto gradient1D = [](int64_t seed) -> float {
            seed = (seed * 1103515245 + 12345) & 0x7fffffff;
            return float(static_cast<double>(seed) / 2147483647) < 0.5f ? -1.0f : 1.0f;
        };

        // Calculate dot products (in 1D, this is just multiplication with distance)
        auto d0 = gradient1D(x0 * 100000 + seed) * fx;
        auto d1 = gradient1D(x1 * 100000 + seed) * (fx - 1.0f);

        // Interpolate between the two gradient influences and scale the result by 3.0
        return tvg::lerp(d0, d1, u) * 3.0f;
    };

    for (int o = 0; o < octaves; ++o) {
        auto repeat = time * freq;
        // Factors (1000000, 2000000) to separate X/Y axes to prevent seed collisions across octaves.
        result.x += perlin1D(repeat, (1000000 + o)) * amp;
        result.y += perlin1D(repeat, (2000000 + o)) * amp;
        freq *= 2.0f;
        amp *= ampm;
    }

    return _point2d(result);
}


static jerry_value_t _temporalWiggle(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto freq = _number(args[0]);
    auto amp = _number(args[1]);
    auto octaves = (argsCnt > 2) ? jerry_value_as_int32(args[2]) : 1;
    auto ampm = (argsCnt > 3) ? _number(args[3]) : 5.0f;
    auto time = (argsCnt > 4) ? _number(args[4]) : data->exp->comp->timeAtFrame(data->frameNo);
    auto wiggleTime = time;

    for (int o = 0; o < octaves; ++o) {
        auto repeat = int(time * freq);
        auto frac = (time * freq - float(repeat));
        for (int i = 0; i < repeat; ++i) {
            wiggleTime += (_rand() * 2.0f - 1.0f) * amp * frac;
        }
        freq *= 2.0f;
        amp *= ampm;
    }

    return _buildValue(data->exp->comp->frameAtTime(wiggleTime), data->exp->property);
}


static LottieProperty::Loop _loopCommon(const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto mode = LottieProperty::Loop::InCycle;

    if (argsCnt > 0) {
        auto name = _name(args[0]);
        if (!strcmp(name, EXP_CYCLE)) mode = LottieProperty::Loop::InCycle;
        else if (!strcmp(name, EXP_PINGPONG)) mode = LottieProperty::Loop::InPingPong;
        else if (!strcmp(name, EXP_OFFSET)) mode = LottieProperty::Loop::InOffset;
        else if (!strcmp(name, EXP_CONTINUE)) mode = LottieProperty::Loop::InContinue;
        tvg::free(name);
    }

    if (mode != LottieProperty::Loop::InCycle && mode != LottieProperty::Loop::InPingPong) {
        TVGLOG("LOTTIE", "Not supported loopIn type = %d", (int) mode);
    }
    return mode;
}


#define LOOP_OUT_OFFSET 4

static jerry_value_t _loopOut(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto mode = static_cast<LottieProperty::Loop>((int) _loopCommon(args, argsCnt) + LOOP_OUT_OFFSET);
    auto key = (argsCnt > 1) ? jerry_value_as_int32(args[1]) : 0;
    return _buildValue(data->exp->property->loop(data->frameNo, key, mode, data->exp->layer->outFrame), data->exp->property);
}


static jerry_value_t _loopOutDuration(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto mode = static_cast<LottieProperty::Loop>((int) _loopCommon(args, argsCnt) + LOOP_OUT_OFFSET);
    auto out = (argsCnt > 1) ? data->exp->comp->frameAtTime(_number(args[1])) : FLT_MAX;
    return _buildValue(data->exp->property->loop(data->frameNo, 0, mode, out), data->exp->property);
}


static jerry_value_t _loopIn(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto mode = _loopCommon(args, argsCnt);
    auto key = (argsCnt > 1) ? jerry_value_as_int32(args[1]) : 0;
    return _buildValue(data->exp->property->loop(data->frameNo, key, mode, data->exp->layer->outFrame), data->exp->property);
}


static jerry_value_t _loopInDuration(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto mode = _loopCommon(args, argsCnt);
    auto in = (argsCnt > 1) ? data->exp->comp->frameAtTime(_number(args[1])) : FLT_MAX;
    return _buildValue(data->exp->property->loop(data->frameNo, 0, mode, in), data->exp->property);
}


static jerry_value_t _key(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto exp = static_cast<LottieExpression*>(jerry_object_get_native_ptr(info->function, nullptr));
    auto frameNo = exp->property->frameNo(jerry_value_as_int32(args[0]));
    auto time = jerry_number(exp->comp->timeAtFrame(frameNo));
    auto value = _buildValue(frameNo, exp->property);

    auto obj = jerry_object();
    jerry_object_set_sz(obj, EXP_TIME, time);
    jerry_object_set_sz(obj, EXP_INDEX, args[0]);
    jerry_object_set_sz(obj, EXP_VALUE, value);

    //direct access, key[0], key[1]
    if (exp->property->type == LottieProperty::Type::Float) {
        jerry_object_set_index(obj, 0, value);
    } else if (exp->property->type == LottieProperty::Type::Scalar || exp->property->type == LottieProperty::Type::Vector) {
        auto v0 = jerry_object_get_index(value, 0);
        auto v1 = jerry_object_get_index(value, 1);
        jerry_object_set_index(obj, 0, v0);
        jerry_object_set_index(obj, 1, v1);
        jerry_value_free(v0);
        jerry_value_free(v1);
    }

    jerry_value_free(time);
    jerry_value_free(value);

    return obj;
}


static void _buildProperty(float frameNo, jerry_value_t context, LottieExpression* exp)
{
    auto value = _buildValue(frameNo, exp->property);
    jerry_object_set_sz(context, EXP_VALUE, value);
    jerry_value_free(value);

    //check if functions are already registered. If not (first build), set all functions.
    //otherwise, update native_ptr and update shared data in ExpContent
    auto check = jerry_object_get_sz(context, "valueAtTime");
    auto init = jerry_value_is_undefined(check);
    jerry_value_free(check);

    //set or update a native function bound to exp
    auto setFunction = [&context, init, exp](jerry_external_handler_t handler, const char* key) {
        jerry_value_t obj;
        if (init) {
            obj = jerry_function_external(handler);
            jerry_object_set_sz(context, key, obj);
        } else {
            obj = jerry_object_get_sz(context, key);
        }
        jerry_object_set_native_ptr(obj, nullptr, exp);
        jerry_value_free(obj);
    };

    //set a float Value
    auto setValue = [&context](const char* key, float val) {
        auto obj = jerry_number(val);
        jerry_object_set_sz(context, key, obj);
        jerry_value_free(obj);
    };

    //register a new native function with shared data
    auto addFunction = [&context](jerry_external_handler_t handler, const char* key, ExpContent* data) {
        auto obj = jerry_function_external(handler);
        jerry_object_set_sz(context, key, obj);
        jerry_object_set_native_ptr(obj, &freeCb, data);
        jerry_value_free(obj);
    };

    //update shared ExpContent in-place
    auto updateContent = [&context, exp, frameNo](const char* key, void* target) {
        auto obj = jerry_object_get_sz(context, key);
        auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(obj, &freeCb));
        if (data) {
            data->exp = exp;
            data->frameNo = frameNo;
            data->data = target;
        } else {
            TVGERR("LOTTIE", "ExpContent lost due to function overwrite");
        }
        jerry_value_free(obj);
    };

    setFunction(_valueAtTime,    "valueAtTime");
    setFunction(_velocityAtTime, "velocityAtTime");
    setFunction(_speedAtTime,    "speedAtTime");
    setFunction(_key,            "key");
    //key(markerName)
    setFunction(_nearestKey,     "nearestKey");

    setValue("velocity", 0.0f);
    setValue("speed", 0.0f);
    setValue("propertyIndex", static_cast<float>(exp->property->ix));
    setValue("numKeys", static_cast<float>(exp->property->frameCnt()));

    if (init) {
        {
            auto data =  _expcontent(exp, frameNo, exp->object, 7);
            addFunction(_wiggle,          "wiggle",          data);
            addFunction(_temporalWiggle,  "temporalWiggle",  data);
            addFunction(_propertyGroup,   "propertyGroup",   data);
            addFunction(_loopIn,          "loopIn",          data);
            addFunction(_loopOut,         "loopOut",         data);
            addFunction(_loopInDuration,  "loopInDuration",  data);
            addFunction(_loopOutDuration, "loopOutDuration", data);
        }

        //smooth(width=.2, samples=5, t=time)

        //name

        {
            auto data =  _expcontent(exp, frameNo, exp->layer, 2);
            //content("name"), #look for the named property from a layer
            addFunction(_content, EXP_CONTENT, data);
            addFunction(_effect, EXP_EFFECT, data);
        }
    } else {
        // since they share the same ExpContent, updating one of them is enough.
        // update _wiggle, _temporalWiggle, _propertyGroup, _loopIn, _loopOut, _loopInDuration, _loopOutDuration
        updateContent("wiggle",    exp->object);

        // update _content, _effect since they share the same ExpContent
        updateContent(EXP_CONTENT, exp->layer);
    }

    //expansions per types
    if (exp->property->type == LottieProperty::Type::PathSet) _buildPath(context, frameNo, exp->property);
}


static jerry_value_t _comp(const jerry_call_info_t* info, const jerry_value_t args[], const jerry_length_t argsCnt)
{
    auto data = static_cast<ExpContent*>(jerry_object_get_native_ptr(info->function, &freeCb));
    auto comp = static_cast<LottieLayer*>(data->obj);
    auto layer = comp->layerById(_idByName(args[0]));

    if (!layer) return jerry_undefined();

    auto obj = jerry_object();
    jerry_object_set_native_ptr(obj, nullptr, layer);
    _buildLayer(obj, data->frameNo, layer, comp, data->exp);

    return obj;
}


static void _buildMath(jerry_value_t context)
{
    auto bm_mul = jerry_function_external(_mul);
    jerry_object_set_sz(context, "$bm_mul", bm_mul);
    jerry_value_free(bm_mul);

    auto bm_sum = jerry_function_external(_add);
    jerry_object_set_sz(context, "$bm_sum", bm_sum);
    jerry_value_free(bm_sum);

    auto bm_add = jerry_function_external(_add);
    jerry_object_set_sz(context, "$bm_add", bm_add);
    jerry_value_free(bm_add);

    auto bm_sub = jerry_function_external(_sub);
    jerry_object_set_sz(context, "$bm_sub", bm_sub);
    jerry_value_free(bm_sub);

    auto bm_div = jerry_function_external(_div);
    jerry_object_set_sz(context, "$bm_div", bm_div);
    jerry_value_free(bm_div);

    auto bm_mod = jerry_function_external(_mod);
    jerry_object_set_sz(context, "$bm_mod", bm_mod);
    jerry_value_free(bm_mod);

    auto mul = jerry_function_external(_mul);
    jerry_object_set_sz(context, "mul", mul);
    jerry_value_free(mul);

    auto sum = jerry_function_external(_add);
    jerry_object_set_sz(context, "sum", sum);
    jerry_value_free(sum);

    auto add = jerry_function_external(_add);
    jerry_object_set_sz(context, "add", add);
    jerry_value_free(add);

    auto sub = jerry_function_external(_sub);
    jerry_object_set_sz(context, "sub", sub);
    jerry_value_free(sub);

    auto div = jerry_function_external(_div);
    jerry_object_set_sz(context, "div", div);
    jerry_value_free(div);

    auto mod = jerry_function_external(_mod);
    jerry_object_set_sz(context, "mod", mod);
    jerry_value_free(mod);

    auto clamp = jerry_function_external(_clamp);
    jerry_object_set_sz(context, "clamp", clamp);
    jerry_value_free(clamp);

    auto dot = jerry_function_external(_dot);
    jerry_object_set_sz(context, "dot", dot);
    jerry_value_free(dot);

    auto cross = jerry_function_external(_cross);
    jerry_object_set_sz(context, "cross", cross);
    jerry_value_free(cross);

    auto normalize = jerry_function_external(_normalize);
    jerry_object_set_sz(context, "normalize", normalize);
    jerry_value_free(normalize);

    auto length = jerry_function_external(_length);
    jerry_object_set_sz(context, "length", length);
    jerry_value_free(length);

    auto random = jerry_function_external(_random);
    jerry_object_set_sz(context, "random", random);
    jerry_value_free(random);

    auto deg2rad = jerry_function_external(_deg2rad);
    jerry_object_set_sz(context, "degreesToRadians", deg2rad);
    jerry_value_free(deg2rad);

    auto rad2deg = jerry_function_external(_rad2deg);
    jerry_object_set_sz(context, "radiansToDegrees", rad2deg);
    jerry_value_free(rad2deg);

    auto linear = jerry_function_external(_linear);
    jerry_object_set_sz(context, "linear", linear);
    jerry_value_free(linear);

    auto ease = jerry_function_external(_ease);
    jerry_object_set_sz(context, "ease", ease);
    jerry_value_free(ease);

    auto easeIn = jerry_function_external(_easeIn);
    jerry_object_set_sz(context, "easeIn", easeIn);
    jerry_value_free(easeIn);

    auto easeOut = jerry_function_external(_easeOut);
    jerry_object_set_sz(context, "easeOut", easeOut);
    jerry_value_free(easeOut);

    //lookAt
}


void LottieExpressions::buildGlobal(Context& context, float frameNo, LottieExpression* exp)
{
    tvg::free(static_cast<ExpContent*>(jerry_object_get_native_ptr(context.comp, &freeCb)));
    jerry_object_set_native_ptr(context.comp, &freeCb, _expcontent(exp, frameNo, exp->layer));

    auto index = jerry_number(exp->layer->ix);
    jerry_object_set_sz(context.global, EXP_INDEX, index);
    jerry_value_free(index);

    auto inPoint = jerry_number(exp->layer->inFrame / exp->comp->frameRate);
    jerry_object_set_sz(context.global, "inPoint", inPoint);
    jerry_value_free(inPoint);

    auto outPoint = jerry_number(exp->layer->outFrame / exp->comp->frameRate);
    jerry_object_set_sz(context.global, "outPoint", outPoint);
    jerry_value_free(outPoint);
}


void LottieExpressions::buildComp(jerry_value_t context, float frameNo, LottieLayer* comp, LottieExpression* exp)
{
    //layer(index) / layer(name) / layer(otherLayer, reIndex)
    auto layer = jerry_function_external(_layer);
    jerry_object_set_sz(context, "layer", layer);

    jerry_object_set_native_ptr(layer, &freeCb, _expcontent(exp, frameNo, comp));
    jerry_value_free(layer);

    auto numLayers = jerry_number((float)comp->children.count);
    jerry_object_set_sz(context, "numLayers", numLayers);
    jerry_value_free(numLayers);
}


void LottieExpressions::buildComp(Context& context, LottieComposition* comp, float frameNo, LottieExpression* exp)
{
    buildComp(context.comp, frameNo, comp->root, exp);

    //marker
    //marker.key(index)
    //marker.key(name)
    //marker.nearestKey(t)
    //marker.numKeys

    //activeCamera

    auto width = jerry_number(comp->w);
    jerry_object_set_sz(context.thisComp, EXP_WIDTH, width);
    jerry_value_free(width);

    auto height = jerry_number(comp->h);
    jerry_object_set_sz(context.thisComp, EXP_HEIGHT, height);
    jerry_value_free(height);

    auto duration = jerry_number(comp->duration());
    jerry_object_set_sz(context.thisComp, "duration", duration);
    jerry_value_free(duration);

    //ntscDropFrame
    //displayStartTime

    auto frameDuration = jerry_number(1.0f / comp->frameRate);
    jerry_object_set_sz(context.thisComp, "frameDuration", frameDuration);
    jerry_value_free(frameDuration);

    //shutterAngle
    //shutterPhase
    //bgColor
    //pixelAspect

    if (comp->name) {
        auto name = jerry_string_sz(comp->name);
        jerry_object_set_sz(context.thisComp, EXP_NAME, name);
        jerry_value_free(name);
    }
}


jerry_value_t LottieExpressions::buildGlobal(Context& context)
{
    context.global = jerry_current_realm();

    //comp(name)
    context.comp = jerry_function_external(_comp);
    jerry_object_set_sz(context.global, "comp", context.comp);

    //footage(name)

    context.thisComp = jerry_object();
    jerry_object_set_sz(context.global, "thisComp", context.thisComp);

    context.thisLayer = jerry_object();
    jerry_object_set_sz(context.global, "thisLayer", context.thisLayer);

    context.thisProperty = jerry_object();
    jerry_object_set_sz(context.global, "thisProperty", context.thisProperty);

    //fromCompToSurface
    //createPath
    //posterizeTime(framesPerSecond)
    //value

    return context.global;
}

jerry_value_t LottieExpressions::evaluate(float frameNo, LottieExpression* exp)
{
    if (exp->disabled) return jerry_undefined();

    auto& context = this->context();

    buildGlobal(context, frameNo, exp);

    //main composition
    buildComp(context, exp->comp, frameNo, exp);

    //this composition
    buildComp(context.thisComp, frameNo, exp->layer->comp, exp);

    //update global context values
    _buildProperty(frameNo, context.global, exp);

    //this layer
    jerry_object_set_native_ptr(context.thisLayer, nullptr, exp->layer);
    _buildLayer(context.thisLayer, frameNo, exp->layer, exp->comp->root, exp);

    //this property
    jerry_object_set_native_ptr(context.thisProperty, nullptr, exp->property);
    _buildProperty(frameNo, context.thisProperty, exp);

    //expansions per object type
    if (exp->object->type == LottieObject::Transform) _buildTransform(context.global, frameNo, static_cast<LottieTransform*>(exp->object));

    //evaluate the code
    auto eval = jerry_eval((jerry_char_t *) exp->code, strlen(exp->code), JERRY_PARSE_NO_OPTS);

    if (jerry_value_is_exception(eval)) {
        TVGERR("LOTTIE", "Failed to dispatch the expressions!");
        jerry_value_free(eval);
        exp->disabled = true;
        return jerry_undefined();
    }

    jerry_value_free(eval);

    return jerry_object_get_sz(context.global, "$bm_rt");
}


/************************************************************************/
/* External Class Implementation                                        */
/************************************************************************/

LottieExpressions* LottieExpressions::instance()
{
    ScopedLock lock(_lockKey);
    if (!_exps) _exps = new LottieExpressions;
    ++_refCnt;
    return _exps;
}


void LottieExpressions::retrieve(LottieExpressions* instance)
{
    if (!instance) return;

    ScopedLock lock(_lockKey);
    if (--_refCnt == 0) {
        delete(instance);
        _exps = nullptr;
    }
}


LottieExpressions::Context& LottieExpressions::context()
{
#ifdef THORVG_THREAD_SUPPORT
    auto tid = this_thread::get_id();
    ScopedLock lock(_lockKey);

    for (auto context : contexts) {
        if (context->tid == tid) return *context;
    }

    auto context = new Context;
    context->tid = tid;
    init(*context);
    contexts.push(context);
    return *context;
#else
    if (contexts.empty()) {
        auto context = new Context;
        init(*context);
        contexts.push(context);
    }
    return *contexts.first();
#endif
}


void LottieExpressions::init(Context& context)
{
    jerry_init(JERRY_INIT_EMPTY);
#ifdef THORVG_THREAD_SUPPORT
    context.ctx = jerry_port_context_get();
#endif
    jerry_register_magic_strings(_magicStrings, MAGIC_STRING_COUNT, _magicLengths);
    _buildMath(buildGlobal(context));
}


void LottieExpressions::clear(Context& context)
{
#ifdef THORVG_THREAD_SUPPORT
    jerry_port_context_set(context.ctx);
#endif
    jerry_value_free(context.thisProperty);
    jerry_value_free(context.thisLayer);
    jerry_value_free(context.thisComp);
    jerry_value_free(context.comp);
    jerry_value_free(context.global);
    jerry_cleanup();
}


LottieExpressions::~LottieExpressions()
{
    for (auto context : contexts) {
        clear(*context);
        delete(context);
    }
    contexts.clear();
}


LottieExpressions::LottieExpressions()
{
    //build magic string arrays from the magic pool
    if (!_magicStrings[0]) {
        auto p = _magicPool;
        for (uint32_t i = 0; i < MAGIC_STRING_COUNT; ++i) {
            _magicStrings[i] = (const jerry_char_t*)p;
            p += _magicLengths[i];
        }
    }
}


void LottieExpressions::update(float curTime)
{
    auto& context = this->context();

    //time, #current time in seconds
    auto time = jerry_number(curTime);
    jerry_object_set_sz(context.global, EXP_TIME, time);
    jerry_value_free(time);
}

Point LottieExpressions::toPoint2d(jerry_value_t obj)
{
    return _point2d(obj);
}


RGB32 LottieExpressions::toColor(jerry_value_t obj)
{
    return _color(obj);
}


float LottieExpressions::toFloat(jerry_value_t obj)
{
    return _number(obj);
}


#endif //THORVG_LOTTIE_EXPRESSIONS_SUPPORT
