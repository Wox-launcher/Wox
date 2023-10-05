package util

import (
	"context"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
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
	syncer zapcore.WriteSyncer
}

func GetLogger() *Log {
	logOnce.Do(func() {
		logInstance = &Log{}
		logInstance.logger, logInstance.syncer = createLogger()
	})
	return logInstance
}

func (l *Log) GetWriter() io.Writer {
	return logInstance.syncer
}

func (l *Log) formatMsg(context context.Context, msg string) string {
	var builder strings.Builder
	builder.Grow(256)
	builder.WriteString(FormatTimestampWithMs(GetSystemTimestamp()))
	builder.WriteString(" G")
	builder.WriteString(LeftPad(strconv.FormatInt(GetGID(), 10), 7, '0'))
	builder.WriteString(" ")
	if traceId, ok := context.Value("trace").(string); ok {
		builder.WriteString(traceId)
	}
	builder.WriteString(msg)
	return builder.String()
}

func (l *Log) Debug(context context.Context, msg string) {
	l.logger.Debug(l.formatMsg(context, msg))
}

func (l *Log) Info(context context.Context, msg string) {
	l.logger.Info(l.formatMsg(context, msg))
}

func (l *Log) Error(context context.Context, msg string) {
	l.logger.Error(l.formatMsg(context, msg))
}

func createLogger() (*zap.Logger, zapcore.WriteSyncer) {
	logFolder := GetLocation().GetLogDirectory()
	if _, err := os.Stat(logFolder); os.IsNotExist(err) {
		os.MkdirAll(logFolder, os.ModePerm)
	}

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
	return zapLogger, writeSyncer
}
