package modules

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"wox/plugin"
	"wox/plugin/system/calculator/core"

	"github.com/shopspring/decimal"
)

const (
	weekdayNames = `(monday|tuesday|wednesday|thursday|friday|saturday|sunday)`
	monthNames   = `(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)`
	timePattern  = `[0-9]{1,2}(:[0-9]{2})?\s*(am|pm)?`

	// Base patterns
	timeInLocationPattern  = `time\s+in\s+([a-zA-Z\s/]+)`
	weekdayInFuturePattern = weekdayNames + `\s+in\s+(\d+)\s*([a-z]*)`
	daysUntilPattern       = `days?\s+until\s+\d+\s*(?:st|nd|rd|th)?\s+` + monthNames + `(?:\s+\d{4})?`
	specificTimePattern    = timePattern + `\s+in\s+([a-zA-Z\s/]+)`
)

type TimeModule struct {
	// Pre-compiled regular expressions
	timeInLocationRe  *regexp.Regexp
	weekdayInFutureRe *regexp.Regexp
	daysUntilRe       *regexp.Regexp
	specificTimeRe    *regexp.Regexp
	api               plugin.API
}

func NewTimeModule(ctx context.Context, api plugin.API) *TimeModule {
	return &TimeModule{
		timeInLocationRe:  regexp.MustCompile(timeInLocationPattern),
		weekdayInFutureRe: regexp.MustCompile(weekdayInFuturePattern),
		daysUntilRe:       regexp.MustCompile(daysUntilPattern),
		specificTimeRe:    regexp.MustCompile(specificTimePattern),
		api:               api,
	}
}

func (m *TimeModule) Name() string {
	return "time"
}

func (m *TimeModule) TokenPatterns() []core.TokenPattern {
	return []core.TokenPattern{
		{
			Pattern:   `[a-zA-Z]+(/[a-zA-Z_]+)*`,
			Type:      core.IdentToken,
			Priority:  10,
			FullMatch: false,
		},
		{
			Pattern:   timeInLocationPattern,
			Type:      core.IdentToken,
			Priority:  100,
			FullMatch: true,
		},
		{
			Pattern:   timePattern,
			Type:      core.NumberToken,
			Priority:  90,
			FullMatch: false,
		},
		{
			Pattern:   weekdayInFuturePattern,
			Type:      core.IdentToken,
			Priority:  85,
			FullMatch: true,
		},
		{
			Pattern:   daysUntilPattern,
			Type:      core.IdentToken,
			Priority:  85,
			FullMatch: true,
		},
	}
}

func (m *TimeModule) CanHandle(ctx context.Context, tokens []core.Token) bool {
	if len(tokens) == 0 {
		m.api.Log(ctx, plugin.LogLevelDebug, "TimeModule.CanHandle: no tokens")
		return false
	}

	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("TimeModule.CanHandle: tokens=%+v", tokens))

	// Join all tokens into a string with spaces
	var sb strings.Builder
	for i, token := range tokens {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(token.Str)
	}
	input := strings.ToLower(strings.TrimSpace(sb.String()))

	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("TimeModule.CanHandle: input=%s", input))

	// Check if input matches any of our patterns
	if m.timeInLocationRe.MatchString(input) {
		m.api.Log(ctx, plugin.LogLevelDebug, "TimeModule.CanHandle: timeInLocation pattern matched")
		return true
	}
	if m.weekdayInFutureRe.MatchString(input) {
		m.api.Log(ctx, plugin.LogLevelDebug, "TimeModule.CanHandle: weekdayInFuture pattern matched")
		return true
	}
	if m.daysUntilRe.MatchString(input) {
		m.api.Log(ctx, plugin.LogLevelDebug, "TimeModule.CanHandle: daysUntil pattern matched")
		return true
	}
	if m.specificTimeRe.MatchString(input) {
		m.api.Log(ctx, plugin.LogLevelDebug, "TimeModule.CanHandle: specificTime pattern matched")
		return true
	}

	m.api.Log(ctx, plugin.LogLevelDebug, "TimeModule.CanHandle: no pattern matched")
	return false
}

