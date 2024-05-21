package system

import (
	"context"
	"errors"
	"fmt"
	"github.com/olahol/melody"
	"github.com/rs/cors"
	"net/http"
	"strconv"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
)

var browserIcon = plugin.NewWoxImageBase64(`data:image/png;base64,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`)
var browserWebsocketPortSettingKey = "browserWebsocketPort"

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &BrowserPlugin{})
}

type BrowserPlugin struct {
	api    plugin.API
	m      *melody.Melody
	server *http.Server
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
			"*",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
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
	if query.IsGlobalQuery() {
		if query.Env.ActiveWindowTitle == "Google Chrome" {

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
		c.api.Log(ctxNew, plugin.LogLevelInfo, fmt.Sprintf("received message: %s", string(msg)))
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
