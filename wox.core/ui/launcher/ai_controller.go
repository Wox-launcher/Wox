package launcher

import (
	"context"
	"sort"
	"strings"
	"time"

	"wox/ui/contract"
)

// aiSettingsSnapshot is the immutable AI tab state consumed by the view layer.
// It carries the inline-settings form snapshot plus the provider catalog load
// status, the shared model/skill catalogs used by chat preview and plugin/requirement
// forms, and the modal model-manager overlay snapshot.
type aiSettingsSnapshot struct {
	Form             *formFieldsSnapshot
	ProviderCatalog  []aiProviderInfo
	ProvidersLoading bool
	ProvidersLoaded  bool
	ProvidersError   string
	Models           []aiModel
	ModelsLoading    bool
	ModelsLoaded     bool
	ModelsError      string
	Skills           []chatSkill
	SkillsLoading    bool
	SkillsLoaded     bool
	SkillsError      string
	ModelManager     *modelManagerSnapshot
}

// aiSettingsController owns the AI tab state: the inline AI settings form (providers,
// MCP servers, skills), the provider catalog that feeds provider-name dropdowns, the
// shared AI model and skill catalogs consumed by chat preview and by plugin/requirement
// forms with selectAIModel fields, and the modal model-manager overlay used by plugin
// dictation/OCR model fields. The controller is free of any *App back-dependency;
// callers wire App-side side effects through callbacks passed to Reload methods.
type aiSettingsController struct {
	deps CommonDeps

	// Inline form state for the AI settings tab (providers / MCP servers / skills tables).
	form *formFieldsState

	// Provider catalog loaded once from core; feeds the AIProviders name dropdown and
	// the default-host autofill in the shared table row editor.
	providerCatalog  []aiProviderInfo
	providersLoading bool
	providersLoaded  bool
	providersError   string

	// Shared AI model catalog. Loaded on demand by requirement/plugin/chat surfaces that
	// need to populate selectAIModel options or let the user pick a chat model. Cleared
	// whenever AIProviders change so the next access refetches.
	models        []aiModel
	modelsLoading bool
	modelsLoaded  bool
	modelsError   string

	// Shared AI skill catalog. Loaded on demand by chat preview for skill-tag resolution
	// and the skill picker. Cleared whenever AISkills change so the next access refetches.
	skills        []chatSkill
	skillsLoading bool
	skillsLoaded  bool
	skillsError   string

	// Modal model-manager overlay state. Owned here because it is opened from plugin
	// settings and rendered in the settings window, but is a self-contained modal flow.
	modelManager *modelManagerState
}

func newAISettingsController(deps CommonDeps) *aiSettingsController {
	return &aiSettingsController{deps: deps}
}

// SetForm installs the AI settings inline form. Passing nil clears it. The form is
// built by the App from loaded settingsData (definitions + table JSON) and then handed
// to the controller so the view layer can read it through the snapshot. The active
// flag on the form is mutated in place by the App (selectSettingTab) to reflect whether
// the AI tab is currently selected.
func (c *aiSettingsController) SetForm(form *formFieldsState) {
	c.form = form
}

// Form returns the live AI settings form pointer. Callers compare table-editor targets
// against this pointer (formTableTargetCurrentLocked, formTableTargetUsesSettingsLocked)
// and mutate it in place on the UI thread when opening or focusing tables.
func (c *aiSettingsController) Form() *formFieldsState {
	return c.form
}

// ProviderCatalog returns a copy of the cached provider catalog. Used by the shared
// table row editor to autofill the default host when the provider name changes.
func (c *aiSettingsController) ProviderCatalog() []aiProviderInfo {
	return append([]aiProviderInfo(nil), c.providerCatalog...)
}

