package system

import (
	"context"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"os"
	"os/exec"
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
		chromeBookmarks := c.loadChromeBookmarkInMacos(ctx)
		c.bookmarks = append(c.bookmarks, chromeBookmarks...)
	}
}

func (c *BrowserBookmarkPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	for _, bookmark := range c.bookmarks {
		var isMatch bool
		var matchScore int

		isNameMatch, nameScore := IsStringMatchScore(ctx, bookmark.Name, query.Search)
		if isNameMatch {
			isMatch = true
			matchScore = nameScore
		} else {
			isUrlMatch, urlScore := IsStringMatchScoreNoPinYin(ctx, bookmark.Url, query.Search)
			if isUrlMatch {
				isMatch = true
				matchScore = urlScore
			}
		}

		if isMatch {
			results = append(results, plugin.QueryResult{
				Title:    bookmark.Name,
				SubTitle: bookmark.Url,
				Score:    matchScore,
				Icon:     plugin.NewWoxImageSvg(`<svg t="1697640255303" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="4021" width="200" height="200"><path d="M512.104171 176.04883h207.925127c8.750356 0 16.042319 7.291963 16.042319 16.04232v640.026042c0 8.750356-11.667141 23.542625-16.042319 16.04232L512.520855 487.936521l-0.208342 0.208342-0.208342 0.416683v-312.512716z" fill="#0288D1" p-id="4022"></path><path d="M303.970702 176.04883h207.925127v312.721058L303.970702 847.95117c-4.375178 7.708647-16.042319-7.291963-16.042319-16.04232V192.09115c0-8.958698 7.291963-16.042319 16.042319-16.04232z" fill="#039BE5" p-id="4023"></path></svg>`),
				Actions: []plugin.QueryResultAction{
					{
						Name: "Open in browser",
						Action: func() {
							c.open(ctx, bookmark.Url)
						},
					},
				},
			})
		}
	}

	return
}

func (c *BrowserBookmarkPlugin) loadChromeBookmarkInMacos(ctx context.Context) (results []Bookmark) {
	bookmarkLocation, _ := homedir.Expand("~/Library/Application Support/Google/Chrome/Default/Bookmarks")
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

func (c *BrowserBookmarkPlugin) open(ctx context.Context, path string) {
	if strings.ToLower(runtime.GOOS) == "darwin" {
		exec.Command("open", path).Start()
	}
	if strings.ToLower(runtime.GOOS) == "windows" {
		exec.Command("cmd", "/C", "start", path).Start()
	}
}
