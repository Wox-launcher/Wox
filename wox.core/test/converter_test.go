package test

import (
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
		// Complex crypto percentage calculations are not supported
		// {
		// 	Name:           "complex crypto convert",
		// 	Query:          "12% of 1btc in jpy",
		// 	ExpectedTitle:  "",
		// 	ExpectedAction: "Copy",
		// 	TitleCheck: func(title string) bool {
		// 		return len(title) > 1 && strings.Contains(title, "¥")
		// 	},
		// 	Timeout:    45 * time.Second,
		// 	ShouldSkip: ShouldSkipNetworkTests(),
		// 	SkipReason: "Network connectivity required for crypto prices and exchange rates",
		// },
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
