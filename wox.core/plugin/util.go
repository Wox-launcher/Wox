package plugin

import (
	"context"
	"wox/setting"
	"wox/util"
)

func IsStringMatch(ctx context.Context, term string, subTerm string) bool {
	IsStringMatchScore, _ := IsStringMatchScore(ctx, term, subTerm)
	return IsStringMatchScore
}

func IsStringMatchScore(ctx context.Context, term string, subTerm string) (bool, int64) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	return util.IsStringMatchScore(term, subTerm, woxSetting.UsePinYin.Get())
}

func IsStringMatchScoreNoPinYin(ctx context.Context, term string, subTerm string) (bool, int64) {
	return util.IsStringMatchScore(term, subTerm, false)
}

func IsStringMatchNoPinYin(ctx context.Context, term string, subTerm string) bool {
	match, _ := util.IsStringMatchScore(term, subTerm, false)
	return match
}
