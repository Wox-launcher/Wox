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

#include "tvgSfntReader.h"

/************************************************************************/
/* Internal Class Implementation                                        */
/************************************************************************/

bool SfntReader::validate(uint32_t offset, uint32_t margin) const
{
    if ((offset > size) || (size - offset < margin)) {
        TVGERR("SFNT", "Invalidate data");
        return false;
    }
    return true;
}

uint32_t SfntReader::table(const char* tag) const
{
    auto tableCnt = u16(4);
    if (!validate(12, (uint32_t)tableCnt * 16)) return 0;
    auto match = bsearch(tag, data + 12, tableCnt, 16, cmpu32);
    if (!match) {
        TVGLOG("SFNT", "No searching table = %s", tag);
        return 0;
    }
    return u32((uint32_t)((uint8_t*)match - data + 8));
}

uint32_t SfntReader::cmap_12_13(uint32_t table, uint32_t codepoint, int fmt) const
{
    // A minimal header is 16 bytes
    auto len = u32(table + 4);
    if (len < 16) return -1;
    if (!validate(table, len)) return -1;

    auto entryCnt = u32(table + 12);

    for (uint32_t i = 0; i < entryCnt; ++i) {
        auto firstCode = u32(table + (i * 12) + 16);
        auto lastCode = u32(table + (i * 12) + 16 + 4);
        if (codepoint < firstCode || codepoint > lastCode) continue;
        auto glyphOffset = u32(table + (i * 12) + 16 + 8);
        if (fmt == 12) return (codepoint - firstCode) + glyphOffset;
        else return glyphOffset;
    }
    return -1;
}

uint32_t SfntReader::cmap_4(uint32_t table, uint32_t codepoint) const
{
    // cmap format 4 only supports the Unicode BMP.
    if (codepoint > 0xffff || !validate(table, 8)) return -1;

    auto segmentCnt = u16(table);
    if (segmentCnt == 0 || (segmentCnt & 1)) return -1;

    // find starting positions of the relevant arrays.
    auto endCodes = table + 8;
    auto startCodes = endCodes + segmentCnt + 2;
    auto idDeltas = startCodes + segmentCnt;
    auto idRangeOffsets = idDeltas + segmentCnt;

    if (!validate(idRangeOffsets, segmentCnt)) return -1;

    // find the segment that contains shortCode by binary searching over the highest codes in the segments.
    // binary search, but returns the next highest element if key could not be found.
    uint8_t* segment = nullptr;
    auto n = segmentCnt / 2;
    if (n > 0) {
        uint8_t key[2] = {(uint8_t)(codepoint >> 8), (uint8_t)codepoint};
        auto bytes = data + endCodes;
        auto low = 0;
        auto high = n - 1;
        while (low != high) {
            auto mid = low + (high - low) / 2;
            auto sample = bytes + mid * 2;
            if (memcmp(key, sample, 2) > 0) {
                low = mid + 1;
            } else {
                high = mid;
            }
        }
        segment = (bytes + low * 2);
    }

    auto segmentIdx = static_cast<uint32_t>(segment - (data + endCodes));

    // look up segment info from the arrays & short circuit if the spec requires.
    auto startCode = u16(startCodes + segmentIdx);
    auto shortCode = static_cast<uint16_t>(codepoint);
    if (startCode > shortCode) return 0;

    auto delta = u16(idDeltas + segmentIdx);
    auto idRangeOffset = u16(idRangeOffsets + segmentIdx);
    // intentional integer under- and overflow.
    if (idRangeOffset == 0) return (shortCode + delta) & 0xffff;

    // calculate offset into glyph array and determine ultimate value.
    auto offset = idRangeOffsets + segmentIdx + idRangeOffset + 2U * (unsigned int)(shortCode - startCode);
    if (!validate(offset, 2)) return -1;

    auto id = u16(offset);
    // intentional integer under- and overflow.
    if (id > 0) return (id + delta) & 0xffff;

    return 0;
}

