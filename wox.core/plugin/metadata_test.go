package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFeatureParamsForGridLayoutParsesJSONNumbers(t *testing.T) {
	metadata := Metadata{
		Features: []MetadataFeature{
			{
				Name: MetadataFeatureGridLayout,
				Params: map[string]any{
					"Columns":     12.0,
					"ShowTitle":   false,
					"ItemPadding": 12.0,
					"ItemMargin":  6.0,
					"AspectRatio": 1.7777778,
				},
			},
		},
	}

	params, err := metadata.GetFeatureParamsForGridLayout()

	require.NoError(t, err)
	assert.Equal(t, 12, params.Columns)
	assert.False(t, params.ShowTitle)
	assert.Equal(t, 12, params.ItemPadding)
	assert.Equal(t, 6, params.ItemMargin)
	assert.Equal(t, 1.7777778, params.AspectRatio)
}

func TestGetFeatureParamsForGridLayoutUsesOutlineFriendlyDefaults(t *testing.T) {
	metadata := Metadata{
		Features: []MetadataFeature{
			{
				Name:   MetadataFeatureGridLayout,
				Params: map[string]any{},
			},
		},
	}

	params, err := metadata.GetFeatureParamsForGridLayout()

	require.NoError(t, err)
	assert.Equal(t, 8, params.Columns)
	// Behavior change: the default grid highlight is now an outline, so missing
	// ItemPadding should not preserve the old filled-background spacing.
	assert.Equal(t, 0, params.ItemPadding)
	assert.Equal(t, 6, params.ItemMargin)
	assert.Equal(t, 1.0, params.AspectRatio)
}

func TestGetFeatureParamsForDebounceParsesJSONNumbers(t *testing.T) {
	metadata := Metadata{
		Features: []MetadataFeature{
			{
				Name: MetadataFeatureDebounce,
				Params: map[string]any{
					"IntervalMs": 200.0,
				},
			},
		},
	}

	params, err := metadata.GetFeatureParamsForDebounce()

	require.NoError(t, err)
	assert.Equal(t, 200, params.IntervalMs)
}

func TestValidateGlancesRejectsDuplicateIds(t *testing.T) {
	metadata := Metadata{
		Glances: []MetadataGlance{
			{Id: "time", Name: "Time"},
			{Id: "time", Name: "Duplicate Time"},
		},
	}

	err := metadata.ValidateGlances()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate glance id")
}

func TestValidateGlancesAcceptsPluginLocalIds(t *testing.T) {
	metadata := Metadata{
		Glances: []MetadataGlance{
			{Id: "time", Name: "Time", RefreshIntervalMs: 60000},
			{Id: "battery", Name: "Battery", RefreshIntervalMs: 60000},
		},
	}

	err := metadata.ValidateGlances()

	require.NoError(t, err)
}
