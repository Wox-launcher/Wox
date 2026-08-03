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

#ifndef _TVG_SFNT_READER_H
#define _TVG_SFNT_READER_H

#include "tvgRender.h"

#define INVALID_GLYPH ((uint32_t)-1)

static inline int cmpu32(const void* a, const void* b)
{
    return memcmp(a, b, 4);
}

struct SfntGlyph
{
    uint32_t idx;        //glyph index
    float advance;       //advance width/height
    float lsb;           //left side bearing
    BBox bbox{};
};

struct SfntGlyphMetrics : SfntGlyph
{
    RenderPath path;     //outline path
};

struct SfntReader
{
    uint8_t* data = nullptr;
    uint32_t size = 0;

    struct
    {
        TextMetrics hhea;      //horizontal header info
        uint16_t unitsPerEm;
        uint16_t numHmtx;      //the number of Horizontal metrics table
        uint8_t locaFormat;    //0 for short offsets, 1 for long
    } metrics;

    SfntReader(uint8_t* data, uint32_t size) :
        data(data), size(size) {}
    virtual ~SfntReader() {}

    virtual bool header();
    virtual bool positioning(uint32_t lglyph, uint32_t rglyph, Point& out) = 0;
    virtual bool convert(SfntGlyph& glyph, uint32_t codepoint, RenderPath& path) = 0;

protected:
    uint32_t cmap = 0;  // character map
    uint32_t hmtx = 0;  // horizontal metrics
    uint32_t maxp = 0;  // maximum profile

    uint32_t table(const char* tag) const;
    bool validate(uint32_t offset, uint32_t margin) const;
    uint32_t cmap_12_13(uint32_t table, uint32_t codepoint, int which) const;
    uint32_t cmap_4(uint32_t table, uint32_t codepoint) const;
    uint32_t cmap_6(uint32_t table, uint32_t codepoint) const;
    uint32_t glyph(uint32_t codepoint);
    bool glyphMetrics(SfntGlyph& glyph);

    // convenient macros
    uint32_t u32(uint32_t offset) const
    {
        auto base = data + offset;
        return (base[0] << 24 | base[1] << 16 | base[2] << 8 | base[3]);
    }

    int32_t i32(uint32_t offset) const
    {
        return (int32_t)u32(offset);
    }

    uint16_t u16(uint32_t offset) const
    {
        auto base = data + offset;
        return (base[0] << 8 | base[1]);
    }

    int16_t i16(uint32_t offset) const
    {
        return (int16_t)u16(offset);
    }

    int8_t i8(uint32_t offset) const
    {
        return (int8_t)u8(offset);
    }

    uint8_t u8(uint32_t offset) const
    {
        return *(data + offset);
    }
};

#endif  //_TVG_SFNT_READER_H