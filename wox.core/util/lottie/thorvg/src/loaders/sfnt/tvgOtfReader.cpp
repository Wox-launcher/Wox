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

#include "tvgOtfReader.h"

/************************************************************************/
/* Internal Class Implementation                                        */
/************************************************************************/

int32_t OtfReader::offset(uint32_t base, uint8_t size)
{
    int32_t ret = 0;
    for (uint8_t i = 0; i < size; ++i) {
        ret = (ret << 8) | u8(base++);
    }
    return ret;
}

uint32_t OtfReader::skip(uint32_t idx)
{
    auto cnt = u16(idx);
    if (cnt == 0) return idx + 2;

    auto offSize = u8(idx + 2);
    auto offsetsBase = idx + 3;
    auto lastOffsetPos = offsetsBase + cnt * offSize;
    uint32_t lastOffset = 0;

    for (uint8_t i = 0; i < offSize; ++i) {
        lastOffset = (lastOffset << 8) | u8(lastOffsetPos + i);
    }

    auto dataStart = offsetsBase + (cnt + 1) * offSize;  // data starts after the offset array
    return dataStart + (lastOffset - 1);                 // CFF index offsets are 1 - based
};

bool OtfReader::name(uint32_t idx)
{
    auto count = u16(idx);
    if (count == 0) return false;

    auto size = u8(idx + 2);
    auto base = idx + 3;
    auto data = base + (count + 1) * size;

    for (uint16_t i = 0; i < count; ++i) {
        auto off1 = offset(base + i * size, size);
        auto off2 = offset(base + (i + 1) * size, size);
        auto start = data + (off1 - 1);
        auto length = off2 - off1;
        printf("Name[%d]: ", i);
        for (int j = 0; j < length; ++j) {
            char c = (char)u8(start + j);
            printf("%c", c);
        }
        printf("\n");
    }

    return true;
}

// common part of DICT interpret
bool OtfReader::DICT(Array<int32_t>& args, uint8_t b, uint32_t& ptr)
{
    // operand decoding
    if (b >= 32) {
        auto v = 0;
        if (b <= 246) {
            v = b - 139;
            ptr += 1;
        } else if (b <= 250) {
            v = ((b - 247) * 256) + u8(ptr + 1) + 108;
            ptr += 2;
        } else if (b <= 254) {
            v = -((b - 251) * 256) - u8(ptr + 1) - 108;
            ptr += 2;
        } else {
            v = i32(ptr + 1);  // b == 255: 16.16 fixed-point, not used as dict offset → skip
            ptr += 5;
        }
        args.push(v);
        return true;
    // 2 bytes integer
    } else if (b == 28) {
        args.push(i16(ptr + 1));
        ptr += 3;
        return true;
    // 4 bytes integer
    } else if (b == 29) {
        args.push(i32(ptr + 1));
        ptr += 5;
        return true;
    // unnecessary. skip operators (0: Copyright, 7: FontMatrix, etc)
    } else if (b == 12) {
        ptr += 2;
        return true;
    }
    return false;
}

uint32_t OtfReader::localSubrs(Array<int32_t>& args, uint32_t offset, uint32_t size)
{
    args.clear();

    auto base = cff + offset;
    auto end = base + size;
    auto ptr = base;

    while (ptr < end) {
        auto b = u8(ptr);
        if (DICT(args, b, ptr)) continue;
        // sub routine offset is relative to the start of Private DICT
        if (b == 19 && args.count > 0) return base + args.last();
        ++ptr;
        args.clear();
    }
    return 0;
}

// table("CFF ")
// ├── Header
// ├── Name INDEX (skipped)
// ├── Top DICT INDEX
// │   ├── CharStrings (op 17) → toCharStrings (relative offset from CFF start)
// │   └── Private (op 18) → [size, offset]
// │        └── Private DICT
// │             └── Subrs (op 19) → subrs (relative offset from Private DICT start)
// │                  └── Local Subr INDEX
// ├── String INDEX (skipped, CFF1 only)
// ├── Global Subr INDEX → gsubrs
// └── CharStrings INDEX → cff + toCharStrings
//     ├── count (u16)
//     ├── offSize (u8)
//     ├── offset array [(count + 1) * offSize]
//     └── data (CharString bytecode per glyph)
bool OtfReader::CFF()
{
    cff = table("CFF ");  // PostScript outlines
    auto cff2 = false;
    if (!cff) {
        cff = table("CFF2");  // variable font
        cff2 = true;
    }
    if (!cff || !validate(cff, 4)) return false;

    auto p = cff + u8(cff + 2);  // skip the header size

    //name(p);
    p = skip(p);  // name

    // parse Top DICT INDEX
    auto count = u16(p);
    if (count == 0) return false;

    Array<int32_t> args(32);  // arguments stack for CFF decoding

    auto size = u8(p + 2);
    auto base = (p + 3);
    auto data = base + (count + 1) * size;
    auto ptr = data + (offset(base, size) - 1);
    auto end = data + (offset(base + size, size) - 1);

    while (ptr < end) {
        auto b = u8(ptr);
        if (DICT(args, b, ptr)) continue;
        // CharStrings
        if (b == 17) {
            toCharStrings = args.last();
        // find the subroutine from private DICT
        } else if (b == 18 && args.count >= 2) {
            subrs = localSubrs(args, args[args.count - 1], args[args.count - 2]);
        }
        ++ptr;
        args.clear();
    }
    if (toCharStrings == 0) return false;
    toCharStrings += cff;

    p = skip(p);             // jump to the end of INDEX
    if (!cff2) p = skip(p);  // only cff1 has the String INDEX

    gsubrs = p;

    return true;
}

