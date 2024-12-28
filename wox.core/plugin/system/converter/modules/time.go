package modules

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"wox/plugin"
	"wox/plugin/system/converter/core"

	"github.com/shopspring/decimal"
)

// TimeUnit represents a time unit with its duration and display name
type TimeUnit struct {
	Duration    time.Duration
	DisplayName string
}

var timeUnits = map[string]TimeUnit{
	"ms": {time.Millisecond, "milliseconds"},
	"s":  {time.Second, "seconds"},
	"m":  {time.Minute, "minutes"},
	"h":  {time.Hour, "hours"},
	"d":  {24 * time.Hour, "days"},
	"w":  {7 * 24 * time.Hour, "weeks"},
	"y":  {365 * 24 * time.Hour, "years"},
}

// getDisplayUnit returns the most appropriate display unit and value for a given duration
func getDisplayUnit(duration time.Duration) (string, float64) {
	switch {
	case duration < time.Second:
		return "milliseconds", float64(duration.Milliseconds())
	case duration < time.Minute:
		return "seconds", duration.Seconds()
	case duration < time.Hour:
		return "minutes", duration.Minutes()
	case duration < 24*time.Hour:
		return "hours", duration.Hours()
	case duration < 7*24*time.Hour:
		return "days", duration.Hours() / 24
	case duration < 30*24*time.Hour:
		return "weeks", duration.Hours() / (24 * 7)
	default:
		return "years", duration.Hours() / (24 * 365)
	}
}

// formatDurationValue formats a duration value with appropriate unit
func formatDurationValue(duration time.Duration) string {
	unit, value := getDisplayUnit(duration)
	if unit == "milliseconds" {
		return fmt.Sprintf("%d %s", int64(value), unit)
	}
	return fmt.Sprintf("%.2f %s", value, unit)
}

var timeZoneAliases = map[string]string{
	// Asia
	"shanghai":  "Asia/Shanghai",
	"beijing":   "Asia/Shanghai",
	"shenzhen":  "Asia/Shanghai",
	"guangzhou": "Asia/Shanghai",
	"chengdu":   "Asia/Shanghai",
	"sz":        "Asia/Shanghai",
	"bj":        "Asia/Shanghai",
	"sh":        "Asia/Shanghai",
	"gz":        "Asia/Shanghai",
	"cd":        "Asia/Shanghai",
	"hongkong":  "Asia/Hong_Kong",
	"hk":        "Asia/Hong_Kong",
	"tokyo":     "Asia/Tokyo",
	"osaka":     "Asia/Tokyo",
	"singapore": "Asia/Singapore",
	"sg":        "Asia/Singapore",
	"taipei":    "Asia/Taipei",
	"seoul":     "Asia/Seoul",
	"bangkok":   "Asia/Bangkok",
	"dubai":     "Asia/Dubai",
	"delhi":     "Asia/Kolkata",
	"mumbai":    "Asia/Kolkata",
	"jakarta":   "Asia/Jakarta",

	// Europe
	"london":    "Europe/London",
	"uk":        "Europe/London",
	"paris":     "Europe/Paris",
	"berlin":    "Europe/Berlin",
	"rome":      "Europe/Rome",
	"madrid":    "Europe/Madrid",
	"amsterdam": "Europe/Amsterdam",
	"brussels":  "Europe/Brussels",
	"zurich":    "Europe/Zurich",
	"moscow":    "Europe/Moscow",
	"stockholm": "Europe/Stockholm",
	"vienna":    "Europe/Vienna",
	"warsaw":    "Europe/Warsaw",

	// North America
	"new york":    "America/New_York",
	"nyc":         "America/New_York",
	"ny":          "America/New_York",
	"la":          "America/Los_Angeles",
	"los angeles": "America/Los_Angeles",
	"sf":          "America/Los_Angeles",
	"chicago":     "America/Chicago",
	"chi":         "America/Chicago",
	"toronto":     "America/Toronto",
	"vancouver":   "America/Vancouver",
	"seattle":     "America/Los_Angeles",
	"boston":      "America/New_York",
	"washington":  "America/New_York",
	"dc":          "America/New_York",
	"miami":       "America/New_York",
	"dallas":      "America/Chicago",
	"houston":     "America/Chicago",

	// Australia & New Zealand
	"sydney":     "Australia/Sydney",
	"melbourne":  "Australia/Melbourne",
	"brisbane":   "Australia/Brisbane",
	"perth":      "Australia/Perth",
	"auckland":   "Pacific/Auckland",
	"wellington": "Pacific/Auckland",

	// South America
	"sao paulo":    "America/Sao_Paulo",
	"buenos aires": "America/Argentina/Buenos_Aires",
	"rio":          "America/Sao_Paulo",
	"santiago":     "America/Santiago",
	"lima":         "America/Lima",

	// Africa
	"cairo":        "Africa/Cairo",
	"johannesburg": "Africa/Johannesburg",
	"lagos":        "Africa/Lagos",
	"nairobi":      "Africa/Nairobi",
	"casablanca":   "Africa/Casablanca",
}

