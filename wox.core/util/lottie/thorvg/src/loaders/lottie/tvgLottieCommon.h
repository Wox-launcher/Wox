/*
 * Copyright (c) 2024 the ThorVG project. All rights reserved.

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

#ifndef _TVG_LOTTIE_COMMON_
#define _TVG_LOTTIE_COMMON_

#include "tvgRender.h"
#include "tvgStr.h"

namespace tvg
{

struct PathSet
{
    Point* pts = nullptr;
    PathCommand* cmds = nullptr;
    uint16_t ptsCnt = 0;
    uint16_t cmdsCnt = 0;

    void convert(RenderPath& to)
    {
        to.cmds.data = cmds;
        to.cmds.count = cmdsCnt;
        to.pts.data = pts;
        to.pts.count = ptsCnt;
    }
};


struct RGB32
{
    int32_t r, g, b;
};


struct ColorStop
{
    Fill::ColorStop* data = nullptr;
    Array<float>* input = nullptr;

    void copy(const ColorStop& rhs, uint32_t cnt)
    {
        if (rhs.data) {
            data = tvg::malloc<Fill::ColorStop>(sizeof(Fill::ColorStop) * cnt);
            memcpy(data, rhs.data, sizeof(Fill::ColorStop) * cnt);
        }
        if (rhs.input) TVGERR("LOTTIE", "Must be populated!");
    }
};


struct TextDocument
{
    char* text = nullptr;
    float height;
    float shift;
    RGB32 color;
    struct {
        Point pos;
        Point size{};
    } bbox;
    struct {
        RGB32 color;
        float width;
        bool below = false;
    } stroke;
    char* name = nullptr;
    float size;
    float tracking = 0.0f;
    float justify = 0.0f;    //horizontal alignment
    uint8_t caps = 0;        //0: Regular, 1: AllCaps, 2: SmallCaps

    void copy(const TextDocument& rhs)
    {
        text = tvg::duplicate(rhs.text);
        name = tvg::duplicate(rhs.name);
    }
};

static inline int32_t REMAP255(float val)
{
    return (int32_t)nearbyintf(val * 255.0f);
}


static inline RGB32 operator-(const RGB32& lhs, const RGB32& rhs)
{
    return {lhs.r - rhs.r, lhs.g - rhs.g, lhs.b - rhs.b};
}


static inline RGB32 operator+(const RGB32& lhs, const RGB32& rhs)
{
    return {lhs.r + rhs.r, lhs.g + rhs.g, lhs.b + rhs.b};
}


static inline RGB32 operator*(const RGB32& lhs, float rhs)
{
    return {(int32_t)nearbyintf(lhs.r * rhs), (int32_t)nearbyintf(lhs.g * rhs), (int32_t)nearbyintf(lhs.b * rhs)};
}


static inline RGB32 lerp(const RGB32& s, const RGB32& e, float t)
{
    return {
        tvg::clamp((int32_t)(s.r + (e.r - s.r) * t), (int32_t)0, (int32_t)255),
        tvg::clamp((int32_t)(s.g + (e.g - s.g) * t), (int32_t)0, (int32_t)255),
        tvg::clamp((int32_t)(s.b + (e.b - s.b) * t), (int32_t)0, (int32_t)255)
    };
}

struct Point3
{
    float x = 0.0f, y = 0.0f, z = 0.0f;
};

static inline Point3 operator+(const Point3& a, const Point3& b)
{
    return {a.x + b.x, a.y + b.y, a.z + b.z};
}
static inline Point3 operator-(const Point3& a, const Point3& b)
{
    return {a.x - b.x, a.y - b.y, a.z - b.z};
}
static inline Point3 operator*(const Point3& a, float t)
{
    return {a.x * t, a.y * t, a.z * t};
}

}

#endif //_TVG_LOTTIE_COMMON_
