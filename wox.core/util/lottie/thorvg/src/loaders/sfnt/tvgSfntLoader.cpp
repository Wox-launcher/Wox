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


#include "tvgStr.h"
#include "tvgSfntLoader.h"

#ifdef THORVG_TTF_LOADER_SUPPORT
    #include "tvgTtfReader.h"
#endif

#ifdef THORVG_OTF_LOADER_SUPPORT
    #include "tvgOtfReader.h"
#endif

#if defined(__linux__)
    #include <fcntl.h>
    #include <unistd.h>
    #include <sys/mman.h>
    #include <sys/stat.h>
#endif

/************************************************************************/
/* Internal Class Implementation                                        */
/************************************************************************/

#define SPACE_GLYPH_IDX 32
#define LINE_FEED_GLYPH_IDX 10
#define DOT_GLYPH_IDX 46


#ifdef THORVG_FILE_IO_SUPPORT

#if defined(_WIN32) && (WINAPI_FAMILY == WINAPI_FAMILY_DESKTOP_APP)

static uint8_t* _map(SfntLoader* loader, const string& path, uint32_t& size)
{
	auto file = CreateFileA(path.c_str(), GENERIC_READ, FILE_SHARE_READ, NULL, OPEN_EXISTING, 0, NULL);
    if (file == INVALID_HANDLE_VALUE) return nullptr;

    DWORD high;
    auto low = GetFileSize(file, &high);
    if (low == INVALID_FILE_SIZE) {
		CloseHandle(file);
        return nullptr;
    }

    size = (uint32_t)((size_t)high << (8 * sizeof(DWORD)) | low);

    loader->mapping = (uint8_t*)CreateFileMapping(file, NULL, PAGE_READONLY, high, low, NULL);

    CloseHandle(file);

    if (!loader->mapping) return nullptr;

    auto data = (uint8_t*)MapViewOfFile(loader->mapping, FILE_MAP_READ, 0, 0, 0);
    if (!data) {
        CloseHandle(loader->mapping);
        loader->mapping = nullptr;
        return nullptr;
    }
    return data;
}

static void _unmap(SfntLoader* loader)
{
    auto reader = loader->reader;
    if (!reader) return;

    if (reader->data) {
        UnmapViewOfFile(reader->data);
        reader->data = nullptr;
    }
    if (loader->mapping) {
        CloseHandle(loader->mapping);
        loader->mapping = nullptr;
    }
}

#elif defined(__linux__)

static uint8_t* _map(TVG_UNUSED SfntLoader* loader, const char* path, uint32_t& size)
{
    auto fd = open(path, O_RDONLY);
    if (fd < 0) return nullptr;

    struct stat info;
    if (fstat(fd, &info) < 0) {
        close(fd);
        return nullptr;
    }

    auto data = (uint8_t*)mmap(NULL, (size_t)info.st_size, PROT_READ, MAP_PRIVATE, fd, 0);
    if (data == (uint8_t*)-1) return nullptr;
    size = (uint32_t)info.st_size;
    close(fd);

    return data;
}

static void _unmap(SfntLoader* loader)
{
    auto reader = loader->reader;
    if (!reader) return;
    if (reader->data == (uint8_t*)-1) return;
    munmap((void*)reader->data, reader->size);
    reader->data = (uint8_t*)-1;
    reader->size = 0;
}
#else
static uint8_t* _map(SfntLoader* loader, const char* path, uint32_t& size)
{
    auto f = fopen(path, "rb");
    if (!f) return nullptr;

    fseek(f, 0, SEEK_END);

    size = ftell(f);
    if (size == 0) {
        fclose(f);
        return nullptr;
    }

    auto data = tvg::malloc<uint8_t>(size);

    fseek(f, 0, SEEK_SET);
    auto ret = fread(data, sizeof(char), size, f);
    if (ret < size) {
        fclose(f);
        return nullptr;
    }

    fclose(f);

    return data;
}

static void _unmap(SfntLoader* loader)
{
    auto reader = loader->reader;
    if (!reader) return;
    tvg::free(reader->data);
    reader->data = nullptr;
    reader->size = 0;
}
#endif

#endif //THORVG_FILE_IO_SUPPORT

