package osvariant

const (
	windows10FirstBuild uint32 = 10240
	windows11FirstBuild uint32 = 22000
)

// windowsPlatformVariantForBuildNumber maps stable Windows build ranges to theme variant names.
func windowsPlatformVariantForBuildNumber(buildNumber uint32) string {
	if buildNumber >= windows11FirstBuild {
		return "win11"
	}
	if buildNumber >= windows10FirstBuild {
		return "win10"
	}
	return ""
}
