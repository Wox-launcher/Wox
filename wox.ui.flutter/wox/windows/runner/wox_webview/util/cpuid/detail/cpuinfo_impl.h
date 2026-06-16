#pragma once

#include "../cpuinfo.h"

namespace cpuid {

struct cpuinfo::impl {
  impl()
      : m_has_fpu(false),
        m_has_mmx(false),
        m_has_sse(false),
        m_has_sse2(false),
        m_has_sse3(false),
        m_has_ssse3(false),
        m_has_sse4_1(false),
        m_has_sse4_2(false),
        m_has_pclmulqdq(false),
        m_has_avx(false),
        m_has_avx2(false),
        m_has_avx512_f(false),
        m_has_avx512_dq(false),
        m_has_avx512_ifma(false),
        m_has_avx512_pf(false),
        m_has_avx512_er(false),
        m_has_avx512_cd(false),
        m_has_avx512_bw(false),
        m_has_avx512_vl(false),
        m_has_avx512_vbmi(false),
        m_has_avx512_vbmi2(false),
        m_has_avx512_vnni(false),
        m_has_avx512_bitalg(false),
        m_has_avx512_vpopcntdq(false),
        m_has_avx512_4vnniw(false),
        m_has_avx512_4fmaps(false),
        m_has_avx512_vp2intersect(false),
        m_has_f16c(false),
        m_has_neon(false) {}

  bool m_has_fpu;
  bool m_has_mmx;
  bool m_has_sse;
  bool m_has_sse2;
  bool m_has_sse3;
  bool m_has_ssse3;
  bool m_has_sse4_1;
  bool m_has_sse4_2;
  bool m_has_pclmulqdq;
  bool m_has_avx;
  bool m_has_avx2;
  bool m_has_avx512_f;
  bool m_has_avx512_dq;
  bool m_has_avx512_ifma;
  bool m_has_avx512_pf;
  bool m_has_avx512_er;
  bool m_has_avx512_cd;
  bool m_has_avx512_bw;
  bool m_has_avx512_vl;
  bool m_has_avx512_vbmi;
  bool m_has_avx512_vbmi2;
  bool m_has_avx512_vnni;
  bool m_has_avx512_bitalg;
  bool m_has_avx512_vpopcntdq;
  bool m_has_avx512_4vnniw;
  bool m_has_avx512_4fmaps;
  bool m_has_avx512_vp2intersect;
  bool m_has_f16c;
  bool m_has_neon;
};
}  // namespace cpuid
