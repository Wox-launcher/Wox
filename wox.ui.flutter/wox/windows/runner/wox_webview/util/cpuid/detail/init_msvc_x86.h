#pragma once

#include <intrin.h>

#include "cpuinfo_impl.h"
#include "extract_x86_flags.h"

namespace cpuid {

void init_cpuinfo(cpuinfo::impl& info) {
  int registers[4];

  // The register information per input can be extracted from here:
  // http://en.wikipedia.org/wiki/CPUID
  //
  // CPUID should be called with EAX=0 first, as this will return the
  // maximum supported EAX input value for future calls
  __cpuid(registers, 0);
  uint32_t maximum_eax = registers[0];

  // Set registers for basic flag extraction, eax=1
  // All CPUs should support index=1
  if (maximum_eax >= 1U) {
    __cpuid(registers, 1);
    extract_x86_flags(info, registers[2], registers[3]);
  }

  // Set registers for extended flags extraction, eax=7 and ecx=0
  // This operation is not supported on older CPUs, so it should be skipped
  // to avoid incorrect results
  if (maximum_eax >= 7U) {
    __cpuidex(registers, 7, 0);
    extract_x86_extended_flags(info, registers[1], registers[2], registers[3]);
  }
}
}  // namespace cpuid
