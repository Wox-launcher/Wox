package extension

import (
	"context"
	"encoding/json"
	"fmt"

	"wox/common"
	"wox/plugin"
	"wox/util/clipboard"
	"wox/util/selection"
	"wox/util/shell"
)

// answerAPI routes host capabilities through the owning Wox plugin, never another extension's namespace.
func (s *commandSession) answerAPI(message runtimeMessage) {
	s.mu.Lock()
	ctx, query, closed := s.ctx, s.query, s.closed
	s.mu.Unlock()
	if closed {
		return
	}
	var result any
	var err error
	if message.Method == "confirmAlert" {
		result, err = s.owner.confirmAlert(ctx, s, message.Params)
	} else {
		result, err = s.owner.invokeAPI(ctx, query, message.Method, message.Params)
	}
	reply := map[string]any{"type": "apiReply", "id": message.ID, "result": result}
	if err != nil {
		reply["error"] = err.Error()
	}
	_ = s.write(reply)
}

// invokeAPI implements the explicit Wox capability allowlist for Raycast commands.
func (p *extensionPlugin) invokeAPI(ctx context.Context, query plugin.Query, method string, raw json.RawMessage) (any, error) {
	var args struct {
		Text   string `json:"text"`
		Title  string `json:"title"`
		Target string `json:"target"`
		Key    string `json:"key"`
		Value  any    `json:"value"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	switch method {
	case "copy":
		p.api.Copy(ctx, plugin.CopyParams{Type: plugin.CopyTypePlainText, Text: args.Text})
	case "readClipboard":
		return clipboard.ReadText()
	case "open":
		return nil, shell.Open(args.Target)
	case "reveal":
		return nil, shell.OpenFileInFolder(args.Target)
	case "hide":
		p.api.HideApp(ctx)
	case "notify":
		p.api.Notify(ctx, args.Title)
	case "clearSearch":
		p.api.ChangeQuery(ctx, common.PlainQuery{QueryType: plugin.QueryTypeInput, QueryText: p.commandQuery(query, query.Command), QueryRefinements: query.Refinements, ContextData: query.ContextData})
	case "selectedText":
		selected, err := selection.GetSelected(ctx)
		if err != nil {
			return nil, err
		}
		return selected.Text, nil
	case "storageGet", "storageSet", "storageRemove", "storageAll", "storageClear":
		p.storageMu.Lock()
		defer p.storageMu.Unlock()
		items := map[string]any{}
		if stored := p.api.GetSetting(ctx, "raycast:localStorage"); stored != "" {
			if err := json.Unmarshal([]byte(stored), &items); err != nil {
				return nil, fmt.Errorf("Raycast LocalStorage: %w", err)
			}
		}
		switch method {
		case "storageGet":
			return items[args.Key], nil
		case "storageAll":
			return items, nil
		case "storageSet":
			items[args.Key] = args.Value
		case "storageRemove":
			delete(items, args.Key)
		case "storageClear":
			items = map[string]any{}
		}
		data, err := json.Marshal(items)
		if err != nil {
			return nil, err
		}
		if len(data) > 2<<20 {
			return nil, fmt.Errorf("Raycast LocalStorage exceeds 2 MiB")
		}
		result := p.api.SetSetting(ctx, plugin.SetSettingOption{Key: "raycast:localStorage", Value: string(data), IsLocal: true})
		if !result.Success {
			return nil, fmt.Errorf("Raycast LocalStorage: %s", result.ErrMsg)
		}
	default:
		return nil, fmt.Errorf("unsupported Raycast host API: %s", method)
	}
	return nil, nil
}
