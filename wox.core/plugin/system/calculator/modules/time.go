package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"wox/plugin"
	"wox/plugin/system/calculator/core"

	"github.com/shopspring/decimal"
)

var timeZoneAliases = map[string]string{
	"shanghai":    "Asia/Shanghai",
	"beijing":     "Asia/Shanghai",
	"london":      "Europe/London",
	"tokyo":       "Asia/Tokyo",
	"paris":       "Europe/Paris",
	"berlin":      "Europe/Berlin",
	"new york":    "America/New_York",
	"la":          "America/Los_Angeles",
	"los angeles": "America/Los_Angeles",
}

type TimeModule struct {
	*regexBaseModule
}

func NewTimeModule(ctx context.Context, api plugin.API) *TimeModule {
	m := &TimeModule{}

	const (
		weekdayNames = `(monday|tuesday|wednesday|thursday|friday|saturday|sunday)`
		monthNames   = `(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)`
		timePattern  = `([0-9]{1,2}(?::[0-9]{2})?\s*(?i:(?:am|pm))?)`
	)

	// Initialize pattern handlers
	handlers := []*patternHandler{
		{
			Pattern:     `(?i:time\s+in\s+([a-zA-Z\s/]+))`,
			Priority:    1000,
			Description: "Get current time in a specific location",
			Handler:     m.handleTimeInLocation,
		},
		{
			Pattern:     `(?i:` + timePattern + `\s+in\s+([a-zA-Z\s/]+))`,
			Priority:    900,
			Description: "Convert specific time from one location to local time",
			Handler:     m.handleSpecificTime,
		},
		{
			Pattern:     `(?i:` + weekdayNames + `\s+in\s+(\d+)\s*([a-z]*))`,
			Priority:    800,
			Description: "Calculate future weekday",
			Handler:     m.handleWeekdayInFuture,
		},
		{
			Pattern:     `(?i:days?\s+until\s+(\d+)(?:st|nd|rd|th)?\s+` + monthNames + `(?:\s+(\d{4}))?)`,
			Priority:    800,
			Description: "Calculate days until a specific date",
			Handler:     m.handleDaysUntil,
		},
	}

	m.regexBaseModule = NewRegexBaseModule(api, "time", handlers)
	return m
}

func (m *TimeModule) Convert(ctx context.Context, value *core.Result, toUnit string) (*core.Result, error) {
	return nil, fmt.Errorf("time conversion not supported")
}

func (m *TimeModule) CanConvertTo(unit string) bool {
	return false
}

// Helper functions

func (m *TimeModule) handleTimeInLocation(ctx context.Context, matches []string) (*core.Result, error) {
	location := strings.ToLower(strings.TrimSpace(matches[1]))

	// Try to find the timezone alias
	if tzName, ok := timeZoneAliases[location]; ok {
		location = tzName
	}

	// Load the location
	loc, err := time.LoadLocation(location)
	if err != nil {
		return nil, fmt.Errorf("unknown location: %s", location)
	}

	// Get current time in location
	now := time.Now().In(loc)
	val := decimal.NewFromInt(now.Unix())
	return &core.Result{
		DisplayValue: m.formatTimeForDisplay(now),
		RawValue:     &val,
		Unit:         location,
	}, nil
}

func (m *TimeModule) handleSpecificTime(ctx context.Context, matches []string) (*core.Result, error) {
	timeStr := matches[1]
	location := strings.ToLower(strings.TrimSpace(matches[2]))

	// Try to find the timezone alias
	if tzName, ok := timeZoneAliases[location]; ok {
		location = tzName
	}

	// Load the source location
	sourceLoc, err := time.LoadLocation(location)
	if err != nil {
		return nil, fmt.Errorf("unknown location: %s", location)
	}

	// Parse time in source timezone
	t, err := m.parseTime(ctx, timeStr)
	if err != nil {
		return nil, err
	}

	// Convert time from source timezone to local timezone
	sourceTime := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, sourceLoc)
	localTime := sourceTime.In(time.Local)

	displayValue := m.formatTimeForDisplay(localTime)
	val := decimal.NewFromInt(localTime.Unix())
	return &core.Result{
		DisplayValue: displayValue,
		RawValue:     &val,
		Unit:         "local",
	}, nil
}

func (m *TimeModule) handleWeekdayInFuture(ctx context.Context, matches []string) (*core.Result, error) {
	targetWeekday := strings.ToLower(matches[1])
	daysStr := matches[2]
	// unit is optional and not used currently
	// unit := matches[3] // might be empty, "days", "day"

	// Parse number of days
	days, err := strconv.Atoi(daysStr)
	if err != nil {
		return nil, fmt.Errorf("invalid number of days: %s", daysStr)
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
		return nil, fmt.Errorf("invalid weekday: %s", targetWeekday)
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
	return &core.Result{
		DisplayValue: displayValue,
		RawValue:     &val,
		Unit:         "date",
	}, nil
}

func (m *TimeModule) handleDaysUntil(ctx context.Context, matches []string) (*core.Result, error) {
	// Parse day
	day, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("invalid day: %s", matches[1])
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
	month, ok := monthMap[strings.ToLower(matches[2])]
	if !ok {
		return nil, fmt.Errorf("invalid month: %s", matches[2])
	}

	// Parse year (use current year if not specified)
	year := time.Now().Year()
	if len(matches) > 3 && matches[3] != "" {
		year, err = strconv.Atoi(matches[3])
		if err != nil {
			return nil, fmt.Errorf("invalid year: %s", matches[3])
		}
	}

	// Create target date
	targetDate := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
	now := time.Now()

	// If the target date is in the past and no year was specified, use next year
	if targetDate.Before(now) && len(matches) <= 3 {
		targetDate = targetDate.AddDate(1, 0, 0)
	}

	// Calculate days until target date
	days := int(targetDate.Sub(now).Hours() / 24)
	val := decimal.NewFromInt(int64(days))
	displayValue := fmt.Sprintf("%d days", days)
	return &core.Result{
		DisplayValue: displayValue,
		RawValue:     &val,
		Unit:         "days",
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
