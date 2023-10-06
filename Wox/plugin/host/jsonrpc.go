package host

type JsonRpcType string

const (
	JsonRpcTypeRequest  JsonRpcType = "WOX_JSONRPC_REQUEST"
	JsonRpcTypeResponse JsonRpcType = "WOX_JSONRPC_RESPONSE"
)

type JsonRpcRequest struct {
	Id         string
	PluginId   string
	PluginName string
	Method     string
	Type       JsonRpcType
	Params     map[string]string
}

type JsonRpcResponse struct {
	Id     string
	Method string
	Type   JsonRpcType
	Result string
	Error  string
}
