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

#ifndef _TVG_LOTTIE_MODEL_H_
#define _TVG_LOTTIE_MODEL_H_

#include "tvgCommon.h"
#include "tvgStr.h"
#include "tvgCompressor.h"
#include "tvgInlist.h"
#include "tvgRender.h"
#include "tvgLottieProperty.h"
#include "tvgLottieRenderPooler.h"
#include "tvgLottieTween.h"

struct LottieComposition;

struct LottieStroke
{
    struct DashAttr
    {
        LottieFloat offset = 0.0f;
        LottieFloat* values = nullptr;
        uint8_t size = 0;
        uint8_t allocated = 0;
    };

    virtual ~LottieStroke()
    {
        if (dashattr) delete[] dashattr->values;
        delete(dashattr);
    }


    LottieFloat& dashValue()
    {
        if (!dashattr) dashattr = new DashAttr;

        if (dashattr->size + 1 > dashattr->allocated) {
            dashattr->allocated = dashattr->size + 2;
            auto newValues = new LottieFloat[dashattr->allocated];
            for (uint8_t i = 0; i < dashattr->size; ++i) {
                newValues[i].copy(dashattr->values[i]);
            }
            delete[] dashattr->values;
            dashattr->values = newValues;
        }

        return dashattr->values[dashattr->size++];
    }


    LottieFloat& dashOffset()
    {
        if (!dashattr) dashattr = new DashAttr;
        return dashattr->offset;
    }

    LottieFloat width = 0.0f;
    DashAttr* dashattr = nullptr;
    float miterLimit = 0;
    StrokeCap cap = StrokeCap::Round;
    StrokeJoin join = StrokeJoin::Round;
};

struct LottieEffect
{
    enum Type : uint8_t {Custom = 5, Tint = 20, Fill, Stroke, Tritone, DropShadow = 25, GaussianBlur = 29};

    virtual ~LottieEffect() {}

    unsigned long nm;  //encoded by djb2
    unsigned long mn;  //encoded by djb2
    int16_t ix;
    Type type;
    bool enable = false;
};


struct LottieFxCustom : LottieEffect
{
    struct Property
    {
        LottieProperty* property;
        unsigned long nm = 0;  //encoded by djb2
        unsigned long mn = 0;  //encoded by djb2
    };

    char* name = nullptr;
    Array<Property> props;

    LottieFxCustom()
    {
        type = LottieEffect::Custom;
    }

    ~LottieFxCustom()
    {
        ARRAY_FOREACH(p, props) delete(p->property);
    }

    Property* property(int type)
    {
        LottieProperty* prop = nullptr;

        switch (type) {
            case 0: //slider
            case 1: prop = new LottieFloat; break; //angle
            case 2: prop = new LottieColor; break; //color
            case 3: prop = new LottieVector; break; //point
            case 4: //checkbox
            case 7: //dropdown
            case 10: prop = new LottieInteger; break;  //effect layer
            case 6: TVGLOG("LOTTIE", "custom ignored?"); break;
            default:
                TVGLOG("LOTTIE", "missing custom property = %d\n", type);
                return nullptr;
        }
        if (prop) {
            props.push({prop, });
            return &props.last();
        }
        return nullptr;
    }

    LottieProperty* property(const char* name)
    {
        auto id = djb2Encode(name);
        ARRAY_FOREACH(p, props) {
            if (p->mn == id || p->nm == id) return p->property;
        }
        return nullptr;
    }
};

struct LottieFxFill : LottieEffect
{
    //LottieInteger mask;
    //LottieInteger allMask;
    LottieColor color;
    //LottieInteger invert;
    //LottieSlider hFeather;
    //LottieSlider vFeather;
    LottieFloat opacity;

    LottieFxFill()
    {
        type = LottieEffect::Fill;
    }
};

struct LottieFxStroke : LottieEffect
{
    LottieInteger mask;
    LottieInteger allMask;
    //LottieInteger sequential;
    LottieColor color;
    LottieFloat size;
    //LottieFloat hardness;    //should support with the blurness?
    LottieFloat opacity;
    LottieFloat begin;
    LottieFloat end;
    //LottieFloat space;
    //LottieInteger style;

    LottieFxStroke()
    {
        type = LottieEffect::Stroke;
    }
};

struct LottieFxTint : LottieEffect
{
    LottieColor black;
    LottieColor white;
    LottieFloat intensity;

    LottieFxTint()
    {
        type = LottieEffect::Tint;
    }
};