type TimeModule struct {
	*regexBaseModule
}

func NewTimeModule(ctx context.Context, api plugin.API) *TimeModule {
	m := &TimeModule{}

	const (
		weekdayNames = `(monday|tuesday|wednesday|thursday|friday|saturday|sunday)`
		monthNames   = `((?i:jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec))`
		timePattern  = `([0-9]{1,2}(?::[0-9]{2})?\s*(?i:(?:am|pm))?)`
		// Simple time units pattern for formats like 1s, 1ms, 1h, 2d, 5w, 8m, 7y
		simpleTimePattern = `(\d+)\s*(ms|s|m|h|d|w|y)`
	)

	// Initialize pattern handlers
	handlers := []*patternHandler{
		{
			Pattern:     `time\s+in\s+([a-zA-Z\s/]+)`,
			Priority:    1000,
			Description: "Get current time in a specific location",
			Handler:     m.handleTimeInLocation,
		},
		{
			Pattern:     timePattern + `\s+in\s+([a-zA-Z\s/]+)`,
			Priority:    900,
			Description: "Convert specific time from one location to local time",
			Handler:     m.handleSpecificTime,
		},
		{
			Pattern:     weekdayNames + `\s+in\s+(\d+)\s*([a-z]*)`,
			Priority:    800,
			Description: "Calculate future weekday",
			Handler:     m.handleWeekdayInFuture,
		},
		{
			Pattern:     `days?\s+until\s+(\d+)(?:st|nd|rd|th)?\s+` + monthNames + `(?:\s+(\d{4}))?`,
			Priority:    800,
			Description: "Calculate days until a specific date",
			Handler:     m.handleDaysUntil,
		},
		{
			Pattern:     simpleTimePattern,
			Priority:    10, //this should be the lowest priority, so it will be the last one to be matched
			Description: "Convert simple time units (e.g., 1s, 1ms, 1h, 2d, 5w, 8m, 7y)",
			Handler:     m.handleSimpleTimeUnit,
		},
	}

	m.regexBaseModule = NewRegexBaseModule(api, "time", handlers)
	return m
}

