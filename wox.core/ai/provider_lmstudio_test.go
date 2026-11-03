package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"wox/common"
	"wox/common/icons"
	"wox/setting"
)

func TestLMStudioProviderModelsAndCredentials(t *testing.T) {
	ctx := context.Background()
	defaultProvider, err := NewProvider(ctx, setting.AIProvider{Name: common.ProviderNameLMStudio})
	if err != nil {
		t.Fatal(err)
	}
	if defaultProvider.GetDefaultHost() != "http://localhost:1234/v1" {
		t.Fatalf("default host = %q", defaultProvider.GetDefaultHost())
	}
	if defaultProvider.GetIcon().ImageData != icons.Get(icons.BrandLMStudio).ImageData {
		t.Fatal("LM Studio provider must use its brand icon")
	}

	for _, key := range []string{"", "secret"} {
		t.Run(fmt.Sprintf("key_set=%t", key != ""), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/models" {
					t.Errorf("path = %q, want /v1/models", r.URL.Path)
				}
				if key != "" && r.Header.Get("Authorization") != "Bearer "+key {
					t.Error("configured API key was not sent")
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"object":"list","data":[{"id":"local-model","object":"model","created":0,"owned_by":"local"}]}`)
			}))
			defer server.Close()
			provider, err := NewProvider(ctx, setting.AIProvider{Name: common.ProviderNameLMStudio, Host: server.URL + "/v1", ApiKey: key, Alias: "local"})
			if err != nil {
				t.Fatal(err)
			}
			models, err := provider.Models(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(models) != 1 || models[0].Name != "local-model" || models[0].Provider != common.ProviderNameLMStudio || models[0].ProviderAlias != "local" {
				t.Fatalf("models = %+v", models)
			}
			if err := provider.Ping(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
}
