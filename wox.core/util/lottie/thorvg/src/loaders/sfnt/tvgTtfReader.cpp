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

#include "tvgTtfReader.h"

/************************************************************************/
/* Internal Class Implementation                                        */
/************************************************************************/

// Returns the offset into the font that the glyph's outline is stored at
uint32_t TtfReader::outlineOffset(uint32_t glyph)
{
    uint32_t cur, next;

    if (metrics.locaFormat == 0) {
        auto base = loca + 2 * glyph;
        if (!validate(base, 4)) {
            TVGERR("SFNT", "invalid outline offset");
            return 0;
        }
        cur = 2U * (uint32_t)u16(base);
        next = 2U * (uint32_t)u16(base + 2);
    } else {
        auto base = loca + 4 * glyph;
        if (!validate(base, 8)) return 0;
        cur = u32(base);
        next = u32(base + 4);
    }
    if (cur == next) return 0;
    return glyf + cur;
}

bool TtfReader::composite(RenderPath& path, SfntGlyph& glyph, uint32_t glyphOffset, const Point& offset, uint16_t depth)
{
    #define ARG_1_AND_2_ARE_WORDS 0x0001
    #define ARGS_ARE_XY_VALUES 0x0002
    #define WE_HAVE_A_SCALE 0x0008
    #define MORE_COMPONENTS 0x0020
    #define WE_HAVE_AN_X_AND_Y_SCALE 0x0040
    #define WE_HAVE_A_TWO_BY_TWO 0x0080

    SfntGlyph compGlyph;
    Point compOffset;
    uint16_t flags;
    auto pointer = glyphOffset + 10;
    do {
        if (!validate(pointer, 4)) return false;
        flags = u16(pointer);
        compGlyph.idx = u16(pointer + 2U);
        if (compGlyph.idx == INVALID_GLYPH) continue;
        pointer += 4U;
        if (flags & ARG_1_AND_2_ARE_WORDS) {
            if (!validate(pointer, 4)) return false;
            // TODO: align to parent point
            compOffset = (flags & ARGS_ARE_XY_VALUES) ? Point{float(i16(pointer)), -float(i16(pointer + 2U))} : Point{0.0f, 0.0f};
            pointer += 4U;
        } else {
            if (!validate(pointer, 2)) return false;
            // TODO: align to parent point
            compOffset = (flags & ARGS_ARE_XY_VALUES) ? Point{float(i8(pointer)), -float(i8(pointer + 1U))} : Point{0.0f, 0.0f};
            pointer += 2U;
        }
        if (flags & WE_HAVE_A_SCALE) {
            if (!validate(pointer, 2)) return false;
            // TODO transformation
            // F2DOT14  scale;    /* Format 2.14 */
            pointer += 2U;
        } else if (flags & WE_HAVE_AN_X_AND_Y_SCALE) {
            if (!validate(pointer, 4)) return false;
            // TODO transformation
            // F2DOT14  xscale;    /* Format 2.14 */
            // F2DOT14  yscale;    /* Format 2.14 */
            pointer += 4U;
        } else if (flags & WE_HAVE_A_TWO_BY_TWO) {
            if (!validate(pointer, 8)) return false;
            // TODO transformation
            // F2DOT14  xscale;    /* Format 2.14 */
            // F2DOT14  scale01;   /* Format 2.14 */
            // F2DOT14  scale10;   /* Format 2.14 */
            // F2DOT14  yscale;    /* Format 2.14 */
            pointer += 8U;
        }
        if (!convert(path, compGlyph, glyphMetrics(compGlyph), offset + compOffset, depth)) return false;
    } while (flags & MORE_COMPONENTS);
    return true;
}

