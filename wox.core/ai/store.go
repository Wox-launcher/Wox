package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"wox/i18n"
	"wox/util"

	"github.com/samber/lo"
)

type storeManifest struct {
	Name string
	Url  string
}

type StoreAICommandQueryHotkey struct {
	Hotkey            string
	HideQueryBox      bool
	HideToolbar       bool
	IsSilentExecution bool
	Width             int
	MaxResultCount    int
	Position          string
}

type StoreAICommandCategory struct {
	Id   string
	Name string
	I18n map[string]map[string]string `json:",omitempty"`
}

type StoreAICommandCatalog struct {
	Categories []StoreAICommandCategory `json:"categories"`
	Templates  []StoreAICommandManifest
}

type StoreAICommandManifest struct {
	Id                     string
	Category               string
	CategoryId             string
	Name                   string
	Description            string
	Author                 string
	Command                string
	Prompt                 string
	ThinkingMode           string
	DefaultAction          string
	Vision                 bool
	RecommendedQueryHotkey StoreAICommandQueryHotkey
	I18n                   map[string]map[string]string `json:",omitempty"`
}

var storeInstance *Store
var storeOnce sync.Once

type Store struct {
	commands   []StoreAICommandManifest
	categories map[string]StoreAICommandCategory
}

func GetStoreManager() *Store {
	storeOnce.Do(func() {
		storeInstance = &Store{}
	})
	return storeInstance
}

func (s *Store) Start(ctx context.Context) {
	s.commands = s.GetStoreAICommandManifests(ctx)

	util.Go(ctx, "load ai command templates", func() {
		for range time.NewTicker(time.Minute * 10).C {
			commands := s.GetStoreAICommandManifests(util.NewTraceContext())
			if len(commands) > 0 {
				s.commands = commands
			}
		}
	})
}

func (s *Store) getStoreManifests(ctx context.Context) []storeManifest {
	return []storeManifest{
		{
			Name: "Wox Official AI Command Store",
			Url:  "https://raw.githubusercontent.com/Wox-launcher/Wox/master/store-ai-command.json",
		},
	}
}

func (s *Store) GetStoreAICommandManifests(ctx context.Context) []StoreAICommandManifest {
	var storeAICommandManifests []StoreAICommandManifest
	categories := map[string]StoreAICommandCategory{}

	for _, store := range s.getStoreManifests(ctx) {
		aiCommandCatalog, manifestErr := s.GetStoreAICommandCatalog(ctx, store)
		if manifestErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to get ai command manifest from %s store: %s", store.Name, manifestErr.Error()))
			continue
		}

		for _, category := range aiCommandCatalog.Categories {
			if category.Id == "" {
				continue
			}
			if _, found := categories[category.Id]; !found {
				categories[category.Id] = category
			}
		}

		for _, manifest := range aiCommandCatalog.Templates {
			_, found := lo.Find(storeAICommandManifests, func(m StoreAICommandManifest) bool {
				return manifest.Id == m.Id
			})
			if found {
				continue
			}

			storeAICommandManifests = append(storeAICommandManifests, manifest)
		}
	}

	s.categories = categories
	util.GetLogger().Info(ctx, fmt.Sprintf("found %d ai commands from stores", len(storeAICommandManifests)))
	return storeAICommandManifests
}

func (s *Store) GetStoreAICommandCatalog(ctx context.Context, store storeManifest) (StoreAICommandCatalog, error) {
	util.GetLogger().Info(ctx, fmt.Sprintf("start to get ai command manifest from %s(%s)", store.Name, store.Url))

	response, getErr := util.HttpGet(ctx, store.Url)
	if getErr != nil {
		return StoreAICommandCatalog{}, getErr
	}

	var catalog StoreAICommandCatalog
	catalogErr := json.Unmarshal(response, &catalog)
	if catalogErr == nil && (catalog.Categories != nil || catalog.Templates != nil) {
		return catalog, nil
	}

	var legacyTemplates []StoreAICommandManifest
	legacyErr := json.Unmarshal(response, &legacyTemplates)
	if legacyErr != nil {
		if catalogErr != nil {
			return StoreAICommandCatalog{}, catalogErr
		}
		return StoreAICommandCatalog{}, legacyErr
	}

	return StoreAICommandCatalog{Templates: legacyTemplates}, nil
}

func translateStoreText(text string, translations map[string]map[string]string) string {
	if !strings.HasPrefix(text, "i18n:") {
		return text
	}

	key := strings.TrimPrefix(text, "i18n:")
	langCode := string(i18n.GetI18nManager().GetCurrentLangCode())
	if langMap, ok := translations[langCode]; ok {
		if translated, ok := langMap[key]; ok {
			return translated
		}
	}

	if langCode != string(i18n.LangCodeEnUs) {
		if langMap, ok := translations[string(i18n.LangCodeEnUs)]; ok {
			if translated, ok := langMap[key]; ok {
				return translated
			}
		}
	}

	return text
}

func (m StoreAICommandManifest) translated(categories map[string]StoreAICommandCategory) StoreAICommandManifest {
	if m.CategoryId != "" {
		if category, ok := categories[m.CategoryId]; ok {
			m.Category = translateStoreText(category.Name, category.I18n)
		} else {
			m.Category = m.CategoryId
		}
	} else {
		m.Category = translateStoreText(m.Category, m.I18n)
	}
	m.Name = translateStoreText(m.Name, m.I18n)
	m.Description = translateStoreText(m.Description, m.I18n)
	m.Prompt = translateStoreText(m.Prompt, m.I18n)
	m.I18n = nil
	return m
}

func (s *Store) GetCommands(ctx context.Context) []StoreAICommandManifest {
	if len(s.commands) == 0 {
		s.commands = s.GetStoreAICommandManifests(ctx)
	}

	translatedCommands := make([]StoreAICommandManifest, 0, len(s.commands))
	for _, command := range s.commands {
		translatedCommands = append(translatedCommands, command.translated(s.categories))
	}
	return translatedCommands
}
