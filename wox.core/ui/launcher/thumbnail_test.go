package launcher

import (
	"encoding/json"
	"testing"
	"wox/plugin"
)

func TestThumbnailPresentationSurvivesCoreConversion(t *testing.T) {
	for _, thumbnail := range []bool{false, true} {
		result := plugin.QueryResult{IconShowContainer: thumbnail}
		payload, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		var decoded plugin.QueryResult
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.IconShowContainer != thumbnail {
			t.Fatal("public icon container flag did not survive JSON transport")
		}
		if fromCoreQueryResult(decoded.ToUI()).IconShowContainer != thumbnail {
			t.Fatal("thumbnail presentation was lost between plugin and launcher")
		}
	}
}
