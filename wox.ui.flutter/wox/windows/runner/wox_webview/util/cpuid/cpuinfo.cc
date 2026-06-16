#include "cpuinfo.h"

#include "detail/cpuinfo_impl.h"

#if defined(_MSC_VER) && (defined(__x86_64__) || defined(_M_X64))
#include "detail/init_msvc_x86.h"
#else
#include "detail/init_unknown.hpp"
#endif

namespace cpuid {

cpuinfo::cpuinfo() : impl_(new impl) { init_cpuinfo(*impl_); }

cpuinfo::~cpuinfo() {}

// x86 member functions
bool cpuinfo::has_fpu() const { return impl_->m_has_fpu; }

bool cpuinfo::has_mmx() const { return impl_->m_has_mmx; }

bool cpuinfo::has_sse() const { return impl_->m_has_sse; }

bool cpuinfo::has_sse2() const { return impl_->m_has_sse2; }

bool cpuinfo::has_sse3() const { return impl_->m_has_sse3; }

bool cpuinfo::has_ssse3() const { return impl_->m_has_ssse3; }

bool cpuinfo::has_sse4_1() const { return impl_->m_has_sse4_1; }

bool cpuinfo::has_sse4_2() const { return impl_->m_has_sse4_2; }

bool cpuinfo::has_pclmulqdq() const { return impl_->m_has_pclmulqdq; }

bool cpuinfo::has_avx() const { return impl_->m_has_avx; }

bool cpuinfo::has_avx2() const { return impl_->m_has_avx2; }

bool cpuinfo::has_avx512_f() const { return impl_->m_has_avx512_f; }

bool cpuinfo::has_avx512_dq() const { return impl_->m_has_avx512_dq; }

bool cpuinfo::has_avx512_ifma() const { return impl_->m_has_avx512_ifma; }

bool cpuinfo::has_avx512_pf() const { return impl_->m_has_avx512_pf; }

bool cpuinfo::has_avx512_er() const { return impl_->m_has_avx512_er; }

bool cpuinfo::has_avx512_cd() const { return impl_->m_has_avx512_cd; }

bool cpuinfo::has_avx512_bw() const { return impl_->m_has_avx512_bw; }

bool cpuinfo::has_avx512_vl() const { return impl_->m_has_avx512_vl; }

bool cpuinfo::has_avx512_vbmi() const { return impl_->m_has_avx512_vbmi; }

bool cpuinfo::has_avx512_vbmi2() const { return impl_->m_has_avx512_vbmi2; }

bool cpuinfo::has_avx512_vnni() const { return impl_->m_has_avx512_vnni; }

bool cpuinfo::has_avx512_bitalg() const { return impl_->m_has_avx512_bitalg; }

bool cpuinfo::has_avx512_vpopcntdq() const {
  return impl_->m_has_avx512_vpopcntdq;
}

bool cpuinfo::has_avx512_4vnniw() const { return impl_->m_has_avx512_4vnniw; }

bool cpuinfo::has_avx512_4fmaps() const { return impl_->m_has_avx512_4fmaps; }

bool cpuinfo::has_avx512_vp2intersect() const {
  return impl_->m_has_avx512_vp2intersect;
}

bool cpuinfo::has_f16c() const { return impl_->m_has_f16c; }

// ARM member functions
bool cpuinfo::has_neon() const { return impl_->m_has_neon; }
}  // namespace cpuid
