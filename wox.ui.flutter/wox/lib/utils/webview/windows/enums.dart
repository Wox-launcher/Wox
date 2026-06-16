/// Loading state
// Order must match WebviewLoadingState (see webview.h)
enum LoadingState { none, loading, navigationCompleted }

/// Pointer button type
// Order must match WebviewPointerButton (see webview.h)
enum PointerButton { none, primary, secondary, tertiary }

/// Pointer Event kind
// Order must match WebviewPointerEventKind (see webview.h)
enum WebviewPointerEventKind { activate, down, enter, leave, up, update }

/// Permission kind
// Order must match WebviewPermissionKind (see webview.h)
enum WebviewPermissionKind { unknown, microphone, camera, geoLocation, notifications, otherSensors, clipboardRead }

enum WebviewPermissionDecision { none, allow, deny }

/// The policy for popup requests.
///
/// [allow] allows popups and will create new windows.
/// [deny] suppresses popups.
/// [sameWindow] displays popup contents in the current WebView.
enum WebviewPopupWindowPolicy { allow, deny, sameWindow }

/// The kind of cross origin resource access for virtual hosts
///
/// [deny] all cross origin requests are denied.
/// [allow] all cross origin requests are allowed.
/// [denyCors] sub resource cross origin requests are allowed, otherwise denied.
///
/// For more detailed information, please refer to
/// [Microsofts](https://docs.microsoft.com/en-us/microsoft-edge/webview2/reference/win32/icorewebview2#corewebview2_host_resource_access_kind)
/// documentation.
// Order must match WebviewHostResourceAccessKind (see webview.h)
enum WebviewHostResourceAccessKind { deny, allow, denyCors }

enum WebErrorStatus {
  WebErrorStatusUnknown,
  WebErrorStatusCertificateCommonNameIsIncorrect,
  WebErrorStatusCertificateExpired,
  WebErrorStatusClientCertificateContainsErrors,
  WebErrorStatusCertificateRevoked,
  WebErrorStatusCertificateIsInvalid,
  WebErrorStatusServerUnreachable,
  WebErrorStatusTimeout,
  WebErrorStatusErrorHTTPInvalidServerResponse,
  WebErrorStatusConnectionAborted,
  WebErrorStatusConnectionReset,
  WebErrorStatusDisconnected,
  WebErrorStatusCannotConnect,
  WebErrorStatusHostNameNotResolved,
  WebErrorStatusOperationCanceled,
  WebErrorStatusRedirectFailed,
  WebErrorStatusUnexpectedError,
  WebErrorStatusValidAuthenticationCredentialsRequired,
  WebErrorStatusValidProxyAuthenticationRequired,
}