struct LottieFxTritone : LottieEffect
{
    LottieColor bright;
    LottieColor midtone;
    LottieColor dark;
    LottieOpacity blend;

    LottieFxTritone()
    {
        type = LottieEffect::Tritone;
    }
};

struct LottieFxDropShadow : LottieEffect
{
    LottieColor color;
    LottieFloat opacity = 0;
    LottieFloat angle = 0.0f;
    LottieFloat distance = 0.0f;
    LottieFloat blurness = 0.0f;

    LottieFxDropShadow()
    {
        type = LottieEffect::DropShadow;
    }
};

struct LottieFxGaussianBlur : LottieEffect
{
    LottieFloat blurness = 0.0f;
    LottieInteger direction = 0;
    LottieInteger wrap = 0;

    LottieFxGaussianBlur()
    {
        type = LottieEffect::GaussianBlur;
    }
};


struct LottieMask
{
    LottiePathSet pathset;
    LottieFloat expand = 0.0f;
    LottieOpacity opacity = 255;
    MaskMethod method;
    bool inverse = false;
};


struct LottieObject
{
    enum Type : uint8_t
    {
        Composition = 0,
        Layer,
        Group,
        Transform,
        SolidFill,
        SolidStroke,
        GradientFill,
        GradientStroke,
        Rect,
        Ellipse,
        Path,
        Polystar,
        Image,
        Trimpath,
        Text,
        Repeater,
        RoundedCorner,
        OffsetPath,
        PuckerBloat,
        ZigZag,
        TextRange,
        Audio
    };

    virtual ~LottieObject()
    {
    }

    virtual LottieProperty* override(LottieProperty* prop, bool release)
    {
        TVGERR("LOTTIE", "Unsupported slot type");
        return nullptr;
    }

    virtual bool mergeable() { return false; }
    virtual LottieProperty* property(uint16_t ix) { return nullptr; }

    unsigned long id = 0;      //unique id by name generated by djb2 encoding
    Type type;
    bool hidden = false;       //remove?
};


struct LottieGlyph
{
    Array<LottieObject*> children;   //glyph shapes.
    float width;
    char* code = nullptr;
    char* family = nullptr;
    char* style = nullptr;
    uint16_t size;
    uint8_t len;

    bool prepare()
    {
        len = code ? strlen(code) : 0;
        return len > 0;
    }

    ~LottieGlyph()
    {
        ARRAY_FOREACH(p, children) delete(*p);
        tvg::free(code);
        tvg::free(family);
        tvg::free(style);
    }
};


struct LottieTextRange : LottieObject
{
    enum Based : uint8_t { Chars = 1, CharsExcludingSpaces, Words, Lines };
    enum Shape : uint8_t { Square = 1, RampUp, RampDown, Triangle, Round, Smooth };
    enum Unit : uint8_t { Percent = 1, Index };

    LottieTextRange()
    {
        LottieObject::type = LottieObject::TextRange;
        style.flags.fillColor = 0;
        style.flags.strokeColor = 0;
        style.flags.strokeWidth = 0;
    }

    ~LottieTextRange()
    {
        tvg::free(interpolator);
    }

    struct {
        LottieColor fillColor = RGB32{255, 255, 255};
        LottieColor strokeColor = RGB32{255, 255, 255};
        LottieVector position = Point{0, 0};
        LottieScalar scale = Point{100, 100};
        LottieFloat letterSpace = 0.0f;
        LottieFloat lineSpace = 0.0f;
        LottieFloat strokeWidth = 0.0f;
        LottieFloat rotation = 0.0f;
        LottieOpacity fillOpacity = 255;
        LottieOpacity strokeOpacity = 255;
        LottieOpacity opacity = 255;
        struct {
            bool fillColor : 1;
            bool strokeColor : 1;
            bool strokeWidth : 1;
        } flags;
    } style;

    LottieFloat offset = 0.0f;
    LottieFloat maxEase = 0.0f;
    LottieFloat minEase = 0.0f;
    LottieFloat maxAmount = 0.0f;
    LottieFloat smoothness = 100.0f;
    LottieFloat start = 0.0f;
    LottieFloat end = 100.0f;
    LottieInterpolator* interpolator = nullptr;
    Based based = Chars;
    Shape shape = Square;
    Unit rangeUnit = Percent;
    uint8_t random = 0;
    bool expressible = false;

    float factor(float frameNo, float totalLen, float idx);

