package plugin

import (
	"context"
	"wox/setting"
	"wox/util/fuzzymatch"
)

func IsStringMatch(ctx context.Context, term string, subTerm string) bool {
	IsStringMatchScore, _ := IsStringMatchScore(ctx, term, subTerm)
	return IsStringMatchScore
}

func IsStringMatchScore(ctx context.Context, term string, subTerm string) (bool, int64) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	result := fuzzymatch.FuzzyMatch(term, subTerm, woxSetting.UsePinYin.Get())
	return result.IsMatch, result.Score
}

func IsStringMatchScoreNoPinYin(ctx context.Context, term string, subTerm string) (bool, int64) {
	result := fuzzymatch.FuzzyMatch(term, subTerm, false)
	return result.IsMatch, result.Score
}

func IsStringMatchNoPinYin(ctx context.Context, term string, subTerm string) bool {
	result := fuzzymatch.FuzzyMatch(term, subTerm, false)
	return result.IsMatch
}
