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

// TimeUnit represents a time unit with its duration and localized display key.
type TimeUnit struct {
	Duration       time.Duration
	TranslationKey string
}

var timeUnits = map[string]TimeUnit{
	"ms": {time.Millisecond, "plugin_converter_time_unit_milliseconds"},
	"s":  {time.Second, "plugin_converter_time_unit_seconds"},
	"m":  {time.Minute, "plugin_converter_time_unit_minutes"},
	"h":  {time.Hour, "plugin_converter_time_unit_hours"},
	"d":  {24 * time.Hour, "plugin_converter_time_unit_days"},
	"w":  {7 * 24 * time.Hour, "plugin_converter_time_unit_weeks"},
	"y":  {365 * 24 * time.Hour, "plugin_converter_time_unit_years"},
}

var weekdayTranslationKeys = [...]string{
	"ui_weekday_sun",
	"ui_weekday_mon",
	"ui_weekday_tue",
	"ui_weekday_wed",
	"ui_weekday_thu",
	"ui_weekday_fri",
	"ui_weekday_sat",
}

var weekdayByName = map[string]time.Weekday{
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
	"sunday":    time.Sunday,
}

var monthByAbbreviation = map[string]time.Month{
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

const (
	timeInLocationPattern  = `(?i)time\s+in\s+([a-z0-9_+\-\s/]+)`
	specificTimePattern    = `(?i)([0-9]{1,2}(?::[0-9]{2})?\s*(?:am|pm)?)\s+in\s+([a-z0-9_+\-\s/]+)`
	weekdayInFuturePattern = `(?i)(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\s+in\s+(\d+)\s*([a-z]*)`
	daysUntilPattern       = `(?i)days?\s+until\s+(\d+)(?:st|nd|rd|th)?\s+(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)(?:\s+(\d{4}))?`
	durationTargetPattern  = `(?i)(?:in|to)\s+(milliseconds?|seconds?|minutes?|hours?|days?|weeks?|years?|ms|s|m|h|d|w|y)`
	durationEqualsPattern  = `(?i)=\s*\?\s*(milliseconds?|seconds?|minutes?|hours?|days?|weeks?|years?|ms|s|m|h|d|w|y)`
	simpleTimeUnitPattern  = `(?i)(\d+)\s*(milliseconds?|seconds?|minutes?|hours?|days?|weeks?|years?|ms|s|m|h|d|w|y)`
)

// Country aliases with multiple time zones point to a representative zone because time queries return a single result.
var timeZoneAliases = map[string]string{
	// UTC
	"utc": "UTC",
	"gmt": "UTC",

	// Asia
	"shanghai":             "Asia/Shanghai",
	"beijing":              "Asia/Shanghai",
	"china":                "Asia/Shanghai",
	"cn":                   "Asia/Shanghai",
	"shenzhen":             "Asia/Shanghai",
	"guangzhou":            "Asia/Shanghai",
	"chengdu":              "Asia/Shanghai",
	"sz":                   "Asia/Shanghai",
	"bj":                   "Asia/Shanghai",
	"sh":                   "Asia/Shanghai",
	"gz":                   "Asia/Shanghai",
	"cd":                   "Asia/Shanghai",
	"hongkong":             "Asia/Hong_Kong",
	"hong kong":            "Asia/Hong_Kong",
	"hk":                   "Asia/Hong_Kong",
	"tokyo":                "Asia/Tokyo",
	"japan":                "Asia/Tokyo",
	"jp":                   "Asia/Tokyo",
	"jst":                  "Asia/Tokyo",
	"osaka":                "Asia/Tokyo",
	"singapore":            "Asia/Singapore",
	"sg":                   "Asia/Singapore",
	"taipei":               "Asia/Taipei",
	"taiwan":               "Asia/Taipei",
	"tw":                   "Asia/Taipei",
	"seoul":                "Asia/Seoul",
	"korea":                "Asia/Seoul",
	"south korea":          "Asia/Seoul",
	"kr":                   "Asia/Seoul",
	"kst":                  "Asia/Seoul",
	"bangkok":              "Asia/Bangkok",
	"thailand":             "Asia/Bangkok",
	"th":                   "Asia/Bangkok",
	"dubai":                "Asia/Dubai",
	"uae":                  "Asia/Dubai",
	"united arab emirates": "Asia/Dubai",
	"ae":                   "Asia/Dubai",
	"delhi":                "Asia/Kolkata",
	"mumbai":               "Asia/Kolkata",
	"india":                "Asia/Kolkata",
	"in":                   "Asia/Kolkata",
	"jakarta":              "Asia/Jakarta",
	"indonesia":            "Asia/Jakarta",
	"id":                   "Asia/Jakarta",
	"malaysia":             "Asia/Kuala_Lumpur",
	"my":                   "Asia/Kuala_Lumpur",
	"philippines":          "Asia/Manila",
	"ph":                   "Asia/Manila",
	"vietnam":              "Asia/Ho_Chi_Minh",
	"vn":                   "Asia/Ho_Chi_Minh",
	"saudi arabia":         "Asia/Riyadh",
	"sa":                   "Asia/Riyadh",
	"israel":               "Asia/Jerusalem",
	"il":                   "Asia/Jerusalem",

	// Europe
	"london":         "Europe/London",
	"uk":             "Europe/London",
	"gb":             "Europe/London",
	"united kingdom": "Europe/London",
	"great britain":  "Europe/London",
	"paris":          "Europe/Paris",
	"france":         "Europe/Paris",
	"fr":             "Europe/Paris",
	"cet":            "Europe/Paris",
	"cest":           "Europe/Paris",
	"berlin":         "Europe/Berlin",
	"germany":        "Europe/Berlin",
	"de":             "Europe/Berlin",
	"rome":           "Europe/Rome",
	"italy":          "Europe/Rome",
	"it":             "Europe/Rome",
	"madrid":         "Europe/Madrid",
	"spain":          "Europe/Madrid",
	"es":             "Europe/Madrid",
	"amsterdam":      "Europe/Amsterdam",
	"netherlands":    "Europe/Amsterdam",
	"nl":             "Europe/Amsterdam",
	"brussels":       "Europe/Brussels",
	"belgium":        "Europe/Brussels",
	"be":             "Europe/Brussels",
	"zurich":         "Europe/Zurich",
	"switzerland":    "Europe/Zurich",
	"ch":             "Europe/Zurich",
	"moscow":         "Europe/Moscow",
	"russia":         "Europe/Moscow",
	"ru":             "Europe/Moscow",
	"stockholm":      "Europe/Stockholm",
	"sweden":         "Europe/Stockholm",
	"se":             "Europe/Stockholm",
	"vienna":         "Europe/Vienna",
	"austria":        "Europe/Vienna",
	"at":             "Europe/Vienna",
	"warsaw":         "Europe/Warsaw",
	"poland":         "Europe/Warsaw",
	"pl":             "Europe/Warsaw",
	"turkey":         "Europe/Istanbul",
	"tr":             "Europe/Istanbul",

	// North America
	"new york":                 "America/New_York",
	"nyc":                      "America/New_York",
	"ny":                       "America/New_York",
	"us":                       "America/New_York",
	"usa":                      "America/New_York",
	"america":                  "America/New_York",
	"united states":            "America/New_York",
	"united states of america": "America/New_York",
	"et":                       "America/New_York",
	"est":                      "America/New_York",
	"edt":                      "America/New_York",
	"la":                       "America/Los_Angeles",
	"los angeles":              "America/Los_Angeles",
	"sf":                       "America/Los_Angeles",
	"pt":                       "America/Los_Angeles",
	"pst":                      "America/Los_Angeles",
	"pdt":                      "America/Los_Angeles",
	"chicago":                  "America/Chicago",
	"chi":                      "America/Chicago",
	"ct":                       "America/Chicago",
	"cst":                      "America/Chicago",
	"cdt":                      "America/Chicago",
	"denver":                   "America/Denver",
	"mt":                       "America/Denver",
	"mst":                      "America/Denver",
	"mdt":                      "America/Denver",
	"toronto":                  "America/Toronto",
	"canada":                   "America/Toronto",
	"ca":                       "America/Toronto",
	"vancouver":                "America/Vancouver",
	"seattle":                  "America/Los_Angeles",
	"boston":                   "America/New_York",
	"washington":               "America/New_York",
	"dc":                       "America/New_York",
	"miami":                    "America/New_York",
	"dallas":                   "America/Chicago",
	"houston":                  "America/Chicago",
	"mexico":                   "America/Mexico_City",
	"mx":                       "America/Mexico_City",
	"mexico city":              "America/Mexico_City",

	// Australia & New Zealand
	"sydney":      "Australia/Sydney",
	"australia":   "Australia/Sydney",
	"au":          "Australia/Sydney",
	"aest":        "Australia/Sydney",
	"aedt":        "Australia/Sydney",
	"melbourne":   "Australia/Melbourne",
	"brisbane":    "Australia/Brisbane",
	"perth":       "Australia/Perth",
	"auckland":    "Pacific/Auckland",
	"new zealand": "Pacific/Auckland",
	"nz":          "Pacific/Auckland",
	"wellington":  "Pacific/Auckland",

	// South America
	"sao paulo":    "America/Sao_Paulo",
	"brazil":       "America/Sao_Paulo",
	"br":           "America/Sao_Paulo",
	"buenos aires": "America/Argentina/Buenos_Aires",
	"argentina":    "America/Argentina/Buenos_Aires",
	"ar":           "America/Argentina/Buenos_Aires",
	"rio":          "America/Sao_Paulo",
	"santiago":     "America/Santiago",
	"chile":        "America/Santiago",
	"cl":           "America/Santiago",
	"lima":         "America/Lima",
	"peru":         "America/Lima",
	"pe":           "America/Lima",

	// Africa
	"cairo":        "Africa/Cairo",
	"egypt":        "Africa/Cairo",
	"eg":           "Africa/Cairo",
	"johannesburg": "Africa/Johannesburg",
	"south africa": "Africa/Johannesburg",
	"za":           "Africa/Johannesburg",
	"lagos":        "Africa/Lagos",
	"nigeria":      "Africa/Lagos",
	"ng":           "Africa/Lagos",
	"nairobi":      "Africa/Nairobi",
	"kenya":        "Africa/Nairobi",
	"ke":           "Africa/Nairobi",
	"casablanca":   "Africa/Casablanca",
	"morocco":      "Africa/Casablanca",
	"ma":           "Africa/Casablanca",
}

// resolveTimeZone accepts friendly aliases while preserving canonical IANA identifiers.
func resolveTimeZone(input string) (string, *time.Location, error) {
	locationName := strings.TrimSpace(input)
	if alias, ok := timeZoneAliases[strings.ToLower(locationName)]; ok {
		locationName = alias
	}

	location, err := time.LoadLocation(locationName)
	if err != nil {
		return "", nil, fmt.Errorf("unknown location: %s", strings.TrimSpace(input))
	}
	return locationName, location, nil
}

// isTimeZoneUnit distinguishes timezone results from duration units.
func isTimeZoneUnit(name string) bool {
	if name != "UTC" && !strings.Contains(name, "/") {
		return false
	}
	_, err := time.LoadLocation(name)
	return err == nil
}

type TimeModule struct {
	*regexBaseModule
}

func NewTimeModule(ctx context.Context, api plugin.API) *TimeModule {
	m := &TimeModule{}

	// Initialize pattern handlers
	handlers := []*patternHandler{
		{
			Pattern:     timeInLocationPattern,
			Priority:    1000,
			Description: "Get current time in a specific location (E.g. time in Tokyo)",
			Handler:     m.handleTimeInLocation,
		},
		{
			Pattern:     specificTimePattern,
			Priority:    900,
			Description: "Convert specific time from one location to local time (E.g. 3pm in Tokyo)",
			Handler:     m.handleSpecificTime,
		},
		{
			Pattern:     weekdayInFuturePattern,
			Priority:    800,
			Description: "Calculate future weekday (E.g. Monday in 3 days)",
			Handler:     m.handleWeekdayInFuture,
		},
		{
			Pattern:     daysUntilPattern,
			Priority:    800,
			Description: "Calculate days until a specific date, (E.g. days until 25th Dec 2023)",
			Handler:     m.handleDaysUntil,
		},
		{
			Pattern:     durationTargetPattern,
			Priority:    700,
			Description: "Handle duration conversion target (E.g. to minutes)",
			Handler:     m.handleDurationConversion,
		},
		{
			Pattern:     durationEqualsPattern,
			Priority:    650,
			Description: "Handle duration conversion target (E.g. =?minutes)",
			Handler:     m.handleDurationConversion,
		},
		{
			Pattern:     simpleTimeUnitPattern,
			Priority:    10, //this should be the lowest priority, so it will be the last one to be matched
			Description: "Convert simple time units (e.g., 1s, 1ms, 1h, 2d, 5w, 8m, 7y), (E.g. 1h to minutes)",
			Handler:     m.handleSimpleTimeUnit,
		},
	}

	m.regexBaseModule = NewRegexBaseModule(api, "time", handlers)
	// IANA identifiers are case-sensitive, so the time module keeps the original query text.
	m.regexBaseModule.preserveCase = true
	return m
}

func (m *TimeModule) TokenPatterns() []core.TokenPattern {
	return []core.TokenPattern{
		{
			Pattern:   timeInLocationPattern,
			Type:      core.IdentToken,
			Priority:  1000,
			FullMatch: false,
		},
		{
			Pattern:   specificTimePattern,
			Type:      core.IdentToken,
			Priority:  900,
			FullMatch: false,
		},
		{
			Pattern:   weekdayInFuturePattern,
			Type:      core.IdentToken,
			Priority:  800,
			FullMatch: false,
		},
		{
			Pattern:   daysUntilPattern,
			Type:      core.IdentToken,
			Priority:  800,
			FullMatch: false,
		},
		{
			Pattern:   durationTargetPattern,
			Type:      core.ConversionToken,
			Priority:  700,
			FullMatch: false,
		},
		{
			Pattern:   durationEqualsPattern,
			Type:      core.ConversionToken,
			Priority:  650,
			FullMatch: false,
		},
		{
			Pattern:   simpleTimeUnitPattern,
			Type:      core.IdentToken,
			Priority:  10,
			FullMatch: false,
		},
	}
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

	if isTimeZoneUnit(result.Unit.Name) && isTimeZoneUnit(toUnit.Name) {
		loc, err := time.LoadLocation(toUnit.Name)
		if err != nil {
			return core.Result{}, fmt.Errorf("unknown location: %s", toUnit.Name)
		}
		unixTimestamp := result.RawValue.IntPart()
		timeInTargetZone := time.Unix(unixTimestamp, 0).In(loc)
		result.RawValue = decimal.NewFromInt(timeInTargetZone.Unix())
		result.Unit = core.Unit{Name: toUnit.Name, Type: core.UnitTypeTime}
		result.DisplayValue = m.formatTimeForDisplay(ctx, timeInTargetZone)
		return result, nil
	}

	// Handle duration unit conversions
	fromUnit, fromOk := timeUnits[result.Unit.Name]
	toTimeUnit, toOk := timeUnits[toUnit.Name]
	if fromOk && toOk {
		newValue := result.RawValue.Mul(decimal.NewFromInt(int64(fromUnit.Duration))).Div(decimal.NewFromInt(int64(toTimeUnit.Duration)))
		return core.Result{
			DisplayValue: m.formatDurationWithUnit(ctx, newValue, toUnit.Name),
			RawValue:     newValue,
			Unit:         toUnit,
			Module:       m,
		}, nil
	}

	return result, errors.New("time conversion not supported")
}

func (m *TimeModule) handleTimeInLocation(ctx context.Context, matches []string) (core.Result, error) {
	location, loc, err := resolveTimeZone(matches[1])
	if err != nil {
		return core.Result{}, err
	}

	// Get current time in location
	now := time.Now().In(loc)
	val := decimal.NewFromInt(now.Unix())
	return core.Result{
		DisplayValue: m.formatTimeForDisplay(ctx, now),
		RawValue:     val,
		Unit:         core.Unit{Name: location, Type: core.UnitTypeTime},
		Module:       m,
	}, nil
}

func (m *TimeModule) handleSpecificTime(ctx context.Context, matches []string) (core.Result, error) {
	timeStr := matches[1]
	_, sourceLoc, err := resolveTimeZone(matches[2])
	if err != nil {
		return core.Result{}, err
	}

	hour, minute, err := parseClock(timeStr)
	if err != nil {
		return core.Result{}, err
	}

	// The source location owns the date. Using the local date here can shift the conversion by one day.
	sourceNow := time.Now().In(sourceLoc)
	sourceTime := time.Date(sourceNow.Year(), sourceNow.Month(), sourceNow.Day(), hour, minute, 0, 0, sourceLoc)
	if sourceTime.Hour() != hour || sourceTime.Minute() != minute {
		return core.Result{}, fmt.Errorf("time does not exist in location due to daylight saving transition: %s", timeStr)
	}
	localTime := sourceTime.In(time.Local)

	displayValue := fmt.Sprintf(m.api.GetTranslation(ctx, "plugin_converter_time_with_date_format"), m.formatTimeForDisplay(ctx, localTime), localTime.Format("2006-01-02"))
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

	targetDay, ok := weekdayByName[targetWeekday]
	if !ok {
		return core.Result{}, fmt.Errorf("invalid weekday: %s", targetWeekday)
	}

	// Calculate target date
	now := time.Now()
	targetDate := now.AddDate(0, 0, days)

	// Find the target weekday on or after the requested minimum date.
	weekdayOffset := (int(targetDay) - int(targetDate.Weekday()) + 7) % 7
	targetDate = targetDate.AddDate(0, 0, weekdayOffset)

	val := decimal.NewFromInt(targetDate.Unix())
	weekday := m.api.GetTranslation(ctx, weekdayTranslationKeys[targetDate.Weekday()])
	displayValue := fmt.Sprintf(m.api.GetTranslation(ctx, "plugin_converter_weekday_date_format"), weekday, targetDate.Format("2006-01-02"))
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

	// The month is in the second capture group, which is in matches[2]
	// The year is in the third capture group (if present), which is in matches[3]
	month, ok := monthByAbbreviation[strings.ToLower(matches[2])]
	if !ok {
		return core.Result{}, fmt.Errorf("invalid month: %s", matches[2])
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	year := today.Year()
	hasExplicitYear := len(matches) > 3 && matches[3] != ""
	if hasExplicitYear {
		year, err = strconv.Atoi(matches[3])
		if err != nil {
			return core.Result{}, fmt.Errorf("invalid year: %s", matches[3])
		}
	}

	targetDate := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
	if targetDate.Year() != year || targetDate.Month() != month || targetDate.Day() != day {
		return core.Result{}, fmt.Errorf("invalid date: %d %s %d", day, matches[2], year)
	}
	if !hasExplicitYear && targetDate.Before(today) {
		year++
		targetDate = time.Date(year, month, day, 0, 0, 0, 0, time.Local)
		if targetDate.Year() != year || targetDate.Month() != month || targetDate.Day() != day {
			return core.Result{}, fmt.Errorf("invalid date: %d %s %d", day, matches[2], year)
		}
	}

	// Compare UTC calendar dates so daylight saving changes cannot add or remove a day.
	todayUTC := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	targetUTC := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, time.UTC)
	days := int(targetUTC.Sub(todayUTC).Hours() / 24)
	val := decimal.NewFromInt(int64(days))
	displayValue := fmt.Sprintf(m.api.GetTranslation(ctx, "plugin_converter_days_count"), days)
	return core.Result{
		DisplayValue: displayValue,
		RawValue:     val,
		Unit:         core.Unit{Name: "days", Type: core.UnitTypeTime},
		Module:       m,
	}, nil
}

// parseClock parses a supported clock value without assigning it to the wrong timezone date.
func parseClock(timeStr string) (int, int, error) {
	timeStr = strings.ToLower(strings.TrimSpace(timeStr))

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
			return t.Hour(), t.Minute(), nil
		}
	}

	return 0, 0, fmt.Errorf("unsupported time format: %s", timeStr)
}