static size_t _codepoints(const char** utf8, const char* end)
{
    auto p = *utf8;

    if (!(*p & 0x80U)) {
        (*utf8) += 1;
        return *p;
    } else if ((*p & 0xe0U) == 0xc0U && (p + 1 < end)) {
        (*utf8) += 2;
        return ((*p & 0x1fU) << 6) + (*(p + 1) & 0x3fU);
    } else if ((*p & 0xf0U) == 0xe0U && (p + 2 < end)) {
        (*utf8) += 3;
        return ((*p & 0x0fU) << 12) + ((*(p + 1) & 0x3fU) << 6) + (*(p + 2) & 0x3fU);
    } else if ((*p & 0xf8U) == 0xf0U && (p + 3 < end)) {
        (*utf8) += 4;
        return ((*p & 0x07U) << 18) + ((*(p + 1) & 0x3fU) << 12) + ((*(p + 2) & 0x3fU) << 6) + (*(p + 3) & 0x3fU);
    }
    TVGERR("SFNT", "Corrupted UTF8??");
    (*utf8) += 1;
    return 0;
}

static void _build(const RenderPath& in, const Point& cursor, const Point& offset, RenderPath& out)
{
    out.cmds.push(in.cmds);
    out.pts.grow(in.pts.count);
    ARRAY_FOREACH(p, in.pts) {
        out.pts.push(*p + cursor + offset);
    }
}


static void _align(const Point& align, const Point& box, const Point& cursor, uint32_t begin, uint32_t end, RenderPath& out)
{
    if (align.x > 0.0f || align.y > 0.0f) {
        auto shift = (box - cursor) * align;
        for (auto p = out.pts.data + begin; p < out.pts.data + end; p++) {
            *p += shift;
        }
    }
}


static void _alignX(float align, float box, float x, uint32_t begin, uint32_t end, RenderPath& out)
{
    if (align > 0.0f) {
        auto shift = (box - x) * align;
        for (auto p = out.pts.data + begin; p < out.pts.data + end; p++) {
            p->x += shift;
        }
    }
}


static void _alignY(float align, float box, float y, uint32_t begin, uint32_t end, RenderPath& out)
{
    if (align > 0.0f) {
        auto shift = (box - y) * align;
        for (auto p = out.pts.data + begin; p < out.pts.data + end; p++) {
            p->y += shift;
        }
    }
}

uint32_t SfntLoader::feedLine(FontMetrics& fm, float box, float x, uint32_t begin, uint32_t end, Point& cursor, RenderPath& out)
{
    _alignX(fm.align.x, box, x, begin, end, out); //align the given line
    cursor.x = 0.0f;
    cursor.y += reader->metrics.hhea.advance * fm.spacing.y;
    ++fm.lines;
    return out.pts.count;
}

void SfntLoader::clear()
{
    if (nomap) {
        if (freeData) tvg::free(reader->data);
        reader->data = nullptr;
        reader->size = 0;
        freeData = false;
        nomap = false;
    } else {
#ifdef THORVG_FILE_IO_SUPPORT
        _unmap(this);
#endif
    }

    tvg::free(name);
    name = nullptr;

    delete (reader);
    reader = nullptr;
}

SfntGlyphMetrics* SfntLoader::request(uint32_t code)
{
    if (code == 0) return nullptr;

    auto it = glyphs.find(code);
    if (it) return &it->val;
    auto& rtgm = glyphs[code];
    if (reader->convert(rtgm, code, rtgm.path)) return &rtgm;

    TVGERR("SFNT", "invalid glyph id, codepoint(0x%x)", code);
    return nullptr;
}

void SfntLoader::wrapNone(FontMetrics& fm, const Point& box, const char* utf8, const char* end, RenderPath& out)
{
    SfntGlyphMetrics* ltgm = nullptr;  // left side glyph between the two adjacent glyphs
    Point cursor = {};
    uint32_t line = 0;  //the begin pos of the last line among path

    while (utf8 < end) {
        auto code = _codepoints(&utf8, end);
        if (code == LINE_FEED_GLYPH_IDX) {
            line = feedLine(fm, box.x, cursor.x, line, out.pts.count, cursor, out);
            continue;
        }
        auto rtgm = request(code);  //right side glyph between the two adjacent glyphs
        if (!rtgm) continue;

        Point offset{};
        if (ltgm) reader->positioning(ltgm->idx, rtgm->idx, offset);

        _build(rtgm->path, cursor, offset, out);
        cursor.x += (rtgm->advance + offset.x) * fm.spacing.x;

        if (cursor.x > fm.size.x) fm.size.x = cursor.x;  //text horizontal size

        //store the base glyph width for italic transform
        if (!ltgm) static_cast<SfntMetrics*>(fm.engine)->baseGlyph = rtgm;
        ltgm = rtgm;
    }
    fm.size.y = height(fm.lines, fm.spacing.y);
    _alignY(fm.align.y, box.y, fm.size.y, 0, line, out); //before the last line
    _align(fm.align, box, {cursor.x, fm.size.y}, line, out.pts.count, out);  //last line
}

