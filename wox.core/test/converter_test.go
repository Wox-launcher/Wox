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
			Name:           "HKD to CNY",
			Query:          "1000hkd in cny",
			ExpectedTitle:  "¥",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return len(title) > 1 && strings.HasPrefix(title, "¥") && title[len("¥")] >= '0' && title[len("¥")] <= '9'
			},
			Timeout: 30 * time.Second,
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
	time24Re := regexp.MustCompile(`^\d{2}:\d{2}`)

	expectedTime := now.Format("15:04")

	// Calculate expected date for "monday in 10 days"
	targetDate := now.AddDate(0, 0, 10)
	for targetDate.Weekday() != time.Monday {
		targetDate = targetDate.AddDate(0, 0, 1)
	}
	expectedMonday := fmt.Sprintf("Mon, %s", targetDate.Format("2006-01-02"))

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
				return time24Re.MatchString(title)
			},
		},
		{
			Name:           "Weekday in future",
			Query:          "monday in 10 days",
			ExpectedTitle:  expectedMonday,
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return strings.Contains(title, "Mon") && strings.Contains(title, "-")
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
				return time24Re.MatchString(title)
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
				return time24Re.MatchString(title)
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
	time24Re := regexp.MustCompile(`^\d{2}:\d{2}`)

	tests := []QueryTest{
		{
			Name:           "UTC time",
			Query:          "time in UTC",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return time24Re.MatchString(title)
			},
		},
		{
			Name:           "London time",
			Query:          "time in London",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return time24Re.MatchString(title)
			},
		},
		{
			Name:           "New York time",
			Query:          "time in New York",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return time24Re.MatchString(title)
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

func TestConverterStorageQueryIntentParity(t *testing.T) {
	suite := NewTestSuite(t)

	tests := []QueryTest{
		{
			Name:           "To conversion syntax parses Byte base unit to Decimal storage unit",
			Query:          "32 bytes to gb",
			ExpectedTitle:  "0.000000032 GB",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Equals-question conversion syntax matches Decimal storage unit output",
			Query:          "32 bytes =? gb",
			ExpectedTitle:  "0.000000032 GB",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Compact byte input parses with equals-question conversion syntax",
			Query:          "32bytes =? gb",
			ExpectedTitle:  "0.000000032 GB",
			ExpectedAction: "Copy",
		},
	}

	suite.RunQueryTests(tests)
}

func TestConverterUnits(t *testing.T) {
	suite := NewTestSuite(t)

	tests := []QueryTest{
		{
			Name:           "Length conversion",
			Query:          "10cm to mm",
			ExpectedTitle:  "100 millimeters",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Weight conversion",
			Query:          "100lb to kg",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return strings.Contains(title, "45.36") && strings.Contains(title, "kilograms")
			},
		},
		{
			Name:           "Temperature conversion",
			Query:          "32f to c",
			ExpectedTitle:  "0 celsius",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Storage Unit full-word form renders Unit symbol form output",
			Query:          "1 gigabyte to gibibyte",
			ExpectedTitle:  "0.9313225746154785 GiB",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Storage Unit symbol form renders Unit symbol form output",
			Query:          "1 GB to MiB",
			ExpectedTitle:  "953.67431640625 MiB",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Storage Byte base unit symbol alias renders symbolized output",
			Query:          "32 b to bytes",
			ExpectedTitle:  "32 B",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Storage Byte base unit singular alias renders symbolized output",
			Query:          "1 byte to b",
			ExpectedTitle:  "1 B",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Storage Byte base unit plural alias renders symbolized output",
			Query:          "2 bytes to byte",
			ExpectedTitle:  "2 B",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Storage Decimal storage unit uses GB output",
			Query:          "32 bytes to gb",
			ExpectedTitle:  "0.000000032 GB",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Storage Binary storage unit uses GiB output",
			Query:          "32 bytes to gib",
			ExpectedTitle:  "",
			ExpectedAction: "Copy",
			TitleCheck: func(title string) bool {
				return strings.Contains(title, "0.0000000298023224") && strings.Contains(title, "GiB")
			},
		},
		{
			Name:           "Storage gb ambiguity resolves to Decimal storage unit bytes",
			Query:          "1 gb to bytes",
			ExpectedTitle:  "1000000000 B",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Storage gib preserves Binary storage unit bytes",
			Query:          "1 gib to bytes",
			ExpectedTitle:  "1073741824 B",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Storage Decimal to Binary storage unit explicit control",
			Query:          "1 gb to gib",
			ExpectedTitle:  "0.9313225746154785 GiB",
			ExpectedAction: "Copy",
		},
	}

	suite.RunQueryTests(tests)
}