uint32_t SfntReader::cmap_6(uint32_t table, uint32_t codepoint) const
{
    // cmap format 6 only supports the Unicode BMP.
    if (codepoint > 0xFFFF) return 0;

    auto firstCode = u16(table);
    auto entryCnt = u16(table + 2);
    if (!validate(table, 4 + 2 * entryCnt)) return -1;

    if (codepoint < firstCode) return -1;
    codepoint -= firstCode;
    if (codepoint >= entryCnt) return -1;
    return u16(table + 4 + 2 * codepoint);
}

bool SfntReader::glyphMetrics(SfntGlyph& glyph)
{
    // glyph is inside long metrics segment.
    if (glyph.idx < metrics.numHmtx) {
        auto offset = hmtx + 4 * glyph.idx;
        if (!validate(offset, 4)) return false;
        glyph.advance = u16(offset);
        glyph.lsb = i16(offset + 2);
    // glyph is inside short metrics segment.
    } else {
        auto boundary = hmtx + 4U * (uint32_t)metrics.numHmtx;
        if (boundary < 4) return false;
        auto offset = boundary - 4;
        if (!validate(offset, 4)) return false;
        glyph.advance = u16(offset);
        offset = boundary + 2 * (glyph.idx - metrics.numHmtx);
        if (!validate(offset, 2)) return false;
        glyph.lsb = i16(offset);
    }
    return true;
}

uint32_t SfntReader::glyph(uint32_t codepoint)
{
    auto entryCnt = u16(cmap + 2);
    if (!validate(cmap, 4 + entryCnt * 8)) return -1;

    // full repertory (non-BMP map).
    for (auto idx = 0; idx < entryCnt; ++idx) {
        auto entry = cmap + 4 + idx * 8;
        auto type = u16(entry) * 0100 + u16(entry + 2);
        // unicode map
        if (type == 0004 || type == 0312) {
            auto table = cmap + u32(entry + 4);
            if (!validate(table, 8)) return -1;
            // dispatch based on cmap format.
            auto format = u16(table);
            switch (format) {
                case 12: return cmap_12_13(table, codepoint, 12);
                default: return -1;
            }
        }
    }
    // Try looking for a BMP map.
    for (auto idx = 0; idx < entryCnt; ++idx) {
        auto entry = cmap + 4 + idx * 8;
        auto type = u16(entry) * 0100 + u16(entry + 2);
        // Unicode BMP
        if (type == 0003 || type == 0301) {
            auto table = cmap + u32(entry + 4);
            if (!validate(table, 6)) return -1;
            // Dispatch based on cmap format.
            switch (u16(table)) {
                case 4: return cmap_4(table + 6, codepoint);
                case 6: return cmap_6(table + 6, codepoint);
                default: return -1;
            }
        }
    }
    return -1;
}

/************************************************************************/
/* External Class Implementation                                        */
/************************************************************************/

bool SfntReader::header()
{
    if (!validate(0, 12)) return false;

    // header
    auto head = table("head");
    if (!head || !validate(head, 54)) return false;

    metrics.unitsPerEm = u16(head + 18);
    metrics.locaFormat = u16(head + 50);

    // horizontal header
    auto hhea = table("hhea");
    if (!hhea || !validate(hhea, 36)) return false;

    metrics.hhea.ascent = i16(hhea + 4);
    metrics.hhea.descent = i16(hhea + 6);
    metrics.hhea.linegap = i16(hhea + 8);
    metrics.hhea.advance = metrics.hhea.ascent - metrics.hhea.descent + metrics.hhea.linegap;
    metrics.numHmtx = u16(hhea + 34);

    maxp = table("maxp");
    if (!maxp || !validate(maxp, 6)) return false;

    // glyph outlines count
    auto glyphs = u16(maxp + 4);
    if (glyphs == 0) return false;

    // horizontal metrics count
    auto hmtxs = u16(hhea + 34);
    if (hmtxs == 0 || hmtxs > glyphs) return false;

    cmap = table("cmap");
    if (!cmap || !validate(cmap, 4)) return false;

    // horizontal metrics
    hmtx = table("hmtx");
    if (!validate(hmtx, metrics.numHmtx * 4)) return false;

    return true;
}