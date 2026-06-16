package util

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logInstance *Log
var logOnce sync.Once

const (
	logRetentionDays = 5
	logFileBaseName  = "wox.log"
)

type Log struct {
	logger      *zap.Logger
	writer      io.Writer
	level       zap.AtomicLevel
	fileWriter  *Lumberjack
	logFolder   string
	clearLogMux sync.Mutex
}

func GetLogger() *Log {
	logOnce.Do(func() {
		logFolder := GetLocation().GetLogDirectory()
		logInstance = CreateLogger(logFolder)
		setCrashOutput(logInstance)
		logInstance.startMaintenanceRoutine()
	})
	return logInstance
}

func setCrashOutput(logInstance *Log) {
	logFile := path.Join(GetLocation().GetLogDirectory(), "crash.log")
	// Open file in append mode instead of create
	crashFile, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logInstance.Error(context.Background(), fmt.Sprintf("failed to open crash log file: %s", err.Error()))
		return
	}
	defer crashFile.Close()

	// Verify write permission
	if err := crashFile.Chmod(0644); err != nil {
		logInstance.Error(context.Background(), fmt.Sprintf("failed to set crash log file permission: %s", err.Error()))
		return
	}

	setCrashOutputErr := debug.SetCrashOutput(crashFile, debug.CrashOptions{})
	if setCrashOutputErr != nil {
		logInstance.Error(context.Background(), fmt.Sprintf("failed to set crash output: %s", setCrashOutputErr.Error()))
		return
	}
}

func CreateLogger(logFolder string) *Log {
	if _, err := os.Stat(logFolder); os.IsNotExist(err) {
		os.MkdirAll(logFolder, os.ModePerm)
	}

	logImpl := &Log{
		logFolder: logFolder,
	}
	defaultLogLevel := zap.InfoLevel
	if IsDev() {
		defaultLogLevel = zap.DebugLevel
	}

	logImpl.logger, logImpl.writer, logImpl.fileWriter, logImpl.level = createLogger(logFolder, defaultLogLevel)
	log.SetFlags(0) // remove default timestamp
	log.SetOutput(logImpl.writer)
	return logImpl
}

func (l *Log) GetWriter() io.Writer {
	return l.writer
}

func (l *Log) CurrentLogPath() string {
	if l.fileWriter == nil {
		return path.Join(l.logFolder, logFileBaseName)
	}
	return l.fileWriter.CurrentFilename()
}

func formatMsg(context context.Context, msg string, level string) string {
	var builder strings.Builder
	builder.Grow(256)
	builder.WriteString(FormatTimestampWithMs(GetSystemTimestamp()))
	builder.WriteString(" G")
	builder.WriteString(LeftPad(strconv.FormatInt(GetGID(), 10), 7, '0'))
	builder.WriteString(" ")
	if traceId := GetContextTraceId(context); traceId != "" {
		builder.WriteString(traceId)
		builder.WriteString(" ")
	}
	builder.WriteString(fmt.Sprintf("[%s] ", level))
	if componentName := GetContextComponentName(context); componentName != "" {
		builder.WriteString(fmt.Sprintf("[%s] ", componentName))
	}
	builder.WriteString(msg)
	return builder.String()
}

func (l *Log) Debug(context context.Context, msg string) {
	l.logger.Debug(formatMsg(context, msg, "DBG"))
}

func (l *Log) Warn(context context.Context, msg string) {
	l.logger.Warn(formatMsg(context, msg, "WRN"))
}

func (l *Log) Info(context context.Context, msg string) {
	l.logger.Info(formatMsg(context, msg, "INF"))
}

func (l *Log) Error(context context.Context, msg string) {
	l.logger.Error(formatMsg(context, msg, "ERR"))
}

func (l *Log) SetLevel(level string) string {
	normalizedLevel := NormalizeLogLevel(level)
	l.level.SetLevel(parseZapLevel(normalizedLevel))
	return normalizedLevel
}

func (l *Log) ClearHistory() error {
	l.clearLogMux.Lock()
	defer l.clearLogMux.Unlock()

	if err := l.fileWriter.Rotate(); err != nil {
		return err
	}

	logFileName := path.Base(l.fileWriter.CurrentFilename())
	entries, err := os.ReadDir(l.logFolder)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if name == logFileName || name == "crash.log" {
			continue
		}

		removeErr := os.Remove(path.Join(l.logFolder, name))
		if removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
	}

	crashLogPath := path.Join(l.logFolder, "crash.log")
	crashFile, err := os.OpenFile(crashLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	return crashFile.Close()
}

func (l *Log) startMaintenanceRoutine() {
	ctx := NewTraceContext()

	Go(ctx, "log maintenance", func() {
		l.runMaintenance(ctx)

		for {
			timer := time.NewTimer(durationUntilNextLocalMidnight(currentTime()))
			<-timer.C
			l.runMaintenance(NewTraceContext())
		}
	})
}

func (l *Log) runMaintenance(ctx context.Context) {
	if l.fileWriter == nil {
		return
	}

	l.clearLogMux.Lock()
	defer l.clearLogMux.Unlock()

	if err := l.fileWriter.EnsureDailyRollover(); err != nil {
		l.Error(ctx, fmt.Sprintf("failed to roll over daily log: %s", err.Error()))
	}

	l.cleanupExpiredLogs(ctx)
}

