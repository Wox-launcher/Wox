package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jinzhu/copier"
	"github.com/olahol/melody"
	"github.com/rs/cors"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"wox/plugin"
	"wox/resource"
	"wox/setting"
	"wox/setting/definition"
	"wox/share"
	"wox/ui/dto"
	"wox/util"
)

var m *melody.Melody
var uiConnected = false

type websocketMsgType string

const (
	WebsocketMsgTypeRequest  websocketMsgType = "WebsocketMsgTypeRequest"
	WebsocketMsgTypeResponse websocketMsgType = "WebsocketMsgTypeResponse"
)

type WebsocketMsg struct {
	Id      string
	Type    websocketMsgType
	Method  string
	Success bool
	Data    any
}

type RestResponse struct {
	Success bool
	Message string
	Data    any
}

func writeSuccessResponse(w http.ResponseWriter, data any) {
	d, marshalErr := json.Marshal(RestResponse{
		Success: true,
		Message: "",
		Data:    data,
	})
	if marshalErr != nil {
		writeErrorResponse(w, marshalErr.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(d)
}

func writeErrorResponse(w http.ResponseWriter, errMsg string) {
	d, _ := json.Marshal(RestResponse{
		Success: false,
		Message: errMsg,
		Data:    "",
	})

	w.Header().Set("Content-Type", "application/json")
	w.Write(d)
}

func serveAndWait(ctx context.Context, port int) {
	m = melody.New()
	m.Config.MaxMessageSize = 1024 * 1024 * 10 // 10MB
	m.Config.MessageBufferSize = 1024 * 1024   // 1MB

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeSuccessResponse(w, "Wox")
	})

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	mux.HandleFunc("/index.html", func(w http.ResponseWriter, r *http.Request) {
		fileContent, err := resource.GetReactFile(util.NewTraceContext(), "index.html")
		if err != nil {
			writeErrorResponse(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(fileContent)
	})

	mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/assets/")
		fileContent, err := resource.GetReactFile(util.NewTraceContext(), "assets", path)
		if err != nil {
			writeErrorResponse(w, err.Error())
			return
		}

		contentType := "text/plain"
		if strings.HasSuffix(path, "js") {
			contentType = "text/javascript; charset=utf-8"
		}
		if strings.HasSuffix(path, "css") {
			contentType = "text/css"
		}

		w.Header().Set("Content-Type", contentType)
		w.Write(fileContent)
	})

	mux.HandleFunc("/image", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			writeErrorResponse(w, "id is empty")
			return
		}

		imagePath, ok := plugin.GetLocalImageMap(id)
		if !ok {
			writeErrorResponse(w, "imagePath is empty")
			return
		}

		if _, statErr := os.Stat(imagePath); os.IsNotExist(statErr) {
			w.Write([]byte("image not exist"))
			return
		}

		w.Header().Set("Cache-Control", "public, max-age=3600")
		http.ServeFile(w, r, imagePath)
	})

	mux.HandleFunc("/preview", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			writeErrorResponse(w, "id is empty")
			return
		}

		preview, err := plugin.GetPluginManager().GetResultPreview(util.NewTraceContext(), id)
		if err != nil {
			writeErrorResponse(w, err.Error())
			return
		}

		writeSuccessResponse(w, preview)
	})

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		m.HandleRequest(w, r)
	})

	mux.HandleFunc("/theme", func(w http.ResponseWriter, r *http.Request) {
		theme := GetUIManager().GetCurrentTheme(util.NewTraceContext())
		writeSuccessResponse(w, theme)
	})

	mux.HandleFunc("/plugin/store", func(w http.ResponseWriter, r *http.Request) {
		manifests := plugin.GetStoreManager().GetStorePluginManifests(util.NewTraceContext())
		var plugins = make([]dto.StorePlugin, len(manifests))
		copyErr := copier.Copy(&plugins, &manifests)
		if copyErr != nil {
			writeErrorResponse(w, copyErr.Error())
			return
		}

		for i, storePlugin := range plugins {
			isInstalled := lo.ContainsBy(plugin.GetPluginManager().GetPluginInstances(), func(item *plugin.Instance) bool {
				return item.Metadata.Id == storePlugin.Id
			})
			plugins[i].Icon = plugin.NewWoxImageUrl(manifests[i].IconUrl)
			plugins[i].IsInstalled = isInstalled
		}

		writeSuccessResponse(w, plugins)
	})

	mux.HandleFunc("/plugin/installed", func(w http.ResponseWriter, r *http.Request) {
		defer util.GoRecover(util.NewTraceContext(), "get installed plugins")

		getCtx := util.NewTraceContext()
		instances := plugin.GetPluginManager().GetPluginInstances()
		var plugins []dto.InstalledPlugin
		for _, instance := range instances {
			var installedPlugin dto.InstalledPlugin
			copyErr := copier.Copy(&installedPlugin, &instance.Metadata)
			if copyErr != nil {
				writeErrorResponse(w, copyErr.Error())
				return
			}

			installedPlugin.IsSystem = instance.IsSystemPlugin
			logger.Debug(getCtx, fmt.Sprintf("get plugin setting: %s", instance.Metadata.Name))
			installedPlugin.SettingDefinitions = lo.Filter(instance.Metadata.SettingDefinitions, func(item definition.PluginSettingDefinitionItem, _ int) bool {
				return !lo.Contains(item.DisabledInPlatforms, util.GetCurrentPlatform())
			})

			//translate setting definition labels
			for i := range installedPlugin.SettingDefinitions {
				if installedPlugin.SettingDefinitions[i].Value != nil {
					installedPlugin.SettingDefinitions[i].Value.Translate(instance.API.GetTranslation)
				}
			}

			var definitionSettings = util.NewHashMap[string, string]()
			for _, item := range instance.Metadata.SettingDefinitions {
				if item.Value != nil {
					settingValue := instance.API.GetSetting(getCtx, item.Value.GetKey())
					definitionSettings.Store(item.Value.GetKey(), settingValue)
				}
			}
			installedPlugin.Settings = *instance.Setting
			//only return user pre-defined settings
			installedPlugin.Settings.Settings = definitionSettings

			iconImg, parseErr := plugin.ParseWoxImage(instance.Metadata.Icon)
			if parseErr == nil {
				installedPlugin.Icon = iconImg
			} else {
				installedPlugin.Icon = plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAAELUlEQVR4nO3ZW2xTdRwH8JPgkxE1XuKFQUe73rb1IriNyYOJvoiALRszvhqffHBLJjEx8Q0TlRiN0RiNrPd27boLY1wUFAQHyquJiTIYpefay7au2yiJG1/zb6Kx/ZfS055T1mS/5JtzXpr+Pufy//9P/gyzURulXIHp28Rp7H5eY/OSc6bRiiPNN9tBQs4bDsFrbN5/AQ2JANO3qRgx9dZ76I2vwingvsQhgHUK2NPQCKeAuOw7Mf72B1hPCEZu9bBrWE8IRm6RH60nBFMNQA3Eh6kVzCzzyOVu5I+HUyvqApREkOZxe5bKR+kVdQFKIcgVLwW4usyrD1ACcSsXKwm4lYvVB1Ar4r7fAWeNCPLClgIcruBFVhRQK4Jc8Vwulj/WZRQqh4i8+X5d5glGaYCDBzp/WYQ5KsJ98JDqCEZJgIO/g53nM9BHpXxMEQHuXnURjFIA0vyOHxfQMiIVxBgW4FIRwSgBcLB3YPt+DrqwWDKGEA9Xz7tlES/9nkPHuQyeP5/By3/crh9gf3wNlpMpaENC2egDHFwHSiBurqL78hLaJlNoPZaCeSIJ01gSu68sqw/YF1uDeTKF5qBQUXR+DkNFiOgbg7BOSBTAOJrIw1QD7J1dzf+Jxs/LitbL4qhjsAAROjAA67hEAQzRBLovLSkPePX6an6U2eblqorWE4en7xCFsIxJFEAfkcoiZANeufo3tMMitnq4qkIArZMp2E+k4H29COEcQPuoSAH0YQm7prPKAMhjsMXFVpUmN4f2qTRsp+dgPTUH21SyJKJtRKQALcNiSYRswLNH46gmTW4W7SfSsP8w/x/AcjIN6/EkvEWPU9AxgNaIQAF0IRFdF7O1AZ75Lg65aXKxsJxKw35mngIQlOVYoiTCHBYogDZQiJANePrbm5AT8uhYT8/hubPzdwW0HU/BMpGA5yCNMIV4CrDdL6DzQrY6wFPfxFBpSPOkabLEuBeAzAOWcYIonCcCr/XDFOQpQLOPIBblA578OoZKQprfcXYeO88tVAwg80D7mARPL40wBjgKoPHy8gFPfHUD90q++Z8W8usauQAyD7RFJbiLEfv7YfBztQMe/3IW5bLFzeYb7/g5UzXANJZE64gIdw+N0Hu52gCPfTGLu2Wrm0XHhQw6Ly7WDDCOJmCOiNQq1r+vHy0etnrAo59fR6mQcZ40Tr7GlAIYogmYhgVqFUsQOjdbHeCRz66hONt8HLqmF9E1nVUcoI9IMIUIYpBGuOLyAQ9/eg3/jybAo/tyFrsuZVUD6MMSjEGeQvj2vgMwLz4gC7D5yAy7+cgMSLYHBbzw2xK6f1Uf0DIswhDgMeQsRPAae0QW4sGP/9zz0Cd/seRTcfeVpboCdCEReh+PIUeNiHWx7dtsDxQibF6mkQpFCHLONOYGvM1Hmm+obd+NYhqg/gG2aOxED6eh5gAAAABJRU5ErkJggg==`)
			}

			plugins = append(plugins, installedPlugin)
		}

		writeSuccessResponse(w, plugins)
	})

	mux.HandleFunc("/plugin/install", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		idResult := gjson.GetBytes(body, "id")
		if !idResult.Exists() {
			writeErrorResponse(w, "id is empty")
			return
		}

		pluginId := idResult.String()

		plugins := plugin.GetStoreManager().GetStorePluginManifests(ctx)
		findPlugin, exist := lo.Find(plugins, func(item plugin.StorePluginManifest) bool {
			if item.Id == pluginId {
				return true
			}
			return false
		})
		if !exist {
			writeErrorResponse(w, "can't find plugin in the store")
			return
		}

		installErr := plugin.GetStoreManager().Install(ctx, findPlugin)
		if installErr != nil {
			writeErrorResponse(w, "can't install plugin: "+installErr.Error())
			return
		}

		writeSuccessResponse(w, "")
	})

	mux.HandleFunc("/plugin/uninstall", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		idResult := gjson.GetBytes(body, "id")
		if !idResult.Exists() {
			writeErrorResponse(w, "id is empty")
			return
		}

		pluginId := idResult.String()

		plugins := plugin.GetPluginManager().GetPluginInstances()
		findPlugin, exist := lo.Find(plugins, func(item *plugin.Instance) bool {
			if item.Metadata.Id == pluginId {
				return true
			}
			return false
		})
		if !exist {
			writeErrorResponse(w, "can't find plugin")
			return
		}

		uninstallErr := plugin.GetStoreManager().Uninstall(ctx, findPlugin)
		if uninstallErr != nil {
			writeErrorResponse(w, "can't uninstall plugin: "+uninstallErr.Error())
			return
		}

		writeSuccessResponse(w, "")
	})

	mux.HandleFunc("/theme/store", func(w http.ResponseWriter, r *http.Request) {
		storeThemes := GetStoreManager().GetThemes()
		var themes = make([]dto.Theme, len(storeThemes))
		copyErr := copier.Copy(&themes, &storeThemes)
		if copyErr != nil {
			writeErrorResponse(w, copyErr.Error())
			return
		}

		for i, storeTheme := range themes {
			isInstalled := lo.ContainsBy(GetUIManager().GetAllThemes(ctx), func(item share.Theme) bool {
				return item.ThemeId == storeTheme.ThemeId
			})
			themes[i].IsInstalled = isInstalled
		}

		writeSuccessResponse(w, themes)
	})

	mux.HandleFunc("/theme/installed", func(w http.ResponseWriter, r *http.Request) {
		installedThemes := GetUIManager().GetAllThemes(ctx)
		var themes = make([]dto.Theme, len(installedThemes))
		copyErr := copier.Copy(&themes, &installedThemes)
		if copyErr != nil {
			writeErrorResponse(w, copyErr.Error())
			return
		}

		writeSuccessResponse(w, themes)
	})

	mux.HandleFunc("/theme/install", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		idResult := gjson.GetBytes(body, "id")
		if !idResult.Exists() {
			writeErrorResponse(w, "id is empty")
			return
		}

		themeId := idResult.String()

		storeThemes := GetStoreManager().GetThemes()
		findTheme, exist := lo.Find(storeThemes, func(item share.Theme) bool {
			if item.ThemeId == themeId {
				return true
			}
			return false
		})
		if !exist {
			writeErrorResponse(w, "can't find theme in theme store")
			return
		}

		installErr := GetStoreManager().Install(ctx, findTheme)
		if installErr != nil {
			writeErrorResponse(w, "can't install theme: "+installErr.Error())
			return
		}

		writeSuccessResponse(w, "")
	})

	mux.HandleFunc("/theme/uninstall", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		idResult := gjson.GetBytes(body, "id")
		if !idResult.Exists() {
			writeErrorResponse(w, "id is empty")
			return
		}

		themeId := idResult.String()

		storeThemes := GetUIManager().GetAllThemes(ctx)
		findTheme, exist := lo.Find(storeThemes, func(item share.Theme) bool {
			if item.ThemeId == themeId {
				return true
			}
			return false
		})
		if !exist {
			writeErrorResponse(w, "can't find theme")
			return
		}

		uninstallErr := GetStoreManager().Uninstall(ctx, findTheme)
		if uninstallErr != nil {
			writeErrorResponse(w, "can't uninstall theme: "+uninstallErr.Error())
			return
		}

		writeSuccessResponse(w, "")
	})

	mux.HandleFunc("/setting/wox", func(w http.ResponseWriter, r *http.Request) {
		woxSetting := setting.GetSettingManager().GetWoxSetting(util.NewTraceContext())

		var settingDto dto.WoxSetting
		copyErr := copier.Copy(&settingDto, &woxSetting)
		if copyErr != nil {
			writeErrorResponse(w, copyErr.Error())
			return
		}

		settingDto.MainHotkey = woxSetting.MainHotkey.Get()
		settingDto.SelectionHotkey = woxSetting.SelectionHotkey.Get()
		settingDto.QueryHotkeys = woxSetting.QueryHotkeys.Get()

		writeSuccessResponse(w, settingDto)
	})

	mux.HandleFunc("/setting/wox/update", func(w http.ResponseWriter, r *http.Request) {
		type keyValuePair struct {
			Key   string
			Value string
		}

		decoder := json.NewDecoder(r.Body)
		var kv keyValuePair
		err := decoder.Decode(&kv)
		if err != nil {
			w.Header().Set("code", "500")
			w.Write([]byte(err.Error()))
			return
		}

		updateErr := setting.GetSettingManager().UpdateWoxSetting(util.NewTraceContext(), kv.Key, kv.Value)
		if updateErr != nil {
			w.Header().Set("code", "500")
			w.Write([]byte(updateErr.Error()))
			return
		}

		writeSuccessResponse(w, "")
	})

	mux.HandleFunc("/setting/plugin/update", func(w http.ResponseWriter, r *http.Request) {
		type keyValuePair struct {
			PluginId string
			Key      string
			Value    string
		}

		decoder := json.NewDecoder(r.Body)
		var kv keyValuePair
		err := decoder.Decode(&kv)
		if err != nil {
			w.Header().Set("code", "500")
			w.Write([]byte(err.Error()))
			return
		}

		pluginInstance, exist := lo.Find(plugin.GetPluginManager().GetPluginInstances(), func(item *plugin.Instance) bool {
			if item.Metadata.Id == kv.PluginId {
				return true
			}
			return false
		})
		if !exist {
			w.Header().Set("code", "500")
			w.Write([]byte("can't find plugin"))
			return
		}

		if kv.Key == "Disabled" {
			pluginInstance.Setting.Disabled = kv.Value == "true"
			pluginInstance.SaveSetting(ctx)
		} else if kv.Key == "TriggerKeywords" {
			pluginInstance.Setting.TriggerKeywords = strings.Split(kv.Value, ",")
			pluginInstance.SaveSetting(ctx)
		} else {
			var isPlatformSpecific = false
			for _, settingDefinition := range pluginInstance.Metadata.SettingDefinitions {
				if settingDefinition.Value.GetKey() == kv.Key {
					isPlatformSpecific = settingDefinition.IsPlatformSpecific
					break
				}
			}
			pluginInstance.API.SaveSetting(util.NewTraceContext(), kv.Key, kv.Value, isPlatformSpecific)
		}

		writeSuccessResponse(w, "")
	})

	mux.HandleFunc("/open/url", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		urlResult := gjson.GetBytes(body, "url")
		if !urlResult.Exists() {
			writeErrorResponse(w, "id is empty")
			return
		}

		util.ShellOpen(urlResult.String())

		writeSuccessResponse(w, "")
	})

	mux.HandleFunc("/backup/now", func(w http.ResponseWriter, r *http.Request) {
		backupErr := setting.GetSettingManager().Backup(util.NewTraceContext(), setting.BackupTypeManual)
		if backupErr != nil {
			writeErrorResponse(w, backupErr.Error())
			return
		}

		writeSuccessResponse(w, "")
	})

	mux.HandleFunc("/backup/restore", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		idResult := gjson.GetBytes(body, "id")
		if !idResult.Exists() {
			writeErrorResponse(w, "id is empty")
			return
		}

		backupId := idResult.String()
		restoreErr := setting.GetSettingManager().Restore(util.NewTraceContext(), backupId)
		if restoreErr != nil {
			writeErrorResponse(w, restoreErr.Error())
			return
		}

		writeSuccessResponse(w, "")
	})

	mux.HandleFunc("/backup/get/all", func(w http.ResponseWriter, r *http.Request) {
		backups, err := setting.GetSettingManager().FindAllBackups(util.NewTraceContext())
		if err != nil {
			writeErrorResponse(w, err.Error())
			return
		}

		writeSuccessResponse(w, backups)
	})

	m.HandleConnect(func(s *melody.Session) {
		if !uiConnected {
			uiConnected = true
			logger.Info(ctx, fmt.Sprintf("ui connected: %s", s.Request.RemoteAddr))

			util.Go(ctx, "post app start", func() {
				time.Sleep(time.Millisecond * 500) // wait for ui to be ready
				GetUIManager().PostAppStart(util.NewTraceContext())
			})
		}
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		ctxNew := util.NewTraceContext()

		logger.Debug(ctxNew, fmt.Sprintf("<UI -> Wox> got request from ui: %s", string(msg)))

		if strings.Contains(string(msg), string(WebsocketMsgTypeRequest)) {
			var request WebsocketMsg
			unmarshalErr := json.Unmarshal(msg, &request)
			if unmarshalErr != nil {
				logger.Error(ctxNew, fmt.Sprintf("failed to unmarshal websocket request: %s", unmarshalErr.Error()))
				return
			}
			util.Go(ctxNew, "handle ui query", func() {
				onUIRequest(ctxNew, request)
			})
		}
	})

	logger.Info(ctx, fmt.Sprintf("websocket server start at：ws://localhost:%d", port))
	handler := cors.Default().Handler(mux)
	err := http.ListenAndServe(fmt.Sprintf("localhost:%d", port), handler)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to start server: %s", err.Error()))
	}
}

