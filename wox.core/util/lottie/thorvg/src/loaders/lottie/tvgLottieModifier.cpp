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

#include "tvgLottieModifier.h"

/************************************************************************/
/* LottieModifier                                                       */
/************************************************************************/

static bool _colinear(const Point* p)
{
    return tvg::zero(*p - *(p + 1)) && tvg::zero(*(p + 2) - *(p + 3));
}

static bool _sharpCorner(const Point* p)
{
    return tvg::zero(*p - *(p + 1)) && tvg::zero(*(p + 1) - *(p + 2));
}

LottieModifier* LottieModifier::decorate(LottieModifier* next)
{
    // let the offset modifer to the end in this chain
    // roundness don't handle lines so far, so roundenss should handled earilier
    // remove this trick once roundess has full coverage.
    // see LottieRoundnessModifier::modify()
    auto p = this;
    while (p) {
        if (!p->next && next->type == Offset) {
            p->next = next;
            return this;
        }
        p = p->next;
    }

    next->next = this;
    return next;
}

/************************************************************************/
/* LottieRoundnessModifier                                              */
/************************************************************************/

Point LottieRoundnessModifier::roundLineCorner(RenderPath& out, const Point& prev, const Point& curr, const Point& next, float r)
{
    auto lenPrev = length(prev - curr);
    auto rPrev = lenPrev > 0.0f ? 0.5f * std::min(lenPrev * 0.5f, r) / lenPrev : 0.0f;
    auto lenNext = length(next - curr);
    auto rNext = lenNext > 0.0f ? 0.5f * std::min(lenNext * 0.5f, r) / lenNext : 0.0f;
    auto dPrev = rPrev * (curr - prev);
    auto dNext = rNext * (curr - next);

    out.lineTo(curr - 2.0f * dPrev);
    auto ret = curr - 2.0f * dNext;
    out.cubicTo(curr - dPrev, curr - dNext, ret);
    return ret;
}

Point LottieRoundnessModifier::roundCurveCorner(RenderPath& path, const Point& prev, const Point& ctrl1, const Point& curr, const Point& next, bool rounded, Point roundTo)
{
    auto lenPrev = length(prev - curr);
    auto tPrev = lenPrev > 0.0f ? std::min(lenPrev * 0.5f, r) / lenPrev : 0.0f;
    auto arcStart = tvg::lerp(curr, prev, tPrev);
    path.cubicTo(rounded ? roundTo : ctrl1, arcStart, arcStart);

    auto lenNext = length(next - curr);
    auto tNext = lenNext > 0.0f ? std::min(lenNext * 0.5f, r) / lenNext : 0.0f;
    auto arcEnd = tvg::lerp(curr, next, tNext);
    path.cubicTo(tvg::lerp(arcStart, curr, 0.5f), tvg::lerp(arcEnd, curr, 0.5f), arcEnd);

    return arcEnd;
}

RenderPath& LottieRoundnessModifier::modify(const RenderPath& in, RenderPath& out, Matrix* transform)
{
    auto& path = (next) ? RenderPath::scratch() : out;
    path.cmds.reserve(in.cmds.count * 2);
    path.pts.reserve((uint32_t)(in.pts.count * 1.5));
    auto pivot = path.pts.count;
    uint32_t startIndex = 0;
    auto rounded = false;
    Point roundTo;

    // TODO: the line case is omitted.

    for (uint32_t iCmds = 0, iPts = 0; iCmds < in.cmds.count; ++iCmds) {
        switch (in.cmds[iCmds]) {
            case PathCommand::MoveTo: {
                startIndex = path.pts.count;
                path.moveTo(in.pts[iPts++]);
                break;
            }
            case PathCommand::CubicTo: {
                auto hasNext = iCmds < in.cmds.count - 1;
                auto nextCmd = hasNext ? in.cmds[iCmds + 1] : PathCommand::MoveTo;
                if (hasNext) {
                    if (_colinear(&in.pts[iPts - 1])) {
                        auto& prev = in.pts[iPts - 1];
                        auto& curr = in.pts[iPts + 2];
                        if (nextCmd == PathCommand::CubicTo && tvg::zero(in.pts[iPts + 2] - in.pts[iPts + 3])) {
                            roundTo = roundLineCorner(path, prev, curr, in.pts[iPts + 5], r);
                            iPts += 3;
                            rounded = true;
                            continue;
                        } else if (nextCmd == PathCommand::Close) {
                            roundTo = roundLineCorner(path, prev, curr, in.pts[2], r);
                            path.pts[startIndex] = path.pts.last();
                            iPts += 3;
                            rounded = true;
                            continue;
                        }
                    } else if (nextCmd == PathCommand::CubicTo && _sharpCorner(&in.pts[iPts + 1])) {
                        roundTo = roundCurveCorner(path, in.pts[iPts - 1], in.pts[iPts], in.pts[iPts + 2], in.pts[iPts + 5], rounded, roundTo);
                        iPts += 3;
                        rounded = true;
                        continue;
                    }
                }
                path.cubicTo(rounded ? roundTo : in.pts[iPts], in.pts[iPts + 1], in.pts[iPts + 2]);
                iPts += 3;
                break;
            }
            case PathCommand::Close: {
                path.close();
                break;
            }
            default: break;
        }
        rounded = false;
    }
    if (transform) {
        for (auto i = pivot; i < path.pts.count; ++i) {
            path.pts[i] *= *transform;
        }
    }

    return path;
}

