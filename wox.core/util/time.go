package util

import (
	"database/sql"
	"fmt"
	"github.com/jinzhu/now"
	"strconv"
	"time"
)

func GetSystemTimestamp() int64 {
	return time.Now().UnixNano() / 1e6
}

func GetSystemTimestampStr() string {
	return strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
}

func GetSystemTime() time.Time {
	return time.Now()
}

func FormatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func FormatNullTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}

	return t.Time.Format("2006-01-02 15:04:05")
}

func FormatTimeWithoutYearAndSeconds(t time.Time) string {
	return t.Format("01-02 15:04")
}

func FormatTimeWithHourAndMinute(t time.Time) string {
	return t.Format("15:04")
}

func IsYesterday(t time.Time) bool {
	yesterday := time.Now().AddDate(0, 0, -1)
	return t.Year() == yesterday.Year() && t.Month() == yesterday.Month() && t.Day() == yesterday.Day()
}

func FormatTimeWithMs(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.000")
}

func FormatDateTime(t time.Time) string {
	return t.Format("20060102")
}

func FormatDateTime2(t time.Time) string {
	return t.Format("2006-01-02")
}

func FormatTimeWithHour(t time.Time) string {
	return t.Format("2006010215")
}

func FormatTimeWithSecond(t time.Time) string {
	return t.Format("20060102150405")
}

// 如果时间为"0001-01-01 00:00:00", 则返回true
func IsEmptyTime(t time.Time) bool {
	return FormatTime(t) == "0001-01-01 00:00:00"
}

// 解析为为本地网络时间
func ParseTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02 15:04:05", s)
	_, offset := time.Now().Zone()
	t = t.Add(time.Second * time.Duration(-offset))
	return t
}

// 解析为为本地网络时间
func ParseTimeOnlyDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	_, offset := time.Now().Zone()
	t = t.Add(time.Second * time.Duration(-offset))
	return t
}

// 解析为为本地网络时间
func ParseTimeWithMicroSeconds(s string) time.Time {
	t, _ := time.Parse("2006-01-02 15:04:05.000000", s)
	_, offset := time.Now().Zone()
	t = t.Add(time.Second * time.Duration(-offset))
	return t
}

func ParseTimeWithoutSeconds(s string) time.Time {
	t, _ := time.Parse("2006-01-02 15:04", s)
	_, offset := time.Now().Zone()
	t = t.Add(time.Second * time.Duration(-offset))
	return t
}

func ParseTimeStamp(timestamp int64) time.Time {
	return ParseTime(FormatTimestamp(timestamp))
}

func ParseTime2(layout string, s string) time.Time {
	t, _ := time.Parse(layout, s)
	_, offset := time.Now().Zone()
	t = t.Add(time.Second * time.Duration(-offset))
	return t
}

func FormatTimestamp(timestamp int64) string {
	sec, nsec := int64(timestamp/1000), int64(timestamp%1000*1e6)
	return time.Unix(sec, nsec).Format("2006-01-02 15:04:05")
}

func FormatTimestamp2(timestamp int64, format string) string {
	sec, nsec := int64(timestamp/1000), int64(timestamp%1000*1e6)
	return time.Unix(sec, nsec).Format(format)
}

func FormatTimestampWithMs(timestamp int64) string {
	sec, nsec := int64(timestamp/1000), int64(timestamp%1000*1e6)
	return time.Unix(sec, nsec).Format("2006-01-02 15:04:05.000")
}

func FormatTimestampWithMicroSeconds(nanoSeconds int64) string {
	return time.Unix(0, nanoSeconds).Format("2006-01-02 15:04:05.000000")
}

func FormatTimestampWithoutYear(timestamp int64) string {
	sec, nsec := int64(timestamp/1000), int64(timestamp%1000*1e6)
	return time.Unix(sec, nsec).Format("01-02 15:04:05")
}

func FormatTimestampWithoutYearAndMonAndDay(timestamp int64) string {
	sec, nsec := int64(timestamp/1000), int64(timestamp%1000*1e6)
	return time.Unix(sec, nsec).Format("15:04:05")
}

func ConvertTimeFromTimestamp(timestamp int64) time.Time {
	sec, nsec := int64(timestamp/1000), int64(timestamp%1000*1e6)
	return time.Unix(sec, nsec)
}

func ConvertTimestampFromTime(t time.Time) (timestamp int64) {
	return t.UnixNano() / int64(time.Millisecond)
}

