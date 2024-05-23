package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/olahol/melody"
	"github.com/rs/cors"
	"github.com/samber/lo"
	"image"
	"net/http"
	"strconv"
	"strings"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/share"
	"wox/util"
)

var browserIcon = plugin.NewWoxImageBase64(`data:image/png;base64,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`)
var chromeIcon = plugin.NewWoxImageBase64(`data:image/png;base64,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`)
var browserWebsocketPortSettingKey = "browserWebsocketPort"

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &BrowserPlugin{})
}

type BrowserPlugin struct {
	api    plugin.API
	m      *melody.Melody
	server *http.Server

	openedTabs []browserTab
	activeTab  browserTab
}

type websocketMsg struct {
	Method string `json:"method"`
	Data   string `json:"data"`
}

type browserTab struct {
	TabId       int    `json:"tabId"`
	WindowId    int    `json:"windowId"`
	TabIndex    int    `json:"tabIndex"`
	Title       string `json:"title"`
	Url         string `json:"url"`
	Pinned      bool   `json:"pinned"`
	Highlighted bool   `json:"highlighted"`
}

func (c *BrowserPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "8f68a760-86a0-46a9-b331-58dcaf091daa",
		Name:          "Browser",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Get opened browser tabs and active url",
		Icon:          browserIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*", "browser",
		},
		Commands: []plugin.MetadataCommand{
			{
				Command:     "summary",
				Description: "Summary current active browser url content",
			},
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: "queryEnv",
				Params: map[string]string{
					"requireActiveWindowName": "true",
					"requireActiveBrowserUrl": "true",
				},
			},
		},
		SettingDefinitions: []definition.PluginSettingDefinitionItem{
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:          browserWebsocketPortSettingKey,
					DefaultValue: "34988",
					Label:        "Server Port",
					Tooltip:      "The port for the websocket server to communicate with the browser extension. Default is 34988. ",
					Style: definition.PluginSettingValueStyle{
						PaddingRight: 10,
					},
					Validators: []validator.PluginSettingValidator{
						&validator.PluginSettingValidatorIsNumber{
							IsInteger: true,
						},
					},
				},
			},
		},
	}
}

func (c *BrowserPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API

	util.Go(ctx, "newWebsocketServer on init", func() {
		err := c.newWebsocketServer(ctx)
		if err != nil {
			c.api.Notify(ctx, "Browser Plugin", fmt.Sprintf("Failed to start websocket server: %s", err.Error()))
		}
	})

	c.api.OnSettingChanged(ctx, func(key, value string) {
		if key == browserWebsocketPortSettingKey {
			util.Go(ctx, "newWebsocketServer on port changed", func() {
				err := c.newWebsocketServer(ctx)
				if err != nil {
					c.api.Notify(ctx, "Browser Plugin", fmt.Sprintf("Failed to start websocket server: %s", err.Error()))
				}
			})
		}
	})
}

func (c *BrowserPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	isInBrowser := strings.ToLower(query.Env.ActiveWindowTitle) == "google chrome"

	if isInBrowser {
		if query.IsGlobalQuery() {
			for _, tab := range c.openedTabs {
				isTitleMatched, titleScore := IsStringMatchScore(ctx, tab.Title, query.Search)
				isUrlMatched, urlScore := strings.Contains(tab.Url, query.Search), int64(1)
				if !isTitleMatched && !isUrlMatched {
					continue
				}

				icon := chromeIcon
				if tabIconImg, err := getWebsiteIconWithCache(ctx, tab.Url); err == nil {
					if backgroundImg, backgroundImgErr := chromeIcon.ToImage(); backgroundImgErr == nil {
						if tabImage, tabImageErr := tabIconImg.ToImage(); tabImageErr == nil {
							resizedImg := imaging.Resize(tabImage, 16, 16, imaging.Lanczos)
							overlayImg := imaging.Overlay(backgroundImg, resizedImg, image.Pt(30, 30), 1)
							overlayWoxImg, overlayWoxImgErr := plugin.NewWoxImage(overlayImg)
							if overlayWoxImgErr == nil {
								icon = overlayWoxImg
							}
						}
					}
				}

				results = append(results, plugin.QueryResult{
					Title:    tab.Title,
					SubTitle: tab.Url,
					Score:    util.MaxInt64(titleScore, urlScore),
					Icon:     icon,
					Actions: []plugin.QueryResultAction{
						{
							Name: "Open",
							Action: func(ctx context.Context, actionContext plugin.ActionContext) {
								c.m.Broadcast([]byte(fmt.Sprintf(`{"method":"highlightTab","data":"{\"tabId\":%d,\"windowId\":%d,\"tabIndex\": %d}"}`, tab.TabId, tab.WindowId, tab.TabIndex)))
							},
						},
					},
				})
			}
		}

		if query.Command == "summary" {
			if query.Env.ActiveBrowserUrl == "" {
				return []plugin.QueryResult{
					{
						Title:    "No active browser url",
						SubTitle: "Please open a browser tab",
						Icon:     browserIcon,
					},
				}
			}

			c.api.ChangeQuery(ctx, share.PlainQuery{
				QueryType: plugin.QueryTypeInput,
				QueryText: "llm tldr " + query.Env.ActiveBrowserUrl,
			})
		}
	}

	return results
}

