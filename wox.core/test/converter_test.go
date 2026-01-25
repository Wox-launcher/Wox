package test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestConverterCrypto(t *testing.T) {
	suite := NewTestSuite(t)

	tests := []QueryTest{
		{
			Name:           "BTC shows equivalent value",
			Query:          "1BTC",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return len(title) > 1 && (strings.Contains(title, "$") || strings.Contains(title, "¥") || strings.Contains(title, "€") || strings.Contains(title, "£"))
			},
			Timeout:    45 * time.Second, // Network dependent
			ShouldSkip: ShouldSkipNetworkTests(),
			SkipReason: "Network connectivity required for crypto prices",
		},
		{
			Name:           "BTC to USD",
			Query:          "1BTC in USD",
			ExpectedTitle:  "$",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return len(title) > 1 && strings.HasPrefix(title, "$") && title[1] >= '0' && title[1] <= '9'
			},
			Timeout:    45 * time.Second,
			ShouldSkip: ShouldSkipNetworkTests(),
			SkipReason: "Network connectivity required for crypto prices",
		},
		{
			Name:           "BTC plus USD",
			Query:          "1BTC + 1 USD",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				// Should convert to user's default currency when crypto is involved
				return len(title) > 1 && (strings.Contains(title, "$") || strings.Contains(title, "¥") || strings.Contains(title, "€"))
			},
			Timeout:    45 * time.Second,
			ShouldSkip: ShouldSkipNetworkTests(),
			SkipReason: "Network connectivity required for crypto prices",
		},
		{
			Name:           "ETH to USD",
			Query:          "1 ETH to USD",
			ExpectedTitle:  "$",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return len(title) > 1 && strings.HasPrefix(title, "$") && title[1] >= '0' && title[1] <= '9'
			},
			Timeout:    45 * time.Second,
			ShouldSkip: ShouldSkipNetworkTests(),
			SkipReason: "Network connectivity required for crypto prices",
		},
		{
			Name:           "BTC + ETH uses default currency",
			Query:          "1BTC + 1ETH",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				// Should convert to user's default currency when crypto is involved
				return len(title) > 1 && (strings.Contains(title, "$") || strings.Contains(title, "¥") || strings.Contains(title, "€"))
			},
			Timeout:    45 * time.Second,
			ShouldSkip: ShouldSkipNetworkTests(),
			SkipReason: "Network connectivity required for crypto prices",
		},
		{
			Name:           "invalid crypto query",
			Query:          "1btc dsfsdf1btc dsfsdf",
			ExpectedTitle:  "",
			ExpectedAction: "Search",
			TitleCheck: func(title string) bool {
				// More flexible check - should contain "Search Google for" and part of the query
				return strings.Contains(title, "Search Google for") && strings.Contains(title, "1btc dsfsdf")
			},
		},
		{
			Name:           "BTC plus number",
			Query:          "1btc + 1",
			ExpectedTitle:  "Search Google for 1btc + 1",
			ExpectedAction: "Search",
		},
	}

	suite.RunQueryTests(tests)
}