// a-b的秒差距
func DiffTimeInSeconds(a, b time.Time) float64 {
	return a.Sub(b).Seconds()
}

func AddHours(a time.Time, hours int) time.Time {
	timeDuration, _ := time.ParseDuration(fmt.Sprintf("%dh", hours))
	return a.Add(timeDuration)
}

func AddSeconds(a time.Time, seconds int) time.Time {
	timeDuration, _ := time.ParseDuration(fmt.Sprintf("%ds", seconds))
	return a.Add(timeDuration)
}

func AddMinutes(a time.Time, minutes int) time.Time {
	timeDuration, _ := time.ParseDuration(fmt.Sprintf("%dm", minutes))
	return a.Add(timeDuration)
}

func AddDays(a time.Time, days int) time.Time {
	return AddHours(a, days*24)
}

func RemoveTime(toRound time.Time) time.Time {
	return time.Date(toRound.Year(), toRound.Month(), toRound.Day(), 0, 0, 0, 0, toRound.Location())
}

func GetCurrentHourDate() time.Time {
	nowTime := GetSystemTime()
	return time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), nowTime.Hour(), 0, 0, 0, nowTime.Location())
}

func GetTodayRange() (from time.Time, to time.Time) {
	nowTime := now.New(GetSystemTime())
	from = nowTime.BeginningOfDay()
	to = nowTime.EndOfDay()
	return
}

func GetBeginningOfToday() time.Time {
	nowTime := now.New(GetSystemTime())
	return nowTime.BeginningOfDay()
}

func GetPast24HRange() (from time.Time, to time.Time) {
	return AddHours(GetSystemTime(), -24), GetSystemTime()
}

func GetPastNHRange(hour int) (from time.Time, to time.Time) {
	return AddHours(GetSystemTime(), -1*hour), GetSystemTime()
}

func GetPast7DayRange() (from time.Time, to time.Time) {
	return AddDays(GetSystemTime(), -7), GetSystemTime()
}

func GetPast30DayRange() (from time.Time, to time.Time) {
	return AddDays(GetSystemTime(), -30), GetSystemTime()
}

func GetThisWeekRange() (from time.Time, to time.Time) {
	nowTime := now.New(GetSystemTime())
	now.WeekStartDay = time.Monday
	from = nowTime.BeginningOfWeek()
	to = nowTime.EndOfWeek()
	return
}

func GetThisMonthRange() (from time.Time, to time.Time) {
	nowTime := now.New(GetSystemTime())
	from = nowTime.BeginningOfMonth()
	to = nowTime.EndOfMonth()
	return
}

func GetMonthRange(year int, month int) (from time.Time, to time.Time) {
	t, _ := now.Parse(fmt.Sprintf("%d-%d", year, month))
	from = now.New(t).BeginningOfMonth()
	to = now.New(t).EndOfMonth()
	return
}

func GetYearRange(year int) (from time.Time, to time.Time) {
	t, _ := now.Parse(fmt.Sprintf("%d", year))
	from = now.New(t).BeginningOfYear()
	to = now.New(t).EndOfYear()
	return
}

// 获取某天的开始和结束时间，day: FORMAT("%s-%s-%s")
func GetDayRange(day string) (from time.Time, to time.Time) {
	t, _ := now.Parse(day)
	from = now.New(t).BeginningOfDay()
	to = now.New(t).EndOfDay()
	return
}

func GetAllTimeRange() (from time.Time, to time.Time) {
	current := GetSystemTime()
	from = time.Date(2000, 1, 1, 0, 0, 0, 0, current.Location())
	to = time.Date(2100, 1, 1, 0, 0, 0, 0, current.Location())
	return
}

func IsToday(t time.Time) bool {
	n := GetSystemTime()
	return n.Year() == t.Year() && n.Month() == t.Month() && n.Day() == t.Day()
}

func IsTomorrow(t time.Time) bool {
	return now.New(t).BeginningOfDay().Equal(now.New(GetSystemTime()).BeginningOfDay().AddDate(0, 0, 1))
}

// 获取两个时间中较晚的一个时间
func GetLaterTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return b
	} else {
		return a
	}
}

// 获取两个时间中较早的一个时间
func GetOlderTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	} else {
		return b
	}
}

func FormatNanoSecondsToMicroSeconds(nano int64) string {
	microSecond := nano / 1000
	milliSecond := microSecond / 1000
	return fmt.Sprintf("%d.%d", milliSecond, microSecond-milliSecond*1000)
}

func GetDaysInMonth(year int, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
