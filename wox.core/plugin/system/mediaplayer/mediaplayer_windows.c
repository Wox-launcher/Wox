#include "mediaplayer_windows.h"

#include <windows.h>
#include <roapi.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>
#include <winstring.h>

typedef enum WoxAsyncStatus {
	WoxAsyncStarted = 0,
	WoxAsyncCompleted = 1,
	WoxAsyncCanceled = 2,
	WoxAsyncError = 3
} WoxAsyncStatus;
typedef struct WoxInspectableVtbl {
	HRESULT (STDMETHODCALLTYPE *QueryInterface)(void* self, REFIID riid, void** out);
	ULONG (STDMETHODCALLTYPE *AddRef)(void* self);
	ULONG (STDMETHODCALLTYPE *Release)(void* self);
	HRESULT (STDMETHODCALLTYPE *GetIids)(void* self, ULONG* iidCount, IID** iids);
	HRESULT (STDMETHODCALLTYPE *GetRuntimeClassName)(void* self, HSTRING* className);
	HRESULT (STDMETHODCALLTYPE *GetTrustLevel)(void* self, int* trustLevel);
} WoxInspectableVtbl;
typedef struct WoxManagerStaticsVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *RequestAsync)(void* self, void** operation);
} WoxManagerStaticsVtbl;
typedef struct WoxManagerVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *GetCurrentSession)(void* self, void** session);
	HRESULT (STDMETHODCALLTYPE *GetSessions)(void* self, void** sessions);
} WoxManagerVtbl;
typedef struct WoxVectorViewVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *GetAt)(void* self, UINT32 index, void** item);
	HRESULT (STDMETHODCALLTYPE *GetSize)(void* self, UINT32* size);
	HRESULT (STDMETHODCALLTYPE *IndexOf)(void* self, void* value, UINT32* index, boolean* found);
	HRESULT (STDMETHODCALLTYPE *GetMany)(void* self, UINT32 startIndex, UINT32 capacity, void** items, UINT32* actual);
} WoxVectorViewVtbl;
typedef struct WoxSessionVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *GetSourceAppUserModelId)(void* self, HSTRING* value);
	HRESULT (STDMETHODCALLTYPE *TryGetMediaPropertiesAsync)(void* self, void** operation);
	HRESULT (STDMETHODCALLTYPE *GetTimelineProperties)(void* self, void** value);
	HRESULT (STDMETHODCALLTYPE *GetPlaybackInfo)(void* self, void** value);
	HRESULT (STDMETHODCALLTYPE *TryPlayAsync)(void* self, void** operation);
	HRESULT (STDMETHODCALLTYPE *TryPauseAsync)(void* self, void** operation);
	HRESULT (STDMETHODCALLTYPE *TryStopAsync)(void* self, void** operation);
	HRESULT (STDMETHODCALLTYPE *TryRecordAsync)(void* self, void** operation);
	HRESULT (STDMETHODCALLTYPE *TryFastForwardAsync)(void* self, void** operation);
	HRESULT (STDMETHODCALLTYPE *TryRewindAsync)(void* self, void** operation);
	HRESULT (STDMETHODCALLTYPE *TrySkipNextAsync)(void* self, void** operation);
	HRESULT (STDMETHODCALLTYPE *TrySkipPreviousAsync)(void* self, void** operation);
	HRESULT (STDMETHODCALLTYPE *TryChangeChannelUpAsync)(void* self, void** operation);
	HRESULT (STDMETHODCALLTYPE *TryChangeChannelDownAsync)(void* self, void** operation);
	HRESULT (STDMETHODCALLTYPE *TryTogglePlayPauseAsync)(void* self, void** operation);
} WoxSessionVtbl;
typedef struct WoxMediaPropertiesVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *GetTitle)(void* self, HSTRING* value);
	HRESULT (STDMETHODCALLTYPE *GetSubtitle)(void* self, HSTRING* value);
	HRESULT (STDMETHODCALLTYPE *GetAlbumArtist)(void* self, HSTRING* value);
	HRESULT (STDMETHODCALLTYPE *GetArtist)(void* self, HSTRING* value);
	HRESULT (STDMETHODCALLTYPE *GetAlbumTitle)(void* self, HSTRING* value);
	HRESULT (STDMETHODCALLTYPE *GetTrackNumber)(void* self, UINT32* value);
	HRESULT (STDMETHODCALLTYPE *GetGenres)(void* self, void** value);
	HRESULT (STDMETHODCALLTYPE *GetAlbumTrackCount)(void* self, UINT32* value);
	HRESULT (STDMETHODCALLTYPE *GetPlaybackType)(void* self, void** value);
	HRESULT (STDMETHODCALLTYPE *GetThumbnail)(void* self, void** value);
} WoxMediaPropertiesVtbl;
typedef struct WoxPlaybackInfoVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *GetControls)(void* self, void** value);
	HRESULT (STDMETHODCALLTYPE *GetPlaybackStatus)(void* self, int* value);
	HRESULT (STDMETHODCALLTYPE *GetPlaybackType)(void* self, void** value);
	HRESULT (STDMETHODCALLTYPE *GetAutoRepeatMode)(void* self, void** value);
	HRESULT (STDMETHODCALLTYPE *GetPlaybackRate)(void* self, void** value);
	HRESULT (STDMETHODCALLTYPE *GetIsShuffleActive)(void* self, void** value);
} WoxPlaybackInfoVtbl;
typedef struct WoxTimeSpan {
	int64_t duration;
} WoxTimeSpan;
typedef struct WoxDateTimeOffset {
	int64_t universal_time;
	int16_t offset_minutes;
} WoxDateTimeOffset;
typedef struct WoxTimelinePropertiesVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *GetStartTime)(void* self, WoxTimeSpan* value);
	HRESULT (STDMETHODCALLTYPE *GetEndTime)(void* self, WoxTimeSpan* value);
	HRESULT (STDMETHODCALLTYPE *GetMinSeekTime)(void* self, WoxTimeSpan* value);
	HRESULT (STDMETHODCALLTYPE *GetMaxSeekTime)(void* self, WoxTimeSpan* value);
	HRESULT (STDMETHODCALLTYPE *GetPosition)(void* self, WoxTimeSpan* value);
	HRESULT (STDMETHODCALLTYPE *GetLastUpdatedTime)(void* self, WoxDateTimeOffset* value);
} WoxTimelinePropertiesVtbl;
typedef struct WoxRandomAccessStreamReferenceVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *OpenReadAsync)(void* self, void** operation);
} WoxRandomAccessStreamReferenceVtbl;
typedef struct WoxRandomAccessStreamVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *GetSize)(void* self, UINT64* value);
	HRESULT (STDMETHODCALLTYPE *SetSize)(void* self, UINT64 value);
	HRESULT (STDMETHODCALLTYPE *GetInputStreamAt)(void* self, UINT64 position, void** stream);
} WoxRandomAccessStreamVtbl;
typedef struct WoxInputStreamVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *ReadAsync)(void* self, void* buffer, UINT32 count, int options, void** operation);
} WoxInputStreamVtbl;
typedef struct WoxBufferFactoryVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *Create)(void* self, UINT32 capacity, void** buffer);
} WoxBufferFactoryVtbl;
typedef struct WoxBufferVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *GetCapacity)(void* self, UINT32* value);
	HRESULT (STDMETHODCALLTYPE *GetLength)(void* self, UINT32* value);
	HRESULT (STDMETHODCALLTYPE *SetLength)(void* self, UINT32 value);
} WoxBufferVtbl;
typedef struct WoxBufferByteAccessVtbl {
	HRESULT (STDMETHODCALLTYPE *QueryInterface)(void* self, REFIID riid, void** out);
	ULONG (STDMETHODCALLTYPE *AddRef)(void* self);
	ULONG (STDMETHODCALLTYPE *Release)(void* self);
	HRESULT (STDMETHODCALLTYPE *Buffer)(void* self, BYTE** value);
} WoxBufferByteAccessVtbl;
typedef struct WoxAsyncInfoVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *GetId)(void* self, UINT32* value);
	HRESULT (STDMETHODCALLTYPE *GetStatus)(void* self, WoxAsyncStatus* value);
	HRESULT (STDMETHODCALLTYPE *GetErrorCode)(void* self, HRESULT* value);
	HRESULT (STDMETHODCALLTYPE *Cancel)(void* self);
	HRESULT (STDMETHODCALLTYPE *Close)(void* self);
} WoxAsyncInfoVtbl;
typedef struct WoxAsyncOperationVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *SetCompleted)(void* self, void* handler);
	HRESULT (STDMETHODCALLTYPE *GetCompleted)(void* self, void** handler);
	HRESULT (STDMETHODCALLTYPE *GetResults)(void* self, void** result);
} WoxAsyncOperationVtbl;
typedef struct WoxAsyncOperationBoolVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *SetCompleted)(void* self, void* handler);
	HRESULT (STDMETHODCALLTYPE *GetCompleted)(void* self, void** handler);
	HRESULT (STDMETHODCALLTYPE *GetResults)(void* self, boolean* result);
} WoxAsyncOperationBoolVtbl;
typedef struct WoxAsyncOperationWithProgressVtbl {
	WoxInspectableVtbl base;
	HRESULT (STDMETHODCALLTYPE *SetProgress)(void* self, void* handler);
	HRESULT (STDMETHODCALLTYPE *GetProgress)(void* self, void** handler);
	HRESULT (STDMETHODCALLTYPE *SetCompleted)(void* self, void* handler);
	HRESULT (STDMETHODCALLTYPE *GetCompleted)(void* self, void** handler);
	HRESULT (STDMETHODCALLTYPE *GetResults)(void* self, void** result);
} WoxAsyncOperationWithProgressVtbl;
static const IID IID_WoxManagerStatics = {0x2050c4ee,0x11a0,0x57de,{0xae,0xd7,0xc9,0x7c,0x70,0x33,0x82,0x45}};
static const IID IID_WoxManager = {0xcace8eac,0xe86e,0x504a,{0xab,0x31,0x5f,0xf8,0xff,0x1b,0xce,0x49}};
static const IID IID_WoxSession = {0x7148c835,0x9b14,0x5ae2,{0xab,0x85,0xdc,0x9b,0x1c,0x14,0xe1,0xa8}};
static const IID IID_WoxMediaProperties = {0x68856cf6,0xadb4,0x54b2,{0xac,0x16,0x05,0x83,0x79,0x07,0xac,0xb6}};
static const IID IID_WoxPlaybackInfo = {0x94b4b6cf,0xe8ba,0x51ad,{0x87,0xa7,0xc1,0x0a,0xde,0x10,0x61,0x27}};
static const IID IID_WoxTimelineProperties = {0xede34136,0x6f25,0x588d,{0x8e,0xcf,0xea,0x5b,0x67,0x35,0xaa,0xa5}};
static const IID IID_WoxVectorViewSession = {0x9f6980d7,0x8dc2,0x39cf,{0x87,0x5b,0xa8,0xa3,0x6d,0xdc,0x24,0x2d}};
static const IID IID_WoxRandomAccessStreamReference = {0x33ee3134,0x1dd6,0x4e3a,{0x80,0x67,0xd1,0xc1,0x62,0xe8,0x64,0x2b}};
static const IID IID_WoxRandomAccessStream = {0x905a0fe1,0xbc53,0x11df,{0x8c,0x49,0x00,0x1e,0x4f,0xc6,0x86,0xda}};
static const IID IID_WoxInputStream = {0x905a0fe2,0xbc53,0x11df,{0x8c,0x49,0x00,0x1e,0x4f,0xc6,0x86,0xda}};
static const IID IID_WoxBufferFactory = {0x71af914d,0xc10f,0x484b,{0xbc,0x50,0x14,0xbc,0x62,0x3b,0x3a,0x27}};
static const IID IID_WoxBuffer = {0x905a0fe0,0xbc53,0x11df,{0x8c,0x49,0x00,0x1e,0x4f,0xc6,0x86,0xda}};
static const IID IID_WoxBufferByteAccess = {0x905a0fef,0xbc53,0x11df,{0x8c,0x49,0x00,0x1e,0x4f,0xc6,0x86,0xda}};
static const IID IID_WoxAsyncInfo = {0x00000036,0x0000,0x0000,{0xc0,0x00,0x00,0x00,0x00,0x00,0x00,0x46}};
static void wox_release(void* value) {
	if (value != NULL) {
		((WoxInspectableVtbl**)(value))[0]->Release(value);
	}
}
static HRESULT wox_qi(void* value, REFIID riid, void** out) {
	*out = NULL;
	if (value == NULL) {
		return E_POINTER;
	}
	return ((WoxInspectableVtbl**)(value))[0]->QueryInterface(value, riid, out);
}
static char* wox_strdup(const char* value) {
	if (value == NULL) {
		return NULL;
	}
	size_t len = strlen(value);
	char* out = (char*)malloc(len + 1);
	if (out == NULL) {
		return NULL;
	}
	memcpy(out, value, len + 1);
	return out;
}
static char* wox_hresult_error(const char* prefix, HRESULT hr) {
	char buf[128];
	snprintf(buf, sizeof(buf), "%s: HRESULT 0x%08lx", prefix, (unsigned long)hr);
	return wox_strdup(buf);
}
static char* wox_hstring_to_utf8(HSTRING value) {
	if (value == NULL) {
		return wox_strdup("");
	}
	UINT32 length = 0;
	PCWSTR raw = WindowsGetStringRawBuffer(value, &length);
	if (raw == NULL || length == 0) {
		return wox_strdup("");
	}
	int size = WideCharToMultiByte(CP_UTF8, 0, raw, (int)length, NULL, 0, NULL, NULL);
	if (size <= 0) {
		return wox_strdup("");
	}
	char* out = (char*)malloc((size_t)size + 1);
	if (out == NULL) {
		return NULL;
	}
	WideCharToMultiByte(CP_UTF8, 0, raw, (int)length, out, size, NULL, NULL);
	out[size] = '\0';
	return out;
}
static char* wox_get_hstring_property(void* object, HRESULT (STDMETHODCALLTYPE *getter)(void*, HSTRING*)) {
	HSTRING value = NULL;
	HRESULT hr = getter(object, &value);
	if (FAILED(hr) || value == NULL) {
		if (value != NULL) {
			WindowsDeleteString(value);
		}
		return wox_strdup("");
	}
	char* out = wox_hstring_to_utf8(value);
	WindowsDeleteString(value);
	return out;
}
static int wox_has_text(const char* value) {
	return value != NULL && value[0] != '\0';
}
static int wox_contains_ascii_ci(const char* value, const char* needle) {
	if (value == NULL || needle == NULL || needle[0] == '\0') {
		return 0;
	}
	size_t needleLen = strlen(needle);
	for (const char* cursor = value; *cursor != '\0'; cursor++) {
		size_t i = 0;
		for (; i < needleLen; i++) {
			char a = cursor[i];
			char b = needle[i];
			if (a >= 'A' && a <= 'Z') {
				a = (char)(a - 'A' + 'a');
			}
			if (b >= 'A' && b <= 'Z') {
				b = (char)(b - 'A' + 'a');
			}
			if (a == '\0' || a != b) {
				break;
			}
		}
		if (i == needleLen) {
			return 1;
		}
	}
	return 0;
}
static int wox_await_object(void* operation, void** result, int with_progress) {
	*result = NULL;
	if (operation == NULL) {
		return E_POINTER;
	}
	void* asyncInfo = NULL;
	HRESULT hr = wox_qi(operation, &IID_WoxAsyncInfo, &asyncInfo);
	if (FAILED(hr)) {
		return hr;
	}
	WoxAsyncStatus status = WoxAsyncStarted;
	for (int i = 0; i < 1000; i++) {
		hr = ((WoxAsyncInfoVtbl**)asyncInfo)[0]->GetStatus(asyncInfo, &status);
		if (FAILED(hr)) {
			wox_release(asyncInfo);
			return hr;
		}
		if (status != WoxAsyncStarted) {
			break;
		}
		Sleep(10);
	}
	if (status == WoxAsyncStarted) {
		((WoxAsyncInfoVtbl**)asyncInfo)[0]->Cancel(asyncInfo);
		wox_release(asyncInfo);
		return HRESULT_FROM_WIN32(WAIT_TIMEOUT);
	}
	if (status == WoxAsyncError) {
		HRESULT asyncHr = E_FAIL;
		((WoxAsyncInfoVtbl**)asyncInfo)[0]->GetErrorCode(asyncInfo, &asyncHr);
		wox_release(asyncInfo);
		return asyncHr;
	}
	if (status != WoxAsyncCompleted) {
		wox_release(asyncInfo);
		return E_ABORT;
	}
	if (with_progress) {
		hr = ((WoxAsyncOperationWithProgressVtbl**)operation)[0]->GetResults(operation, result);
	} else {
		hr = ((WoxAsyncOperationVtbl**)operation)[0]->GetResults(operation, result);
	}
	wox_release(asyncInfo);
	return hr;
}
static int wox_await_bool(void* operation, int* result) {
	*result = 0;
	if (operation == NULL) {
		return E_POINTER;
	}
	void* asyncInfo = NULL;
	HRESULT hr = wox_qi(operation, &IID_WoxAsyncInfo, &asyncInfo);
	if (FAILED(hr)) {
		return hr;
	}
	WoxAsyncStatus status = WoxAsyncStarted;
	for (int i = 0; i < 1000; i++) {
		hr = ((WoxAsyncInfoVtbl**)asyncInfo)[0]->GetStatus(asyncInfo, &status);
		if (FAILED(hr)) {
			wox_release(asyncInfo);
			return hr;
		}
		if (status != WoxAsyncStarted) {
			break;
		}
		Sleep(10);
	}
	if (status == WoxAsyncStarted) {
		((WoxAsyncInfoVtbl**)asyncInfo)[0]->Cancel(asyncInfo);
		wox_release(asyncInfo);
		return HRESULT_FROM_WIN32(WAIT_TIMEOUT);
	}
	if (status == WoxAsyncError) {
		HRESULT asyncHr = E_FAIL;
		((WoxAsyncInfoVtbl**)asyncInfo)[0]->GetErrorCode(asyncInfo, &asyncHr);
		wox_release(asyncInfo);
		return asyncHr;
	}
	if (status != WoxAsyncCompleted) {
		wox_release(asyncInfo);
		return E_ABORT;
	}
	boolean ok = 0;
	hr = ((WoxAsyncOperationBoolVtbl**)operation)[0]->GetResults(operation, &ok);
	wox_release(asyncInfo);
	if (FAILED(hr)) {
		return hr;
	}
	*result = ok ? 1 : 0;
	return S_OK;
}
static HRESULT wox_get_manager(void** manager) {
	*manager = NULL;
	HSTRING className = NULL;
	HRESULT hr = WindowsCreateString(
		L"Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager",
		(UINT32)wcslen(L"Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager"),
		&className
	);
	if (FAILED(hr)) {
		return hr;
	}
	void* statics = NULL;
	hr = RoGetActivationFactory(className, &IID_WoxManagerStatics, &statics);
	WindowsDeleteString(className);
	if (FAILED(hr)) {
		return hr;
	}
	void* operation = NULL;
	hr = ((WoxManagerStaticsVtbl**)statics)[0]->RequestAsync(statics, &operation);
	wox_release(statics);
	if (FAILED(hr)) {
		return hr;
	}
	void* operationResult = NULL;
	hr = wox_await_object(operation, &operationResult, 0);
	wox_release(operation);
	if (FAILED(hr)) {
		return hr;
	}
	hr = wox_qi(operationResult, &IID_WoxManager, manager);
	wox_release(operationResult);
	return hr;
}
static HRESULT wox_get_media_properties(void* session, void** properties) {
	*properties = NULL;
	if (session == NULL) {
		return E_POINTER;
	}
	void* sessionItf = NULL;
	HRESULT hr = wox_qi(session, &IID_WoxSession, &sessionItf);
	if (FAILED(hr)) {
		return hr;
	}
	void* operation = NULL;
	hr = ((WoxSessionVtbl**)sessionItf)[0]->TryGetMediaPropertiesAsync(sessionItf, &operation);
	wox_release(sessionItf);
	if (FAILED(hr)) {
		return hr;
	}
	void* operationResult = NULL;
	hr = wox_await_object(operation, &operationResult, 0);
	wox_release(operation);
	if (FAILED(hr)) {
		return hr;
	}
	hr = wox_qi(operationResult, &IID_WoxMediaProperties, properties);
	wox_release(operationResult);
	return hr;
}
static int wox_get_playback_status(void* session) {
	if (session == NULL) {
		return 0;
	}
	void* sessionItf = NULL;
	HRESULT hr = wox_qi(session, &IID_WoxSession, &sessionItf);
	if (FAILED(hr) || sessionItf == NULL) {
		return 0;
	}

	void* playbackObject = NULL;
	hr = ((WoxSessionVtbl**)sessionItf)[0]->GetPlaybackInfo(sessionItf, &playbackObject);
	wox_release(sessionItf);
	if (FAILED(hr) || playbackObject == NULL) {
		return 0;
	}

	void* playback = NULL;
	if (FAILED(wox_qi(playbackObject, &IID_WoxPlaybackInfo, &playback)) || playback == NULL) {
		playback = playbackObject;
	} else {
		wox_release(playbackObject);
	}

	int status = 0;
	((WoxPlaybackInfoVtbl**)playback)[0]->GetPlaybackStatus(playback, &status);
	wox_release(playback);
	return status;
}
static int wox_is_music_source(const char* sourceApp) {
	return wox_contains_ascii_ci(sourceApp, "music") ||
		wox_contains_ascii_ci(sourceApp, "spotify") ||
		wox_contains_ascii_ci(sourceApp, "itunes") ||
		wox_contains_ascii_ci(sourceApp, "netease") ||
		wox_contains_ascii_ci(sourceApp, "foobar") ||
		wox_contains_ascii_ci(sourceApp, "vlc");
}
static int wox_media_properties_score(void* session, void* properties) {
	if (properties == NULL) {
		return -1;
	}
	WoxMediaPropertiesVtbl* vtbl = ((WoxMediaPropertiesVtbl**)properties)[0];
	char* title = wox_get_hstring_property(properties, vtbl->GetTitle);
	char* artist = wox_get_hstring_property(properties, vtbl->GetArtist);
	char* album = wox_get_hstring_property(properties, vtbl->GetAlbumTitle);
	void* thumbnail = NULL;
	HRESULT thumbHr = vtbl->GetThumbnail(properties, &thumbnail);
	int playbackStatus = wox_get_playback_status(session);
	int hasThumbnail = SUCCEEDED(thumbHr) && thumbnail != NULL;

	int score = 0;
	if (playbackStatus == 4) {
		score += 10000;
	}
	if (wox_has_text(title)) {
		score += 10;
	}
	if (wox_has_text(artist)) {
		score += 20;
	}
	if (wox_has_text(album)) {
		score += 20;
	}
	if (hasThumbnail) {
		score += 100;
	}
	char* sourceApp = NULL;
	void* sessionItf = NULL;
	if (session != NULL && SUCCEEDED(wox_qi(session, &IID_WoxSession, &sessionItf)) && sessionItf != NULL) {
		HSTRING appId = NULL;
		if (SUCCEEDED(((WoxSessionVtbl**)sessionItf)[0]->GetSourceAppUserModelId(sessionItf, &appId)) && appId != NULL) {
			sourceApp = wox_hstring_to_utf8(appId);
			WindowsDeleteString(appId);
		}
		wox_release(sessionItf);
	}
	// Playing sessions should always outrank paused metadata; music apps and artwork break ties between active sessions.
	if (wox_is_music_source(sourceApp)) {
		score += 1000;
	}
	if (!wox_has_text(artist) && !wox_has_text(album) && (wox_contains_ascii_ci(sourceApp, "chrome") || wox_contains_ascii_ci(sourceApp, "msedge") || wox_contains_ascii_ci(sourceApp, "firefox") || wox_contains_ascii_ci(sourceApp, "electron"))) {
		score -= 20;
	}
	free(title);
	free(artist);
	free(album);
	free(sourceApp);
	wox_release(thumbnail);
	return score;
}
static HRESULT wox_select_session(void* manager, void** selected) {
	*selected = NULL;
	void* managerItf = NULL;
	HRESULT hr = wox_qi(manager, &IID_WoxManager, &managerItf);
	if (FAILED(hr)) {
		return hr;
	}
	void* current = NULL;
	((WoxManagerVtbl**)managerItf)[0]->GetCurrentSession(managerItf, &current);
	void* sessionsObject = NULL;
	hr = ((WoxManagerVtbl**)managerItf)[0]->GetSessions(managerItf, &sessionsObject);
	wox_release(managerItf);
	if (FAILED(hr)) {
		if (current != NULL) {
			*selected = current;
			return S_OK;
		}
		return hr;
	}
	void* sessions = NULL;
	hr = wox_qi(sessionsObject, &IID_WoxVectorViewSession, &sessions);
	if (FAILED(hr)) {
		sessions = sessionsObject;
	} else {
		wox_release(sessionsObject);
	}
	UINT32 size = 0;
	hr = ((WoxVectorViewVtbl**)sessions)[0]->GetSize(sessions, &size);
	if (FAILED(hr)) {
		wox_release(sessions);
		if (current != NULL) {
			*selected = current;
			return S_OK;
		}
		return hr;
	}
	int bestScore = -1;
	void* bestSession = NULL;
	for (UINT32 i = 0; i < size; i++) {
		void* session = NULL;
		if (FAILED(((WoxVectorViewVtbl**)sessions)[0]->GetAt(sessions, i, &session)) || session == NULL) {
			continue;
		}
		void* properties = NULL;
		int score = 0;
		if (SUCCEEDED(wox_get_media_properties(session, &properties)) && properties != NULL) {
			score = wox_media_properties_score(session, properties);
			wox_release(properties);
		}
		if (current != NULL && session == current) {
			score += 5;
		}
		if (score > bestScore) {
			wox_release(bestSession);
			bestSession = session;
			bestScore = score;
		} else {
			wox_release(session);
		}
	}
	wox_release(sessions);
	wox_release(current);
	if (bestSession == NULL) {
		return S_FALSE;
	}
	*selected = bestSession;
	return S_OK;
}
static HRESULT wox_read_thumbnail(void* thumbnail, unsigned char** outBytes, int* outLen) {
	*outBytes = NULL;
	*outLen = 0;
	if (thumbnail == NULL) {
		return S_FALSE;
	}
	void* thumbItf = NULL;
	HRESULT hr = wox_qi(thumbnail, &IID_WoxRandomAccessStreamReference, &thumbItf);
	if (FAILED(hr)) {
		return hr;
	}
	void* operation = NULL;
	hr = ((WoxRandomAccessStreamReferenceVtbl**)thumbItf)[0]->OpenReadAsync(thumbItf, &operation);
	wox_release(thumbItf);
	if (FAILED(hr)) {
		return hr;
	}
	void* streamObject = NULL;
	hr = wox_await_object(operation, &streamObject, 0);
	wox_release(operation);
	if (FAILED(hr)) {
		return hr;
	}
	void* stream = NULL;
	hr = wox_qi(streamObject, &IID_WoxRandomAccessStream, &stream);
	wox_release(streamObject);
	if (FAILED(hr)) {
		return hr;
	}
	UINT64 size64 = 0;
	hr = ((WoxRandomAccessStreamVtbl**)stream)[0]->GetSize(stream, &size64);
	if (FAILED(hr) || size64 == 0 || size64 > 10 * 1024 * 1024) {
		wox_release(stream);
		return FAILED(hr) ? hr : S_FALSE;
	}
	void* input = NULL;
	hr = ((WoxRandomAccessStreamVtbl**)stream)[0]->GetInputStreamAt(stream, 0, &input);
	wox_release(stream);
	if (FAILED(hr)) {
		return hr;
	}
	HSTRING bufferClass = NULL;
	hr = WindowsCreateString(L"Windows.Storage.Streams.Buffer", (UINT32)wcslen(L"Windows.Storage.Streams.Buffer"), &bufferClass);
	if (FAILED(hr)) {
		wox_release(input);
		return hr;
	}
	void* bufferFactory = NULL;
	hr = RoGetActivationFactory(bufferClass, &IID_WoxBufferFactory, &bufferFactory);
	WindowsDeleteString(bufferClass);
	if (FAILED(hr)) {
		wox_release(input);
		return hr;
	}
	void* bufferObject = NULL;
	UINT32 readSize = (UINT32)size64;
	hr = ((WoxBufferFactoryVtbl**)bufferFactory)[0]->Create(bufferFactory, readSize, &bufferObject);
	wox_release(bufferFactory);
	if (FAILED(hr)) {
		wox_release(input);
		return hr;
	}
	void* buffer = NULL;
	hr = wox_qi(bufferObject, &IID_WoxBuffer, &buffer);
	wox_release(bufferObject);
	if (FAILED(hr)) {
		wox_release(input);
		return hr;
	}
	void* readOperation = NULL;
	hr = ((WoxInputStreamVtbl**)input)[0]->ReadAsync(input, buffer, readSize, 0, &readOperation);
	wox_release(input);
	if (FAILED(hr)) {
		wox_release(buffer);
		return hr;
	}
	void* readBufferObject = NULL;
	hr = wox_await_object(readOperation, &readBufferObject, 1);
	wox_release(readOperation);
	if (FAILED(hr)) {
		wox_release(buffer);
		return hr;
	}
	void* readBuffer = NULL;
	hr = wox_qi(readBufferObject, &IID_WoxBuffer, &readBuffer);
	wox_release(readBufferObject);
	if (FAILED(hr)) {
		wox_release(buffer);
		return hr;
	}
	UINT32 length = 0;
	hr = ((WoxBufferVtbl**)readBuffer)[0]->GetLength(readBuffer, &length);
	if (FAILED(hr) || length == 0) {
		wox_release(readBuffer);
		wox_release(buffer);
		return FAILED(hr) ? hr : S_FALSE;
	}
	void* byteAccess = NULL;
	hr = wox_qi(readBuffer, &IID_WoxBufferByteAccess, &byteAccess);
	if (FAILED(hr)) {
		wox_release(readBuffer);
		wox_release(buffer);
		return hr;
	}
	BYTE* rawBytes = NULL;
	hr = ((WoxBufferByteAccessVtbl**)byteAccess)[0]->Buffer(byteAccess, &rawBytes);
	if (FAILED(hr) || rawBytes == NULL) {
		wox_release(byteAccess);
		wox_release(readBuffer);
		wox_release(buffer);
		return FAILED(hr) ? hr : E_POINTER;
	}
	unsigned char* copied = (unsigned char*)malloc(length);
	if (copied == NULL) {
		wox_release(byteAccess);
		wox_release(readBuffer);
		wox_release(buffer);
		return E_OUTOFMEMORY;
	}
	memcpy(copied, rawBytes, length);
	*outBytes = copied;
	*outLen = (int)length;
	wox_release(byteAccess);
	wox_release(readBuffer);
	wox_release(buffer);
	return S_OK;
}
static void wox_fill_media_info_from_session(void* session, WoxMediaInfo* info) {
	void* properties = NULL;
	HRESULT hr = wox_get_media_properties(session, &properties);
	if (FAILED(hr) || properties == NULL) {
		info->error = wox_hresult_error("get media properties failed", hr);
		return;
	}
	WoxMediaPropertiesVtbl* props = ((WoxMediaPropertiesVtbl**)properties)[0];
	info->title = wox_get_hstring_property(properties, props->GetTitle);
	info->artist = wox_get_hstring_property(properties, props->GetArtist);
	info->album = wox_get_hstring_property(properties, props->GetAlbumTitle);
	void* thumbnail = NULL;
	hr = props->GetThumbnail(properties, &thumbnail);
	if (SUCCEEDED(hr) && thumbnail != NULL) {
		wox_read_thumbnail(thumbnail, &info->artwork, &info->artwork_len);
	}
	wox_release(thumbnail);
	wox_release(properties);
	void* sessionItf = NULL;
	hr = wox_qi(session, &IID_WoxSession, &sessionItf);
	if (SUCCEEDED(hr) && sessionItf != NULL) {
		HSTRING appId = NULL;
		if (SUCCEEDED(((WoxSessionVtbl**)sessionItf)[0]->GetSourceAppUserModelId(sessionItf, &appId)) && appId != NULL) {
			info->app_id = wox_hstring_to_utf8(appId);
			info->app_name = wox_hstring_to_utf8(appId);
			WindowsDeleteString(appId);
		}
		info->playback_status = wox_get_playback_status(session);
		void* timelineObject = NULL;
		if (SUCCEEDED(((WoxSessionVtbl**)sessionItf)[0]->GetTimelineProperties(sessionItf, &timelineObject)) && timelineObject != NULL) {
			void* timeline = NULL;
			if (FAILED(wox_qi(timelineObject, &IID_WoxTimelineProperties, &timeline)) || timeline == NULL) {
				timeline = timelineObject;
			} else {
				wox_release(timelineObject);
			}
			if (timeline != NULL) {
				WoxTimeSpan endTime;
				WoxTimeSpan maxSeekTime;
				WoxTimeSpan position;
				memset(&endTime, 0, sizeof(endTime));
				memset(&maxSeekTime, 0, sizeof(maxSeekTime));
				memset(&position, 0, sizeof(position));
				if (SUCCEEDED(((WoxTimelinePropertiesVtbl**)timeline)[0]->GetEndTime(timeline, &endTime))) {
					info->duration = endTime.duration / 10000000;
				}
				if (info->duration == 0 && SUCCEEDED(((WoxTimelinePropertiesVtbl**)timeline)[0]->GetMaxSeekTime(timeline, &maxSeekTime))) {
					info->duration = maxSeekTime.duration / 10000000;
				}
				if (SUCCEEDED(((WoxTimelinePropertiesVtbl**)timeline)[0]->GetPosition(timeline, &position))) {
					info->position = position.duration / 10000000;
				}
				wox_release(timeline);
			}
		}
		wox_release(sessionItf);
	}
	if (!wox_has_text(info->title)) {
		info->title = wox_strdup("Unknown Media");
	}
	if (!wox_has_text(info->app_name)) {
		info->app_name = wox_strdup("Unknown Media App");
	}
	if (!wox_has_text(info->app_id)) {
		info->app_id = wox_strdup(info->app_name);
	}
	info->has_media = 1;
}
WoxMediaInfo wox_get_media_info(void) {
	WoxMediaInfo info;
	memset(&info, 0, sizeof(info));
	HRESULT hr = RoInitialize(RO_INIT_MULTITHREADED);
	int shouldUninitialize = SUCCEEDED(hr);
	if (FAILED(hr) && hr != RPC_E_CHANGED_MODE) {
		info.error = wox_hresult_error("RoInitialize failed", hr);
		return info;
	}
	void* manager = NULL;
	hr = wox_get_manager(&manager);
	if (FAILED(hr) || manager == NULL) {
		info.error = wox_hresult_error("get media manager failed", hr);
		if (shouldUninitialize) {
			RoUninitialize();
		}
		return info;
	}
	void* session = NULL;
	hr = wox_select_session(manager, &session);
	wox_release(manager);
	if (FAILED(hr) || session == NULL) {
		if (shouldUninitialize) {
			RoUninitialize();
		}
		return info;
	}
	wox_fill_media_info_from_session(session, &info);
	wox_release(session);
	if (shouldUninitialize) {
		RoUninitialize();
	}
	return info;
}
int wox_control_media(const char* command, char** error) {
	*error = NULL;
	HRESULT hr = RoInitialize(RO_INIT_MULTITHREADED);
	int shouldUninitialize = SUCCEEDED(hr);
	if (FAILED(hr) && hr != RPC_E_CHANGED_MODE) {
		*error = wox_hresult_error("RoInitialize failed", hr);
		return 0;
	}
	void* manager = NULL;
	hr = wox_get_manager(&manager);
	if (FAILED(hr) || manager == NULL) {
		*error = wox_hresult_error("get media manager failed", hr);
		if (shouldUninitialize) {
			RoUninitialize();
		}
		return 0;
	}
	void* session = NULL;
	hr = wox_select_session(manager, &session);
	wox_release(manager);
	if (FAILED(hr) || session == NULL) {
		*error = wox_strdup("no active media session");
		if (shouldUninitialize) {
			RoUninitialize();
		}
		return 0;
	}
	void* sessionItf = NULL;
	hr = wox_qi(session, &IID_WoxSession, &sessionItf);
	wox_release(session);
	if (FAILED(hr) || sessionItf == NULL) {
		*error = wox_hresult_error("query media session failed", hr);
		if (shouldUninitialize) {
			RoUninitialize();
		}
		return 0;
	}
	WoxSessionVtbl* vtbl = ((WoxSessionVtbl**)sessionItf)[0];
	void* operation = NULL;
	if (strcmp(command, "play") == 0) {
		hr = vtbl->TryPlayAsync(sessionItf, &operation);
	} else if (strcmp(command, "pause") == 0) {
		hr = vtbl->TryPauseAsync(sessionItf, &operation);
	} else if (strcmp(command, "next") == 0) {
		hr = vtbl->TrySkipNextAsync(sessionItf, &operation);
	} else if (strcmp(command, "previous") == 0) {
		hr = vtbl->TrySkipPreviousAsync(sessionItf, &operation);
	} else if (strcmp(command, "toggle") == 0) {
		hr = vtbl->TryTogglePlayPauseAsync(sessionItf, &operation);
	} else {
		wox_release(sessionItf);
		*error = wox_strdup("unsupported media command");
		if (shouldUninitialize) {
			RoUninitialize();
		}
		return 0;
	}
	wox_release(sessionItf);
	if (FAILED(hr) || operation == NULL) {
		*error = wox_hresult_error("start media control failed", hr);
		if (shouldUninitialize) {
			RoUninitialize();
		}
		return 0;
	}
	int ok = 0;
	hr = wox_await_bool(operation, &ok);
	wox_release(operation);
	if (FAILED(hr)) {
		*error = wox_hresult_error("run media control failed", hr);
		if (shouldUninitialize) {
			RoUninitialize();
		}
		return 0;
	}
	if (shouldUninitialize) {
		RoUninitialize();
	}
	return ok;
}
void wox_free_media_info(WoxMediaInfo* info) {
	if (info == NULL) {
		return;
	}
	free(info->title);
	free(info->artist);
	free(info->album);
	free(info->app_name);
	free(info->app_id);
	free(info->artwork);
	free(info->error);
	memset(info, 0, sizeof(*info));
}
void wox_free_string(char* value) {
	free(value);
}
