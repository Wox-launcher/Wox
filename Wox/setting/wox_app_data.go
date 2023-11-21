package setting

import (
	"context"
	"fmt"
	"wox/share"
	"wox/util"
)

type ResultHash string

type WoxAppData struct {
	QueryHistories  []QueryHistory
	ActionedResults *util.HashMap[ResultHash, []ActionedResult]
}

type QueryHistory struct {
	Query     share.ChangedQuery
	Timestamp int64
}

type ActionedResult struct {
	Timestamp int64
}

func NewResultHash(pluginId string, title, subTitle string) ResultHash {
	return ResultHash(util.Md5([]byte(fmt.Sprintf("%s%s%s", pluginId, title, subTitle))))
}

func GetDefaultWoxAppData(ctx context.Context) WoxAppData {
	return WoxAppData{
		QueryHistories:  []QueryHistory{},
		ActionedResults: util.NewHashMap[ResultHash, []ActionedResult](),
	}
}
