package util

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

var logInstance *Log
var logOnce sync.Once

type Log struct {
	logger *zap.Logger
	writer io.Writer
}

func GetLogger() *Log {
	logOnce.Do(func() {
		logFolder := GetLocation().GetLogDirectory()
		logInstance = CreateLogger(logFolder)
	})
	return logInstance
}

func CreateLogger(logFolder string) *Log {
	if _, err := os.Stat(logFolder); os.IsNotExist(err) {
		os.MkdirAll(logFolder, os.ModePerm)
	}

	logImpl := &Log{}
	logImpl.logger, logImpl.writer = createLogger(logFolder)
	log.SetFlags(0) // remove default timestamp
	log.SetOutput(logImpl.writer)
	return logImpl
}

func (l *Log) GetWriter() io.Writer {
	return logInstance.writer
}

func formatMsg(context context.Context, msg string, level string) string {
	var builder strings.Builder
	builder.Grow(256)
	builder.WriteString(FormatTimestampWithMs(GetSystemTimestamp()))
	builder.WriteString(" G")
	builder.WriteString(LeftPad(strconv.FormatInt(GetGID(), 10), 7, '0'))
	builder.WriteString(" ")
	if traceId, ok := context.Value("trace").(string); ok {
		builder.WriteString(traceId)
		builder.WriteString(" ")
	}
	builder.WriteString(fmt.Sprintf("[%s] ", level))
	builder.WriteString(msg)
	return builder.String()
}

func (l *Log) Debug(context context.Context, msg string) {
	l.logger.Debug(formatMsg(context, msg, "DBG"))
}

func (l *Log) Info(context context.Context, msg string) {
	l.logger.Info(formatMsg(context, msg, "INF"))
}

func (l *Log) Error(context context.Context, msg string) {
	l.logger.Error(formatMsg(context, msg, "ERR"))
}

func createLogger(logFolder string) (*zap.Logger, io.Writer) {
	writeSyncer := zapcore.AddSync(&Lumberjack{
		Filename:  path.Join(logFolder, "log"),
		LocalTime: true,
		MaxSize:   500, // megabytes
		MaxAge:    3,   // days
	})

	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeTime = nil
	cfg.EncodeLevel = nil

	zapLogger := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		writeSyncer,
		zap.DebugLevel,
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

	return zapLogger, writer
}
