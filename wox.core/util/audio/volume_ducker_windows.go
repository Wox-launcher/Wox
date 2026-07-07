package audio

/*
#cgo LDFLAGS: -lole32 -lmmeapi
#include <windows.h>
#include <mmdeviceapi.h>
#include <endpointvolume.h>

// getSystemVolumeWin returns the current system output volume (0-100).
int getSystemVolumeWin();
// setSystemVolumeWin sets the system output volume (0-100).
void setSystemVolumeWin(int volume);
*/
import "C"

// getSystemVolume returns the current system output volume (0-100) on Windows.
func getSystemVolume() (int, error) {
	return int(C.getSystemVolumeWin()), nil
}

// setSystemVolume sets the system output volume (0-100) on Windows.
func setSystemVolume(volume int) error {
	C.setSystemVolumeWin(C.int(volume))
	return nil
}
