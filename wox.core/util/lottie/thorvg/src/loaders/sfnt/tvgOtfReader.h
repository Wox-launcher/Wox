/*
 * Copyright (c) 2026 ThorVG project. All rights reserved.

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

#ifndef _TVG_OTF_READER_H_
#define _TVG_OTF_READER_H_

#include "tvgSfntReader.h"

#define SUBROUTINE_MAX 10  // fixed by otf spec

struct OtfReader : SfntReader
{
    OtfReader(uint8_t* data, uint32_t size) :
        SfntReader(data, size)
    {}

    bool header() override;
    bool positioning(uint32_t lglyph, uint32_t rglyph, Point& out) override;
    bool convert(SfntGlyph& glyph, uint32_t codepoint, RenderPath& path) override;

private:
    struct SubRoutine
    {
        uint32_t p, end;
    };

    uint32_t cff = 0;            // postscript outlines
    uint32_t toCharStrings = 0;  // CharStrings offset in Top DICT INDEX in the cff table
    uint32_t gsubrs = 0;         // global Subr INDEX
    uint32_t subrs = 0;          // local Subr INDEX

    uint32_t skip(uint32_t idx);
    int32_t offset(uint32_t base, uint8_t size);

    bool CFF();
    bool name(uint32_t idx);
    bool DICT(Array<int32_t>& args, uint8_t b, uint32_t& ptr);
    uint32_t localSubrs(Array<int32_t>& args, uint32_t offset, uint32_t size);
    bool charStrings(RenderPath& path, uint32_t glyph);

    // CharStrings decoding sub-routines
    bool subRoutine(float operand, uint32_t subrs, uint32_t& p, uint32_t& end, SubRoutine* stack, int& ssp);
    void operand(Array<float>& v, uint8_t b, uint32_t& p);
    void alternatingCurve(Array<float>& v, Point& pos, RenderPath& path, bool horizontal);
    void alignedCurve(Array<float>& v, Point& pos, RenderPath& path, bool horizontal);
    void rrCurveTo(Array<float>& v, Point& pos, RenderPath& path);
    void rLineCurve(Array<float>& v, Point& pos, RenderPath& path);
    void rCurveLine(Array<float>& v, Point& pos, RenderPath& path);
};

#endif  //_TVG_OTF_READER_H_