// ReloadProviders fetches the provider catalog from core. It is a no-op if a reload has
// already completed or is in flight. onLoaded is invoked on a successful load so the
// caller (App) can refresh the AI settings form dropdown and any open row editor without
// the controller needing a back-reference to *App. onLoaded runs in the UI apply transaction.
func (c *aiSettingsController) ReloadProviders(ctx context.Context, service contract.AICatalogSettingsServices, sessionID string, onLoaded func(providers []aiProviderInfo)) {
	shouldLoad := false
	if !c.deps.OnUI("start loading AI provider catalog", func() {
		if c.providersLoading || c.providersLoaded {
			return
		}
		c.providersLoading = true
		c.providersError = ""
		shouldLoad = true
		c.deps.Invalidate()
	}) || !shouldLoad {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	loaded, err := service.AIProviders(timeoutCtx, sessionID)
	cancel()
	providers := make([]aiProviderInfo, len(loaded))
	for index, provider := range loaded {
		providers[index] = aiProviderInfo{
			Name:        provider.Name,
			Icon:        woxImage{ImageType: provider.Icon.ImageType, ImageData: provider.Icon.ImageData},
			DefaultHost: provider.DefaultHost,
		}
	}

	c.deps.OnUI("apply AI provider catalog", func() {
		c.providersLoading = false
		if err != nil {
			c.providersError = err.Error()
		} else {
			c.providerCatalog = providers
			c.providersError = ""
			c.providersLoaded = true
		}
		c.deps.Invalidate()
		if err == nil && onLoaded != nil {
			onLoaded(providers)
		}
	})
}

// SetProviderError records a provider-catalog error from outside the reload path.
func (c *aiSettingsController) SetProviderError(msg string) {
	c.providersError = msg
}

// ResetModels marks the shared model catalog as stale so the next access refetches it
// from core. Called after AIProviders settings change (applyAISettingsRawLocked).
func (c *aiSettingsController) ResetModels() {
	c.modelsLoaded = false
	c.modelsError = ""
}

// ResetSkills marks the shared skill catalog as stale so the next access refetches it
// from core. Called after AISkills settings change (applyAISettingsRawLocked).
func (c *aiSettingsController) ResetSkills() {
	c.skillsLoaded = false
	c.skillsError = ""
}

// Models returns a copy of the shared model catalog. Callers that need indexed access
// should use ModelAt instead so the slice is not mutated underneath them.
func (c *aiSettingsController) Models() []aiModel {
	return append([]aiModel(nil), c.models...)
}

// ModelsCount returns the number of cached models without allocating a copy. Used by
// chat preview panel navigation helpers that only need the count for wraparound math.
func (c *aiSettingsController) ModelsCount() int {
	return len(c.models)
}

// ModelAt returns the model at the given index and false when the index is out of range.
// Used by chat preview's selectChatModel and the panel navigation helpers.
func (c *aiSettingsController) ModelAt(index int) (aiModel, bool) {
	if index < 0 || index >= len(c.models) {
		return aiModel{}, false
	}
	return c.models[index], true
}

// ModelsLoading reports whether a model-catalog load is in flight.
func (c *aiSettingsController) ModelsLoading() bool {
	return c.modelsLoading
}

// ModelsLoaded reports whether the model catalog has been loaded at least once and is
// still considered fresh (not reset by ResetModels).
func (c *aiSettingsController) ModelsLoaded() bool {
	return c.modelsLoaded
}

// ModelsError returns the last model-catalog load error, if any.
func (c *aiSettingsController) ModelsError() string {
	return c.modelsError
}

// SetModelsLoading records that a model-catalog load is starting. Returns the previous
// value so callers can detect a racing load and skip starting a second one.
func (c *aiSettingsController) SetModelsLoading(loading bool) bool {
	previous := c.modelsLoading
	c.modelsLoading = loading
	return previous
}

// SetModels stores the freshly loaded model catalog, marks the catalog as loaded, and
// clears any prior error. Called by App.loadAIModels after the core call returns.
func (c *aiSettingsController) SetModels(models []aiModel) {
	c.models = models
	c.modelsLoading = false
	c.modelsLoaded = true
	c.modelsError = ""
}

// SetModelsError records a model-catalog load failure and clears the in-flight flag.
func (c *aiSettingsController) SetModelsError(msg string) {
	c.modelsLoading = false
	c.modelsLoaded = true
	c.modelsError = msg
}

// Skills returns a copy of the shared skill catalog. Callers that need indexed access
// should use SkillAt instead.
func (c *aiSettingsController) Skills() []chatSkill {
	return append([]chatSkill(nil), c.skills...)
}

// SkillsCount returns the number of cached skills without allocating a copy.
func (c *aiSettingsController) SkillsCount() int {
	return len(c.skills)
}

// SkillAt returns the skill at the given index and false when the index is out of range.
// Used by chat preview's insertChatSkill and the panel navigation helpers.
func (c *aiSettingsController) SkillAt(index int) (chatSkill, bool) {
	if index < 0 || index >= len(c.skills) {
		return chatSkill{}, false
	}
	return c.skills[index], true
}

// SkillsLoading reports whether a skill-catalog load is in flight.
func (c *aiSettingsController) SkillsLoading() bool {
	return c.skillsLoading
}

// SkillsLoaded reports whether the skill catalog has been loaded at least once and is
// still considered fresh.
func (c *aiSettingsController) SkillsLoaded() bool {
	return c.skillsLoaded
}

// SkillsError returns the last skill-catalog load error, if any.
func (c *aiSettingsController) SkillsError() string {
	return c.skillsError
}

// SetSkillsLoading records that a skill-catalog load is starting. Returns the previous
// value so callers can detect a racing load.
func (c *aiSettingsController) SetSkillsLoading(loading bool) bool {
	previous := c.skillsLoading
	c.skillsLoading = loading
	return previous
}

// SetSkills stores the freshly loaded skill catalog, marks it loaded, and clears any
// prior error. Called by App.loadAISkills after the core call returns.
func (c *aiSettingsController) SetSkills(skills []chatSkill) {
	c.skills = skills
	c.skillsLoading = false
	c.skillsLoaded = true
	c.skillsError = ""
}

// SetSkillsError records a skill-catalog load failure and clears the in-flight flag.
func (c *aiSettingsController) SetSkillsError(msg string) {
	c.skillsLoading = false
	c.skillsLoaded = true
	c.skillsError = msg
}

// LoadAIModels fetches the core model catalog, sorts it, and stores it through SetModels.
// onLoaded is invoked on success so the App can refresh the requirement/plugin/table
// row forms that consume selectAIModel options and reset the chat-preview panel selection.
// onLoaded runs in the UI apply transaction and receives the freshly loaded models.
func (c *aiSettingsController) LoadAIModels(ctx context.Context, service contract.AICatalogSettingsServices, sessionID string, onLoaded func(models []aiModel)) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	loaded, err := service.AIModels(timeoutCtx, sessionID)
	cancel()
	models := make([]aiModel, len(loaded))
	for index, model := range loaded {
		models[index] = aiModel{Name: model.Name, Provider: model.Provider, ProviderAlias: model.ProviderAlias}
	}
	if err == nil {
		sort.Slice(models, func(i, j int) bool {
			left := models[i].Provider + "\x00" + models[i].ProviderAlias + "\x00" + models[i].Name
			right := models[j].Provider + "\x00" + models[j].ProviderAlias + "\x00" + models[j].Name
			return left < right
		})
	}
	c.deps.OnUI("apply AI model catalog", func() {
		if err == nil {
			c.SetModels(models)
		} else {
			c.SetModelsError(err.Error())
		}
		if onLoaded != nil {
			if err != nil {
				onLoaded(nil)
			} else {
				onLoaded(models)
			}
		}
	})
}

