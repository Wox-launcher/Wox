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

#ifndef _TVG_LOTTIE_MODIFIER_H_
#define _TVG_LOTTIE_MODIFIER_H_

#include "tvgCommon.h"
#include "tvgArray.h"
#include "tvgMath.h"
#include "tvgRender.h"

struct LottieModifier
{
    enum Type : uint8_t
    {
        Roundness = 0,
        Offset,
        PuckerBloat,
        ZigZag
    };

    LottieModifier* next = nullptr;
    Type type;

    LottieModifier(Type type) :
        type(type) {}

    virtual ~LottieModifier()
    {
        delete (next);
    }

    virtual void path(const RenderPath& in, RenderPath& out, Matrix* transform) = 0;
    virtual void polystar(const RenderPath& in, RenderPath& out, float outerRoundness, bool hasRoundness) = 0;
    virtual void rect(const RenderPath& in, RenderPath& out, const Point& pos, const Point& size, float r, bool clockwise) = 0;
    virtual void ellipse(const RenderPath& in, RenderPath& out, const Point& center, const Point& radius, bool clockwise) = 0;

    LottieModifier* decorate(LottieModifier* next);
};

struct LottieRoundnessModifier : LottieModifier
{
    static constexpr float ROUNDNESS_EPSILON = 1.0f;
    float r;

    LottieRoundnessModifier(float r) :
        LottieModifier(Roundness), r(r) {}

    void path(const RenderPath& in, RenderPath& out, Matrix* transform) override;
    void polystar(const RenderPath& in, RenderPath& out, float outerRoundness, bool hasRoundness) override;
    void rect(const RenderPath& in, RenderPath& out, const Point& pos, const Point& size, float r, bool clockwise) override;
    void ellipse(const RenderPath& in, RenderPath& out, const Point& center, const Point& radius, bool clockwise) override;

private:
    RenderPath& modify(const RenderPath& in, RenderPath& out, Matrix* transform);
    Point roundLineCorner(RenderPath& out, const Point& prev, const Point& curr, const Point& next, float r);
    Point roundCurveCorner(RenderPath& path, const Point& prev, const Point& ctrl1, const Point& curr, const Point& next, bool rounded, Point roundTo);
};

struct LottieOffsetModifier : LottieModifier
{
    float offset;
    float miterLimit;
    StrokeJoin join;

    LottieOffsetModifier(float offset, float miter = 4.0f, StrokeJoin join = StrokeJoin::Round) :
        LottieModifier(Offset), offset(offset), miterLimit(miter), join(join) {}

    void path(const RenderPath& in, RenderPath& out, Matrix* transform) override;
    void polystar(const RenderPath& in, RenderPath& out, float outerRoundness, bool hasRoundness) override;
    void rect(const RenderPath& in, RenderPath& out, const Point& pos, const Point& size, float r, bool clockwise) override;
    void ellipse(const RenderPath& in, RenderPath& out, const Point& center, const Point& radius, bool clockwise) override;

private:
    struct State
    {
        Line line{};
        Line firstLine{};
        uint32_t movetoOutIndex = 0;
        uint32_t movetoInIndex = 0;
        bool moveto = false;
    };

    RenderPath& modify(const RenderPath& in, RenderPath& out, Matrix* transform);
    void cubic(RenderPath& path, Point* pts, State& state, float offset, float threshold, bool& degeneratedLine3);
    bool intersected(Line& line1, Line& line2, Point& intersection, bool& inside);
    Line shift(Point& p1, Point& p2, float offset);
    void line(RenderPath& out, PathCommand* inCmds, uint32_t inCmdsCnt, Point* inPts, uint32_t& curPt, uint32_t curCmd, State& state, float offset, bool degenerated);
    void corner(RenderPath& out, Line& line, Line& nextLine, uint32_t movetoIndex, bool nextClose);
};

struct LottiePuckerBloatModifier : LottieModifier
{
    float amount;

    LottiePuckerBloatModifier(float amount) :
        LottieModifier(PuckerBloat), amount(amount) {}

    void path(const RenderPath& in, RenderPath& out, Matrix* transform) override;
    void polystar(const RenderPath& in, RenderPath& out, float outerRoundness, bool hasRoundness) override;
    void rect(const RenderPath& in, RenderPath& out, const Point& pos, const Point& size, float r, bool clockwise) override;
    void ellipse(const RenderPath& in, RenderPath& out, const Point& center, const Point& radius, bool clockwise) override;

private:
    Point center(const PathCommand* cmds, uint32_t cmdsCnt, const Point* pts);
};

struct LottieZigZagModifier : LottieModifier
{
    enum PointType : uint8_t { Corner = 1, Smooth = 2 };

    struct Vertex
    {
        Point v;
        Point in;
        Point out;
    };

    float amp;
    int freq;
    PointType point;

    LottieZigZagModifier(float amp, int freq, PointType point) :
        LottieModifier(ZigZag), amp(amp), freq(freq), point(point) {}

    void path(const RenderPath& in, RenderPath& out, Matrix* transform) override;
    void polystar(const RenderPath& in, RenderPath& out, float outerRoundness, bool hasRoundness) override;
    void rect(const RenderPath& in, RenderPath& out, const Point& pos, const Point& size, float r, bool clockwise) override;
    void ellipse(const RenderPath& in, RenderPath& out, const Point& center, const Point& radius, bool clockwise) override;

private:
    RenderPath& modify(const RenderPath& in, RenderPath& out, Matrix* transform);
    Vertex ridge(const Point& at, const Point& dir, float sign, float inAmp, float outAmp) const;
    void corner(uint32_t cur, float sign, const Array<Point>& verts, Array<Vertex>& ridges) const;
    float segment(uint32_t idx, float sign, const Array<Point>& verts, const Array<Point>& ins, const Array<Point>& outs, Array<Vertex>& ridges) const;
    void flush(bool closed, Array<Point>& verts, Array<Point>& ins, Array<Point>& outs, Array<Vertex>& ridges, RenderPath& out) const;
};

#endif