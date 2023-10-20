package system

import (
	"context"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"os"
	"runtime"
	"strings"
	"wox/plugin"
	"wox/util"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &BrowserBookmarkPlugin{})
}

type Bookmark struct {
	Name string
	Url  string
}

type BrowserBookmarkPlugin struct {
	api       plugin.API
	bookmarks []Bookmark
}

func (c *BrowserBookmarkPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "95d041d3-be7e-4b20-8517-88dda2db280b",
		Name:          "BrowserBookmark",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Search browser bookmarks",
		Icon:          "",
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (c *BrowserBookmarkPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API

	if strings.ToLower(runtime.GOOS) == "darwin" {
		profiles := []string{"Default", "Profile 1", "Profile 2", "Profile 3"}
		for _, profile := range profiles {
			chromeBookmarks := c.loadChromeBookmarkInMacos(ctx, profile)
			c.bookmarks = append(c.bookmarks, chromeBookmarks...)
		}
	}
}

func (c *BrowserBookmarkPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	for _, b := range c.bookmarks {
		var bookmark = b
		var isMatch bool
		var matchScore int

		isNameMatch, nameScore := IsStringMatchScore(ctx, bookmark.Name, query.Search)
		if isNameMatch {
			isMatch = true
			matchScore = nameScore
		} else {
			//url match must be exact part match
			if strings.Contains(bookmark.Url, query.Search) {
				isUrlMatch, urlScore := IsStringMatchScoreNoPinYin(ctx, bookmark.Url, query.Search)
				if isUrlMatch {
					isMatch = true
					matchScore = urlScore
				}
			}
		}

		if isMatch {
			results = append(results, plugin.QueryResult{
				Title:    bookmark.Name,
				SubTitle: bookmark.Url,
				Score:    matchScore,
				Icon:     plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAYAAACqaXHeAAAACXBIWXMAAAsTAAALEwEAmpwYAAAC9UlEQVR4nO1bTW8SURSdahfWdf0l6u9whfoPdNtNo2Ye07jRtNbE2LU1UZgHiZLUiJpIqXVhWw2ygkZMdGNTrMaApYChHHOnSJpWyswwH8z03uQkBN5M7j1z77sfvFEUFhYWlj6iaTihSlxWdbxUJcpCAk5h5skv4PMVW6Br99+ro9uLqI5LCjCiOCHXkzij6lhy0mi3CDiArBbD+EDGaws4LXR8cMt4lwkgvJ9IYsw2AULippvGe0AAhYVm7+lnMarq+Bl0AoSOH2SLnad/3m3jPSFAAjfiOGedAB0XwkIA2WKZgKhEJCwEkC1MgFWJsgeAQ0DwHgDeBAVnAXAaFFwH4BAJt1JtbHyvo1mrmUJ9u4aVYjM8hdCDTMu08f9QqexYIqBamDQwlARoCeD1xz/IlxqmkPvUwKOl1qH7TB8goF26ilLuHvRXOUwlWgboM31Hv+1fOx2GUjiaaONrfgaVwjUsv03hbmqr51r6jdZUi5P4kp81rg08AcILkpkAuOMBc+ldxJdbphB70zIyR2g8YC69i8a2tSywUa4fbwK+hYkAYSMEbj8NUQiIAIEJkOwBEVdK4UzefClsFVQ6P+6UzrPP2lhbb/53HelAungeAvM2miG7zRN1kUeto8bMn3a4bL4dtgpqn98V9tpn8oRqdadnau2XXXgPkLwJRhwPAREgsAdIn2eCvyu17jRotdg0NjinNkvSoV+XORQzwZXOQJRSm9MZw5c0qFmYCa6uN3FnYe8pkSdQkeNUwUQ6+FIIiQCBCZDsARHfByLxI/BwsdWNY9rRaXhi9lrSITAjseYRoM2M7mvl7zYC6dCPhGARULZOwH0/CBDHPQREgMAESPaAyNAelRVewM5RWeHRYWlPkMRZywRoWYzSUXPflR8QqsRWJImTlgkgETqm/DbAAaiKXZlIYoxeOwns09exps3jlDKIaDGMqzoW/TbGBjIDvzTVFWBETeCiKpEWEptDYFwvbAodz42879RrcywsLEqY5S+u/BSNeloCCQAAAABJRU5ErkJggg==`),
				Actions: []plugin.QueryResultAction{
					{
						Name: "Open in browser",
						Action: func() {
							util.ShellOpen(bookmark.Url)
						},
					},
				},
			})
		}
	}

	return
}

func (c *BrowserBookmarkPlugin) loadChromeBookmarkInMacos(ctx context.Context, profile string) (results []Bookmark) {
	bookmarkLocation, _ := homedir.Expand(fmt.Sprintf("~/Library/Application Support/Google/Chrome/%s/Bookmarks", profile))
	file, readErr := os.ReadFile(bookmarkLocation)
	if readErr != nil {
		c.api.Log(ctx, fmt.Sprintf("error reading chrome bookmark file: %s", readErr.Error()))
		return
	}

	groups := util.FindRegexGroups(`(?ms)name": "(?P<name>.*?)",.*?type": "url",.*?"url": "(?P<url>.*?)".*?}, {`, string(file))
	for _, group := range groups {
		c.api.Log(ctx, fmt.Sprintf("name: %v, url: %s", group["name"], group["url"]))
		results = append(results, Bookmark{
			Name: group["name"],
			Url:  group["url"],
		})
	}

	return results
}