func (m *TimeModule) Convert(ctx context.Context, result core.Result, toUnit core.Unit) (core.Result, error) {
	if result.Unit.Type != core.UnitTypeTime {
		return result, errors.New("time conversion not supported for non-time unit")
	}
	if toUnit.Type != core.UnitTypeTime {
		return result, errors.New("time conversion not supported for non-time unit")
	}
	if result.Unit.Name == toUnit.Name {
		return result, nil
	}

	//if result unit name is timezone, and to unit is timezone, convert result to target timezone
	//convert result to unix timestamp, and then convert to target timezone
	isResultTimeZone := false
	isTargetTimeZone := false
	for _, tz := range timeZoneAliases {
		if result.Unit.Name == tz {
			isResultTimeZone = true
		}
		if toUnit.Name == tz {
			isTargetTimeZone = true
		}
	}
	if isResultTimeZone && isTargetTimeZone {
		loc, err := time.LoadLocation(toUnit.Name)
		if err != nil {
			return core.Result{}, fmt.Errorf("unknown location: %s", toUnit.Name)
		}
		unixTimestamp := result.RawValue.IntPart()
		timeInTargetZone := time.Unix(unixTimestamp, 0).In(loc)
		result.RawValue = decimal.NewFromInt(timeInTargetZone.Unix())
		result.Unit = core.Unit{Name: toUnit.Name, Type: core.UnitTypeTime}
		result.DisplayValue = m.formatTimeForDisplay(timeInTargetZone)
		return result, nil
	}

	// Handle duration unit conversions
	fromUnit, fromOk := timeUnits[result.Unit.Name]
	toTimeUnit, toOk := timeUnits[toUnit.Name]
	if fromOk && toOk {
		// Convert the raw value to duration
		duration := time.Duration(result.RawValue.IntPart()) * fromUnit.Duration
		// Convert to target unit
		newValue := decimal.NewFromInt(int64(duration / toTimeUnit.Duration))
		return core.Result{
			DisplayValue: fmt.Sprintf("%.2f %s", newValue.InexactFloat64(), toTimeUnit.DisplayName),
			RawValue:     newValue,
			Unit:         toUnit,
			Module:       m,
		}, nil
	}

	return result, errors.New("time conversion not supported")
}

func (m *TimeModule) handleTimeInLocation(ctx context.Context, matches []string) (core.Result, error) {
	location := strings.ToLower(strings.TrimSpace(matches[1]))

	// Try to find the timezone alias
	if tzName, ok := timeZoneAliases[location]; ok {
		location = tzName
	}

	// Load the location
	loc, err := time.LoadLocation(location)
	if err != nil {
		return core.Result{}, fmt.Errorf("unknown location: %s", location)
	}

	// Get current time in location
	now := time.Now().In(loc)
	val := decimal.NewFromInt(now.Unix())
	return core.Result{
		DisplayValue: m.formatTimeForDisplay(now),
		RawValue:     val,
		Unit:         core.Unit{Name: location, Type: core.UnitTypeTime},
		Module:       m,
	}, nil
}

func (m *TimeModule) handleSpecificTime(ctx context.Context, matches []string) (core.Result, error) {
	timeStr := matches[1]
	location := strings.ToLower(strings.TrimSpace(matches[2]))

	// Try to find the timezone alias
	if tzName, ok := timeZoneAliases[location]; ok {
		location = tzName
	}

	// Load the source location
	sourceLoc, err := time.LoadLocation(location)
	if err != nil {
		return core.Result{}, fmt.Errorf("unknown location: %s", location)
	}

	// Parse time in source timezone
	t, err := m.parseTime(ctx, timeStr)
	if err != nil {
		return core.Result{}, err
	}

	// Convert time from source timezone to local timezone
	sourceTime := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, sourceLoc)
	localTime := sourceTime.In(time.Local)

	displayValue := m.formatTimeForDisplay(localTime)
	val := decimal.NewFromInt(localTime.Unix())
	return core.Result{
		DisplayValue: displayValue,
		RawValue:     val,
		Unit:         core.Unit{Name: "local", Type: core.UnitTypeTime},
		Module:       m,
	}, nil
}

func (m *TimeModule) handleWeekdayInFuture(ctx context.Context, matches []string) (core.Result, error) {
	targetWeekday := strings.ToLower(matches[1])
	daysStr := matches[2]
	// unit is optional and not used currently
	// unit := matches[3] // might be empty, "days", "day"

	// Parse number of days
	days, err := strconv.Atoi(daysStr)
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid number of days: %s", daysStr)
	}

	// Get target weekday
	weekdayMap := map[string]time.Weekday{
		"monday":    time.Monday,
		"tuesday":   time.Tuesday,
		"wednesday": time.Wednesday,
		"thursday":  time.Thursday,
		"friday":    time.Friday,
		"saturday":  time.Saturday,
		"sunday":    time.Sunday,
	}
	targetDay, ok := weekdayMap[targetWeekday]
	if !ok {
		return core.Result{}, fmt.Errorf("invalid weekday: %s", targetWeekday)
	}

	// Calculate target date
	now := time.Now()
	targetDate := now.AddDate(0, 0, days)

	// Find the next occurrence of the target weekday after the target date
	for targetDate.Weekday() != targetDay {
		targetDate = targetDate.AddDate(0, 0, 1)
	}

	val := decimal.NewFromInt(targetDate.Unix())
	displayValue := fmt.Sprintf("%s (%s)", targetDate.Format("2006-01-02"), targetDate.Weekday().String())
	return core.Result{
		DisplayValue: displayValue,
		RawValue:     val,
		Unit:         core.Unit{Name: "date", Type: core.UnitTypeTime},
		Module:       m,
	}, nil
}