void SfntLoader::wrapChar(FontMetrics& fm, const Point& box, const char* utf8, const char* end, RenderPath& out)
{
    SfntGlyphMetrics* ltgm = nullptr;  // left side glyph between the two adjacent glyphs
    uint32_t line = 0;  //the begin pos of the last line among path
    Point cursor = {};

    while (utf8 < end) {
        auto code = _codepoints(&utf8, end);
        if (code == LINE_FEED_GLYPH_IDX) {
            line = feedLine(fm, box.x, cursor.x, line, out.pts.count, cursor, out);
            continue;
        }
        auto rtgm = request(code); //right side glyph between the two adjacent glyphs
        if (!rtgm) continue;

        Point offset{};
        if (ltgm) reader->positioning(ltgm->idx, rtgm->idx, offset);

        auto xadv = (rtgm->advance + offset.x) * fm.spacing.x;

        //normal scenario
        if (xadv < box.x) {
            if (cursor.x + xadv > box.x) {
                line = feedLine(fm, box.x, cursor.x, line, out.pts.count, cursor, out);
            }
            _build(rtgm->path, cursor, offset, out);
            cursor.x += xadv;
        //not enough layout space, force pushing
        } else {
            _build(rtgm->path, cursor, offset, out);
            line = feedLine(fm, box.x, cursor.x, line, out.pts.count, cursor, out);
        }

        if (cursor.x > fm.size.x) fm.size.x = cursor.x;  //text horizontal size

        //store the base glyph width for italic transform
        if (!ltgm) static_cast<SfntMetrics*>(fm.engine)->baseGlyph = rtgm;
        ltgm = rtgm;
    }
    fm.size.y = height(fm.lines, fm.spacing.y);
    _alignY(fm.align.y, box.y, fm.size.y, 0, line, out); //before the last line
    _align(fm.align, box, {cursor.x, fm.size.y}, line, out.pts.count, out);  //last line
}

void SfntLoader::wrapWord(FontMetrics& fm, const Point& box, const char* utf8, const char* end, RenderPath& out, bool smart)
{
    SfntGlyphMetrics* ltgm = nullptr;  // left side glyph between the two adjacent glyphs
    auto line = 0;  //the begin pos of the last line among path
    auto word = 0;  //the begin pos of the last word among path
    auto wadv = 0.0f;  //word advance size
    auto hadv = reader->metrics.hhea.advance * fm.spacing.y;  // line advance size
    Point cursor = {};

    while (utf8 < end) {
        auto code = _codepoints(&utf8, end);
        if (code == LINE_FEED_GLYPH_IDX) {
            line = feedLine(fm, box.x, cursor.x, line, out.pts.count, cursor, out);
            continue;
        }
        auto rtgm = request(code); //right side glyph between the two adjacent glyphs
        if (!rtgm) continue;

        Point offset{};
        if (ltgm) reader->positioning(ltgm->idx, rtgm->idx, offset);

        auto xadv = (rtgm->advance + offset.x) * fm.spacing.x;

        //try line-wrap
        if (cursor.x + xadv > box.x) {
            //wrapping only if enough space for the current word
            if ((cursor.x - wadv) + xadv < box.x) {
                _alignX(fm.align.x, box.x, wadv, line, word, out);  //align the current line
                //shift the wrapping word to the next line
                for (auto p = out.pts.begin() + word; p < out.pts.end(); p++) {
                    p->x -= wadv;
                    p->y += hadv;
                }
                cursor.x -= wadv;
                cursor.y += hadv;
                line = word;
                wadv = 0;
                ++fm.lines;
            //not enough space, line wrap by character
            } else if (smart) {
                line = feedLine(fm, box.x, cursor.x, line, out.pts.count, cursor, out);
            }
        }
        _build(rtgm->path, cursor, offset, out);
        cursor.x += xadv;

        //capture the word start
        if (code == SPACE_GLYPH_IDX) {
            word = out.pts.count;
            wadv = cursor.x;
        }

        if (cursor.x > fm.size.x) fm.size.x = cursor.x;  //text horizontal size

        //store the base glyph width for italic transform
        if (!ltgm) static_cast<SfntMetrics*>(fm.engine)->baseGlyph = rtgm;
        ltgm = rtgm;
    }
    fm.size.y = height(fm.lines, fm.spacing.y);
    _alignY(fm.align.y, box.y, fm.size.y, 0, line, out); //before the last line
    _align(fm.align, box, {cursor.x, fm.size.y}, line, out.pts.count, out);  //last line
}

