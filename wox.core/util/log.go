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

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logInstance *Log
var logOnce sync.Once

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

	logFileName := path.Base(l.fileWriter.Filename)
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
		Filename:  path.Join(logFolder, "log"),
		LocalTime: true,
		MaxSize:   500, // megabytes
		MaxAge:    3,   // days
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
