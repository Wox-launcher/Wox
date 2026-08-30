//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"wox/util"
	"wox/util/filesearchservice"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func main() {
	args := nonEmptyArgs(os.Args[1:])
	if len(args) == 0 {
		if err := svc.Run(filesearchservice.Name, &serviceHandler{updateRequested: make(chan struct{}, 1)}); err != nil {
			os.Exit(1)
		}
		return
	}
	var err error
	ownerSID := argumentValue(args, "--owner-sid")
	indexPath := argumentValue(args, "--index-dir")
	switch args[0] {
	case "install":
		err = install(ownerSID, indexPath)
	case "start":
		err = install(ownerSID, indexPath)
	case "uninstall":
		err = uninstall()
	case "finish-update":
		var pid uint64
		pid, err = strconv.ParseUint(argumentValue(args, "--pid"), 10, 32)
		if err == nil {
			err = filesearchservice.FinishUpdate(uint32(pid), argumentValue(args, "--old-path"))
		}
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}
	resultPath := argumentValue(args, "--result")
	if err != nil {
		util.GetLogger().Error(context.Background(), "file index service command failed: "+err.Error())
		if resultPath != "" {
			_ = os.WriteFile(resultPath, []byte(err.Error()), 0600)
		}
		os.Exit(1)
	}
	if resultPath != "" {
		_ = os.WriteFile(resultPath, nil, 0600)
	}
}

func nonEmptyArgs(args []string) []string {
	result := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "" {
			result = append(result, arg)
		}
	}
	return result
}

func argumentValue(args []string, name string) string {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == name {
			return args[index+1]
		}
	}
	return ""
}

type serviceHandler struct {
	updateRequested chan struct{}
	operationMu     sync.Mutex
}

func (service *serviceHandler) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ownerSID, err := filesearchservice.LoadOwnerSID()
	if err != nil {
		return false, 1
	}
	index := filesearchservice.NewColumnIndex(ctx)
	defer index.Close()
	if indexPath, loadErr := filesearchservice.LoadIndexDirectory(ownerSID); loadErr == nil {
		if resumeErr := index.Resume(ctx, ownerSID, indexPath); resumeErr != nil {
			util.GetLogger().Warn(ctx, "file index service could not resume saved index: "+resumeErr.Error())
		}
	}
	go func() {
		_ = filesearchservice.ServeWithStats(ctx, ownerSID, index.Stats, func(request filesearchservice.Request, emit func(filesearchservice.SnapshotEntry) error) (func(), error) {
			switch request.Method {
			case "search":
				results, err := index.Search(ctx, request.Query, request.Limit, request.UsePinyin)
				if err != nil {
					return nil, err
				}
				for _, result := range results {
					if err := emit(result); err != nil {
						return nil, err
					}
				}
				return nil, nil
			case "rebuild":
				index.RebuildAsync()
				return nil, nil
			case "pause":
				service.operationMu.Lock()
				defer service.operationMu.Unlock()
				return nil, index.Pause(ctx)
			case "resume":
				service.operationMu.Lock()
				defer service.operationMu.Unlock()
				if err := index.Resume(ctx, ownerSID, request.IndexPath); err != nil {
					return nil, err
				}
				return nil, filesearchservice.SaveIndexDirectory(ownerSID, request.IndexPath)
			case "snapshot":
				service.operationMu.Lock()
				defer service.operationMu.Unlock()
				rootPaths := request.RootPaths
				if len(rootPaths) == 0 {
					rootPaths = []string{request.RootPath}
				}
				return nil, filesearchservice.EnumerateSnapshots(ctx, rootPaths, emit)
			case "update":
				service.operationMu.Lock()
				defer service.operationMu.Unlock()
				if err := filesearchservice.StageUpdate(request); err != nil {
					return nil, err
				}
				return func() {
					select {
					case service.updateRequested <- struct{}{}:
					default:
					}
				}, nil
			default:
				return nil, fmt.Errorf("unsupported method %q", request.Method)
			}
		})
	}()
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case <-service.updateRequested:
			changes <- svc.Status{State: svc.StopPending}
			return false, 0
		case request, ok := <-requests:
			if !ok {
				return false, 0
			}
			switch request.Cmd {
			case svc.Interrogate:
				changes <- request.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				return false, 0
			}
		}
	}
}