void SfntLoader::wrapEllipsis(FontMetrics& fm, const Point& box, const char* utf8, const char* end, RenderPath& out)
{
    SfntGlyphMetrics* ltgm = nullptr;  // left side glyph between the two adjacent glyphs
    auto line = 0;  //the begin pos of the last line among path
    Point cursor = {};
    struct {
        uint32_t pts = 0;
        uint32_t cmds = 0;
        float xadv = 0.0f;
    } capture;   //the last path for reverting the last character if the ... space is not enough
    auto stop = false;

    while (utf8 < end) {
        auto code = _codepoints(&utf8, end);
        if (code == LINE_FEED_GLYPH_IDX) {
            line = feedLine(fm, box.x, cursor.x, line, out.pts.count, cursor, out);
            continue;
        }
        auto rtgm = request(code); //right side glyph between the two adjacent glyphs
        if (!rtgm) continue;

        Point offset{};
        if (ltgm) reader->positioning(ltgm->idx, rtgm->idx, offset);

        auto xadv = (rtgm->advance + offset.x) * fm.spacing.x;

        //normal case
        if (cursor.x + xadv < box.x) {
            capture = {out.pts.count, out.cmds.count, xadv};
            _build(rtgm->path, cursor, offset, out);
            cursor.x += xadv;
        //ellipsis
        } else {
            rtgm = request(DOT_GLYPH_IDX); //right side glyph between the two adjacent glyphs
            if (!rtgm) {
                TVGERR("SFNT", "Cannot support ... since no glyph data");
                return;
            }
            offset = {};
            reader->positioning(rtgm->idx, rtgm->idx, offset);
            //not enough space, revert one character back
            if (cursor.x + (rtgm->advance + offset.x) * 3 > box.x) {
                out.pts.count = capture.pts;
                out.cmds.count = capture.cmds;
                cursor.x -= capture.xadv;
            }
            //append ...
            auto tmp = (rtgm->advance + offset.x) * fm.spacing.x;
            for (int i = 0; i < 3; ++i) {
                _build(rtgm->path, cursor, offset, out);
                cursor.x += tmp;
            }
            stop = true;
        }

        if (cursor.x > fm.size.x) fm.size.x = cursor.x;  //text horizontal size

        //store the base glyph width for italic transform
        if (!ltgm) static_cast<SfntMetrics*>(fm.engine)->baseGlyph = rtgm;

        if (stop) break;  //stop the process if the ellipsis is applied

        ltgm = rtgm;
    }
    fm.size.y = height(fm.lines, fm.spacing.y);
    _alignY(fm.align.y, box.y, fm.size.y, 0, line, out); //before the last line
    _align(fm.align, box, {cursor.x, fm.size.y}, line, out.pts.count, out);  //last line
}

SfntReader* SfntLoader::gen(uint8_t* data, uint32_t size)
{
    if (size > 4) {
        auto type = uint32_t(data[0] << 24 | data[1] << 16 | data[2] << 8 | data[3]);
        if (type == 0x00010000 || type == 0x74727565) {  // ttf (scalable font)
#ifdef THORVG_TTF_LOADER_SUPPORT
            return new TtfReader(data, size);
#endif
            TVGLOG("SFNT", "TrueType (TTF) is not supported");
        } else if (type == 0x4F54544F) {  // otf (OTTO)
#ifdef THORVG_OTF_LOADER_SUPPORT
            return new OtfReader(data, size);
#endif
            TVGLOG("SFNT", "OpenType (OTF) is not supported");
        }
    }
    TVGERR("SFNT", "Invalid SFNT format!");
    return nullptr;
}

/************************************************************************/
/* External Class Implementation                                        */
/************************************************************************/