bool OtfReader::subRoutine(float operand, uint32_t subrs, uint32_t& p, uint32_t& end, SubRoutine* subRoutines, int& srp)
{
    if (srp >= SUBROUTINE_MAX) return false;

    auto count = u16(subrs);
    if (count == 0) return false;  // no available subroutines

    auto size = u8(subrs + 2);              // offset size in bytes
    auto base = subrs + 3;                  // start of offset array
    auto data = base + (count + 1) * size;  // start of subroutine data

    // compute subroutine bias (CFF specification)
    int bias = (count < 1240) ? 107 : (count < 33900) ? 1131 : 32768;

    // apply bias to get actual subroutine index
    auto subrIdx = (int)round(operand) + bias;
    if (subrIdx < 0 || subrIdx >= count) return false;

    // read subroutine byte range from offset array
    auto off1 = offset(base + subrIdx * size, size);
    auto off2 = offset(base + (subrIdx + 1) * size, size);
    if (off2 <= off1) return false;

    subRoutines[srp++] = {p, end};  // save current execution state

    // jump to subroutine program
    p = data + (off1 - 1);
    end = data + (off2 - 1);

    return true;
}

void OtfReader::alternatingCurve(Array<float>& v, Point& pos, RenderPath& path, bool horizontal)
{
    Point cp1, cp2;
    auto last = false;
    for (uint32_t i = 0; i + 3 < v.count; i += last ? 5 : 4) {
        last = (v.count - i == 5);
        if (horizontal) {
            cp1 = pos + Point{v[i], 0};
            cp2 = cp1 + Point{v[i + 1], -v[i + 2]};
            pos = cp2 + Point{(last ? v[i + 4] : 0.0f), -v[i + 3]};
        } else {
            cp1 = pos + Point{0, -v[i]};
            cp2 = cp1 + Point{v[i + 1], -v[i + 2]};
            pos = cp2 + Point{v[i + 3], (last ? -v[i + 4] : 0.0f)};
        }
        path.cubicTo(cp1, cp2, pos);
        horizontal = !horizontal;
    }
}

void OtfReader::alignedCurve(Array<float>& v, Point& pos, RenderPath& path, bool horizontal)
{
    uint32_t i = 0;
    auto d = 0.0f;  // optional operand (odd count)
    Point cp1, cp2;

    if (v.count % 4 == 1) d = v[i++];

    for (; i + 3 < v.count; i += 4) {
        if (horizontal) {
            cp1 = pos + Point{v[i], -d};
            cp2 = cp1 + Point{v[i + 1], -v[i + 2]};
            pos = cp2 + Point{v[i + 3], 0};
        } else {
            cp1 = pos + Point{d, -v[i]};
            cp2 = cp1 + Point{v[i + 1], -v[i + 2]};
            pos = cp2 + Point{0, -v[i + 3]};
        }
        path.cubicTo(cp1, cp2, pos);
        d = 0.0f;
    }
}

void OtfReader::rrCurveTo(Array<float>& v, Point& pos, RenderPath& path)
{
    for (uint32_t i = 0; i + 5 < v.count; i += 6) {
        auto cp1 = pos + Point{v[i], -v[i + 1]};
        auto cp2 = cp1 + Point{v[i + 2], -v[i + 3]};
        pos = cp2 + Point{v[i + 4], -v[i + 5]};
        path.cubicTo(cp1, cp2, pos);
    }
}

void OtfReader::rCurveLine(Array<float>& v, Point& pos, RenderPath& path)
{
    if (v.count >= 8) {
        rrCurveTo(v, pos, path);
        pos += {v[v.count - 2], -v[v.count - 1]};
        path.lineTo(pos);
    }
}

void OtfReader::rLineCurve(Array<float>& v, Point& pos, RenderPath& path)
{
    for (uint32_t i = 0; i + 6 < v.count; i += 2) {
        pos += {v[i], -v[i + 1]};
        path.lineTo(pos);
    }
    auto i = v.count - 6;
    auto cp1 = pos + Point{v[i], -v[i + 1]};
    auto cp2 = cp1 + Point{v[i + 2], -v[i + 3]};
    pos = cp2 + Point{v[i + 4], -v[i + 5]};
    path.cubicTo(cp1, cp2, pos);
}