func (c *BrowserPlugin) newWebsocketServer(ctx context.Context) error {
	serverPortStr := c.api.GetSetting(ctx, browserWebsocketPortSettingKey)
	if serverPortStr == "" {
		return fmt.Errorf("server port is empty")
	}
	port, parseErr := strconv.Atoi(serverPortStr)
	if parseErr != nil {
		return fmt.Errorf("failed to parse server port: %s", parseErr.Error())
	}

	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Starting browser websocket server at port %d", port))

	// close the existing server
	if c.server != nil {
		c.api.Log(ctx, plugin.LogLevelInfo, "closing existing server")
		closeErr := c.server.Shutdown(ctx)
		if closeErr != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to close server: %s", closeErr.Error()))
			return fmt.Errorf("failed to close server: %s", closeErr.Error())
		}
	}
	if c.m != nil {
		c.api.Log(ctx, plugin.LogLevelInfo, "closing existing melody")
		closeErr := c.m.Close()
		if closeErr != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to close melody: %s", closeErr.Error()))
			return fmt.Errorf("failed to close melody: %s", closeErr.Error())
		}
	}

	c.m = melody.New()
	c.m.Config.MaxMessageSize = 1024 * 1024 * 10 // 10MB
	c.m.Config.MessageBufferSize = 1024 * 1024   // 1MB

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c.m.HandleRequest(w, r)
	})

	c.m.HandleMessage(func(s *melody.Session, msg []byte) {
		ctxNew := util.NewTraceContext()
		//c.api.Log(ctxNew, plugin.LogLevelInfo, fmt.Sprintf("received message: %s", string(msg)))

		var request websocketMsg
		unmarshalErr := json.Unmarshal(msg, &request)
		if unmarshalErr != nil {
			c.api.Log(ctxNew, plugin.LogLevelError, fmt.Sprintf("failed to unmarshal websocket request: %s", unmarshalErr.Error()))
			return
		}

		util.Go(ctxNew, "handle chrome extension request", func() {
			switch request.Method {
			case "ping":
				err := c.m.Broadcast([]byte(`{"method":"pong"}`))
				if err != nil {
					c.api.Log(ctxNew, plugin.LogLevelError, fmt.Sprintf("failed to broadcast pong: %s", err.Error()))
					return
				}
			case "tabs":
				c.onUpdateTabs(ctxNew, request.Data)
			default:
				c.api.Log(ctxNew, plugin.LogLevelError, fmt.Sprintf("unknown websocket method: %s", request.Method))
			}
		})
	})

	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("browser websocket server start at：ws://localhost:%d", port))
	c.server = &http.Server{Addr: fmt.Sprintf("localhost:%d", port), Handler: cors.Default().Handler(mux)}
	err := c.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to start server: %s", err.Error()))
		return fmt.Errorf("failed to start server: %s", err.Error())
	}

	return nil
}

func (c *BrowserPlugin) onUpdateTabs(ctx context.Context, data string) {
	var tabs []browserTab
	err := json.Unmarshal([]byte(data), &tabs)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to unmarshal tabs: %s", err.Error()))
		return
	}

	activeTab, exist := lo.Find(tabs, func(tab browserTab) bool {
		return tab.Highlighted
	})
	if exist {
		c.activeTab = activeTab
		plugin.GetPluginManager().SetActiveBrowserUrl(activeTab.Url)
	}

	// filter invalid tabs
	c.openedTabs = lo.Filter(tabs, func(tab browserTab, _ int) bool {
		return tab.Url != ""
	})

	util.Go(ctx, "index browser icons", func() {
		for _, tab := range c.openedTabs {
			getWebsiteIconWithCache(ctx, tab.Url)
		}
	})
}
