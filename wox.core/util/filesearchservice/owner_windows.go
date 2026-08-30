//go:build windows

package filesearchservice

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func ownerSIDPath() (string, error) {
	programData := strings.TrimSpace(os.Getenv("ProgramData"))
	if programData == "" {
		return "", fmt.Errorf("ProgramData is unavailable")
	}
	return filepath.Join(programData, "Wox", "FileIndexService", "owner.sid"), nil
}

func indexDirectoryConfigPath() (string, error) {
	path, err := ownerSIDPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "index.path"), nil
}

// SaveOwnerSID records the Windows account allowed to use the privileged pipe.
func SaveOwnerSID(value string) error {
	value = strings.TrimSpace(value)
	if _, err := windows.StringToSid(value); err != nil {
		return fmt.Errorf("invalid service owner SID: %w", err)
	}
	path, err := ownerSIDPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := setOwnerSIDACL(filepath.Dir(path), value, true); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		return err
	}
	return setOwnerSIDACL(path, value, false)
}

func setOwnerSIDACL(path string, ownerSID string, directory bool) error {
	ownerAccess := "GR"
	if directory {
		ownerAccess = "GRGX"
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;" + ownerAccess + ";;;" + ownerSID + ")")
	if err != nil {
		return err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	)
}

// LoadOwnerSID returns the account authorized during installation.
func LoadOwnerSID() (string, error) {
	path, err := ownerSIDPath()
	if err != nil {
		return "", err
	}
	value, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	ownerSID := strings.TrimSpace(string(value))
	if _, err := windows.StringToSid(ownerSID); err != nil {
		return "", fmt.Errorf("invalid service owner SID: %w", err)
	}
	return ownerSID, nil
}

// SaveIndexDirectory records the owner-scoped Wox cache path used after service restarts.
func SaveIndexDirectory(ownerSID string, indexPath string) error {
	if err := validateIndexDirectoryPath(ownerSID, indexPath); err != nil {
		return err
	}
	configPath, err := indexDirectoryConfigPath()
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, []byte(filepath.Clean(indexPath)), 0644); err != nil {
		return err
	}
	return setOwnerSIDACL(configPath, ownerSID, false)
}

// LoadIndexDirectory returns the owner-scoped cache path saved during install or resume.
func LoadIndexDirectory(ownerSID string) (string, error) {
	configPath, err := indexDirectoryConfigPath()
	if err != nil {
		return "", err
	}
	value, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}
	indexPath := strings.TrimSpace(string(value))
	if err := validateIndexDirectoryPath(ownerSID, indexPath); err != nil {
		return "", err
	}
	return filepath.Clean(indexPath), nil
}

func validateIndexDirectoryPath(ownerSID string, indexPath string) error {
	profilePath, err := profilePathForSID(ownerSID)
	if err != nil {
		return err
	}
	return validateIndexDirectoryForProfile(profilePath, indexPath)
}

func validateIndexDirectoryForProfile(profilePath string, indexPath string) error {
	want, err := filepath.Abs(filepath.Join(profilePath, ".wox", "filesearch", IndexDirectory))
	if err != nil {
		return err
	}
	got, err := filepath.Abs(strings.TrimSpace(indexPath))
	if err != nil || strings.TrimSpace(indexPath) == "" || !strings.EqualFold(filepath.Clean(got), filepath.Clean(want)) {
		return fmt.Errorf("file index directory must be %s", want)
	}
	return nil
}

func profilePathForSID(ownerSID string) (string, error) {
	if _, err := windows.StringToSid(ownerSID); err != nil {
		return "", fmt.Errorf("invalid service owner SID: %w", err)
	}
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList\`+ownerSID, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("open owner profile: %w", err)
	}
	defer key.Close()
	value, _, err := key.GetStringValue("ProfileImagePath")
	if err != nil {
		return "", fmt.Errorf("read owner profile: %w", err)
	}
	expanded, err := registry.ExpandString(value)
	if err != nil {
		return "", fmt.Errorf("expand owner profile: %w", err)
	}
	return filepath.Clean(expanded), nil
}

func currentUserSID() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}

// RemoveOwnerSID removes the per-install pipe authorization record.
func RemoveOwnerSID() {
	path, err := ownerSIDPath()
	if err == nil {
		_ = os.RemoveAll(filepath.Dir(path))
	}
}