// LoadAISkills fetches the enabled skill catalog from core, filters out disabled/empty
// skills, sorts by source/name, and stores it through SetSkills. onLoaded is invoked on
// success so the App can reset the chat-preview skill panel selection.
func (c *aiSettingsController) LoadAISkills(ctx context.Context, service contract.AICatalogSettingsServices, sessionID string, onLoaded func(skills []chatSkill)) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	loaded, err := service.AISkills(timeoutCtx, sessionID)
	cancel()
	skills := make([]chatSkill, len(loaded))
	for index, skill := range loaded {
		skills[index] = chatSkill{
			ID: skill.ID, Name: skill.Name, Description: skill.Description, Path: skill.Path, ManifestPath: skill.ManifestPath,
			Source: skill.Source, SourceName: skill.SourceName, Error: skill.Error, Enabled: skill.Enabled,
		}
	}
	if err == nil {
		filtered := make([]chatSkill, 0, len(skills))
		for _, skill := range skills {
			if !skill.Enabled || strings.TrimSpace(skill.Name) == "" {
				continue
			}
			filtered = append(filtered, skill)
		}
		sort.SliceStable(filtered, func(i, j int) bool {
			left := filtered[i].SourceName + "\x00" + filtered[i].Source + "\x00" + filtered[i].Name
			right := filtered[j].SourceName + "\x00" + filtered[j].Source + "\x00" + filtered[j].Name
			return left < right
		})
		skills = filtered
	}
	c.deps.OnUI("apply AI skill catalog", func() {
		if err == nil {
			c.SetSkills(skills)
		} else {
			c.SetSkillsError(err.Error())
		}
		if onLoaded != nil {
			if err != nil {
				onLoaded(nil)
			} else {
				onLoaded(skills)
			}
		}
	})
}

// ModelManager returns the live UI-owned model-manager overlay state; Snapshot copies it for the view layer.
func (c *aiSettingsController) ModelManager() *modelManagerState {
	return c.modelManager
}

// SetModelManager installs (or clears, when nil) the model-manager overlay state.
func (c *aiSettingsController) SetModelManager(state *modelManagerState) {
	c.modelManager = state
}

// Snapshot returns a copy of the AI state for the view layer.
func (c *aiSettingsController) Snapshot() aiSettingsSnapshot {
	var form *formFieldsSnapshot
	if c.form != nil {
		snapshot := snapshotFormFieldsLocked(c.form)
		form = &snapshot
	}
	return aiSettingsSnapshot{
		Form:             form,
		ProviderCatalog:  append([]aiProviderInfo(nil), c.providerCatalog...),
		ProvidersLoading: c.providersLoading,
		ProvidersLoaded:  c.providersLoaded,
		ProvidersError:   c.providersError,
		Models:           append([]aiModel(nil), c.models...),
		ModelsLoading:    c.modelsLoading,
		ModelsLoaded:     c.modelsLoaded,
		ModelsError:      c.modelsError,
		Skills:           append([]chatSkill(nil), c.skills...),
		SkillsLoading:    c.skillsLoading,
		SkillsLoaded:     c.skillsLoaded,
		SkillsError:      c.skillsError,
		ModelManager:     snapshotModelManagerLocked(c.modelManager),
	}
}
