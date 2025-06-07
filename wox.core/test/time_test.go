package test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCalculatorTime(t *testing.T) {
	suite := NewTestSuite(t)
	now := time.Now()

	// Get current time
	hour := now.Hour()
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
	expectedTime := fmt.Sprintf("%d:%02d %s", hour, now.Minute(), ampm)

	// Calculate expected date for "monday in 10 days"
	targetDate := now.AddDate(0, 0, 10)
	for targetDate.Weekday() != time.Monday {
		targetDate = targetDate.AddDate(0, 0, 1)
	}
	expectedMonday := fmt.Sprintf("%s (Monday)", targetDate.Format("2006-01-02"))

	// Calculate expected days until Christmas 2025
	christmas := time.Date(2025, time.December, 25, 0, 0, 0, 0, time.Local)
	daysUntilChristmas := int(christmas.Sub(now).Hours() / 24)
	expectedDaysUntil := fmt.Sprintf("%d days", daysUntilChristmas)

	tests := []QueryTest{
		{
			Name:           "Time in location",
			Query:          "time in Shanghai",
			ExpectedTitle:  expectedTime,
			ExpectedAction: "Copy result",
			TitleCheck: func(title string) bool {
				// More flexible time check - should contain time format
				return strings.Contains(title, "AM") || strings.Contains(title, "PM") || strings.Contains(title, ":")
			},
		},
		{
			Name:           "Weekday in future",
			Query:          "monday in 10 days",
			ExpectedTitle:  expectedMonday,
			ExpectedAction: "Copy result",
			TitleCheck: func(title string) bool {
				// Should contain date and Monday
				return strings.Contains(title, "Monday") && strings.Contains(title, "-")
			},
		},
		{
			Name:           "Days until Christmas",
			Query:          "days until 25 Dec 2025",
			ExpectedTitle:  expectedDaysUntil,
			ExpectedAction: "Copy result",
			TitleCheck: func(title string) bool {
				// Should contain "days"
				return strings.Contains(title, "days")
			},
		},
		{
			Name:           "Specific time in location",
			Query:          "3:30 pm in tokyo",
			ExpectedTitle:  "",
			ExpectedAction: "Copy result",
			TitleCheck: func(title string) bool {
				// Should contain time format
				return strings.Contains(title, "PM") || strings.Contains(title, "AM") || strings.Contains(title, ":")
			},
		},
		{
			Name:           "Simple time unit",
			Query:          "100ms",
			ExpectedTitle:  "100 milliseconds",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Time conversion",
			Query:          "1h",
			ExpectedTitle:  "1.00 hours",
			ExpectedAction: "Copy result",
		},
	}

	suite.RunQueryTests(tests)
}

func TestTimeZoneConversions(t *testing.T) {
	suite := NewTestSuite(t)

	tests := []QueryTest{
		{
			Name:           "UTC time",
			Query:          "time in UTC",
			ExpectedTitle:  "",
			ExpectedAction: "Copy result",
			TitleCheck: func(title string) bool {
				return strings.Contains(title, ":") && (strings.Contains(title, "AM") || strings.Contains(title, "PM"))
			},
		},
		{
			Name:           "London time",
			Query:          "time in London",
			ExpectedTitle:  "",
			ExpectedAction: "Copy result",
			TitleCheck: func(title string) bool {
				return strings.Contains(title, ":") && (strings.Contains(title, "AM") || strings.Contains(title, "PM"))
			},
		},
		{
			Name:           "New York time",
			Query:          "time in New York",
			ExpectedTitle:  "",
			ExpectedAction: "Copy result",
			TitleCheck: func(title string) bool {
				return strings.Contains(title, ":") && (strings.Contains(title, "AM") || strings.Contains(title, "PM"))
			},
		},
	}

	suite.RunQueryTests(tests)
}
