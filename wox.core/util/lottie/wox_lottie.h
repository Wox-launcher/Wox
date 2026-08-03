#ifndef WOX_LOTTIE_H
#define WOX_LOTTIE_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct WoxLottie WoxLottie;

WoxLottie* wox_lottie_create(const char* json, size_t length, uint32_t width, uint32_t height, int* error_code);
float wox_lottie_duration(const WoxLottie* animation);
int wox_lottie_render(WoxLottie* animation, float progress, uint8_t* rgba, size_t length);
void wox_lottie_destroy(WoxLottie* animation);

#ifdef __cplusplus
}
#endif

#endif
