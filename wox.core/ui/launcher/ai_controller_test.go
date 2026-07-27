package launcher

import (
	"context"
	"errors"
	"testing"

	"wox/ui/contract"
)

type aiFakeService struct {
	providers []contract.AIProvider
	models    []contract.AIModel
	skills    []contract.AISkill
	err       error
}

func (f *aiFakeService) AIProviders(_ context.Context, _ string) ([]contract.AIProvider, error) {
	return append([]contract.AIProvider(nil), f.providers...), f.err
}

func (f *aiFakeService) AIModels(_ context.Context, _ string) ([]contract.AIModel, error) {
	return append([]contract.AIModel(nil), f.models...), f.err
}

func (f *aiFakeService) AISkills(_ context.Context, _ string) ([]contract.AISkill, error) {
	return append([]contract.AISkill(nil), f.skills...), f.err
}

func newAIDeps() (CommonDeps, *int) {
	calls := 0
	deps := CommonDeps{
		Invalidate: func() { calls++ },
		Translate:  func(s string) string { return s },
	}
	return deps, &calls
}

func TestAIControllerReloadProvidersSuccess(t *testing.T) {
	deps, invalidateCalls := newAIDeps()
	c := newAISettingsController(deps)
	service := &aiFakeService{providers: []contract.AIProvider{{Name: "OpenAI", DefaultHost: "api.openai.com"}}}
	loaded := false
	c.ReloadProviders(context.Background(), service, "session", func(providers []aiProviderInfo) {
		if len(providers) != 1 || providers[0].Name != "OpenAI" {
			t.Fatalf("unexpected providers in onLoaded: %+v", providers)
		}
		loaded = true
	})
	snap := c.Snapshot()
	if !snap.ProvidersLoaded || snap.ProvidersError != "" || len(snap.ProviderCatalog) != 1 {
		t.Fatalf("after reload: %+v", snap)
	}
	if !loaded {
		t.Fatalf("onLoaded callback should have been invoked")
	}
	if *invalidateCalls < 2 {
		t.Fatalf("Invalidate should be called at least twice, got %d", *invalidateCalls)
	}
}

func TestAIControllerReloadProvidersError(t *testing.T) {
	deps, _ := newAIDeps()
	c := newAISettingsController(deps)
	service := &aiFakeService{err: errors.New("network down")}
	c.ReloadProviders(context.Background(), service, "session", nil)
	snap := c.Snapshot()
	if snap.ProvidersLoaded || snap.ProvidersError == "" {
		t.Fatalf("error should be recorded: %+v", snap)
	}
}

func TestAIControllerLoadModelsSortsTypedCatalog(t *testing.T) {
	deps, _ := newAIDeps()
	c := newAISettingsController(deps)
	service := &aiFakeService{models: []contract.AIModel{
		{Name: "z-model", Provider: "OpenAI"},
		{Name: "a-model", Provider: "Anthropic"},
	}}
	c.LoadAIModels(context.Background(), service, "session", nil)
	models := c.Models()
	if len(models) != 2 || models[0].Provider != "Anthropic" || models[1].Provider != "OpenAI" {
		t.Fatalf("models should be sorted by provider and name: %+v", models)
	}
}

func TestAIControllerLoadSkillsFiltersAndSortsTypedCatalog(t *testing.T) {
	deps, _ := newAIDeps()
	c := newAISettingsController(deps)
	service := &aiFakeService{skills: []contract.AISkill{
		{ID: "disabled", Name: "Disabled", Source: "user", Enabled: false},
		{ID: "b", Name: "Beta", Source: "user", SourceName: "User", Enabled: true},
		{ID: "a", Name: "Alpha", Source: "builtin", SourceName: "Built-in", Enabled: true},
	}}
	c.LoadAISkills(context.Background(), service, "session", nil)
	skills := c.Skills()
	if len(skills) != 2 || skills[0].ID != "a" || skills[1].ID != "b" {
		t.Fatalf("skills should filter disabled entries and sort by source/name: %+v", skills)
	}
}

func TestAIControllerSetModelsAndGetModels(t *testing.T) {
	deps, _ := newAIDeps()
	c := newAISettingsController(deps)
	models := []aiModel{{Name: "gpt-4", Provider: "OpenAI"}, {Name: "claude", Provider: "Anthropic"}}
	c.SetModels(models)
	got := c.Models()
	if len(got) != 2 {
		t.Fatalf("Models() should return 2 entries, got %d", len(got))
	}
	first, ok := c.ModelAt(0)
	if !ok || first.Name != "gpt-4" {
		t.Fatalf("ModelAt(0) should return gpt-4, got %+v ok=%v", first, ok)
	}
	if _, ok := c.ModelAt(99); ok {
		t.Fatalf("ModelAt(99) should return false for out-of-range index")
	}
	if !c.ModelsLoaded() {
		t.Fatalf("ModelsLoaded should be true after SetModels")
	}
	if c.ModelsError() != "" {
		t.Fatalf("ModelsError should be empty after SetModels, got %q", c.ModelsError())
	}
}

func TestAIControllerSetSkillsAndGetSkills(t *testing.T) {
	deps, _ := newAIDeps()
	c := newAISettingsController(deps)
	skills := []chatSkill{{ID: "s1", Name: "Summarize"}, {ID: "s2", Name: "Translate"}}
	c.SetSkills(skills)
	got := c.Skills()
	if len(got) != 2 {
		t.Fatalf("Skills() should return 2 entries, got %d", len(got))
	}
	first, ok := c.SkillAt(0)
	if !ok || first.ID != "s1" {
		t.Fatalf("SkillAt(0) should return s1, got %+v ok=%v", first, ok)
	}
	if _, ok := c.SkillAt(99); ok {
		t.Fatalf("SkillAt(99) should return false for out-of-range index")
	}
	if !c.SkillsLoaded() {
		t.Fatalf("SkillsLoaded should be true after SetSkills")
	}
}

func TestAIControllerModelsLoadingState(t *testing.T) {
	deps, _ := newAIDeps()
	c := newAISettingsController(deps)
	if c.ModelsLoading() {
		t.Fatalf("ModelsLoading should be false initially")
	}
	c.SetModelsLoading(true)
	if !c.ModelsLoading() {
		t.Fatalf("ModelsLoading should be true after SetModelsLoading(true)")
	}
	c.SetModelsLoading(false)
	if c.ModelsLoading() {
		t.Fatalf("ModelsLoading should be false after SetModelsLoading(false)")
	}
}