void LottieRoundnessModifier::path(const RenderPath& in, RenderPath& out, Matrix* transform)
{
    auto& result = modify(in, out, transform);
    if (next) return next->path(result, out, nullptr);
}

void LottieRoundnessModifier::polystar(const RenderPath& in, RenderPath& out, float outerRoundness, bool hasRoundness)
{
    constexpr auto ROUNDED_POLYSTAR_MAGIC_NUMBER = 0.47829f;

    auto& path = (next) ? RenderPath::scratch() : out;

    auto len = length(in.pts[1] - in.pts[2]);
    auto r = len > 0.0f ? ROUNDED_POLYSTAR_MAGIC_NUMBER * std::min(len * 0.5f, this->r) / len : 0.0f;

    if (hasRoundness) {
        path.cmds.grow((uint32_t)(1.5 * in.cmds.count));
        path.pts.grow((uint32_t)(4.5 * in.cmds.count));

        int start = 3 * tvg::zero(outerRoundness);
        path.moveTo(in.pts[start]);

        for (uint32_t i = 1 + start; i < in.pts.count; i += 6) {
            auto& prev = in.pts[i];
            auto& curr = in.pts[i + 2];
            auto& next = (i < in.pts.count - start) ? in.pts[i + 4] : in.pts[2];
            auto& nextCtrl = (i < in.pts.count - start) ? in.pts[i + 5] : in.pts[3];
            auto dNext = r * (curr - next);
            auto dPrev = r * (curr - prev);

            auto p0 = curr - 2.0f * dPrev;
            auto p1 = curr - dPrev;
            auto p2 = curr - dNext;
            auto p3 = curr - 2.0f * dNext;

            path.cubicTo(prev, p0, p0);
            path.cubicTo(p1, p2, p3);
            path.cubicTo(p3, next, nextCtrl);
        }
    } else {
        path.cmds.grow(2 * in.cmds.count);
        path.pts.grow(4 * in.cmds.count);

        auto dPrev = r * (in.pts[1] - in.pts[0]);
        auto p = in.pts[0] + 2.0f * dPrev;
        path.moveTo(p);

        for (uint32_t i = 1; i < in.pts.count; ++i) {
            auto& curr = in.pts[i];
            auto& next = (i == in.pts.count - 1) ? in.pts[1] : in.pts[i + 1];
            auto dNext = r * (curr - next);

            auto p0 = curr - 2.0f * dPrev;
            auto p1 = curr - dPrev;
            auto p2 = curr - dNext;
            auto p3 = curr - 2.0f * dNext;

            path.lineTo(p0);
            path.cubicTo(p1, p2, p3);

            dPrev = -1.0f * dNext;
        }
    }
    path.cmds.push(PathCommand::Close);

    if (next) return next->polystar(path, out, outerRoundness, hasRoundness);
}

void LottieRoundnessModifier::rect(const RenderPath& in, RenderPath& out, const Point& pos, const Point& size, float r, bool clockwise)
{
    auto& path = (next) ? RenderPath::scratch() : out;

    if (r == 0.0f) r = std::min(this->r, std::max(size.x, size.y) * 0.5f);

    // we know this is the first request in the chain because other modifers would not trigger rect() call
    path.addRect(pos.x, pos.y, size.x, size.y, r, r, clockwise);

    if (next) return next->path(path, out, nullptr);
}