void OtfReader::operand(Array<float>& v, uint8_t b, uint32_t& p)
{
    if (b >= 32) {
        if (b <= 246) {
            v.push(float(b - 139));
            p += 1;
        } else if (b <= 250) {
            v.push(float((b - 247) * 256 + u8(p + 1) + 108));
            p += 2;
        } else if (b <= 254) {
            v.push(float(-((b - 251) * 256) - u8(p + 1) - 108));
            p += 2;
        } else {
            v.push(i32(p + 1) / 65536.0f);  // 16.16 fixed-point
            p += 5;
        }
    } else if (b == 28) {
        v.push(float(i16(p + 1)));
        p += 3;
    } else if (b == 29) {
        v.push(float(i32(p + 1)));
        p += 5;
    }
}

bool OtfReader::charStrings(RenderPath& path, uint32_t glyph)
{
    // get the glyph data offset
    auto count = u16(toCharStrings);
    if (glyph >= count) return false;

    auto size = u8(toCharStrings + 2);
    auto base = toCharStrings + 3;
    auto data = base + (count + 1) * size;
    auto offset1 = offset(base + glyph * size, size);
    auto offset2 = offset(base + (glyph + 1) * size, size);

    auto len = offset2 - offset1;
    if (len <= 0) return false;
    auto p = data + (offset1 - 1);
    auto end = (p + len);

    Point pos = {};  // current point (absolute)

    SubRoutine subRoutines[SUBROUTINE_MAX];   // subroutine stack
    auto srp = 0;  // subroutine stack pointer
    Array<float> values(40);  // values stack for path coordinates

    while (p < end) {
        auto b = u8(p);
        switch (b) {
            case 4: {  // vmoveto
                pos.y -= values.pick();
                path.moveTo(pos);
                break;
            }
            case 5: {  // rlineto
                for (uint32_t i = 0; i + 1 < values.count; i += 2) {
                    pos.x += values[i];
                    pos.y -= values[i + 1];
                    path.lineTo(pos);
                }
                break;
            }
            case 6: {  // hlineto: dx1 dy2 dx3 ...
                for (uint32_t i = 0; i < values.count; i++) {
                    if (i % 2 == 0) pos.x += values[i];
                    else pos.y -= values[i];
                    path.lineTo(pos);
                }
                break;
            }
            case 7: {  // vlineto: dy1 dx2 dy3 ...
                for (uint32_t i = 0; i < values.count; i++) {
                    if (i % 2 == 0) pos.y -= values[i];
                    else pos.x += values[i];
                    path.lineTo(pos);
                }
                break;
            }
            case 8: {  // rrcurveto: dx1 dy1 dx2 dy2 dx3 dy3 ...
                rrCurveTo(values, pos, path);
                break;
            }
            case 10: {
                if (this->subRoutine(values.pick(), subrs, p, end, subRoutines, srp)) continue;
                return false;
            }
            case 11: {  // return from the subroutine
                if (srp <= 0) return false;
                auto& s = subRoutines[--srp];
                p = s.p;
                end = s.end;
                ++p;
                continue;
            }
            case 12: {  // 2-byte escape operator
                p += 2;
                break;
            }
            case 14: {  // close
                path.close();
                break;
            }
            case 21: {  // rmoveto
                pos.y -= values.pick();
                pos.x += values.pick();
                path.moveTo(pos);
                break;
            }
            case 22: {  // hmoveto
                pos.x += values.pick();
                path.moveTo(pos);
                break;
            }
            case 24: {  // rcurveline
                rCurveLine(values, pos, path);
                break;
            }
            case 25: {  // rlinecurve
                rLineCurve(values, pos, path);
                break;
            }
            case 26: {  // vvcurveto
                alignedCurve(values, pos, path, false);
                break;
            }
            case 27: {  // hhcurveto
                alignedCurve(values, pos, path, true);
                break;
            }
            case 29: {
                if (this->subRoutine(values.pick(), gsubrs, p, end, subRoutines, srp)) continue;
                return false;
            }
            case 30: {  // vhcurveto
                alternatingCurve(values, pos, path, false);
                break;
            }
            case 31: {  // hvcurveto
                alternatingCurve(values, pos, path, true);
                break;
            }
            default: {
                operand(values, b, p);
                continue;
            }
        }
        values.clear();
        ++p;
    }
    return true;
}

/************************************************************************/
/* External Class Implementation                                        */
/************************************************************************/

bool OtfReader::header()
{
    if (!SfntReader::header()) return false;
    return CFF();
}

bool OtfReader::positioning(uint32_t lglyph, uint32_t rglyph, Point& out)
{
    // TODO:
    return false;
}

bool OtfReader::convert(SfntGlyph& glyph, uint32_t codepoint, RenderPath& path)
{
    glyph.idx = SfntReader::glyph(codepoint);
    if (glyph.idx == INVALID_GLYPH) return false;
    if (glyphMetrics(glyph)) return charStrings(path, glyph.idx);
    return false;
}