func (m *TimeModule) handleDaysUntil(ctx context.Context, matches []string) (core.Result, error) {
	// Parse day
	day, err := strconv.Atoi(matches[1])
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid day: %s", matches[1])
	}

	// Parse month
	monthMap := map[string]time.Month{
		"jan": time.January,
		"feb": time.February,
		"mar": time.March,
		"apr": time.April,
		"may": time.May,
		"jun": time.June,
		"jul": time.July,
		"aug": time.August,
		"sep": time.September,
		"oct": time.October,
		"nov": time.November,
		"dec": time.December,
	}
	// The month is in the second capture group, which is in matches[2]
	// The year is in the third capture group (if present), which is in matches[3]
	month, ok := monthMap[strings.ToLower(matches[2])]
	if !ok {
		return core.Result{}, fmt.Errorf("invalid month: %s", matches[2])
	}

	// Parse year (use current year if not specified)
	year := time.Now().Year()
	if len(matches) > 3 && matches[3] != "" {
		year, err = strconv.Atoi(matches[3])
		if err != nil {
			return core.Result{}, fmt.Errorf("invalid year: %s", matches[3])
		}
	}

	// Create target date
	targetDate := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
	now := time.Now()

	// Calculate days until target date
	days := int(targetDate.Sub(now).Hours() / 24)
	val := decimal.NewFromInt(int64(days))
	displayValue := fmt.Sprintf("%d days", days)
	return core.Result{
		DisplayValue: displayValue,
		RawValue:     val,
		Unit:         core.Unit{Name: "days", Type: core.UnitTypeTime},
		Module:       m,
	}, nil
}

func (m *TimeModule) parseTime(ctx context.Context, timeStr string) (time.Time, error) {
	timeStr = strings.ToLower(strings.TrimSpace(timeStr))

	// Get current time as base
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Try parsing with AM/PM format first
	formats := []string{
		"3:04 pm",
		"3:04pm",
		"3pm",
		"3 pm",
		"15:04",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			// Use the parsed hour and minute, but keep today's date and local timezone
			return time.Date(today.Year(), today.Month(), today.Day(), t.Hour(), t.Minute(), 0, 0, today.Location()), nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported time format: %s", timeStr)
}

func (m *TimeModule) formatTimeForDisplay(t time.Time) string {
	hour := t.Hour()
	ampm := "AM"
	if hour >= 12 {
		ampm = "PM"
		if hour > 12 {
			hour -= 12
		}
	}
	if hour == 0 {
		hour = 12
	}
	return fmt.Sprintf("%d:%02d %s", hour, t.Minute(), ampm)
}

func (m *TimeModule) handleSimpleTimeUnit(ctx context.Context, matches []string) (core.Result, error) {
	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid number: %s", matches[1])
	}

	unit := strings.ToLower(matches[2])
	timeUnit, ok := timeUnits[unit]
	if !ok {
		return core.Result{}, fmt.Errorf("unsupported time unit: %s", unit)
	}

	duration := time.Duration(value) * timeUnit.Duration
	displayUnit, _ := getDisplayUnit(duration)
	displayValue := formatDurationValue(duration)

	return core.Result{
		DisplayValue: displayValue,
		RawValue:     decimal.NewFromInt(int64(duration)),
		Unit:         core.Unit{Name: displayUnit, Type: core.UnitTypeTime},
		Module:       m,
	}, nil
}
