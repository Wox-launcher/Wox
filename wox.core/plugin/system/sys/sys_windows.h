#ifndef WOX_SYS_AUDIO_WINDOWS_H
#define WOX_SYS_AUDIO_WINDOWS_H

#include <windows.h>

#ifdef __cplusplus
extern "C" {
#endif

HRESULT wox_sys_set_master_volume(float level);
HRESULT wox_sys_volume_step_up(void);
HRESULT wox_sys_volume_step_down(void);
HRESULT wox_sys_toggle_mute(void);
HRESULT wox_sys_show_task_view(void);

#ifdef __cplusplus
}
#endif

#endif