func (m *TimeModule) formatTimeForDisplay(ctx context.Context, t time.Time) string {
	weekday := m.api.GetTranslation(ctx, weekdayTranslationKeys[t.Weekday()])
	format := m.api.GetTranslation(ctx, "plugin_converter_time_format")
	return fmt.Sprintf(format, t.Hour(), t.Minute(), weekday)
}

func (m *TimeModule) handleSimpleTimeUnit(ctx context.Context, matches []string) (core.Result, error) {
	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid number: %s", matches[1])
	}

	unit := strings.ToLower(matches[2])
	normalizedUnit, ok := normalizeTimeUnit(unit)
	if !ok {
		return core.Result{}, fmt.Errorf("unsupported time unit: %s", unit)
	}

	timeUnit, ok := timeUnits[normalizedUnit]
	if !ok {
		return core.Result{}, fmt.Errorf("unsupported time unit: %s", unit)
	}

	rawValue := decimal.NewFromInt(value)
	displayUnit := normalizedUnit
	switch normalizedUnit {
	case "h":
		displayUnit = "m"
	case "w":
		displayUnit = "d"
	case "d":
		displayUnit = "w"
	}

	displayValueDecimal := rawValue
	if displayUnit != normalizedUnit {
		displayValueDecimal = rawValue.Mul(decimal.NewFromInt(int64(timeUnit.Duration))).Div(decimal.NewFromInt(int64(timeUnits[displayUnit].Duration)))
	}

	return core.Result{
		DisplayValue: m.formatDurationWithUnit(ctx, displayValueDecimal, displayUnit),
		RawValue:     displayValueDecimal,
		Unit:         core.Unit{Name: displayUnit, Type: core.UnitTypeTime},
		Module:       m,
	}, nil
}