bool TtfReader::points(uint32_t outline, uint8_t* flags, Point* pts, uint32_t ptsCnt, const Point& offset)
{
    #define X_CHANGE_IS_SMALL 0x02
    #define X_CHANGE_IS_POSITIVE 0x10
    #define X_CHANGE_IS_ZERO 0x10
    #define Y_CHANGE_IS_SMALL 0x04
    #define Y_CHANGE_IS_ZERO 0x20
    #define Y_CHANGE_IS_POSITIVE 0x20

    long accum = 0L;

    for (uint32_t i = 0; i < ptsCnt; ++i) {
        if (flags[i] & X_CHANGE_IS_SMALL) {
            if (!validate(outline, 1)) {
                return false;
            }
            auto value = (long)u8(outline++);
            auto bit = (uint8_t) !!(flags[i] & X_CHANGE_IS_POSITIVE);
            accum -= (value ^ -bit) + bit;
        } else if (!(flags[i] & X_CHANGE_IS_ZERO)) {
            if (!validate(outline, 2)) return false;
            accum += i16(outline);
            outline += 2;
        }
        pts[i].x = offset.x + (float)accum;
    }

    accum = 0L;

    for (uint32_t i = 0; i < ptsCnt; ++i) {
        if (flags[i] & Y_CHANGE_IS_SMALL) {
            if (!validate(outline, 1)) return false;
            auto value = (long)u8(outline++);
            auto bit = (uint8_t) !!(flags[i] & Y_CHANGE_IS_POSITIVE);
            accum -= (value ^ -bit) + bit;
        } else if (!(flags[i] & Y_CHANGE_IS_ZERO)) {
            if (!validate(outline, 2)) return false;
            accum += i16(outline);
            outline += 2;
        }
        pts[i].y = offset.y - (float)accum;
    }
    return true;
}

bool TtfReader::flags(uint32_t* outline, uint8_t* flags, uint32_t flagsCnt)
{
    #define REPEAT_FLAG 0x08

    auto offset = *outline;
    uint8_t value = 0;
    uint8_t repeat = 0;

    for (uint32_t i = 0; i < flagsCnt; ++i) {
        if (repeat) {
            --repeat;
        } else {
            if (!validate(offset, 1)) return false;
            value = u8(offset++);
            if (value & REPEAT_FLAG) {
                if (!validate(offset, 1)) return false;
                repeat = u8(offset++);
            }
        }
        flags[i] = value;
    }
    *outline = offset;
    return true;
}

uint32_t TtfReader::glyphMetrics(SfntGlyph& glyph)
{
    if (!SfntReader::glyphMetrics(glyph)) return 0;

    auto glyphOffset = outlineOffset(glyph.idx);
    // glyph without outline
    if (glyphOffset == 0) {
        glyph.bbox = {};
        return 0;
    }
    if (!validate(glyphOffset, 10)) return 0;

    //read the bounding box from the font file verbatim.
    glyph.bbox.min.x = static_cast<float>(i16(glyphOffset + 2));
    glyph.bbox.min.y = static_cast<float>(i16(glyphOffset + 4));
    glyph.bbox.max.x = static_cast<float>(i16(glyphOffset + 6));
    glyph.bbox.max.y = static_cast<float>(i16(glyphOffset + 8));

    return glyphOffset;
}

