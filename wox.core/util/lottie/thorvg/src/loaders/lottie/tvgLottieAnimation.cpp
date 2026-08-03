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

#include "tvgCommon.h"
#include "thorvg_lottie.h"
#include "tvgLottieLoader.h"
#include "tvgAnimation.h"


LottieAnimation::LottieAnimation() = default;
LottieAnimation::~LottieAnimation() = default;

#define FETCH_LOADER(RET_VAL) \
    auto loader = static_cast<LottieLoader*>(tvg::to<PictureImpl>(pImpl->picture)->loader); \
    if (!loader) return RET_VAL


uint32_t LottieAnimation::gen(const char* slot) noexcept
{
    FETCH_LOADER(0);
    return loader->gen(slot);
}


Result LottieAnimation::apply(uint32_t id) noexcept
{
    FETCH_LOADER(Result::InsufficientCondition);

    if (loader->apply(id)) {
        PAINT(pImpl->picture)->mark(RenderUpdateFlag::All);
        return Result::Success;
    }

    return Result::InvalidArguments;
}


Result LottieAnimation::del(uint32_t id) noexcept
{
    FETCH_LOADER(Result::InsufficientCondition);

    if (loader->del(id)) {
        PAINT(pImpl->picture)->mark(RenderUpdateFlag::All);
        return Result::Success;
    }

    return Result::InvalidArguments;
}


Result LottieAnimation::segment(const char* marker) noexcept
{
    FETCH_LOADER(Result::InsufficientCondition);

    if (!marker) {
        loader->segment(0.0f, FLT_MAX);
        return Result::Success;
    }
    
    float begin, end;
    if (!loader->segment(marker, begin, end)) return Result::InvalidArguments;
    return Animation::segment(begin, end);
}


Result LottieAnimation::tween(float from, float to, float progress) noexcept
{
    FETCH_LOADER(Result::InsufficientCondition);
    if (!loader->tween(from, to, progress)) return Result::InsufficientCondition;
    PAINT(pImpl->picture)->mark(RenderUpdateFlag::All);
    return Result::Success;
}

Result LottieAnimation::tween(float progress) noexcept
{
    FETCH_LOADER(Result::InsufficientCondition);
    if (!loader->tween(progress)) return Result::InsufficientCondition;
    PAINT(pImpl->picture)->mark(RenderUpdateFlag::All);
    return Result::Success;
}

Result LottieAnimation::tweenTo(float to) noexcept
{
    FETCH_LOADER(Result::InsufficientCondition);
    if (static_cast<LottieLoader*>(loader)->tweenTo(to)) return Result::Success;
    return Result::InsufficientCondition;
}

uint32_t LottieAnimation::markersCnt() noexcept
{
    FETCH_LOADER(0);
    return loader->markersCnt();
}

TVG_DEPRECATED const char* LottieAnimation::marker(uint32_t idx) noexcept
{
    return marker(idx, nullptr, nullptr);
}

const char* LottieAnimation::marker(uint32_t idx, float* begin, float* end) noexcept
{
    FETCH_LOADER(nullptr);
    return loader->markers(idx, begin, end);
}

Result LottieAnimation::quality(uint8_t value) noexcept
{
    if (value > 100) return Result::InvalidArguments;
    FETCH_LOADER(Result::InsufficientCondition);
    if (!loader->quality(value)) return Result::InsufficientCondition;
    return Result::Success;
}


Result LottieAnimation::resolver(std::function<void(const LottieAudioResolver&, void*)> func, void* data) noexcept
{
    FETCH_LOADER(Result::InsufficientCondition);
    loader->resolver(std::move(func), data);
    return Result::Success;
}


LottieAnimation* LottieAnimation::gen() noexcept
{
    return new LottieAnimation;
}