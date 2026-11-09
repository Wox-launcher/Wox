package host

import (
	"context"
	"encoding/json"
	"fmt"
	"wox/plugin"
)

// readOSPermissionOption rejects values outside the public permission enum.
func readOSPermissionOption(request JsonRpcRequest) (plugin.OSPermissionOption, error) {
	var option plugin.OSPermissionOption
	payload, err := requirePluginParam(request.Params, "option")
	if err != nil {
		return option, err
	}
	if err := json.Unmarshal([]byte(payload), &option); err != nil {
		return option, fmt.Errorf("failed to unmarshal OS permission option: %w", err)
	}
	if !option.Permission.IsValid() {
		return option, fmt.Errorf("invalid OS permission %q", option.Permission)
	}
	return option, nil
}

// handleGetOSPermissionStatus routes a passive permission check.
func (w *WebsocketHost) handleGetOSPermissionStatus(ctx context.Context, instance *plugin.Instance, request JsonRpcRequest) (any, error) {
	option, err := readOSPermissionOption(request)
	if err != nil {
		return nil, err
	}
	return instance.API.GetOSPermissionStatus(ctx, option), nil
}

// handleRequestOSPermission routes an explicit permission request.
func (w *WebsocketHost) handleRequestOSPermission(ctx context.Context, instance *plugin.Instance, request JsonRpcRequest) (any, error) {
	option, err := readOSPermissionOption(request)
	if err != nil {
		return nil, err
	}
	return instance.API.RequestOSPermission(ctx, option), nil
}
