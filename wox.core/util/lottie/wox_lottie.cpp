#include "wox_lottie.h"

#include <algorithm>
#include <cstdlib>
#include <cstring>

#include "thorvg_capi.h"

struct WoxLottie {
    Tvg_Animation animation;
    Tvg_Canvas canvas;
    uint32_t* pixels;
    uint32_t width;
    uint32_t height;
    float duration;
    float total_frames;
};

// Release the canvas before its animation drops the shared picture reference.
static void destroy_animation(WoxLottie* animation)
{
    if (!animation) return;
    if (animation->canvas) tvg_canvas_destroy(animation->canvas);
    if (animation->animation) tvg_animation_del(animation->animation);
    std::free(animation->pixels);
    std::free(animation);
    tvg_engine_term();
}

WoxLottie* wox_lottie_create(const char* json, size_t length, uint32_t width, uint32_t height, int* error_code)
{
    if (error_code) *error_code = 0;
    if (!json || length == 0 || width == 0 || height == 0) {
        if (error_code) *error_code = 1;
        return nullptr;
    }
    if (tvg_engine_init(0) != TVG_RESULT_SUCCESS) {
        if (error_code) *error_code = 2;
        return nullptr;
    }

    auto animation = static_cast<WoxLottie*>(std::calloc(1, sizeof(WoxLottie)));
    if (!animation) {
        tvg_engine_term();
        if (error_code) *error_code = 3;
        return nullptr;
    }
    animation->width = width;
    animation->height = height;
    animation->animation = tvg_animation_new();
    auto picture = animation->animation ? tvg_animation_get_picture(animation->animation) : nullptr;
    if (!picture || tvg_picture_load_data(picture, json, static_cast<uint32_t>(length), "lottie+json", nullptr, true) != TVG_RESULT_SUCCESS) {
        if (error_code) *error_code = 4;
        destroy_animation(animation);
        return nullptr;
    }
    if (tvg_picture_set_size(picture, static_cast<float>(width), static_cast<float>(height)) != TVG_RESULT_SUCCESS ||
        tvg_animation_get_duration(animation->animation, &animation->duration) != TVG_RESULT_SUCCESS ||
        tvg_animation_get_total_frame(animation->animation, &animation->total_frames) != TVG_RESULT_SUCCESS) {
        if (error_code) *error_code = 5;
        destroy_animation(animation);
        return nullptr;
    }

    animation->pixels = static_cast<uint32_t*>(std::calloc(static_cast<size_t>(width) * height, sizeof(uint32_t)));
    animation->canvas = tvg_swcanvas_create(TVG_ENGINE_OPTION_NONE);
    if (!animation->pixels || !animation->canvas ||
        tvg_swcanvas_set_target(animation->canvas, animation->pixels, width, width, height, TVG_COLORSPACE_ABGR8888) != TVG_RESULT_SUCCESS ||
        tvg_canvas_add(animation->canvas, picture) != TVG_RESULT_SUCCESS) {
        if (error_code) *error_code = 6;
        destroy_animation(animation);
        return nullptr;
    }
    return animation;
}

float wox_lottie_duration(const WoxLottie* animation)
{
    return animation ? animation->duration : 0.0f;
}

int wox_lottie_render(WoxLottie* animation, float progress, uint8_t* rgba, size_t length)
{
    if (!animation || !rgba || length < static_cast<size_t>(animation->width) * animation->height * 4) return 1;
    progress = std::max(0.0f, std::min(progress, 1.0f));
    float frame = progress * std::max(0.0f, animation->total_frames - 0.001f);
    auto frame_result = tvg_animation_set_frame(animation->animation, frame);
    if (frame_result != TVG_RESULT_SUCCESS && frame_result != TVG_RESULT_INSUFFICIENT_CONDITION) return 2;
    if (tvg_canvas_update(animation->canvas) != TVG_RESULT_SUCCESS ||
        tvg_canvas_draw(animation->canvas, true) != TVG_RESULT_SUCCESS ||
        tvg_canvas_sync(animation->canvas) != TVG_RESULT_SUCCESS) return 3;
    std::memcpy(rgba, animation->pixels, static_cast<size_t>(animation->width) * animation->height * 4);
    return 0;
}

void wox_lottie_destroy(WoxLottie* animation)
{
    destroy_animation(animation);
}
