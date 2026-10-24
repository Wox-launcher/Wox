package shell

// MS-SHLLINK header, LinkFlags and ExtraData layouts. Keep wire values here so
// validation reads in terms of fields rather than undocumented byte offsets.
const (
	shortcutHeaderSize      = 76
	shortcutCLSIDOffset     = 4
	shortcutFlagsOffset     = 20
	shortcutShowOffset      = 60
	shortcutReservedOffset  = 64
	shortcutReservedSize    = 12
	shortcutMaxFileSize     = 1 << 20
	shortcutWordSize        = 2
	shortcutDWORDSize       = 4
	shortcutGUIDSize        = 16
	shortcutLinkInfoMinSize = 28

	shortcutHasTargetIDList       = 0x1
	shortcutHasLinkInfo           = 0x2
	shortcutHasName               = 0x4
	shortcutHasRelativePath       = 0x8
	shortcutHasWorkingDirectory   = 0x10
	shortcutHasArguments          = 0x20
	shortcutHasIconLocation       = 0x40
	shortcutIsUnicode             = 0x80
	shortcutHasEnvironment        = 0x200
	shortcutRunAsUser             = 0x2000
	shortcutHasEnvironmentIcon    = 0x4000
	shortcutForceNoLinkTrack      = 0x40000
	shortcutEnableTargetMetadata  = 0x80000
	shortcutPreferEnvironmentPath = 0x02000000
	shortcutAllowedFlags          = shortcutHasTargetIDList | shortcutHasLinkInfo | shortcutHasName |
		shortcutHasRelativePath | shortcutHasWorkingDirectory | shortcutHasArguments |
		shortcutHasIconLocation | shortcutIsUnicode | shortcutHasEnvironment | shortcutRunAsUser |
		shortcutHasEnvironmentIcon | shortcutForceNoLinkTrack | shortcutEnableTargetMetadata | shortcutPreferEnvironmentPath

	shortcutShowNormal             = 1
	shortcutShowMaximized          = 3
	shortcutShowMinNoActive        = 7
	shortcutExtraHeaderSize        = 8
	shortcutExtraSignatureOffset   = 4
	shortcutEnvironmentBlock       = 0xa0000001
	shortcutTrackerBlock           = 0xa0000003
	shortcutSpecialFolderBlock     = 0xa0000005
	shortcutEnvironmentIconBlock   = 0xa0000007
	shortcutPropertyStoreBlock     = 0xa0000009
	shortcutKnownFolderBlock       = 0xa000000b
	shortcutEnvironmentBlockSize   = 788
	shortcutTrackerBlockSize       = 96
	shortcutSpecialFolderBlockSize = 16
	shortcutKnownFolderBlockSize   = 28
)

// MS-PROPSTORE serialized property storage and typed-value layouts.
const (
	shortcutStoreVersion           = 0x53505331
	shortcutStoreVersionOffset     = 4
	shortcutStoreFormatOffset      = 8
	shortcutStoreHeaderSize        = 24
	shortcutPropertyIDOffset       = 4
	shortcutPropertyReservedOffset = 8
	shortcutPropertyTypeOffset     = 9
	shortcutPropertyPaddingOffset  = 11
	shortcutPropertyHeaderSize     = 13
	shortcutPropertyStringOffset   = shortcutPropertyHeaderSize + shortcutDWORDSize
	shortcutPropertyBoolSize       = shortcutPropertyHeaderSize + shortcutWordSize
	shortcutPropertyPaddedBoolSize = 17
	shortcutPropertyGUIDSize       = shortcutPropertyHeaderSize + shortcutGUIDSize
	shortcutVariantBool            = 11
	shortcutVariantString          = 31
	shortcutVariantGUID            = 72
	shortcutVariantTrue            = 0xffff
	shortcutCreatorSIDProperty     = 4
	shortcutVolumeIDProperty       = 104
	shortcutAppIDProperty          = 5
	shortcutDualModeProperty       = 11
	shortcutToastActivatorProperty = 26
)

// GUIDs use their on-disk little-endian representation, not display-string byte order.
var (
	shortcutShellLinkCLSID     = []byte{1, 20, 2, 0, 0, 0, 0, 0, 0xc0, 0, 0, 0, 0, 0, 0, 0x46}
	shortcutCreatorSIDFormat   = []byte{0xe2, 0x8a, 0x58, 0x46, 0xbc, 0x4c, 0x38, 0x43, 0xbb, 0xfc, 0x13, 0x93, 0x26, 0x98, 0x6d, 0xce}
	shortcutVolumeIDFormat     = []byte{0xb1, 0x16, 0x6d, 0x44, 0xad, 0x8d, 0x70, 0x48, 0xa7, 0x48, 0x40, 0x2e, 0xa4, 0x3d, 0x78, 0x8c}
	shortcutAppUserModelFormat = []byte{0x55, 0x28, 0x4c, 0x9f, 0x79, 0x9f, 0x39, 0x4b, 0xa8, 0xd0, 0xe1, 0xd4, 0x2d, 0xe1, 0xd5, 0xf3}
)

// Default activation must preserve Shell identity; explicit launches may use
// known app identity and toast metadata without delegating activation to Shell.
type shortcutLaunchPurpose string

const (
	shortcutDefaultActivation  shortcutLaunchPurpose = "default activation"
	shortcutExplicitExecutable shortcutLaunchPurpose = "explicit executable"
)
