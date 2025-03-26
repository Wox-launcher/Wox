package util

import (
	"time"
)

func GetSystemTimestamp() int64 {
	return time.Now().UnixNano() / 1e6
}

func GetSystemTime() time.Time {
	return time.Now()
}

func FormatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func FormatDateTime(t time.Time) string {
	return t.Format("20060102")
}

func FormatTimestamp(timestamp int64) string {
	sec, nsec := int64(timestamp/1000), int64(timestamp%1000*1e6)
	return time.Unix(sec, nsec).Format("2006-01-02 15:04:05")
}

func FormatTimestampWithMs(timestamp int64) string {
	sec, nsec := int64(timestamp/1000), int64(timestamp%1000*1e6)
	return time.Unix(sec, nsec).Format("2006-01-02 15:04:05.000")
}
