//go:build windows

package filesearchservice

import (
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	cmsgSignerCertInfoParam = 7
	serviceProductName      = "Wox File Index Service"
	serviceOriginalName     = "wox-file-index-service.exe"
)

var (
	crypt32              = windows.NewLazySystemDLL("crypt32.dll")
	cryptMsgGetParamProc = crypt32.NewProc("CryptMsgGetParam")
	cryptMsgCloseProc    = crypt32.NewProc("CryptMsgClose")
)

// StageUpdate validates and stages a signed newer helper, then launches its SYSTEM restarter.
func StageUpdate(request Request) error {
	candidatePath, err := filepath.Abs(strings.TrimSpace(request.CandidatePath))
	if err != nil || candidatePath == "" {
		return fmt.Errorf("invalid update candidate path")
	}
	candidateVersion, err := semver.NewVersion(strings.TrimSpace(request.Version))
	if err != nil {
		return fmt.Errorf("invalid candidate version: %w", err)
	}
	installedVersion, err := semver.NewVersion(EmbeddedVersion)
	if err != nil || !candidateVersion.GreaterThan(installedVersion) {
		return fmt.Errorf("candidate version %s is not newer than %s", candidateVersion, EmbeddedVersion)
	}
	wantHash, err := hex.DecodeString(strings.TrimSpace(request.SHA256))
	if err != nil || len(wantHash) != sha256.Size {
		return fmt.Errorf("invalid candidate SHA-256")
	}

	candidate, err := openLockedExecutable(candidatePath)
	if err != nil {
		return err
	}
	defer candidate.Close()
	currentPath, err := os.Executable()
	if err != nil {
		return err
	}
	current, err := openLockedExecutable(currentPath)
	if err != nil {
		return err
	}
	defer current.Close()

	actualHash, err := hashLockedFile(candidate)
	if err != nil {
		return err
	}
	if !equalBytes(actualHash, wantHash) {
		return fmt.Errorf("candidate SHA-256 does not match Wox request")
	}
	if err := validateServiceIdentity(candidatePath, candidate, request.Version); err != nil {
		return err
	}
	if err := verifyAuthenticode(candidatePath, candidate); err != nil {
		return fmt.Errorf("candidate signature: %w", err)
	}
	if err := verifyAuthenticode(currentPath, current); err != nil {
		return fmt.Errorf("installed service signature: %w", err)
	}
	candidateSigner, err := signerCertificateHash(candidatePath)
	if err != nil {
		return fmt.Errorf("candidate signer: %w", err)
	}
	currentSigner, err := signerCertificateHash(currentPath)
	if err != nil {
		return fmt.Errorf("installed service signer: %w", err)
	}
	if !equalBytes(candidateSigner, currentSigner) {
		return fmt.Errorf("candidate signer does not match the installed Wox service")
	}
	if err := samePEMachine(candidate, current); err != nil {
		return err
	}

	programFiles := strings.TrimSpace(os.Getenv("ProgramFiles"))
	if programFiles == "" {
		return errors.New("ProgramFiles is unavailable")
	}
	destinationDir := filepath.Join(programFiles, "Wox", "FileIndexService", candidateVersion.String())
	if err := os.MkdirAll(destinationDir, 0755); err != nil {
		return err
	}
	destinationPath := filepath.Join(destinationDir, serviceOriginalName)
	if err := copyLockedExecutable(candidate, destinationPath); err != nil {
		return err
	}
	destination, err := openLockedExecutable(destinationPath)
	if err != nil {
		return err
	}
	destinationHash, hashErr := hashLockedFile(destination)
	destination.Close()
	if hashErr != nil {
		return fmt.Errorf("staged service hash verification failed: %w", hashErr)
	}
	if !equalBytes(destinationHash, wantHash) {
		return fmt.Errorf("staged service SHA-256 does not match the candidate")
	}

	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	service, err := m.OpenService(Name)
	if err != nil {
		return err
	}
	defer service.Close()
	config, err := service.Config()
	if err != nil {
		return err
	}
	oldPath := strings.Trim(config.BinaryPathName, `"`)
	config.BinaryPathName = destinationPath
	if err := service.UpdateConfig(config); err != nil {
		return err
	}
	command := exec.Command(destinationPath, "finish-update", "--pid", strconv.Itoa(os.Getpid()), "--old-path", oldPath)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP}
	if err := command.Start(); err != nil {
		config.BinaryPathName = oldPath
		_ = service.UpdateConfig(config)
		return err
	}
	_ = command.Process.Release()
	return nil
}

