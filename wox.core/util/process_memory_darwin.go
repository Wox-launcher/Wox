//go:build darwin

package util

/*
#include <errno.h>
#include <libproc.h>
#include <stdint.h>
#include <sys/resource.h>

static int woxDarwinReadProcessFootprint(int pid, uint64_t *footprint) {
	// Activity Monitor's Memory column is closer to the task footprint than RSS.
	// proc_pid_rusage exposes ri_phys_footprint without shelling out to ps.
	struct rusage_info_v2 info;
	int ret = proc_pid_rusage(pid, RUSAGE_INFO_V2, (rusage_info_t *)&info);
	if (ret != 0) {
		return errno == 0 ? ret : errno;
	}

	*footprint = info.ri_phys_footprint;
	return 0;
}
*/
import "C"
import "fmt"

func getProcessMemoryBytes(pid int) (uint64, error) {
	var footprint C.uint64_t
	if errCode := C.woxDarwinReadProcessFootprint(C.int(pid), &footprint); errCode != 0 {
		return 0, fmt.Errorf("failed to read process footprint for pid %d: errno %d", pid, int(errCode))
	}

	// Feature change: macOS debug memory should be comparable with Activity
	// Monitor's Memory column, so use physical footprint instead of RSS/Real Mem.
	return uint64(footprint), nil
}
