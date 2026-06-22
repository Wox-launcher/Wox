//go:build darwin

package glance

/*
#include <mach/mach.h>
#include <mach/mach_host.h>
#include <stdint.h>

static int woxDarwinReadCPUTicks(uint64_t *idle, uint64_t *total) {
	// Mach exposes these counters directly; the Go layer keeps the shared
	// sampler simple while this wrapper avoids spawning fragile shell commands.
	host_cpu_load_info_data_t info;
	mach_msg_type_number_t count = HOST_CPU_LOAD_INFO_COUNT;
	kern_return_t kr = host_statistics(mach_host_self(), HOST_CPU_LOAD_INFO, (host_info_t)&info, &count);
	if (kr != KERN_SUCCESS) {
		return (int)kr;
	}

	uint64_t nextTotal = 0;
	for (int i = 0; i < CPU_STATE_MAX; i++) {
		nextTotal += info.cpu_ticks[i];
	}

	*idle = info.cpu_ticks[CPU_STATE_IDLE];
	*total = nextTotal;
	return 0;
}

static int woxDarwinReadMemory(uint64_t *totalBytes, uint64_t *pageSize, uint64_t *freePages, uint64_t *inactivePages, uint64_t *purgeablePages) {
	// Keep all Mach memory calls together so the Go calculation receives one
	// consistent snapshot and does not parse localized vm_stat text.
	host_t host = mach_host_self();

	host_basic_info_data_t basicInfo;
	mach_msg_type_number_t basicCount = HOST_BASIC_INFO_COUNT;
	kern_return_t kr = host_info(host, HOST_BASIC_INFO, (host_info_t)&basicInfo, &basicCount);
	if (kr != KERN_SUCCESS) {
		return (int)kr;
	}

	vm_size_t nativePageSize = 0;
	kr = host_page_size(host, &nativePageSize);
	if (kr != KERN_SUCCESS) {
		return (int)kr;
	}

	vm_statistics64_data_t vmStats;
	mach_msg_type_number_t vmCount = HOST_VM_INFO64_COUNT;
	kr = host_statistics64(host, HOST_VM_INFO64, (host_info64_t)&vmStats, &vmCount);
	if (kr != KERN_SUCCESS) {
		return (int)kr;
	}

	*totalBytes = basicInfo.max_mem;
	*pageSize = nativePageSize;
	*freePages = vmStats.free_count;
	*inactivePages = vmStats.inactive_count;
	*purgeablePages = vmStats.purgeable_count;
	return 0;
}
*/
import "C"
import (
	"context"
)

func readCPUSample(ctx context.Context) (cpuSample, bool) {
	_ = ctx
	var idle C.uint64_t
	var total C.uint64_t
	if C.woxDarwinReadCPUTicks(&idle, &total) != 0 || total == 0 {
		return cpuSample{}, false
	}

	// Bug fix: modern macOS no longer exposes kern.cp_time on every machine.
	// Mach host CPU counters provide the same cumulative idle/total model used
	// by the shared sampler without spawning a fragile sysctl process.
	return cpuSample{idle: uint64(idle), total: uint64(total), valid: true}, true
}

func readMemoryPercent(ctx context.Context) (float64, bool) {
	_ = ctx
	var totalBytes C.uint64_t
	var pageSize C.uint64_t
	var freePages C.uint64_t
	var inactivePages C.uint64_t
	var purgeablePages C.uint64_t
	if C.woxDarwinReadMemory(&totalBytes, &pageSize, &freePages, &inactivePages, &purgeablePages) != 0 || totalBytes == 0 || pageSize == 0 {
		return 0, false
	}

	totalPages := uint64(totalBytes) / uint64(pageSize)
	if totalPages == 0 {
		return 0, false
	}

	// Darwin includes speculative pages in free_count, so adding a separate
	// speculative field would double-count reclaimable memory.
	availablePages := uint64(freePages) + uint64(inactivePages) + uint64(purgeablePages)
	if availablePages > totalPages {
		availablePages = totalPages
	}

	// Bug fix: the previous macOS calculation only treated free/speculative
	// pages as available, so normal inactive cache made Memory look pinned near
	// 99%. Inactive and purgeable pages are reclaimable, so excluding them from
	// used memory better matches macOS pressure semantics.
	return 100 * float64(totalPages-availablePages) / float64(totalPages), true
}
