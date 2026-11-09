package permission

type OSPermission string

const OSPermissionMicrophone OSPermission = "microphone"

func (p OSPermission) IsValid() bool {
	return p == OSPermissionMicrophone
}

type OSPermissionStatus string

const (
	OSPermissionGranted       OSPermissionStatus = "granted"
	OSPermissionNotDetermined OSPermissionStatus = "notDetermined"
	OSPermissionDenied        OSPermissionStatus = "denied"
	OSPermissionRestricted    OSPermissionStatus = "restricted"
	OSPermissionUnsupported   OSPermissionStatus = "unsupported"
)

func GetOSPermissionStatus(p OSPermission) OSPermissionStatus {
	switch p {
	case OSPermissionMicrophone:
		return microphonePermissionStatus()
	default:
		return OSPermissionUnsupported
	}
}

// OSPermissionPrompt contains the text shown before the system authorization dialog.
type OSPermissionPrompt struct {
	Title         string
	Reason        string
	ContinueLabel string
	CancelLabel   string
}

func RequestOSPermission(p OSPermission, prompt *OSPermissionPrompt) OSPermissionStatus {
	switch p {
	case OSPermissionMicrophone:
		return requestMicrophonePermission(prompt)
	default:
		return OSPermissionUnsupported
	}
}
