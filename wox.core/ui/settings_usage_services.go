package ui

import (
	"context"

	"wox/ui/contract"
)

// UsageStats returns the report fields consumed by the embedded settings UI.
func (s *CoreServices) UsageStats(ctx context.Context, sessionID string, period string) (contract.UsageStats, error) {
	response, err := getUsageStats(uiServiceContext(ctx, sessionID), period)
	if err != nil {
		return contract.UsageStats{}, err
	}

	result := contract.UsageStats{
		Period:          response.Period,
		PeriodOpened:    response.PeriodOpened,
		PeriodAppLaunch: response.PeriodAppLaunch,
		PeriodAppsUsed:  response.PeriodAppsUsed,
		PeriodActions:   response.PeriodActions,
		UsageDays:       response.UsageDays,
		MostActiveHour:  response.MostActiveHour,
		MostActiveDay:   response.MostActiveDay,
		OpenedByDay:     make([]contract.UsageStatsDay, len(response.OpenedByDay)),
		TopApps:         make([]contract.UsageStatsItem, len(response.TopApps)),
		TopPlugins:      make([]contract.UsageStatsItem, len(response.TopPlugins)),
	}
	for index, day := range response.OpenedByDay {
		result.OpenedByDay[index] = contract.UsageStatsDay{Date: day.Date, Count: day.Count}
	}
	for index, item := range response.TopApps {
		result.TopApps[index] = contract.UsageStatsItem{ID: item.Id, Name: item.Name, Count: item.Count, Icon: item.Icon}
	}
	for index, item := range response.TopPlugins {
		result.TopPlugins[index] = contract.UsageStatsItem{ID: item.Id, Name: item.Name, Count: item.Count, Icon: item.Icon}
	}
	return result, nil
}