void SfntLoader::transform(Paint* paint, FontMetrics& fm, float italicShear)
{
    auto engine = static_cast<SfntMetrics*>(fm.engine);
    auto scale = 1.0f / fm.scale;
    auto baseWidth = (engine->baseGlyph) ? engine->baseGlyph->bbox.w() : 0.0f;
    Matrix m = {scale, -italicShear * scale, italicShear * baseWidth * scale, 0, scale, reader->metrics.hhea.ascent * scale, 0, 0, 1};
    paint->transform(m);
}

SfntLoader::SfntLoader() :
    FontLoader(FileType::Sfnt)
{
}

SfntLoader::~SfntLoader()
{
    clear();
}

bool SfntLoader::open(const char* path, TVG_UNUSED const LoaderOps* ops)
{
#ifdef THORVG_FILE_IO_SUPPORT
    uint32_t size;
    auto data = _map(this, path, size);
    if (!data) return false;
    reader = gen(data, size);
    if (reader) {
        name = tvg::filename(path);
        return reader->header();
    }
#endif
    return false;
}

bool SfntLoader::open(const char* data, uint32_t size, TVG_UNUSED const LoaderOps* ops, bool copy)
{
    reader = gen((uint8_t*)data, size);
    if (!reader) return false;
    nomap = true;

    if (copy) {
        reader->data = tvg::malloc<uint8_t>(size);
        memcpy((char*)reader->data, data, reader->size);
        freeData = true;
    }

    return reader->header();
}

bool SfntLoader::get(FontMetrics& fm, char* text, uint32_t len, RenderPath& out)
{
    out.clear();

    fm.lines = 1;

    if (!text || fm.fontSize == 0.0f) return false;

    fm.scale = reader->metrics.unitsPerEm / (fm.fontSize * FontLoader::DPI);
    fm.size = {};
    if (!fm.engine) fm.engine = tvg::calloc<SfntMetrics>(1, sizeof(SfntMetrics));

    auto box = fm.box * fm.scale;
    auto end = text + len;

    if (fm.wrap == TextWrap::None || fm.box.x == 0.0f) wrapNone(fm, box, text, end, out);
    else if (fm.wrap == TextWrap::Character) wrapChar(fm, box, text, end, out);
    else if (fm.wrap == TextWrap::Word) wrapWord(fm, box, text, end, out, false);
    else if (fm.wrap == TextWrap::Smart) wrapWord(fm, box, text, end, out, true);
    else if (fm.wrap == TextWrap::Ellipsis) wrapEllipsis(fm, box, text, end, out);
    else return false;

    return true;
}

void SfntLoader::release(FontMetrics& fm)
{
    tvg::free(fm.engine);
    fm.engine = nullptr;
}

void SfntLoader::copy(const FontMetrics& in, FontMetrics& out)
{
    release(out);
    out = in;
    if (in.engine) {
        out.engine = tvg::calloc<SfntMetrics>(1, sizeof(SfntMetrics));
        *static_cast<SfntMetrics*>(out.engine) = *static_cast<SfntMetrics*>(in.engine);
    }
}

void SfntLoader::metrics(const FontMetrics& fm, TextMetrics& out)
{
    auto scale = (fm.fontSize * FontLoader::DPI) / reader->metrics.unitsPerEm;

    out.advance = reader->metrics.hhea.advance * scale;
    out.ascent = reader->metrics.hhea.ascent * scale;
    out.descent = reader->metrics.hhea.descent * scale;
    out.linegap = reader->metrics.hhea.linegap * scale;
}

bool SfntLoader::metrics(const FontMetrics& fm, const char* ch, GlyphMetrics& out, const char** next)
{
    auto code = _codepoints(&ch, ch + strlen(ch));
    auto glyph = request(code);
    if (!glyph) return false;

    auto scale = (fm.fontSize * FontLoader::DPI) / reader->metrics.unitsPerEm;
    out.advance = glyph->advance * scale;
    out.bearing = glyph->lsb * scale;

    // if no info, calculate it manually
    if (glyph->bbox.zero()) {
        if (glyph->path.bounds(nullptr, glyph->bbox)) {
            // convert y since sfnt y coordinates origin is reverted
            auto temp = glyph->bbox.min.y;
            glyph->bbox.min.y = -glyph->bbox.max.y;
            glyph->bbox.max.y = -temp;
        }
    }
    out.min = glyph->bbox.min * scale;
    out.max = glyph->bbox.max * scale;

    if (next) *next = ch;
    return true;
}