    void color(float frameNo, RGB32& fillColor, RGB32& strokeColor, float factor, LottieTween& tween, LottieExpressions* exps)
    {
        if (style.flags.fillColor) {
            auto color = style.fillColor(frameNo, tween, exps);
            fillColor.r = tvg::lerp<uint8_t>(fillColor.r, color.r, factor);
            fillColor.g = tvg::lerp<uint8_t>(fillColor.g, color.g, factor);
            fillColor.b = tvg::lerp<uint8_t>(fillColor.b, color.b, factor);
        }
        if (style.flags.strokeColor) {
            auto color = style.strokeColor(frameNo, tween, exps);
            strokeColor.r = tvg::lerp<uint8_t>(strokeColor.r, color.r, factor);
            strokeColor.g = tvg::lerp<uint8_t>(strokeColor.g, color.g, factor);
            strokeColor.b = tvg::lerp<uint8_t>(strokeColor.b, color.b, factor);
        }
    }

    LottieProperty* override(LottieProperty* prop, bool release) override
    {
        LottieProperty* backup = nullptr;
        if (style.fillColor.sid == prop->sid) {
            if (release) style.fillColor.release();
            else backup = new LottieColor(style.fillColor);
            style.fillColor.copy(*static_cast<LottieColor*>(prop), false);
        } else if (style.strokeColor.sid == prop->sid) {
            if (release) style.strokeColor.release();
            else backup = new LottieColor(style.strokeColor);
            style.strokeColor.copy(*static_cast<LottieColor*>(prop), false);
        } else if (style.position.sid == prop->sid) {
            if (release) style.position.release();
            else backup = new LottieVector(style.position);
            style.position.copy(*static_cast<LottieVector*>(prop), false);
        } else if (style.scale.sid == prop->sid) {
            if (release) style.scale.release();
            else backup = new LottieScalar(style.scale);
            style.scale.copy(*static_cast<LottieScalar*>(prop), false);
        } else if (style.rotation.sid == prop->sid) {
            if (release) style.rotation.release();
            else backup = new LottieFloat(style.rotation);
            style.rotation.copy(*static_cast<LottieFloat*>(prop), false);
        } else if (style.letterSpace.sid == prop->sid) {
            if (release) style.letterSpace.release();
            else backup = new LottieFloat(style.letterSpace);
            style.letterSpace.copy(*static_cast<LottieFloat*>(prop), false);
        } else if (style.lineSpace.sid == prop->sid) {
            if (release) style.lineSpace.release();
            else backup = new LottieFloat(style.lineSpace);
            style.lineSpace.copy(*static_cast<LottieFloat*>(prop), false);
        } else if (style.strokeWidth.sid == prop->sid) {
            if (release) style.strokeWidth.release();
            else backup = new LottieFloat(style.strokeWidth);
            style.strokeWidth.copy(*static_cast<LottieFloat*>(prop), false);
        } else if (style.fillOpacity.sid == prop->sid) {
            if (release) style.fillOpacity.release();
            else backup = new LottieOpacity(style.fillOpacity);
            style.fillOpacity.copy(*static_cast<LottieOpacity*>(prop), false);
        } else if (style.strokeOpacity.sid == prop->sid) {
            if (release) style.strokeOpacity.release();
            else backup = new LottieOpacity(style.strokeOpacity);
            style.strokeOpacity.copy(*static_cast<LottieOpacity*>(prop), false);
        } else if (style.opacity.sid == prop->sid) {
            if (release) style.opacity.release();
            else backup = new LottieOpacity(style.opacity);
            style.opacity.copy(*static_cast<LottieOpacity*>(prop), false);
        }
        return backup;
    }
};


struct LottieFont
{
    enum Origin : uint8_t {Local = 0, CssURL, ScriptURL, FontURL};

    ~LottieFont()
    {
        if (b64src) Text::unload(name);
        ARRAY_FOREACH(p, chars) delete(*p);
        tvg::free(style);
        tvg::free(family);
        tvg::free(name);
        tvg::free(b64src);
        tvg::free(mime);
    }

    union {
        char* b64src = nullptr;
        char* path;
    };

    Array<LottieGlyph*> chars;
    char* name = nullptr;
    char* family = nullptr;
    char* style = nullptr;
    char* mime = nullptr;
    uint32_t size = 0;
    float ascent = 0.0f;
    Origin origin = Local;

    void prepare();
};

struct LottieMarker
{
    char* name = nullptr;
    float time = 0.0f;
    float duration = 0.0f;
    
    ~LottieMarker()
    {
        tvg::free(name);
    }
};


