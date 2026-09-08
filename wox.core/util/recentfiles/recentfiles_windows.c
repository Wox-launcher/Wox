#define COBJMACROS
#define WIN32_LEAN_AND_MEAN
#define CINTERFACE
#define CONST_VTABLE

#include <stdlib.h>
#include <windows.h>
#include <objbase.h>
#include <objidl.h>
#include <shlobj.h>
#include <shobjidl.h>

#include "recentfiles_windows.h"

static int enter_com(int *owned) {
	HRESULT hr;

	if (owned == NULL) {
		return -1;
	}
	*owned = 0;
	hr = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
	if (hr == S_OK) {
		*owned = 1;
		return 0;
	}
	if (hr == S_FALSE || hr == RPC_E_CHANGED_MODE) {
		return 0;
	}
	return -1;
}

static void leave_com(int owned) {
	if (owned) {
		CoUninitialize();
	}
}

int wox_recent_resolve_lnk(const wchar_t *lnk, wchar_t *out, int out_chars) {
	IShellLinkW *link = NULL;
	IPersistFile *persist = NULL;
	HRESULT hr;
	int owned = 0;

	if (lnk == NULL || out == NULL || out_chars <= 1) {
		return -1;
	}
	out[0] = 0;
	if (enter_com(&owned) != 0) {
		return -1;
	}

	hr = CoCreateInstance(&CLSID_ShellLink, NULL, CLSCTX_INPROC_SERVER, &IID_IShellLinkW, (void **)&link);
	if (FAILED(hr) || link == NULL) {
		leave_com(owned);
		return -1;
	}

	hr = IShellLinkW_QueryInterface(link, &IID_IPersistFile, (void **)&persist);
	if (FAILED(hr) || persist == NULL) {
		IShellLinkW_Release(link);
		leave_com(owned);
		return -1;
	}

	hr = IPersistFile_Load(persist, lnk, STGM_READ);
	if (FAILED(hr)) {
		IPersistFile_Release(persist);
		IShellLinkW_Release(link);
		leave_com(owned);
		return -1;
	}

	hr = IShellLinkW_GetPath(link, out, out_chars, NULL, SLGP_RAWPATH);
	IPersistFile_Release(persist);
	IShellLinkW_Release(link);
	leave_com(owned);
	if (FAILED(hr) || out[0] == 0) {
		return -1;
	}
	return 0;
}

int wox_recent_read_ole_stream(const wchar_t *storage_path, const wchar_t *stream_name, unsigned char **out, int *out_len) {
	IStorage *storage = NULL;
	IStream *stream = NULL;
	STATSTG stat;
	HRESULT hr;
	ULONG read = 0;
	int owned = 0;
	unsigned char *buffer = NULL;

	if (out != NULL) {
		*out = NULL;
	}
	if (out_len != NULL) {
		*out_len = 0;
	}
	if (storage_path == NULL || stream_name == NULL || out == NULL || out_len == NULL) {
		return -1;
	}
	if (enter_com(&owned) != 0) {
		return -1;
	}

	hr = StgOpenStorage(storage_path, NULL, STGM_READ | STGM_SHARE_DENY_WRITE, NULL, 0, &storage);
	if (FAILED(hr) || storage == NULL) {
		hr = StgOpenStorage(storage_path, NULL, STGM_READ | STGM_SHARE_DENY_NONE, NULL, 0, &storage);
	}
	if (FAILED(hr) || storage == NULL) {
		leave_com(owned);
		return -1;
	}

	hr = IStorage_OpenStream(storage, stream_name, NULL, STGM_READ | STGM_SHARE_EXCLUSIVE, 0, &stream);
	if (FAILED(hr) || stream == NULL) {
		IStorage_Release(storage);
		leave_com(owned);
		return -1;
	}

	ZeroMemory(&stat, sizeof(stat));
	hr = IStream_Stat(stream, &stat, STATFLAG_NONAME);
	if (FAILED(hr) || stat.cbSize.QuadPart <= 0 || stat.cbSize.QuadPart > 8 * 1024 * 1024) {
		IStream_Release(stream);
		IStorage_Release(storage);
		leave_com(owned);
		return -1;
	}

	buffer = (unsigned char *)malloc((size_t)stat.cbSize.QuadPart);
	if (buffer == NULL) {
		IStream_Release(stream);
		IStorage_Release(storage);
		leave_com(owned);
		return -1;
	}

	hr = IStream_Read(stream, buffer, (ULONG)stat.cbSize.QuadPart, &read);
	IStream_Release(stream);
	IStorage_Release(storage);
	leave_com(owned);
	if (FAILED(hr) || read == 0) {
		free(buffer);
		return -1;
	}

	*out = buffer;
	*out_len = (int)read;
	return 0;
}

void wox_recent_free(void *ptr) {
	free(ptr);
}

void wox_recent_record_access(const wchar_t *path) {
	int owned = 0;

	if (path == NULL || path[0] == 0) {
		return;
	}
	if (enter_com(&owned) != 0) {
		return;
	}
	SHAddToRecentDocs(SHARD_PATHW, path);
	leave_com(owned);
}
