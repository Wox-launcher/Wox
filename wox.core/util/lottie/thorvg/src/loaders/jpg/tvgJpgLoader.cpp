/*
 * Copyright (c) 2021 - 2026 ThorVG project. All rights reserved.

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

#include "tvgJpgLoader.h"

/************************************************************************/
/* Internal Class Implementation                                        */
/************************************************************************/

void JpgLoader::clear()
{
    jpgdDelete(decoder);
    if (freeData) tvg::free(data);
    decoder = nullptr;
    data = nullptr;
    freeData = false;
}


void JpgLoader::run(unsigned tid)
{
    surface.cs = ColorSpace::ABGR8888;
    surface.buf8 = jpgdDecompress(decoder);
    surface.stride = static_cast<uint32_t>(w);
    surface.w = static_cast<uint32_t>(w);
    surface.h = static_cast<uint32_t>(h);
    surface.channelSize = sizeof(uint32_t);
    surface.premultiplied = true;
    surface.alphaIgnored = true;

    clear();
}


/************************************************************************/
/* External Class Implementation                                        */
/************************************************************************/

JpgLoader::JpgLoader() : ImageLoader(FileType::Jpg)
{

}


JpgLoader::~JpgLoader()
{
    done();
    clear();
    tvg::free(surface.buf8);
}

bool JpgLoader::open(const char* path, TVG_UNUSED const LoaderOps* ops)
{
#ifdef THORVG_FILE_IO_SUPPORT
    int width, height;
    if (!(decoder = jpgdHeader(path, &width, &height))) return false;

    w = static_cast<float>(width);
    h = static_cast<float>(height);

    return true;
#else
    return false;
#endif
}

bool JpgLoader::open(const char* data, uint32_t size, TVG_UNUSED const LoaderOps* ops, bool copy)
{
    if (copy) {
        this->data = tvg::malloc<char>(size);
        if (!this->data) return false;
        memcpy((char *)this->data, data, size);
        freeData = true;
    } else {
        this->data = (char *) data;
        freeData = false;
    }

    int width, height;
    decoder = jpgdHeader(this->data, size, &width, &height);
    if (!decoder) return false;

    w = static_cast<float>(width);
    h = static_cast<float>(height);

    return true;
}



bool JpgLoader::read()
{
    if (!Loader::read()) return true;

    if (!decoder || w == 0 || h == 0) return false;

    TaskScheduler::request(this);

    return true;
}


bool JpgLoader::close()
{
    if (!Loader::close()) return false;
    this->done();
    return true;
}


RenderSurface* JpgLoader::bitmap()
{
    this->done();
    return ImageLoader::bitmap();
}