void LottieRoundnessModifier::ellipse(const RenderPath& in, RenderPath& out, const Point& center, const Point& radius, bool clockwise)
{
    // bypass because it's already a circle.
    if (next) return next->ellipse(in, out, center, radius, clockwise);
    else {
        out.cmds.push(in.cmds);
        out.pts.push(in.pts);
    }
}

/************************************************************************/
/* LottieOffsetModifier                                                 */
/************************************************************************/

bool LottieOffsetModifier::intersected(Line& line1, Line& line2, Point& intersection, bool& inside)
{
    if (tvg::zero(line1.pt2 - line2.pt1)) {
        intersection = line1.pt2;
        inside = true;
        return true;
    }

    constexpr float epsilon = 1e-3f;
    float denom = (line1.pt2.x - line1.pt1.x) * (line2.pt2.y - line2.pt1.y) - (line1.pt2.y - line1.pt1.y) * (line2.pt2.x - line2.pt1.x);
    if (fabsf(denom) < epsilon) return false;

    float t = ((line2.pt1.x - line1.pt1.x) * (line2.pt2.y - line2.pt1.y) - (line2.pt1.y - line1.pt1.y) * (line2.pt2.x - line2.pt1.x)) / denom;
    float u = ((line2.pt1.x - line1.pt1.x) * (line1.pt2.y - line1.pt1.y) - (line2.pt1.y - line1.pt1.y) * (line1.pt2.x - line1.pt1.x)) / denom;

    intersection.x = line1.pt1.x + t * (line1.pt2.x - line1.pt1.x);
    intersection.y = line1.pt1.y + t * (line1.pt2.y - line1.pt1.y);
    inside = t >= -epsilon && t <= 1.0f + epsilon && u >= -epsilon && u <= 1.0f + epsilon;

    return true;
}

Line LottieOffsetModifier::shift(Point& p1, Point& p2, float offset)
{
    auto scaledNormal = normal(p1, p2) * offset;
    return {p1 + scaledNormal, p2 + scaledNormal};
}

void LottieOffsetModifier::corner(RenderPath& out, Line& line, Line& nextLine, uint32_t movetoOutIndex, bool nextClose)
{
    bool inside{};
    Point intersect{};
    if (intersected(line, nextLine, intersect, inside)) {
        if (inside) {
            if (nextClose) out.pts[movetoOutIndex] = intersect;
            out.pts.push(intersect);
        } else {
            out.pts.push(line.pt2);
            if (join == StrokeJoin::Round) {
                out.cubicTo((line.pt2 + intersect) * 0.5f, (nextLine.pt1 + intersect) * 0.5f, nextLine.pt1);
            } else if (join == StrokeJoin::Miter) {
                auto norm = normal(line.pt1, line.pt2);
                auto nextNorm = normal(nextLine.pt1, nextLine.pt2);
                auto miterDirection = (norm + nextNorm) / length(norm + nextNorm);
                if (1.0f <= miterLimit * fabsf(miterDirection.x * norm.x + miterDirection.y * norm.y)) out.lineTo(intersect);
                out.lineTo(nextLine.pt1);
            } else out.lineTo(nextLine.pt1);
        }
    } else out.pts.push(line.pt2);
}