bool TtfReader::convert(RenderPath& path, SfntGlyph& glyph, uint32_t glyphOffset, const Point& offset, uint16_t depth)
{
    #define ON_CURVE 0x01

    if (glyphOffset == 0) return true;

    auto outlineCnt = i16(glyphOffset);
    if (outlineCnt == 0) return false;
    if (outlineCnt < 0) {
        uint16_t maxComponentDepth = 16;
        if (validate(maxp, 32) && u32(maxp) >= 0x00010000U) {  // >= version 1.0
            maxComponentDepth = u16(maxp + 30);
        }
        if (depth > maxComponentDepth) return false;
        return composite(path, glyph, glyphOffset, offset, depth + 1);
    }
    auto cntrsCnt = (uint32_t)outlineCnt;
    auto outline = glyphOffset + 10;
    if (!validate(outline, cntrsCnt * 2 + 2)) return false;

    auto ret = false;
    auto ptsCnt = u16(outline + (cntrsCnt - 1) * 2) + 1;
    auto endPts = tvg::malloc<size_t>(cntrsCnt * sizeof(size_t));  // the index of the contour points.

    for (uint32_t i = 0; i < cntrsCnt; ++i) {
        endPts[i] = (uint32_t)u16(outline);
        outline += 2;
    }
    outline += 2U + u16(outline);

    auto flags = tvg::malloc<uint8_t>(ptsCnt * sizeof(uint8_t));
    if (this->flags(&outline, flags, ptsCnt)) {
        auto pts = tvg::malloc<Point>(ptsCnt * sizeof(Point));
        if (this->points(outline, flags, pts, ptsCnt, offset)) {
            // generate tvg paths
            path.cmds.reserve(ptsCnt);
            path.pts.reserve(ptsCnt);
            uint32_t begin = 0;
            for (uint32_t i = 0; i < cntrsCnt; ++i) {
                // contour must start with move to
                auto offCurve = !(flags[begin] & ON_CURVE);
                auto ptsBegin = offCurve ? (pts[begin] + pts[endPts[i]]) * 0.5f : pts[begin];
                path.moveTo(ptsBegin);
                auto cnt = endPts[i] - begin + 1;
                for (uint32_t x = 1; x < cnt; ++x) {
                    if (flags[begin + x] & ON_CURVE) {
                        if (offCurve) {
                            path.cubicTo(path.pts.last() + (2.0f / 3.0f) * (pts[begin + x - 1] - path.pts.last()), pts[begin + x] + (2.0f / 3.0f) * (pts[begin + x - 1] - pts[begin + x]), pts[begin + x]);
                            offCurve = false;
                        } else {
                            path.lineTo(pts[begin + x]);
                        }
                    } else {
                        if (offCurve) {
                            auto end = (pts[begin + x] + pts[begin + x - 1]) * 0.5f;
                            path.cubicTo(path.pts.last() + (2.0f / 3.0f) * (pts[begin + x - 1] - path.pts.last()), end + (2.0f / 3.0f) * (pts[begin + x - 1] - end), end);
                        } else {
                            offCurve = true;
                        }
                    }
                }
                if (offCurve) {
                    path.cubicTo(path.pts.last() + (2.0f / 3.0f) * (pts[begin + cnt - 1] - path.pts.last()), ptsBegin + (2.0f / 3.0f) * (pts[begin + cnt - 1] - ptsBegin), ptsBegin);
                }
                // contour must end with close
                path.close();
                begin = endPts[i] + 1;
            }
            ret = true;
        }
        tvg::free(pts);
    }
    tvg::free(flags);
    tvg::free(endPts);
    return ret;
}

/************************************************************************/
/* External Class Implementation                                        */
/************************************************************************/

bool TtfReader::header()
{
    if (!SfntReader::header()) return false;

    kern = table("kern");
    if (kern) {
        if (!validate(kern, 4)) return false;
        if (u16(kern) != 0) return false;
    }

    loca = table("loca");
    glyf = table("glyf");

    if (!loca || !glyf) return false;

    return true;
}

bool TtfReader::positioning(uint32_t lglyph, uint32_t rglyph, Point& out)
{
    #define HORIZONTAL_KERNING 0x01
    #define MINIMUM_KERNING 0x02
    #define CROSS_STREAM_KERNING 0x04

    auto kern = this->kern;
    if (!kern) return false;

    // kern tables
    auto tableCnt = u16(kern + 2);
    kern += 4;

    while (tableCnt > 0) {
        // read subtable header.
        if (!validate(kern, 6)) return false;
        auto length = u16(kern + 2);
        auto format = u8(kern + 4);
        auto flags = u8(kern + 5);
        kern += 6;

        if (format == 0 && (flags & HORIZONTAL_KERNING) && !(flags & MINIMUM_KERNING)) {
            // read format 0 header.
            if (!validate(kern, 8)) return false;
            auto pairCnt = u16(kern);
            kern += 8;

            // look up character code pair via binary search.
            uint8_t key[4];
            key[0] = (lglyph >> 8) & 0xff;
            key[1] = lglyph & 0xff;
            key[2] = (rglyph >> 8) & 0xff;
            key[3] = rglyph & 0xff;

            auto match = bsearch(key, data + kern, pairCnt, 6, cmpu32);

            if (match) {
                auto value = i16((uint32_t)((uint8_t*)match - data + 4));
                if (flags & CROSS_STREAM_KERNING) out.y += value;
                else out.x += value;
            }
        }
        kern += length;
        --tableCnt;
    }

    return true;
}

bool TtfReader::convert(SfntGlyph& glyph, uint32_t codepoint, RenderPath& path)
{
    glyph.idx = SfntReader::glyph(codepoint);
    if (glyph.idx == INVALID_GLYPH) return false;
    return convert(path, glyph, glyphMetrics(glyph), {}, 1);
}