// FinishUpdate waits for the old service to exit, starts the staged service, and rolls back on failure.
func FinishUpdate(oldPID uint32, oldPath string) error {
	if oldPID == 0 || strings.TrimSpace(oldPath) == "" {
		return fmt.Errorf("invalid update restart arguments")
	}
	if process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, oldPID); err == nil {
		waitResult, waitErr := windows.WaitForSingleObject(process, 30_000)
		windows.CloseHandle(process)
		if waitErr != nil || waitResult != windows.WAIT_OBJECT_0 {
			return fmt.Errorf("old service process did not exit: result=%d err=%v", waitResult, waitErr)
		}
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	service, err := m.OpenService(Name)
	if err != nil {
		return err
	}
	defer service.Close()
	if err := startAndWait(service, 20*time.Second); err != nil {
		config, configErr := service.Config()
		if configErr != nil {
			return fmt.Errorf("start updated service: %w; load rollback config: %v", err, configErr)
		}
		config.BinaryPathName = oldPath
		if configErr = service.UpdateConfig(config); configErr != nil {
			return fmt.Errorf("start updated service: %w; rollback config: %v", err, configErr)
		}
		if rollbackErr := startAndWait(service, 20*time.Second); rollbackErr != nil {
			return fmt.Errorf("start updated service: %w; rollback start: %v", err, rollbackErr)
		}
		return fmt.Errorf("updated service failed to start; restored %s", oldPath)
	}
	removeOldVersionDirectory(oldPath)
	return nil
}

func openLockedExecutable(path string) (*os.File, error) {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(pathPointer, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

func hashLockedFile(file *os.File) ([]byte, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}

func validateServiceIdentity(path string, file *os.File, expectedVersion string) error {
	productName, err := versionString(path, "ProductName")
	if err != nil || productName != serviceProductName {
		return fmt.Errorf("candidate product name is not %q", serviceProductName)
	}
	originalName, err := versionString(path, "OriginalFilename")
	if err != nil || !strings.EqualFold(originalName, serviceOriginalName) {
		return fmt.Errorf("candidate original filename is not %q", serviceOriginalName)
	}
	fileVersion, err := versionString(path, "FileVersion")
	if err != nil {
		return fmt.Errorf("candidate version: %w", err)
	}
	wantVersion, err := semver.NewVersion(expectedVersion)
	if err != nil {
		return err
	}
	gotVersion, err := semver.NewVersion(fileVersion)
	if err != nil || !gotVersion.Equal(wantVersion) {
		return fmt.Errorf("candidate file version %q does not match %s", fileVersion, wantVersion)
	}
	_, err = file.Seek(0, io.SeekStart)
	return err
}

func versionString(path string, key string) (string, error) {
	var zero windows.Handle
	size, err := windows.GetFileVersionInfoSize(path, &zero)
	if err != nil || size == 0 {
		return "", fmt.Errorf("read version size: %w", err)
	}
	buffer := make([]byte, size)
	if err := windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&buffer[0])); err != nil {
		return "", err
	}
	var translationPointer unsafe.Pointer
	var translationLength uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&buffer[0]), `\VarFileInfo\Translation`, unsafe.Pointer(&translationPointer), &translationLength); err != nil || translationLength < 4 {
		return "", fmt.Errorf("read version translation: %w", err)
	}
	translation := unsafe.Slice((*uint16)(translationPointer), 2)
	query := fmt.Sprintf(`\StringFileInfo\%04x%04x\%s`, translation[0], translation[1], key)
	var valuePointer unsafe.Pointer
	var valueLength uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&buffer[0]), query, unsafe.Pointer(&valuePointer), &valueLength); err != nil || valueLength == 0 {
		return "", fmt.Errorf("read version value %s: %w", key, err)
	}
	value := unsafe.Slice((*uint16)(valuePointer), valueLength)
	return windows.UTF16ToString(value), nil
}

