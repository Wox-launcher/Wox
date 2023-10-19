package system

import (
	"context"
	"wox/setting"
	"wox/util"
)

func IsStringMatchScore(ctx context.Context, term string, subTerm string) (bool, int) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	return util.IsStringMatchScore(term, subTerm, woxSetting.UsePinYin)
}

func IsStringMatchScoreNoPinYin(ctx context.Context, term string, subTerm string) (bool, int) {
	return util.IsStringMatchScore(term, subTerm, false)
}

func IsStringMatchNoPinYin(ctx context.Context, term string, subTerm string) bool {
	match, _ := util.IsStringMatchScore(term, subTerm, false)
	return match
}