func install(ownerSID string, indexPath string) error {
	if err := filesearchservice.SaveOwnerSID(ownerSID); err != nil {
		return err
	}
	if err := filesearchservice.SaveIndexDirectory(ownerSID, indexPath); err != nil {
		return err
	}
	programFiles := os.Getenv("ProgramFiles")
	if programFiles == "" {
		return errors.New("ProgramFiles is unavailable")
	}
	destinationDir := filepath.Join(programFiles, "Wox", "FileIndexService", filesearchservice.EmbeddedVersion)
	if err := os.MkdirAll(destinationDir, 0755); err != nil {
		return err
	}
	source, err := os.Executable()
	if err != nil {
		return err
	}
	destination := filepath.Join(destinationDir, "wox-file-index-service.exe")
	if err := copyExecutable(source, destination); err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(filesearchservice.Name)
	if err == nil {
		defer s.Close()
		config, configErr := s.Config()
		if configErr != nil {
			return configErr
		}
		config.BinaryPathName = destination
		config.DisplayName = filesearchservice.DisplayName
		config.Description = filesearchservice.Description
		config.StartType = mgr.StartAutomatic
		if err := s.UpdateConfig(config); err != nil {
			return err
		}
		return restartService(s)
	}
	s, err = m.CreateService(filesearchservice.Name, destination, mgr.Config{
		DisplayName: filesearchservice.DisplayName,
		Description: filesearchservice.Description,
		StartType:   mgr.StartAutomatic,
	})
	if err != nil {
		return err
	}
	defer s.Close()
	return s.Start()
}

// restartService applies a repaired binary path even when the old service is still running.
func restartService(service *mgr.Service) error {
	status, err := service.Query()
	if err != nil {
		return err
	}
	if status.State != svc.Stopped {
		if _, err := service.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return err
		}
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			status, err = service.Query()
			if err != nil {
				return err
			}
			if status.State == svc.Stopped {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if status.State != svc.Stopped {
			return fmt.Errorf("timed out stopping existing file index service")
		}
	}
	return service.Start()
}

func uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(filesearchservice.Name)
	if err != nil {
		filesearchservice.RemoveOwnerSID()
		return nil
	}
	config, _ := s.Config()
	status, _ := s.Control(svc.Stop)
	stopDeadline := time.Now().Add(20 * time.Second)
	for status.State != svc.Stopped {
		if time.Now().After(stopDeadline) {
			s.Close()
			return fmt.Errorf("timed out stopping file index service")
		}
		time.Sleep(100 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			break
		}
	}
	if err := s.Delete(); err != nil {
		s.Close()
		return err
	}
	s.Close()
	removeInstallDirectory(config.BinaryPathName)
	filesearchservice.RemoveOwnerSID()
	return nil
}

// removeInstallDirectory deletes service versions only after validating the SCM path is inside Wox's install root.
func removeInstallDirectory(binaryPath string) {
	programFiles := strings.TrimSpace(os.Getenv("ProgramFiles"))
	if programFiles == "" || strings.TrimSpace(binaryPath) == "" {
		return
	}
	base, baseErr := filepath.Abs(filepath.Join(programFiles, "Wox", "FileIndexService"))
	binary, binaryErr := filepath.Abs(strings.Trim(binaryPath, `"`))
	if baseErr != nil || binaryErr != nil {
		return
	}
	relative, err := filepath.Rel(base, binary)
	if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, `..\`) {
		return
	}
	_ = os.RemoveAll(base)
}

func copyExecutable(sourcePath, destinationPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	temporary := destinationPath + ".new"
	destination, err := os.OpenFile(temporary, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return windows.MoveFileEx(windows.StringToUTF16Ptr(temporary), windows.StringToUTF16Ptr(destinationPath), windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
