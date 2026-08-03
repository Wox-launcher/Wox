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

#ifndef _TVG_TTF_READER_H_
#define _TVG_TTF_READER_H_

#include "tvgSfntReader.h"

struct TtfReader : SfntReader
{
    TtfReader(uint8_t* data, uint32_t size) :
        SfntReader(data, size) {}

    bool header() override;
    bool positioning(uint32_t lglyph, uint32_t rglyph, Point& out) override;
    bool convert(SfntGlyph& glyph, uint32_t codepoint, RenderPath& path) override;

private:
    uint32_t loca = 0;  // index to location
    uint32_t glyf = 0;  // glyph outline
    uint32_t kern = 0;  // kerning

    uint32_t outlineOffset(uint32_t glyph);
    bool convert(RenderPath& path, SfntGlyph& glyph, uint32_t glyphOffset, const Point& offset, uint16_t depth);
    bool composite(RenderPath& path, SfntGlyph& glyph, uint32_t glyphOffset, const Point& offset, uint16_t depth);
    bool points(uint32_t outline, uint8_t* flags, Point* pts, uint32_t ptsCnt, const Point& offset);
    bool flags(uint32_t* outline, uint8_t* flags, uint32_t flagsCnt);
    uint32_t glyphMetrics(SfntGlyph& glyph);
};

#endif  //_TVG_TTF_READER_H_