struct LottieTextFollowPath
{
private:
    RenderPath path;
    PathCommand* cmds;
    uint32_t cmdsCnt;
    Point* pts;
    Point* start;
    float totalLen;
    float currentLen;
    Point split(float dLen, float lenSearched, float& angle);
    void rewind();

public:
    LottieFloat firstMargin = 0.0f;
    LottieMask* mask;
    int8_t maskIdx = -1;

    Point position(float lenSearched, float& angle);
    float prepare(LottieMask* mask, float frameNo, float scale, LottieTween& tween, LottieExpressions* exps);
};


struct LottieText : LottieObject, LottieRenderPooler<tvg::Shape>
{
    struct AlignOption
    {
        enum Group : uint8_t { Chars = 1, Word = 2, Line = 3, All = 4 };
        Group group = Chars;
        LottieScalar anchor{};
    } alignOp;

    LottieText()
    {
        LottieObject::type = LottieObject::Text;
    }

    LottieProperty* override(LottieProperty* prop, bool release) override
    {
        LottieProperty* backup = nullptr;
        if (release) doc.release();
        else backup = new LottieTextDoc(doc);
        doc.copy(*static_cast<LottieTextDoc*>(prop), false);
        return backup;
    }

    LottieProperty* property(uint16_t ix) override
    {
        if (doc.ix == ix) return &doc;
        return nullptr;
    }

    LottieTextDoc doc;
    LottieFont* font = nullptr;
    LottieTextFollowPath* follow = nullptr;
    Array<LottieTextRange*> ranges;

    ~LottieText()
    {
        ARRAY_FOREACH(p, ranges) delete(*p);
        delete(follow);
    }
};


struct LottieTrimpath : LottieObject
{
    enum Type : uint8_t { Simultaneous = 1, Individual = 2 };

    LottieTrimpath()
    {
        LottieObject::type = LottieObject::Trimpath;
    }

    bool mergeable() override
    {
        if (!start.frames && start.value == 0.0f && !end.frames && end.value == 100.0f && !offset.frames && offset.value == 0.0f) return true;
        return false;
    }

    LottieProperty* property(uint16_t ix) override
    {
        if (start.ix == ix) return &start;
        if (end.ix == ix) return &end;
        if (offset.ix == ix) return &offset;
        return nullptr;
    }

    void segment(float frameNo, float& start, float& end, LottieTween& tween, LottieExpressions* exps);

    LottieFloat start = 0.0f;
    LottieFloat end = 100.0f;
    LottieFloat offset = 0.0f;
    Type type = Simultaneous;
};


struct LottieShape : LottieObject, LottieRenderPooler<Shape>
{
    bool clockwise = true;   //clockwise or counter-clockwise

    virtual ~LottieShape() {}

    bool mergeable() override
    {
        return true;
    }

    LottieShape(LottieObject::Type type)
    {
        LottieObject::type = type;
    }
};


struct LottieRoundedCorner : LottieObject
{
    LottieRoundedCorner()
    {
        LottieObject::type = LottieObject::RoundedCorner;
    }

    LottieProperty* property(uint16_t ix) override
    {
        if (radius.ix == ix) return &radius;
        return nullptr;
    }

    LottieFloat radius = 0.0f;
};


struct LottiePath : LottieShape
{
    LottiePath() : LottieShape(LottieObject::Path) {}

    LottieProperty* property(uint16_t ix) override
    {
        if (pathset.ix == ix) return &pathset;
        return nullptr;
    }

    LottiePathSet pathset;
};


struct LottieRect : LottieShape
{
    LottieRect() : LottieShape(LottieObject::Rect) {}

    LottieProperty* property(uint16_t ix) override
    {
        if (position.ix == ix) return &position;
        if (size.ix == ix) return &size;
        if (radius.ix == ix) return &radius;
        return nullptr;
    }

    LottieVector position = Point{0.0f, 0.0f};
    LottieScalar size = Point{0.0f, 0.0f};
    LottieFloat radius = 0.0f;       //rounded corner radius
};


struct LottiePolyStar : LottieShape
{
    enum Type : uint8_t {Star = 1, Polygon};

    LottiePolyStar() : LottieShape(LottieObject::Polystar) {}

    LottieProperty* property(uint16_t ix) override
    {
        if (position.ix == ix) return &position;
        if (innerRadius.ix == ix) return &innerRadius;
        if (outerRadius.ix == ix) return &outerRadius;
        if (innerRoundness.ix == ix) return &innerRoundness;
        if (outerRoundness.ix == ix) return &outerRoundness;
        if (rotation.ix == ix) return &rotation;
        if (ptsCnt.ix == ix) return &ptsCnt;
        return nullptr;
    }

