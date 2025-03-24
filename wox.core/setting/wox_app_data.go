package setting

import (
	"context"
	"fmt"
	"wox/entity"
	"wox/util"
)

type ResultHash string

type WoxAppData struct {
	QueryHistories  []QueryHistory
	ActionedResults *util.HashMap[ResultHash, []ActionedResult]
	FavoriteResults *util.HashMap[ResultHash, bool]
}

type QueryHistory struct {
	Query     entity.PlainQuery
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
		FavoriteResults: util.NewHashMap[ResultHash, bool](),
	}
}