func (m *TimeModule) handleDurationConversion(ctx context.Context, matches []string) (core.Result, error) {
	unit := strings.ToLower(matches[1])
	normalizedUnit, ok := normalizeTimeUnit(unit)
	if !ok {
		return core.Result{}, fmt.Errorf("unsupported time unit: %s", unit)
	}

	return core.Result{
		DisplayValue: fmt.Sprintf(m.api.GetTranslation(ctx, "plugin_converter_time_to_unit"), m.api.GetTranslation(ctx, timeUnits[normalizedUnit].TranslationKey)),
		RawValue:     decimal.NewFromInt(0),
		Unit:         core.Unit{Name: normalizedUnit, Type: core.UnitTypeTime},
		Module:       m,
	}, nil
}

func (m *TimeModule) formatDurationWithUnit(ctx context.Context, value decimal.Decimal, unitName string) string {
	displayName := m.api.GetTranslation(ctx, timeUnits[unitName].TranslationKey)
	if unitName == "w" {
		if value.Equal(value.Truncate(0)) {
			return fmt.Sprintf("%s %s", value.StringFixed(0), displayName)
		}
		truncated := value.Truncate(3)
		return fmt.Sprintf("%s %s", truncated.StringFixed(3), displayName)
	}

	if value.Equal(value.Truncate(0)) {
		return fmt.Sprintf("%s %s", value.StringFixed(0), displayName)
	}

	rounded := value.Round(2)
	return fmt.Sprintf("%s %s", rounded.StringFixed(2), displayName)
}

func normalizeTimeUnit(unit string) (string, bool) {
	switch strings.ToLower(unit) {
	case "ms", "millisecond", "milliseconds":
		return "ms", true
	case "s", "sec", "secs", "second", "seconds":
		return "s", true
	case "m", "min", "mins", "minute", "minutes":
		return "m", true
	case "h", "hr", "hrs", "hour", "hours":
		return "h", true
	case "d", "day", "days":
		return "d", true
	case "w", "week", "weeks":
		return "w", true
	case "y", "year", "years":
		return "y", true
	default:
		return "", false
	}
}