    LottieVector position = Point{0.0f, 0.0f};
    LottieFloat innerRadius = 0.0f;
    LottieFloat outerRadius = 0.0f;
    LottieFloat innerRoundness = 0.0f;
    LottieFloat outerRoundness = 0.0f;
    LottieFloat rotation = 0.0f;
    LottieFloat ptsCnt = 0.0f;
    Type type = Polygon;
};


struct LottieEllipse : LottieShape
{
    LottieEllipse() : LottieShape(LottieObject::Ellipse) {}

    LottieProperty* property(uint16_t ix) override
    {
        if (position.ix == ix) return &position;
        if (size.ix == ix) return &size;
        return nullptr;
    }

    LottieVector position = Point{0.0f, 0.0f};
    LottieScalar size = Point{0.0f, 0.0f};
};


struct LottieTransform : LottieObject
{
    struct SeparateCoord
    {
        LottieFloat x = 0.0f;
        LottieFloat y = 0.0f;
    };

    SeparateCoord* separateCoord()
    {
        if (!coords) coords = new SeparateCoord;
        return coords;
    }

    ~LottieTransform()
    {
        delete(coords);
        delete (ddd);
    }

    LottieTransform()
    {
        LottieObject::type = LottieObject::Transform;
    }

    bool mergeable() override
    {
        return true;
    }

    LottieProperty* property(uint16_t ix) override
    {
        if (position.ix == ix) return &position;
        if (rotation.ix == ix) return &rotation;
        if (scale.ix == ix) return &scale;
        if (anchor.ix == ix) return &anchor;
        if (opacity.ix == ix) return &opacity;
        if (skewAngle.ix == ix) return &skewAngle;
        if (skewAxis.ix == ix) return &skewAxis;
        if (coords) {
            if (coords->x.ix == ix) return &coords->x;
            if (coords->y.ix == ix) return &coords->y;
        }
        return nullptr;
    }

    LottieProperty* override(LottieProperty* prop, bool release) override
    {
        LottieProperty* backup = nullptr;
        if (rotation.sid == prop->sid) {
            if (release) rotation.release();
            else backup = new LottieFloat(rotation);
            rotation.copy(*static_cast<LottieFloat*>(prop), false);
        } else if (scale.sid == prop->sid) {
            if (release) scale.release();
            else backup = new LottieScalar(scale);
            scale.copy(*static_cast<LottieScalar*>(prop), false);
        } else if (position.sid == prop->sid) {
            if (release) position.release();
            else backup = new LottieVector(position);
            position.copy(*static_cast<LottieVector*>(prop), false);
        } else if (opacity.sid == prop->sid) {
            if (release) opacity.release();
            else backup = new LottieOpacity(opacity);
            opacity.copy(*static_cast<LottieOpacity*>(prop), false);
        } else if (skewAngle.sid == prop->sid) {
            if (release) skewAngle.release();
            else backup = new LottieFloat(skewAngle);
            skewAngle.copy(*static_cast<LottieFloat*>(prop), false);
        } else if (skewAxis.sid == prop->sid) {
            if (release) skewAxis.release();
            else backup = new LottieFloat(skewAxis);
            skewAxis.copy(*static_cast<LottieFloat*>(prop), false);
        }
        return backup;
    }

    LottieVector position = Point{0.0f, 0.0f};
    LottieFloat rotation = 0.0f;           //z rotation
    LottieScalar scale = Point{100.0f, 100.0f};
    LottieScalar anchor = Point{0.0f, 0.0f};
    LottieOpacity opacity = 255;
    LottieFloat skewAngle = 0.0f;
    LottieFloat skewAxis = 0.0f;

    SeparateCoord* coords = nullptr;       //either a position or separate coordinates

    struct Dimension3
    {
        LottieFloat rx = 0.0f, ry = 0.0f;  // use the rotation for z rotation
        LottieScalar3 orient = Point3{0.0f, 0.0f, 0.0f};
    }* ddd = nullptr;
};


struct LottieSolid : LottieObject 
{
    LottieColor color = RGB32{255, 255, 255};
    LottieOpacity opacity = 255;

    LottieProperty* property(uint16_t ix) override
    {
        if (color.ix == ix) return &color;
        if (opacity.ix == ix) return &opacity;
        return nullptr;
    }