void LottieOffsetModifier::line(RenderPath& out, PathCommand* inCmds, uint32_t inCmdsCnt, Point* inPts, uint32_t& curPt, uint32_t curCmd, State& state, float offset, bool degenerated)
{
    if (tvg::zero(inPts[curPt - 1] - inPts[curPt])) {
        ++curPt;
        return;
    }

    if (inCmds[curCmd - 1] != PathCommand::LineTo) state.line = shift(inPts[curPt - 1], inPts[curPt], offset);

    if (state.moveto) {
        state.movetoOutIndex = out.pts.count;
        out.moveTo(state.line.pt1);
        state.firstLine = state.line;
        state.moveto = false;
    }

    auto nonDegeneratedCubic = [&](uint32_t cmd, uint32_t pt) {
        return inCmds[cmd] == PathCommand::CubicTo && !tvg::zero(inPts[pt] - inPts[pt + 1]) && !tvg::zero(inPts[pt + 2] - inPts[pt + 3]);
    };

    out.cmds.push(PathCommand::LineTo);

    if (curCmd + 1 == inCmdsCnt || inCmds[curCmd + 1] == PathCommand::MoveTo || nonDegeneratedCubic(curCmd + 1, curPt + degenerated)) {
        out.pts.push(state.line.pt2);
        ++curPt;
        return;
    }

    Line nextLine = state.firstLine;
    if (inCmds[curCmd + 1] == PathCommand::LineTo) nextLine = shift(inPts[curPt + degenerated], inPts[curPt + 1 + degenerated], offset);
    else if (inCmds[curCmd + 1] == PathCommand::CubicTo) nextLine = shift(inPts[curPt + 1 + degenerated], inPts[curPt + 2 + degenerated], offset);
    else if (inCmds[curCmd + 1] == PathCommand::Close && !tvg::zero(inPts[curPt + degenerated] - inPts[state.movetoInIndex + degenerated]))
        nextLine = shift(inPts[curPt + degenerated], inPts[state.movetoInIndex + degenerated], offset);

    corner(out, state.line, nextLine, state.movetoOutIndex, inCmds[curCmd + 1] == PathCommand::Close);

    state.line = nextLine;
    ++curPt;
}

void LottieOffsetModifier::cubic(RenderPath& path, Point* pts, State& state, float offset, float threshold, bool& degeneratedLine3)
{
    Point intersect{};
    Array<Bezier> stack{5};
    bool degeneratedLine1{};
    bool inside{};
    stack.push({pts[0], pts[1], pts[2], pts[3]});

    while (!stack.empty()) {
        auto& bezier = stack.last();
        auto len = tvg::length(bezier.start - bezier.ctrl1) + tvg::length(bezier.ctrl1 - bezier.ctrl2) + tvg::length(bezier.ctrl2 - bezier.end);

        if (len > threshold * bezier.length() && len > 1.0f) {
            Bezier next;
            bezier.split(0.5f, next);
            stack.push(next);
            continue;
        }
        stack.pop();

        degeneratedLine1 = tvg::zero(bezier.start - bezier.ctrl1);
        auto line1 = degeneratedLine1 ? state.line : shift(bezier.start, bezier.ctrl1, offset);
        auto line2 = shift(bezier.ctrl1, bezier.ctrl2, offset);

        //line3 from the previous iteration was degenerated to a point - calculate intersection with the last valid line (state.line)
        if (degeneratedLine3) {
            intersected(degeneratedLine1 ? line2 : line1, state.line, intersect, inside);
            path.pts.push(intersect);
            path.pts.push(intersect);
        }

        degeneratedLine3 = tvg::zero(bezier.ctrl2 - bezier.end);
        auto& line3 = state.line = degeneratedLine3 ? line2 : shift(bezier.ctrl2, bezier.end, offset);

        if (state.moveto) {
            state.movetoOutIndex = path.pts.count;
            path.moveTo(line1.pt1);
            state.firstLine = line1;
            state.moveto = false;
        }

        if (degeneratedLine1) path.pts.push(path.pts.last());
        else {
            intersected(line1, line2, intersect, inside);
            path.pts.push(intersect);
        }

        if (!degeneratedLine3) {
            intersected(line2, line3, intersect, inside);
            path.pts.push(intersect);
            path.pts.push(line3.pt2);
        }
        path.cmds.push(PathCommand::CubicTo);
    }
}

