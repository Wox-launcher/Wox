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

#include "webp/decode.h"
#include "tvgWebpLoader.h"


/************************************************************************/
/* Internal Class Implementation                                        */
/************************************************************************/

void WebpLoader::clear()
{
    if (freeData) tvg::free(data);
    data = nullptr;
    freeData = false;
}


void WebpLoader::run(unsigned tid)
{
    // static loader WebPDecodeRGBA/WebPDecodeBGRA returns a premultiplied version.
    if (surface.cs == ColorSpace::ARGB8888 || surface.cs == ColorSpace::ARGB8888S) {
        surface.buf8 = WebPDecodeBGRA(data, size, nullptr, nullptr);
        surface.cs = ColorSpace::ARGB8888;
    } else  {
        surface.buf8 = WebPDecodeRGBA(data, size, nullptr, nullptr);
        surface.cs = ColorSpace::ABGR8888;
    }

    surface.stride = static_cast<uint32_t>(w);
    surface.w = static_cast<uint32_t>(w);
    surface.h = static_cast<uint32_t>(h);
    surface.channelSize = sizeof(uint32_t);
    surface.premultiplied = true;

    clear();
}


/************************************************************************/
/* External Class Implementation                                        */
/************************************************************************/

WebpLoader::WebpLoader() : ImageLoader(FileType::Webp)
{
}


WebpLoader::~WebpLoader()
{
    done();
    clear();
    tvg::free(surface.buf8);
}

bool WebpLoader::open(const char* path, TVG_UNUSED const LoaderOps* ops)
{
#ifdef THORVG_FILE_IO_SUPPORT
    if (!(data = (uint8_t*)Loader::open(path, size))) return false;

    WebPBitstreamFeatures features;
    if (WebPGetFeatures(data, size, &features)) return false;
    w = static_cast<float>(features.width);
    h = static_cast<float>(features.height);
    surface.alphaIgnored = !features.has_alpha;
    freeData = true;
    return true;
#else
    return false;
#endif
}

bool WebpLoader::open(const char* data, uint32_t size, TVG_UNUSED const LoaderOps* ops, bool copy)
{
    if (copy) {
        this->data = tvg::malloc<uint8_t>(size);
        if (!this->data) return false;
        memcpy((uint8_t*)this->data, data, size);
        freeData = true;
    } else {
        this->data = (uint8_t*) data;
        freeData = false;
    }

    WebPBitstreamFeatures features;
    if (WebPGetFeatures(this->data, size, &features)) return false;
    w = static_cast<float>(features.width);
    h = static_cast<float>(features.height);
    surface.alphaIgnored = !features.has_alpha;
    this->size = size;

    return true;
}


bool WebpLoader::read()
{
    if (!Loader::read()) return true;

    if (!data || w == 0 || h == 0) return false;

    surface.cs = ImageLoader::cs;

    TaskScheduler::request(this);

    return true;
}


bool WebpLoader::close()
{
    if (!Loader::close()) return false;
    this->done();
    return true;
}


RenderSurface* WebpLoader::bitmap()
{
    this->done();

    return ImageLoader::bitmap();
}