    LottieProperty* override(LottieProperty* prop, bool release) override
    {
        LottieProperty* backup = nullptr;
        if (color.sid == prop->sid) {
            if (release) color.release();
            else backup = new LottieColor(color);
            color.copy(*static_cast<LottieColor*>(prop), false);
        } else if (opacity.sid == prop->sid) {
            if (release) opacity.release();
            else backup = new LottieOpacity(opacity);
            opacity.copy(*static_cast<LottieOpacity*>(prop), false);
        }
        return backup;
    }
};


struct LottieSolidStroke : LottieSolid, LottieStroke
{
    LottieSolidStroke()
    {
        LottieObject::type = LottieObject::SolidStroke;
    }

    LottieProperty* property(uint16_t ix) override
    {
        if (width.ix == ix) return &width;
        if (dashattr) {
            for (uint8_t i = 0; i < dashattr->size ; ++i)
                if (dashattr->values[i].ix == ix) return &dashattr->values[i];
        }
        return LottieSolid::property(ix);
    }
};


struct LottieSolidFill : LottieSolid
{
    LottieSolidFill()
    {
        LottieObject::type = LottieObject::SolidFill;
    }

    FillRule rule = FillRule::NonZero;
};


struct LottieGradient : LottieObject
{
    bool prepare()
    {
        if (!colorStops.populated) {
            auto count = colorStops.count;  //colorstop count can be modified after population
            if (colorStops.frames) {
                ARRAY_FOREACH(v, *colorStops.frames) {
                    colorStops.count = populate(v->value, count);
                }
            } else {
                colorStops.count = populate(colorStops.value, count);
            }
            colorStops.populated = true;
        }
        if (start.frames || end.frames || height.frames || angle.frames || opacity.frames || colorStops.frames) return true;
        return false;
    }

    LottieProperty* property(uint16_t ix) override
    {
        if (start.ix == ix) return &start;
        if (end.ix == ix) return &end;
        if (height.ix == ix) return &height;
        if (angle.ix == ix) return &angle;
        if (opacity.ix == ix) return &opacity;
        if (colorStops.ix == ix) return &colorStops;
        return nullptr;
    }

    LottieProperty* override(LottieProperty* prop, bool release) override
    {
        LottieProperty* backup = nullptr;
        if (colorStops.sid == prop->sid) {
            if (release) colorStops.release();
            else backup = new LottieColorStop(colorStops);
            colorStops.copy(*static_cast<LottieColorStop*>(prop), false);
            prepare();
        } else if (opacity.sid == prop->sid) {
            if (release) opacity.release();
            else backup = new LottieOpacity(opacity);
            opacity.copy(*static_cast<LottieOpacity*>(prop), false);
        }
        return backup;
    }

    uint32_t populate(ColorStop& color, size_t count);
    Fill* fill(float frameNo, uint8_t opacity, LottieTween& tween, LottieExpressions* exps);

    LottieScalar start = Point{0.0f, 0.0f};
    LottieScalar end = Point{0.0f, 0.0f};
    LottieFloat height = 0.0f;
    LottieFloat angle = 0.0f;
    LottieOpacity opacity = 255;
    LottieColorStop colorStops;
    uint8_t id = 0; //1: linear, 2: radial
    bool opaque = true; //fully opaque or not
};


struct LottieGradientFill : LottieGradient
{
    LottieGradientFill()
    {
        LottieObject::type = LottieObject::GradientFill;
    }

    FillRule rule = FillRule::NonZero;
};


struct LottieGradientStroke : LottieGradient, LottieStroke
{
    LottieGradientStroke()
    {
        LottieObject::type = LottieObject::GradientStroke;
    }

    LottieProperty* property(uint16_t ix) override
    {
        if (width.ix == ix) return &width;
        if (dashattr) {
            for (uint8_t i = 0; i < dashattr->size ; ++i)
                if (dashattr->values[i].ix == ix) return &dashattr->values[i];
        }
        return LottieGradient::property(ix);
    }
};


struct LottieImage : LottieObject
{
    LottieBitmap bitmap;
    bool resolved = false;

    LottieProperty* override(LottieProperty* prop, bool release) override
    {
        LottieProperty* backup = nullptr;
        if (release) bitmap.release();
        else backup = new LottieBitmap(bitmap);
        bitmap.copy(*static_cast<LottieBitmap*>(prop), false);
        return backup;
    }

    void prepare(bool external);
};


