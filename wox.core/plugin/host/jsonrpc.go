package host

type JsonRpcType string

const (
	JsonRpcTypeRequest   JsonRpcType = "WOX_JSONRPC_REQUEST"
	JsonRpcTypeResponse  JsonRpcType = "WOX_JSONRPC_RESPONSE"
	JsonRpcTypeSystemLog JsonRpcType = "WOX_JSONRPC_SYSTEM_LOG"
)

type JsonRpcRequest struct {
	TraceId    string
	Id         string
	PluginId   string
	PluginName string
	Method     string
	Type       JsonRpcType
	Params     map[string]string
}

type JsonRpcResponse struct {
	TraceId string
	Id      string
	Method  string
	Type    JsonRpcType
	Result  any
	Error   string
}
