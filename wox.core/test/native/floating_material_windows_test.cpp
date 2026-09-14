// Run from the repository root:
// g++ -std=c++17 wox.core/test/native/floating_material_windows_test.cpp -o tone-test.exe && ./tone-test.exe
#include "../../ui/runtime/floating_material_windows_tone.h"
#include <algorithm>
#include <array>
#include <cassert>
#include <cmath>

// Simulate Direct2D's default premultiplied-alpha color-matrix contract.
static std::array<float, 4> apply(const D2D1_MATRIX_5X4_F &matrix, std::array<float, 4> pixel) {
  const float alpha = pixel[3];
  if (alpha == 0) {
    return {};
  }
  const float *m = &matrix._11;
  std::array<float, 4> output{};
  for (int channel = 0; channel < 4; ++channel) {
    float value = m[16 + channel];
    for (int input = 0; input < 4; ++input) {
      value += m[input * 4 + channel] * (input == 3 ? alpha : pixel[input] / alpha);
    }
    output[channel] = std::clamp(value, 0.0f, 1.0f) * (channel == 3 ? 1.0f : alpha);
  }
  return output;
}

// Keep dark/light backgrounds stable and suppress white and saturated foregrounds at every alpha.
int main() {
  for (const auto &base : {std::array<float, 3>{18 / 255.0f, 18 / 255.0f, 22 / 255.0f},
                          std::array<float, 3>{0.9f, 0.92f, 0.95f}}) {
    const auto matrix = floating_material_tone_matrix(base[0], base[1], base[2]);
    for (const float alpha : {0.0f, 0.25f, 0.75f, 1.0f}) {
      const std::array<float, 4> backdrop{base[0] * alpha, base[1] * alpha, base[2] * alpha, alpha};
      const auto stable = apply(matrix, backdrop);
      for (int channel = 0; channel < 4; ++channel) {
        assert(std::abs(stable[channel] - backdrop[channel]) < 0.00001f);
      }
      for (const auto &source : {std::array<float, 4>{alpha, alpha, alpha, alpha},
                                std::array<float, 4>{0, alpha, 0, alpha},
                                std::array<float, 4>{alpha, 0, 0, alpha}}) {
        const auto toned = apply(matrix, source);
        float before = 0, after = 0;
        for (int channel = 0; channel < 3; ++channel) {
          before = std::max(before, std::abs(source[channel] - backdrop[channel]));
          after = std::max(after, std::abs(toned[channel] - backdrop[channel]));
          assert(toned[channel] >= 0 && toned[channel] <= alpha);
        }
        assert(after <= before * 0.251f);
        assert(toned[3] == alpha);
      }
    }
  }
}