struct LottieAudio : LottieObject
{
    union {
        char* data = nullptr;
        char* path;
    };
    char* mimeType = nullptr;
    uint32_t size = 0;

    LottieAudio() { LottieObject::type = LottieObject::Audio; }

    ~LottieAudio()
    {
        tvg::free(data);
        tvg::free(mimeType);
    }
};


struct LottieRepeater : LottieObject
{
    LottieRepeater()
    {
        LottieObject::type = LottieObject::Repeater;
    }

    LottieProperty* property(uint16_t ix) override
    {
        if (copies.ix == ix) return &copies;
        if (offset.ix == ix) return &offset;
        if (position.ix == ix) return &position;
        if (rotation.ix == ix) return &rotation;
        if (scale.ix == ix) return &scale;
        if (anchor.ix == ix) return &anchor;
        if (startOpacity.ix == ix) return &startOpacity;
        if (endOpacity.ix == ix) return &endOpacity;
        return nullptr;
    }

    LottieFloat copies = 0.0f;
    LottieFloat offset = 0.0f;

    //Transform
    LottieVector position = Point{0.0f, 0.0f};
    LottieFloat rotation = 0.0f;
    LottieScalar scale = Point{100.0f, 100.0f};
    LottieScalar anchor = Point{0.0f, 0.0f};
    LottieOpacity startOpacity = 255;
    LottieOpacity endOpacity = 255;
    bool inorder = true;        //true: higher,  false: lower
};


struct LottieOffsetPath : LottieObject
{
    LottieOffsetPath()
    {
        LottieObject::type = LottieObject::OffsetPath;
    }

    LottieFloat offset = 0.0f;
    LottieFloat miterLimit = 4.0f;
    StrokeJoin join = StrokeJoin::Miter;
};

struct LottiePuckerBloat : LottieObject
{
    LottiePuckerBloat()
    {
        LottieObject::type = LottieObject::PuckerBloat;
    }

    LottieFloat amount = 0.0f;
};

struct LottieZigZag : LottieObject
{
    LottieZigZag()
    {
        LottieObject::type = LottieObject::ZigZag;
    }

    LottieFloat amplitude = 0.0f;
    LottieInteger frequency = 0;
    LottieInteger point = 1; //1: corner, 2: smooth
};

struct LottieGroup : LottieObject, LottieRenderPooler<tvg::Shape>
{
    LottieGroup();

    virtual ~LottieGroup()
    {
        ARRAY_FOREACH(p, children) delete(*p);
    }

    void prepare(LottieObject::Type type = LottieObject::Group);
    bool mergeable() override { return allowMerge; }
    LottieProperty* property(uint16_t ix) override;

    LottieObject* content(unsigned long id)
    {
        if (this->id == id) return this;

        //source has children, find recursively.
        ARRAY_FOREACH(p, children) {
            auto child = *p;
            if (child->type == LottieObject::Type::Group || child->type == LottieObject::Type::Layer) {
                if (auto ret = static_cast<LottieGroup*>(child)->content(id)) return ret;
            } else if (child->id == id) return child;
        }
        return nullptr;
    }

    Scene* scene = nullptr;
    Array<LottieObject*> children;
    BlendMethod blendMethod = BlendMethod::Normal;

    bool reqFragment : 1;   //requirement to fragment the render context
    bool buildDone : 1;     //completed in building the composition.
    bool trimpath : 1;      //this group has a trimpath.
    bool visible : 1;       //this group has visible contents.
    bool allowMerge : 1;    //if this group is consisted of simple (transformed) shapes.
};


struct LottieLayer : LottieGroup
{
    enum Type : uint8_t {Precomp = 0, Solid, Image, Null, Shape, Text, Audio};

    LottieLayer();
    ~LottieLayer();

    bool mergeable() override { return false; }
    void prepare(RGB32* color = nullptr);
    float remap(LottieComposition* comp, float frameNo, LottieExpressions* exp);
    LottieProperty* property(uint16_t ix) override;

    char* name = nullptr;
    LottieLayer* parent = nullptr;
    LottieFloat timeRemap = -1.0f;
    LottieLayer* comp = nullptr;  //Precompositor, current layer is belonges.
    LottieTransform* transform = nullptr;
    Array<LottieMask*> masks;
    Array<LottieEffect*> effects;
    LottieLayer* matteTarget = nullptr;

    LottieRenderPooler<tvg::Shape> statical;  //static pooler for solid fill and clipper

    float timeStretch = 1.0f;
    float w = 0.0f, h = 0.0f;
    float inFrame = 0.0f;
    float outFrame = 0.0f;
    float startFrame = 0.0f;

