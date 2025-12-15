package plugin

import (
	"context"
	"wox/setting"
	"wox/util"
)

type cacheResult struct {
	match bool
	score int64
}

var pinyinMatchCache = util.NewHashMap[string, cacheResult]()

func IsStringMatch(ctx context.Context, term string, subTerm string) bool {
	IsStringMatchScore, _ := IsStringMatchScore(ctx, term, subTerm)
	return IsStringMatchScore
}

func IsStringMatchScore(ctx context.Context, term string, subTerm string) (bool, int64) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if woxSetting.UsePinYin.Get() {
		key := term + subTerm
		if result, ok := pinyinMatchCache.Load(key); ok {
			return result.match, result.score
		}
	}

	match, score := util.IsStringMatchScore(term, subTerm, woxSetting.UsePinYin.Get())
	if woxSetting.UsePinYin.Get() {
		key := term + subTerm
		pinyinMatchCache.Store(key, cacheResult{match, score})
	}
	return match, score
}

func IsStringMatchScoreNoPinYin(ctx context.Context, term string, subTerm string) (bool, int64) {
	return util.IsStringMatchScore(term, subTerm, false)
}

func IsStringMatchNoPinYin(ctx context.Context, term string, subTerm string) bool {
	match, _ := util.IsStringMatchScore(term, subTerm, false)
	return match
}