func requestUI(ctx context.Context, request WebsocketMsg) {
	request.Type = WebsocketMsgTypeRequest
	request.Success = true
	marshalData, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to marshal websocket request: %s", marshalErr.Error()))
		return
	}

	jsonData, _ := json.Marshal(request.Data)
	util.GetLogger().Info(ctx, fmt.Sprintf("[Wox -> UI] %s: %s", request.Method, jsonData))
	m.Broadcast(marshalData)
}

func responseUI(ctx context.Context, response WebsocketMsg) {
	response.Type = WebsocketMsgTypeResponse
	marshalData, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to marshal websocket response: %s", marshalErr.Error()))
		return
	}
	m.Broadcast(marshalData)
}

func responseUISuccessWithData(ctx context.Context, request WebsocketMsg, data any) {
	responseUI(ctx, WebsocketMsg{
		Id:      request.Id,
		Type:    WebsocketMsgTypeResponse,
		Method:  request.Method,
		Success: true,
		Data:    data,
	})
}

func responseUISuccess(ctx context.Context, request WebsocketMsg) {
	responseUISuccessWithData(ctx, request, nil)
}

func responseUIError(ctx context.Context, request WebsocketMsg, errMsg string) {
	responseUI(ctx, WebsocketMsg{
		Id:      request.Id,
		Type:    WebsocketMsgTypeResponse,
		Method:  request.Method,
		Success: false,
		Data:    errMsg,
	})
}