    struct AudioControl {
        LottieFloat volume = 100.0f;
        float prevVolume = -1.0f;
        bool prevActive = false;
    } *audioCtrl = nullptr;

    unsigned long rid = 0;      //pre-composition reference id.
    int16_t mix = -1;           //index of the matte layer.
    int16_t pix = -1;           //index of the parent layer.
    int16_t ix = -1;            //index of the current layer.

    struct {
        float frameNo = -1.0f;
        Matrix matrix;
        uint8_t opacity;
    } cache;

    MaskMethod matteType = MaskMethod::None;
    Type type = Null;
    bool effect : 1;        // true if any effect is activated in its tree
    bool autoOrient : 1;
    bool matteSrc : 1;

    AudioControl* audio()
    {
        if (!audioCtrl) audioCtrl = new AudioControl;
        return audioCtrl;
    }

    LottieEffect* effectById(unsigned long id)
    {
        ARRAY_FOREACH(p, effects) {
            if (id == (*p)->nm || id == (*p)->mn) return *p;
        }
        return nullptr;
    }

    LottieEffect* effectByIdx(int16_t ix)
    {
        ARRAY_FOREACH(p, effects) {
            if (ix == (*p)->ix) return *p;
        }
        return nullptr;
    }

    LottieLayer* layerById(unsigned long id)
    {
        ARRAY_FOREACH(p, children) {
            if ((*p)->type != LottieObject::Type::Layer) continue;
            auto layer = static_cast<LottieLayer*>(*p);
            if (layer->id == id) return layer;
        }
        return nullptr;
    }

    LottieLayer* layerByIdx(int16_t ix)
    {
        ARRAY_FOREACH(p, children) {
            if ((*p)->type != LottieObject::Type::Layer) continue;
            auto layer = static_cast<LottieLayer*>(*p);
            if (layer->ix == ix) return layer;
        }
        return nullptr;
    }
};


struct LottieSlot
{
    struct Pair
    {
        LottieObject* obj;
        LottieProperty* prop;
    };

    void add(uint32_t slotcode, LottieProperty* prop);
    void apply(LottieProperty* prop, bool byDefault = false);
    void reset();

    LottieSlot(LottieLayer* layer, LottieObject* parent, unsigned long sid, LottieObject* obj, LottieProperty::Type type) : context{layer, parent}, sid(sid), type(type)
    {
        pairs.push({obj, nullptr});
    }

    ~LottieSlot()
    {
        if (!overridden) return;
        ARRAY_FOREACH(pair, pairs) delete(pair->prop);
    }

    struct {
        LottieLayer* layer;
        LottieObject* parent;
    } context;

    unsigned long sid;    // djb2 encoded
    Array<Pair> pairs;    // object-property pairs that can be overridden by this slot
    LottieProperty::Type type;

    bool overridden = false;
};


struct LottieComposition
{
    ~LottieComposition();

    void clear()
    {
        if (root && root->scene) root->scene->remove();
    }

    float duration() const
    {
        return frameCnt() / frameRate;  // in second
    }

    float frameAtTime(float timeInSec) const
    {
        auto p = timeInSec / duration();
        if (p < 0.0f) p = 0.0f;
        return p * frameCnt();
    }

    float timeAtFrame(float frameNo)
    {
        return (frameNo - root->inFrame) / frameRate;
    }

    float frameCnt() const
    {
        return root->outFrame - root->inFrame;
    }

    LottieLayer* asset(unsigned long id)
    {
        ARRAY_FOREACH(p, assets) {
            auto obj = *p;
            if (obj->id == id && obj->type == LottieObject::Layer) return static_cast<LottieLayer*>(obj);
        }
        return nullptr;
    }

    void clamp(float& frameNo)
    {
        frameNo += root->inFrame;
        if (frameNo < root->inFrame) frameNo = root->inFrame;
        if (frameNo >= root->outFrame) frameNo = root->outFrame - 1;
    }

    LottieLayer* root = nullptr;
    char* version = nullptr;
    char* name = nullptr;
    float w, h;
    float frameRate;
    Array<LottieObject*> assets;
    Array<LottieInterpolator*> interpolators;
    Array<LottieFont*> fonts;
    Array<LottieSlot*> slots;
    Array<LottieMarker*> markers;
    bool expressions = false;
    bool initiated = false;
    uint8_t quality = 50;
};

#endif //_TVG_LOTTIE_MODEL_H_
