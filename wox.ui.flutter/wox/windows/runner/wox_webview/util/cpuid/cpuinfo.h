#pragma once

#include <memory>

namespace cpuid {

class cpuinfo {
 public:
  struct impl;

  cpuinfo();
  ~cpuinfo();

  // Has X87 FPU
  bool has_fpu() const;

  // Return true if the CPU supports MMX
  bool has_mmx() const;

  // Return true if the CPU supports SSE
  bool has_sse() const;

  // Return true if the CPU supports SSE2
  bool has_sse2() const;

  // Return true if the CPU supports SSE3
  bool has_sse3() const;

  // Return true if the CPU supports SSSE3
  bool has_ssse3() const;

  // Return true if the CPU supports SSE 4.1
  bool has_sse4_1() const;

  // Return true if the CPU supports SSE 4.2
  bool has_sse4_2() const;

  // Return true if the CPU supports pclmulqdq
  bool has_pclmulqdq() const;

  // Return true if the CPU supports AVX
  bool has_avx() const;

  // Return true if the CPU supports AVX2
  bool has_avx2() const;

  // Return true if the CPU supports AVX512F
  bool has_avx512_f() const;

  // Return true if the CPU supports AVX512DQ
  bool has_avx512_dq() const;

  // Return true if the CPU supports AVX512_IFMA
  bool has_avx512_ifma() const;

  // Return true if the CPU supports AVX512PF
  bool has_avx512_pf() const;

  // Return true if the CPU supports AVX512ER
  bool has_avx512_er() const;

  // Return true if the CPU supports AVX512CD
  bool has_avx512_cd() const;

  // Return true if the CPU supports AVX512BW
  bool has_avx512_bw() const;

  // Return true if the CPU supports AVX512VL
  bool has_avx512_vl() const;

  // Return true if the CPU supports AVX512_VBMI
  bool has_avx512_vbmi() const;

  // Return true if the CPU supports AVX512_VBMI2
  bool has_avx512_vbmi2() const;

  // Return true if the CPU supports AVX512_VNNI
  bool has_avx512_vnni() const;

  // Return true if the CPU supports AVX512_BITALG
  bool has_avx512_bitalg() const;

  // Return true if the CPU supports AVX512_VPOPCNTDQ
  bool has_avx512_vpopcntdq() const;

  // Return true if the CPU supports AVX512_4VNNIW
  bool has_avx512_4vnniw() const;

  // Return true if the CPU supports AVX512_4FMAPS
  bool has_avx512_4fmaps() const;

  // Return true if the CPU supports AVX512_VP2INTERSECT
  bool has_avx512_vp2intersect() const;

  // Return true if the CPU supports F16C
  bool has_f16c() const;

  // Return true if the CPU supports NEON
  bool has_neon() const;

 private:
  // Private implementation
  std::unique_ptr<impl> impl_;
};
}  // namespace cpuid