RenderPath& LottieOffsetModifier::modify(const RenderPath& in, RenderPath& out, TVG_UNUSED Matrix* transform)
{
    auto clockwise = [](Point* pts, uint32_t n) {
        auto area = 0.0f;
        for (uint32_t i = 0; i < n - 1; i++) {
            area += cross(pts[i], pts[i + 1]);
        }
        area += cross(pts[n - 1], pts[0]);
        return area < 0.0f;
    };

    auto& path = (next) ? RenderPath::scratch() : out;
    path.cmds.reserve(in.cmds.count * 2);
    path.pts.reserve(in.pts.count * (join == StrokeJoin::Round ? 4 : 2));

    State state;
    auto offset = clockwise(in.pts.data, in.pts.count) ? this->offset : -this->offset;
    auto threshold = 1.0f / fabsf(offset) + 1.0f;
    bool degeneratedLine3{};

    for (uint32_t iCmd = 0, iPt = 0; iCmd < in.cmds.count; ++iCmd) {
        switch (in.cmds[iCmd]) {
            case PathCommand::MoveTo: {
                state.moveto = true;
                state.movetoInIndex = iPt++;
                break;
            }
            case PathCommand::LineTo: {
                line(out, in.cmds.data, in.cmds.count, in.pts.data, iPt, iCmd, state, offset, false);
                break;
            }
            case PathCommand::CubicTo: {
                //cubic degenerated to a line
                if (_colinear(in.pts.data + iPt - 1)) {
                    ++iPt;
                    line(out, in.cmds.data, in.cmds.count, in.pts.data, iPt, iCmd, state, offset, true);
                    ++iPt;
                    continue;
                }
                cubic(path, in.pts.data + iPt - 1, state, offset, threshold, degeneratedLine3);
                iPt += 3;
                break;
            }
            default: {
                if (!tvg::zero(in.pts[iPt - 1] - in.pts[state.movetoInIndex])) {
                    path.cmds.push(PathCommand::LineTo);
                    corner(out, state.line, state.firstLine, state.movetoOutIndex, true);
                }
                path.cmds.push(PathCommand::Close);
            }
        }
    }
    return path;
}

void LottieOffsetModifier::path(const RenderPath& in, RenderPath& out, Matrix* transform)
{
    auto& result = modify(in, out, nullptr);
    if (next) next->path(result, out, nullptr);
}

void LottieOffsetModifier::polystar(const RenderPath& in, RenderPath& out, float outerRoundness, bool hasRoundness)
{
    auto& result = modify(in, out, nullptr);
    if (next) next->polystar(result, out, outerRoundness, hasRoundness);
}

void LottieOffsetModifier::rect(const RenderPath& in, RenderPath& out, const Point& pos, const Point& size, float r, bool clockwise)
{
    path(in, out, nullptr);
}

void LottieOffsetModifier::ellipse(const RenderPath& in, RenderPath& out, const Point& center, const Point& radius, bool clockwise)
{
    auto& path = (next) ? RenderPath::scratch() : out;
    // we know this is the first request in the chain because other modifers would not trigger ellipse() call
    path.addCircle(center.x, center.y, radius.x + offset, radius.y + offset, clockwise);
    if (next) return next->ellipse(path, out, center, radius, clockwise);
}

/************************************************************************/
/* LottiePuckerBloatModifier                                            */
/************************************************************************/

Point LottiePuckerBloatModifier::center(const PathCommand* cmds, uint32_t cmdsCnt, const Point* pts)
{
    Point center{};
    auto count = 0;
    auto p = pts;
    auto start = p;

    for (uint32_t i = 0; i < cmdsCnt; ++i) {
        switch (cmds[i]) {
            case PathCommand::MoveTo: {
                start = p;
                ++p;
                break;
            }
            case PathCommand::CubicTo: {
                center = center + *(p - 1) + *p + *(p + 1) + *(p + 2);
                p += 3;
                count += 4;
                break;
            }
            case PathCommand::LineTo: {
                center = center + *(p - 1) + *p;
                ++p;
                count += 2;
                break;
            }
            case PathCommand::Close: {
                if (!tvg::zero(*(p - 1) - *start)) {
                    center = center + *(p - 1) + *start;
                    count += 2;
                }
                break;
            }
        }
    }
    return count > 0 ? center / (float)count : Point{0, 0};
}