func TestConverterCurrency(t *testing.T) {
	suite := NewTestSuite(t)

	tests := []QueryTest{
		{
			Name:           "Single Currency",
			Query:          "100USD",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return len(title) > 1 && (strings.Contains(title, "$") || strings.Contains(title, "¥") || strings.Contains(title, "€") || strings.Contains(title, "£"))
			},
			Timeout:    30 * time.Second,
			ShouldSkip: ShouldSkipNetworkTests(),
			SkipReason: "Network connectivity required for exchange rates",
		},
		{
			Name:           "USD to EUR",
			Query:          "100 USD in EUR",
			ExpectedTitle:  "€",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return len(title) > 1 && strings.HasPrefix(title, "€") && title[len("€")] >= '0' && title[len("€")] <= '9'
			},
			Timeout:    30 * time.Second,
			ShouldSkip: ShouldSkipNetworkTests(),
			SkipReason: "Network connectivity required for exchange rates",
		},
		{
			Name:           "EUR to USD",
			Query:          "50 EUR = ? USD",
			ExpectedTitle:  "$",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return len(title) > 1 && strings.HasPrefix(title, "$") && title[1] >= '0' && title[1] <= '9'
			},
			Timeout:    30 * time.Second,
			ShouldSkip: ShouldSkipNetworkTests(),
			SkipReason: "Network connectivity required for exchange rates",
		},
		{
			Name:           "USD to CNY",
			Query:          "100 USD to CNY",
			ExpectedTitle:  "¥",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return len(title) > 1 && strings.HasPrefix(title, "¥") && title[len("¥")] >= '0' && title[len("¥")] <= '9'
			},
			Timeout:    30 * time.Second,
			ShouldSkip: ShouldSkipNetworkTests(),
			SkipReason: "Network connectivity required for exchange rates",
		},
		{
			Name:           "complex convert",
			Query:          "12% of $321 in jpy",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return len(title) > 1 && (strings.Contains(title, "$") || strings.Contains(title, "¥") || strings.Contains(title, "€") || strings.Contains(title, "£"))
			},
			Timeout:    30 * time.Second,
			ShouldSkip: ShouldSkipNetworkTests(),
			SkipReason: "Network connectivity required for exchange rates",
		},
	}

	suite.RunQueryTests(tests)
}

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

	// Calculate expected days until Christmas 2030
	christmas := time.Date(2030, time.December, 25, 0, 0, 0, 0, time.Local)
	daysUntilChristmas := int(christmas.Sub(now).Hours() / 24)
	expectedDaysUntil := fmt.Sprintf("%d days", daysUntilChristmas)
	daysUntilRe := regexp.MustCompile(`^\d+ days$`)

	tests := []QueryTest{
		{
			Name:           "Time in location",
			Query:          "time in Shanghai",
			ExpectedTitle:  expectedTime,
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				// More flexible time check - should contain time format
				return strings.Contains(title, "AM") || strings.Contains(title, "PM") || strings.Contains(title, ":")
			},
		},
		{
			Name:           "Weekday in future",
			Query:          "monday in 10 days",
			ExpectedTitle:  expectedMonday,
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				// Should contain date and Monday
				return strings.Contains(title, "Monday") && strings.Contains(title, "-")
			},
		},
		{
			Name:           "Days until Christmas",
			Query:          "days until 25 Dec 2030",
			ExpectedTitle:  expectedDaysUntil,
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return daysUntilRe.MatchString(title)
			},
		},
		{
			Name:           "Specific time in location",
			Query:          "3:30 pm in tokyo",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				// Should contain time format
				return strings.Contains(title, "PM") || strings.Contains(title, "AM") || strings.Contains(title, ":")
			},
		},
		{
			Name:           "Simple time unit",
			Query:          "100ms",
			ExpectedTitle:  "100 milliseconds",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Time conversion",
			Query:          "1h",
			ExpectedTitle:  "60 minutes",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Specific time in location",
			Query:          "3pm in Tokyo",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return strings.Contains(title, "PM") || strings.Contains(title, "AM")
			},
		},
		{
			Name:           "Simple time unit conversion",
			Query:          "1h to minutes",
			ExpectedTitle:  "60 minutes",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return title == "60 minutes"
			},
		},
		{
			Name:           "Simple time unit conversion with plural hour",
			Query:          "1h",
			ExpectedTitle:  "60 minutes",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return title == "60 minutes"
			},
		},
		{
			Name:           "Simple time unit conversion with plural week",
			Query:          "1 week",
			ExpectedTitle:  "7 days",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return title == "7 days"
			},
		},
		{
			Name:           "Simple time unit conversion with plural week",
			Query:          "10 days",
			ExpectedTitle:  "1.428 weeks",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return title == "1.428 weeks"
			},
		},
		{
			Name:           "Weekday in future",
			Query:          "Monday in 3 days",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				// should show a date
				return strings.Contains(title, ", 202")
			},
		},
		{
			Name:           "Days until specific date",
			Query:          "days until 25th Dec 2030",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return daysUntilRe.MatchString(title)
			},
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
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return strings.Contains(title, ":") && (strings.Contains(title, "AM") || strings.Contains(title, "PM"))
			},
		},
		{
			Name:           "London time",
			Query:          "time in London",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return strings.Contains(title, ":") && (strings.Contains(title, "AM") || strings.Contains(title, "PM"))
			},
		},
		{
			Name:           "New York time",
			Query:          "time in New York",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return strings.Contains(title, ":") && (strings.Contains(title, "AM") || strings.Contains(title, "PM"))
			},
		},
	}

	suite.RunQueryTests(tests)
}

func TestConverterBase(t *testing.T) {
	suite := NewTestSuite(t)

	tests := []QueryTest{
		{
			Name:           "Hex to Dec",
			Query:          "0xff to dec",
			ExpectedTitle:  "255 dec",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return title == "255 dec"
			},
		},
		{
			Name:           "Dec to Hex",
			Query:          "255 dec to hex",
			ExpectedTitle:  "FF hex",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return title == "FF hex"
			},
		},
		{
			Name:           "Bin to Dec",
			Query:          "0b1010 to dec",
			ExpectedTitle:  "10 dec",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return title == "10 dec"
			},
		},
		{
			Name:           "Oct to Dec",
			Query:          "17 oct to dec",
			ExpectedTitle:  "15 dec",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return title == "15 dec"
			},
		},
	}

	suite.RunQueryTests(tests)
}
