package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"

	"github.com/olahol/melody"
	"github.com/rs/cors"
	"github.com/samber/lo"
)

var browserIcon = common.PluginBrowserIcon
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
		Name:          "i18n:plugin_browser_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_browser_plugin_description",
		Icon:          browserIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*", "browser",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureQueryEnv,
				Params: map[string]any{
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
					Label:        "i18n:plugin_browser_server_port",
					Tooltip:      "i18n:plugin_browser_server_port_tooltip",
					Style: definition.PluginSettingValueStyle{
						PaddingRight: 10,
					},
					Validators: []validator.PluginSettingValidator{
						{
							Type: validator.PluginSettingValidatorTypeIsNumber,
							Value: &validator.PluginSettingValidatorIsNumber{
								IsInteger: true,
							},
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
			c.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_browser_server_start_error"), err.Error()))
		}
	})

	c.api.OnSettingChanged(ctx, func(key, value string) {
		if key == browserWebsocketPortSettingKey {
			util.Go(ctx, "newWebsocketServer on port changed", func() {
				err := c.newWebsocketServer(ctx)
				if err != nil {
					c.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_browser_server_start_error"), err.Error()))
				}
			})
		}
	})
}

func (c *BrowserPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	// only show results when the active window is a browser in global query
	isInBrowser := strings.ToLower(query.Env.ActiveWindowTitle) == "google chrome"
	if query.IsGlobalQuery() && !isInBrowser {
		return results
	}

	for _, tab := range c.openedTabs {
		isTitleMatched, titleScore := plugin.IsStringMatchScore(ctx, tab.Title, query.Search)
		isUrlMatched, urlScore := strings.Contains(tab.Url, query.Search), int64(1)
		if !isTitleMatched && !isUrlMatched {
			continue
		}

		icon := common.ChromeIcon
		if tabIcon, err := getWebsiteIconWithCache(ctx, tab.Url); err == nil {
			icon = common.ChromeIcon.Overlay(tabIcon, 0.4, 0.6, 0.6)
		}

		results = append(results, plugin.QueryResult{
			Title:    tab.Title,
			SubTitle: tab.Url,
			Score:    util.MaxInt64(titleScore, urlScore),
			Icon:     icon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_browser_open_tab",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						c.m.Broadcast([]byte(fmt.Sprintf(`{"method":"highlightTab","data":"{\"tabId\":%d,\"windowId\":%d,\"tabIndex\": %d}"}`, tab.TabId, tab.WindowId, tab.TabIndex)))
					},
				},
			},
		})
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

	//remove duplicate tabs
	uniqueTabs := lo.UniqBy(tabs, func(tab browserTab) string {
		return tab.Url
	})
	// filter invalid tabs
	c.openedTabs = lo.Filter(uniqueTabs, func(tab browserTab, _ int) bool {
		return tab.Url != ""
	})

	util.Go(ctx, "index browser icons", func() {
		for _, tab := range c.openedTabs {
			getWebsiteIconWithCache(ctx, tab.Url)
		}
	})
}
