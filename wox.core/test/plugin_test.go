package test

import (
	"testing"
)

func TestUrlPlugin(t *testing.T) {
	suite := NewTestSuite(t)

	tests := []QueryTest{
		{
			Name:           "Domain only",
			Query:          "google.com",
			ExpectedTitle:  "google.com",
			ExpectedAction: "Open",
		},
		{
			Name:           "With https",
			Query:          "https://www.google.com",
			ExpectedTitle:  "https://www.google.com",
			ExpectedAction: "Open",
		},
		{
			Name:           "With path",
			Query:          "github.com/Wox-launcher/Wox",
			ExpectedTitle:  "github.com/Wox-launcher/Wox",
			ExpectedAction: "Open",
		},
		{
			Name:           "With query parameters",
			Query:          "google.com/search?q=wox",
			ExpectedTitle:  "google.com/search?q=wox",
			ExpectedAction: "Open",
		},
	}

	suite.RunQueryTests(tests)
}

func TestSystemPlugin(t *testing.T) {
	suite := NewTestSuite(t)

	tests := []QueryTest{
		{
			Name:           "Lock command",
			Query:          "lock",
			ExpectedTitle:  "Lock PC",
			ExpectedAction: "Execute",
		},
		{
			Name:           "Settings command",
			Query:          "settings",
			ExpectedTitle:  "Open Wox Settings",
			ExpectedAction: "Execute",
		},
		{
			Name:           "Empty trash command",
			Query:          "trash",
			ExpectedTitle:  "Empty Trash",
			ExpectedAction: "Execute",
		},
		{
			Name:           "Exit command",
			Query:          "exit",
			ExpectedTitle:  "Exit",
			ExpectedAction: "Execute",
		},
	}

	suite.RunQueryTests(tests)
}

func TestWebSearchPlugin(t *testing.T) {
	suite := NewTestSuite(t)

	tests := []QueryTest{
		{
			Name:           "Google search",
			Query:          "g wox launcher",
			ExpectedTitle:  "Search Google for wox launcher",
			ExpectedAction: "Search",
		},
	}

	suite.RunQueryTests(tests)
}