void LottiePuckerBloatModifier::path(const RenderPath& in, RenderPath& out, Matrix* transform)
{
    auto& path = next ? RenderPath::scratch() : out;

    // LineTo segments are expanded to CubicTo, so pts capacity can grow up to 3x
    path.cmds.reserve(in.cmds.count);
    path.pts.reserve(in.pts.count * 3);

    auto center = this->center(in.cmds.data, in.cmds.count, in.pts.data);
    auto a = amount * 0.01f;
    auto pts = in.pts.data;
    auto startPts = pts;

    for (uint32_t i = 0; i < in.cmds.count; ++i) {
        switch (in.cmds[i]) {
            case PathCommand::MoveTo: {
                startPts = pts;
                // anchor points move toward center
                path.pts.push(*pts + (center - *pts) * a);
                path.cmds.push(PathCommand::MoveTo);
                ++pts;
                break;
            }
            case PathCommand::CubicTo: {
                // control points move away from center, end (anchor) moves toward center
                path.pts.push(*pts - (center - *pts) * a);
                path.pts.push(*(pts + 1) - (center - *(pts + 1)) * a);
                path.pts.push(*(pts + 2) + (center - *(pts + 2)) * a);
                path.cmds.push(PathCommand::CubicTo);
                pts += 3;
                break;
            }
            case PathCommand::LineTo: {
                // convert to CubicTo: prev and curr as control points (away), curr as end (toward)
                path.pts.push(*(pts - 1) - (center - *(pts - 1)) * a);
                path.pts.push(*pts - (center - *pts) * a);
                path.pts.push(*pts + (center - *pts) * a);
                path.cmds.push(PathCommand::CubicTo);
                ++pts;
                break;
            }
            case PathCommand::Close: {
                // if last pt != start pt, add implicit closing segment as CubicTo
                if (!tvg::zero(*(pts - 1) - *startPts)) {
                    path.pts.push(*(pts - 1) - (center - *(pts - 1)) * a);
                    path.pts.push(*startPts - (center - *startPts) * a);
                    path.pts.push(*startPts + (center - *startPts) * a);
                    path.cmds.push(PathCommand::CubicTo);
                }
                path.cmds.push(PathCommand::Close);
                break;
            }
        }
    }

    if (transform) {
        for (uint32_t i = 0; i < path.pts.count; ++i) {
            path.pts[i] *= *transform;
        }
    }

    if (next) return next->path(path, out, transform);
}

void LottiePuckerBloatModifier::polystar(const RenderPath& in, RenderPath& out, TVG_UNUSED float, TVG_UNUSED bool)
{
    path(in, out, nullptr);
}

void LottiePuckerBloatModifier::rect(const RenderPath& in, RenderPath& out, TVG_UNUSED const Point&, TVG_UNUSED const Point&, TVG_UNUSED float, TVG_UNUSED bool)
{
    path(in, out, nullptr);
}

void LottiePuckerBloatModifier::ellipse(const RenderPath& in, RenderPath& out, TVG_UNUSED const Point&, TVG_UNUSED const Point&, TVG_UNUSED bool)
{
    path(in, out, nullptr);
}

/************************************************************************/
/* LottieZigZagModifier                                                 */
/************************************************************************/

LottieZigZagModifier::Vertex LottieZigZagModifier::ridge(const Point& at, const Point& dir, float sign, float inAmp, float outAmp) const
{
    auto len = tvg::length(dir);
    if (len < FLT_EPSILON) return {at, at, at};
    auto pos = at + tvg::normal({}, dir) * (sign * amp);
    auto unit = dir / len;
    return {pos, pos - unit * inAmp, pos + unit * outAmp};
}

void LottieZigZagModifier::corner(uint32_t cur, float sign, const Array<Point>& verts, Array<Vertex>& ridges) const
{
    auto curIdx = cur % verts.count;
    auto prevIdx = (curIdx == 0) ? verts.count - 1 : curIdx - 1;
    auto nextIdx = (curIdx + 1) % verts.count;
    auto dir = verts[nextIdx] - verts[prevIdx];
    auto outAmp = 0.0f, inAmp = 0.0f;
    if (point == Smooth) {
        outAmp = tvg::length(verts[nextIdx] - verts[curIdx]) / ((freq + 1) * 2);
        inAmp = tvg::length(verts[prevIdx] - verts[curIdx]) / ((freq + 1) * 2);
    }
    ridges.push(ridge(verts[curIdx], dir, sign, inAmp, outAmp));
}