func (l *Log) cleanupExpiredLogs(ctx context.Context) {
	if l.fileWriter == nil {
		return
	}

	if err := l.fileWriter.millRunOnce(); err != nil {
		l.Error(ctx, fmt.Sprintf("failed to cleanup rotated logs: %s", err.Error()))
	}

	removedCount, err := l.cleanupStandaloneLogFiles()
	if err != nil {
		l.Error(ctx, fmt.Sprintf("failed to cleanup standalone logs: %s", err.Error()))
		return
	}
	if removedCount > 0 {
		l.Info(ctx, fmt.Sprintf("cleaned up %d expired standalone log files", removedCount))
	}
}

func (l *Log) cleanupStandaloneLogFiles() (int, error) {
	if l.fileWriter == nil || logRetentionDays <= 0 {
		return 0, nil
	}

	cutoff := startOfDay(currentTime()).AddDate(0, 0, -logRetentionDays)
	entries, err := os.ReadDir(l.logFolder)
	if err != nil {
		return 0, err
	}

	removedCount := 0
	currentLogFileName := path.Base(l.fileWriter.CurrentFilename())
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if name == currentLogFileName {
			continue
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			return removedCount, infoErr
		}
		if !isExpiredLogFile(name, info.ModTime(), cutoff) {
			continue
		}

		filePath := path.Join(l.logFolder, name)
		if name == "crash.log" {
			// Keep the crash log inode because the runtime may still write to it after startup.
			truncateErr := os.Truncate(filePath, 0)
			if truncateErr != nil && !os.IsNotExist(truncateErr) {
				return removedCount, truncateErr
			}
		} else {
			removeErr := os.Remove(filePath)
			if removeErr != nil && !os.IsNotExist(removeErr) {
				return removedCount, removeErr
			}
		}
		removedCount++
	}

	return removedCount, nil
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func durationUntilNextLocalMidnight(t time.Time) time.Duration {
	nextMidnight := startOfDay(t).AddDate(0, 0, 1)
	return nextMidnight.Sub(t)
}

// isExpiredLogFile keeps retention decisions in one place for daily log files
// and legacy standalone logs that do not carry a date in their filename.
func isExpiredLogFile(name string, modTime time.Time, cutoff time.Time) bool {
	if logDate, ok := dateFromDailyLogName(name); ok {
		return logDate.Before(cutoff)
	}

	switch name {
	case logFileBaseName, "crash.log", "update.log":
		return modTime.Before(cutoff)
	default:
		return false
	}
}

// dateFromDailyLogName extracts the YYYYMMDD suffix from daily backup logs.
func dateFromDailyLogName(name string) (time.Time, bool) {
	normalizedName := strings.TrimSuffix(name, compressSuffix)
	ext := path.Ext(logFileBaseName)
	prefix := strings.TrimSuffix(logFileBaseName, ext) + "."
	if !strings.HasPrefix(normalizedName, prefix) || !strings.HasSuffix(normalizedName, ext) {
		return time.Time{}, false
	}

	datePart := strings.TrimSuffix(strings.TrimPrefix(normalizedName, prefix), ext)
	if len(datePart) < len(dailyTimeFormat) {
		return time.Time{}, false
	}
	if suffix := datePart[len(dailyTimeFormat):]; suffix != "" && !strings.HasPrefix(suffix, ".") {
		return time.Time{}, false
	}
	datePart = datePart[:len(dailyTimeFormat)]
	if _, err := strconv.Atoi(datePart); err != nil {
		return time.Time{}, false
	}

	parsedDate, err := time.ParseInLocation(dailyTimeFormat, datePart, currentTime().Location())
	if err != nil {
		return time.Time{}, false
	}
	return parsedDate, true
}

func NormalizeLogLevel(level string) string {
	normalizedLevel := strings.ToUpper(strings.TrimSpace(level))
	if normalizedLevel == "DEBUG" {
		return "DEBUG"
	}
	return "INFO"
}

func parseZapLevel(level string) zapcore.Level {
	if NormalizeLogLevel(level) == "DEBUG" {
		return zap.DebugLevel
	}
	return zap.InfoLevel
}

func createLogger(logFolder string, initialLevel zapcore.Level) (*zap.Logger, io.Writer, *Lumberjack, zap.AtomicLevel) {
	fileWriter := &Lumberjack{
		Filename:  path.Join(logFolder, logFileBaseName),
		LocalTime: true,
		Daily:     true,
	}
	writeSyncer := zapcore.AddSync(fileWriter)
	atomicLevel := zap.NewAtomicLevelAt(initialLevel)

	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeTime = nil
	cfg.EncodeLevel = nil

	zapLogger := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		writeSyncer,
		atomicLevel,
	))

	reader, writer := io.Pipe()
	Go(NewTraceContext(), "log reader", func() {
		defer reader.Close()
		defer writer.Close()
		buf := make([]byte, 2048)
		for {
			n, err := reader.Read(buf)
			if err != nil {
				break
			}
			msg := string(buf[:n])
			//remove newline in msg
			msg = strings.TrimRight(msg, "\n")
			zapLogger.Info(formatMsg(NewTraceContext(), fmt.Sprintf("[SYS LOG] %s", msg), "INF"))
		}
	})

	return zapLogger, writer, fileWriter, atomicLevel
}