func (m *TimeModule) Parse(ctx context.Context, tokens []core.Token) (*core.Value, error) {
	// Join all tokens into a string with spaces
	var sb strings.Builder
	for i, t := range tokens {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(t.Str)
	}
	input := strings.ToLower(strings.TrimSpace(sb.String()))

	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("TimeModule.Parse: input=%s", input))

	// Check for timezone query first
	if matches := m.timeInLocationRe.FindStringSubmatch(input); len(matches) > 0 {
		m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Time in location matches: %v", matches))
		if len(matches) < 2 {
			return nil, fmt.Errorf("invalid time in location format")
		}
		location := strings.ToLower(strings.TrimSpace(matches[len(matches)-1]))
		m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Extracted location: %s", location))

		// Try to find the timezone alias
		if tzName, ok := timeZoneAliases[location]; ok {
			location = tzName
		}
		m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Resolved timezone: %s", location))

		// Load the location
		loc, err := time.LoadLocation(location)
		if err != nil {
			m.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to load location: %v", err))
			return nil, fmt.Errorf("unknown location: %s", location)
		}

		// Get current time in location
		now := time.Now().In(loc)
		return &core.Value{
			Amount: decimal.NewFromInt(now.Unix()),
			Unit:   "timestamp",
		}, nil
	}

	// Check for specific time in location
	if matches := m.specificTimeRe.FindStringSubmatch(input); len(matches) > 0 {
		m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Specific time matches: %v", matches))
		if len(matches) < 2 {
			return nil, fmt.Errorf("invalid specific time format")
		}

		timeStr := matches[1]
		location := strings.ToLower(strings.TrimSpace(matches[len(matches)-1]))

		// Parse the time
		t, err := parseTime(ctx, timeStr)
		if err != nil {
			return nil, err
		}

		// Try to find the timezone alias
		if tzName, ok := timeZoneAliases[location]; ok {
			location = tzName
		}

		// Load the location
		loc, err := time.LoadLocation(location)
		if err != nil {
			return nil, fmt.Errorf("unknown location: %s", location)
		}

		// Set the location
		t = t.In(loc)

		return &core.Value{
			Amount: decimal.NewFromInt(t.Unix()),
			Unit:   "timestamp",
		}, nil
	}

	// Check for weekday in future
	if matches := m.weekdayInFutureRe.FindStringSubmatch(input); len(matches) > 0 {
		m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Weekday matches: %v", matches))
		if len(matches) < 3 {
			return nil, fmt.Errorf("invalid weekday format")
		}

		weekday := weekdayMap[matches[1]]
		number, _ := strconv.Atoi(matches[2])
		unit := normalizeUnit(ctx, matches[3])
		if unit == "" {
			unit = "week" // Default unit is week
		}

		var result time.Time
		switch unit {
		case "week":
			result = calculateNextWeekday(ctx, weekday, number)
		case "day":
			result = time.Now().AddDate(0, 0, number)
		case "month":
			result = time.Now().AddDate(0, number, 0)
		default:
			result = calculateNextWeekday(ctx, weekday, number) // Default to week calculation
		}

		return &core.Value{
			Amount: decimal.NewFromInt(result.Unix()),
			Unit:   "timestamp",
		}, nil
	}

	// Check for days until
	if matches := m.daysUntilRe.FindStringSubmatch(input); len(matches) > 0 {
		m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Days until matches: %v", matches))
		if len(matches) < 3 {
			return nil, fmt.Errorf("invalid days until format")
		}

		// Extract day, month and year
		parts := strings.Fields(matches[0])
		dayStr := strings.TrimRight(parts[2], "stndrdth")
		day, err := strconv.Atoi(dayStr)
		if err != nil {
			m.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to parse day: %v", err))
			return nil, fmt.Errorf("invalid day: %s", dayStr)
		}

		monthStr := strings.ToLower(matches[len(matches)-1])
		month := monthMap[monthStr]

		year := time.Now().Year()
		if len(matches) > 3 {
			year, _ = strconv.Atoi(matches[3])
		}

		days, err := calculateDaysUntil(ctx, day, month, year)
		if err != nil {
			m.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to calculate days: %v", err))
			return nil, err
		}

		return &core.Value{
			Amount: decimal.NewFromInt(int64(days)),
			Unit:   "days",
		}, nil
	}

	m.api.Log(ctx, plugin.LogLevelDebug, "No pattern matched")
	return nil, fmt.Errorf("unsupported time format")
}

func (m *TimeModule) Calculate(ctx context.Context, tokens []core.Token) (*core.Value, error) {
	return m.Parse(ctx, tokens)
}

func (m *TimeModule) Convert(ctx context.Context, value *core.Value, toUnit string) (*core.Value, error) {
	return nil, fmt.Errorf("time module does not support conversion")
}

func (m *TimeModule) CanConvertTo(unit string) bool {
	return false
}

var timeZoneAliases = map[string]string{
	"tokyo":     "Asia/Tokyo",
	"beijing":   "Asia/Shanghai",
	"shanghai":  "Asia/Shanghai",
	"london":    "Europe/London",
	"paris":     "Europe/Paris",
	"new york":  "America/New_York",
	"nyc":       "America/New_York",
	"la":        "America/Los_Angeles",
	"sydney":    "Australia/Sydney",
	"singapore": "Asia/Singapore",
	"hong kong": "Asia/Hong_Kong",
	"berlin":    "Europe/Berlin",
	"moscow":    "Europe/Moscow",
	"dubai":     "Asia/Dubai",
	"seoul":     "Asia/Seoul",
	"bangkok":   "Asia/Bangkok",
	"vancouver": "America/Vancouver",
	"toronto":   "America/Toronto",
	"sao paulo": "America/Sao_Paulo",
	"melbourne": "Australia/Melbourne",
	"japan":     "Asia/Tokyo",
	"china":     "Asia/Shanghai",
	"korea":     "Asia/Seoul",
	"uk":        "Europe/London",
	"us":        "America/New_York",
}

var weekdayMap = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
}

var monthMap = map[string]time.Month{
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

func parseTime(ctx context.Context, timeStr string) (time.Time, error) {
	now := time.Now()
	timeStr = strings.ToLower(strings.TrimSpace(timeStr))

	formats := []string{
		"3:04pm",
		"3pm",
		"15:04",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local), nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
}

func calculateNextWeekday(ctx context.Context, weekday time.Weekday, weeks int) time.Time {
	now := time.Now()
	current := now

	// Find the next occurrence of the specified weekday
	daysUntilNext := (int(weekday) - int(current.Weekday()) + 7) % 7
	if daysUntilNext == 0 {
		daysUntilNext = 7
	}

	// Add the specified number of weeks
	return current.AddDate(0, 0, daysUntilNext+(weeks-1)*7)
}

func calculateDaysUntil(ctx context.Context, day int, month time.Month, year int) (int, error) {
	now := time.Now()
	if year == 0 {
		year = now.Year()
		// If target date is before current date, use next year
		if month < now.Month() || (month == now.Month() && day < now.Day()) {
			year++
		}
	}

	target := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
	days := int(target.Sub(now).Hours() / 24)
	return days, nil
}

func normalizeUnit(ctx context.Context, unit string) string {
	// Remove spaces and plural 's'
	unit = strings.TrimSpace(unit)
	unit = strings.TrimSuffix(unit, "s")
	return unit
}