float LottieZigZagModifier::segment(uint32_t idx, float sign, const Array<Point>& verts, const Array<Point>& ins, const Array<Point>& outs, Array<Vertex>& ridges) const
{
    if (freq == 0) return sign;
    auto i0 = idx;
    auto i1 = (idx + 1) % verts.count;
    Point p0 = verts[i0], p1 = outs[i0], p2 = ins[i1], p3 = verts[i1];
    auto sAmp = (point == Smooth) ? tvg::length(p3 - p0) / ((freq + 1) * 2) : 0.0f;
    Bezier bz{p0, p1, p2, p3};
    for (int k = 0; k < freq; ++k) {
        auto t = float(k + 1) / float(freq + 1);
        ridges.push(ridge(bz.at(t), bz.tangent(t), sign, sAmp, sAmp));
        sign = -sign;
    }
    return sign;
}


void LottieZigZagModifier::flush(bool closed, Array<Point>& verts, Array<Point>& ins, Array<Point>& outs, Array<Vertex>& ridges, RenderPath& dst) const
{
    if (verts.count == 0) return;

    //collapse closing duplicate vertex
    if (closed && verts.count > 1 && tvg::zero(verts.last() - verts[0])) {
        ins[0] = ins.last();
        verts.pop();
        ins.pop();
        outs.pop();
    }
    if (verts.count < 2) return;

    auto segCount = closed ? verts.count : verts.count - 1;

    ridges.clear();
    ridges.reserve((segCount + 1) * (freq + 1));

    auto sign = 1.0f;
    corner(0, sign, verts, ridges);
    for (uint32_t i = 0; i < segCount; ++i) {
        sign = segment(i, -sign, verts, ins, outs, ridges);
        corner(i + 1, sign, verts, ridges);
    }

    if (ridges.count == 0) return;
    dst.moveTo(ridges[0].v);
    for (uint32_t i = 1; i < ridges.count; ++i) {
        dst.cubicTo(ridges[i - 1].out, ridges[i].in, ridges[i].v);
    }
    if (closed) dst.close();
}


RenderPath& LottieZigZagModifier::modify(const RenderPath& in, RenderPath& out, Matrix* transform)
{
    auto& path = (next) ? RenderPath::scratch() : out;
    auto pivot = path.pts.count;
    path.cmds.reserve(path.cmds.count + in.cmds.count * (freq + 2));
    path.pts.reserve(path.pts.count + in.pts.count * (freq + 2));

    Array<Point> verts, ins, outs;
    Array<Vertex> ridges;
    auto closed = false;

    for (uint32_t iCmd = 0, iPt = 0; iCmd < in.cmds.count; ++iCmd) {
        switch (in.cmds[iCmd]) {
            case PathCommand::MoveTo: {
                flush(closed, verts, ins, outs, ridges, path);
                verts.clear();
                ins.clear();
                outs.clear();
                closed = false;
                auto first = in.pts[iPt++];
                verts.push(first);
                ins.push(first);
                outs.push(first);
                break;
            }
            case PathCommand::CubicTo: {
                outs.last() = in.pts[iPt];
                ins.push(in.pts[iPt + 1]);
                verts.push(in.pts[iPt + 2]);
                outs.push(in.pts[iPt + 2]);
                iPt += 3;
                break;
            }
            case PathCommand::LineTo: {
                outs.last() = verts.last();
                ins.push(in.pts[iPt]);
                verts.push(in.pts[iPt]);
                outs.push(in.pts[iPt]);
                ++iPt;
                break;
            }
            case PathCommand::Close: {
                closed = true;
                break;
            }
            default: break;
        }
    }
    flush(closed, verts, ins, outs, ridges, path);

    if (transform) {
        for (auto i = pivot; i < path.pts.count; ++i) {
            path.pts[i] *= *transform;
        }
    }

    return path;
}

void LottieZigZagModifier::path(const RenderPath& in, RenderPath& out, Matrix* transform)
{
    auto& result = modify(in, out, transform);
    if (next) next->path(result, out, nullptr);
}

void LottieZigZagModifier::polystar(const RenderPath& in, RenderPath& out, TVG_UNUSED float, TVG_UNUSED bool)
{
    path(in, out, nullptr);
}

void LottieZigZagModifier::rect(const RenderPath& in, RenderPath& out, TVG_UNUSED const Point&, TVG_UNUSED const Point&, TVG_UNUSED float, TVG_UNUSED bool)
{
    path(in, out, nullptr);
}

void LottieZigZagModifier::ellipse(const RenderPath& in, RenderPath& out, TVG_UNUSED const Point&, TVG_UNUSED const Point&, TVG_UNUSED bool)
{
    path(in, out, nullptr);
}