func verifyAuthenticode(path string, file *os.File) error {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	fileInfo := windows.WinTrustFileInfo{
		Size:     uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})),
		FilePath: pathPointer,
		File:     windows.Handle(file.Fd()),
	}
	data := windows.WinTrustData{
		Size:                            uint32(unsafe.Sizeof(windows.WinTrustData{})),
		UIChoice:                        windows.WTD_UI_NONE,
		RevocationChecks:                windows.WTD_REVOKE_NONE,
		UnionChoice:                     windows.WTD_CHOICE_FILE,
		StateAction:                     windows.WTD_STATEACTION_VERIFY,
		ProvFlags:                       windows.WTD_CACHE_ONLY_URL_RETRIEVAL | windows.WTD_REVOCATION_CHECK_NONE,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(&fileInfo),
	}
	verifyErr := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &data)
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	_ = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &data)
	return verifyErr
}

func signerCertificateHash(path string) ([]byte, error) {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var encoding uint32
	var contentType uint32
	var formatType uint32
	var store windows.Handle
	var message windows.Handle
	if err := windows.CryptQueryObject(
		windows.CERT_QUERY_OBJECT_FILE,
		unsafe.Pointer(pathPointer),
		windows.CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED_EMBED,
		windows.CERT_QUERY_FORMAT_FLAG_BINARY,
		0,
		&encoding,
		&contentType,
		&formatType,
		&store,
		&message,
		nil,
	); err != nil {
		return nil, err
	}
	defer windows.CertCloseStore(store, 0)
	defer cryptMsgCloseProc.Call(uintptr(message))

	var infoSize uint32
	if result, _, callErr := cryptMsgGetParamProc.Call(uintptr(message), cmsgSignerCertInfoParam, 0, 0, uintptr(unsafe.Pointer(&infoSize))); result == 0 {
		return nil, callErr
	}
	info := make([]byte, infoSize)
	if result, _, callErr := cryptMsgGetParamProc.Call(uintptr(message), cmsgSignerCertInfoParam, 0, uintptr(unsafe.Pointer(&info[0])), uintptr(unsafe.Pointer(&infoSize))); result == 0 {
		return nil, callErr
	}
	certificate, err := windows.CertFindCertificateInStore(store, encoding, 0, windows.CERT_FIND_SUBJECT_CERT, unsafe.Pointer(&info[0]), nil)
	if err != nil {
		return nil, err
	}
	defer windows.CertFreeCertificateContext(certificate)
	encoded := unsafe.Slice(certificate.EncodedCert, certificate.Length)
	hash := sha256.Sum256(encoded)
	return hash[:], nil
}

func samePEMachine(candidate *os.File, current *os.File) error {
	candidatePE, err := pe.NewFile(candidate)
	if err != nil {
		return fmt.Errorf("candidate PE: %w", err)
	}
	defer candidatePE.Close()
	currentPE, err := pe.NewFile(current)
	if err != nil {
		return fmt.Errorf("installed service PE: %w", err)
	}
	defer currentPE.Close()
	if candidatePE.Machine != currentPE.Machine {
		return fmt.Errorf("candidate architecture does not match installed service")
	}
	return nil
}

func copyLockedExecutable(source *os.File, destinationPath string) error {
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return err
	}
	temporaryPath := fmt.Sprintf("%s.new-%d", destinationPath, os.Getpid())
	_ = os.Remove(temporaryPath)
	destination, err := os.OpenFile(temporaryPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	syncErr := destination.Sync()
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	return windows.MoveFileEx(windows.StringToUTF16Ptr(temporaryPath), windows.StringToUTF16Ptr(destinationPath), windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

func startAndWait(service *mgr.Service, timeout time.Duration) error {
	if err := service.Start(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return err
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := service.Query()
		if err != nil {
			return err
		}
		if status.State == svc.Running {
			return nil
		}
		if status.State == svc.Stopped {
			return fmt.Errorf("service stopped with exit code %d", status.Win32ExitCode)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("service start timed out")
}

func removeOldVersionDirectory(oldPath string) {
	programFiles := strings.TrimSpace(os.Getenv("ProgramFiles"))
	if programFiles == "" {
		return
	}
	base, baseErr := filepath.Abs(filepath.Join(programFiles, "Wox", "FileIndexService"))
	target, targetErr := filepath.Abs(filepath.Dir(strings.Trim(oldPath, `"`)))
	current, currentErr := os.Executable()
	if baseErr != nil || targetErr != nil || currentErr != nil || strings.EqualFold(target, filepath.Dir(current)) {
		return
	}
	relative, err := filepath.Rel(base, target)
	if err != nil || relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, `..\`) {
		return
	}
	_ = os.RemoveAll(target)
}

func equalBytes(left []byte, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}

// HashFile returns the SHA-256 used by Wox's update request.
func HashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
