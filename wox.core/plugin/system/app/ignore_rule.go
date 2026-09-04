package app

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"wox/plugin"
	"wox/util"
)

const ignoreRulesSettingKey = "IgnoreRules"

// appIgnoreRule is one IgnoreRules row.
// IncludeFuture hides every current and future app that matches Pattern.
// Otherwise only Apps are hidden, and Pattern is just the editor search.
type appIgnoreRule struct {
	Pattern       string       `json:"Pattern,omitempty"`
	IncludeFuture bool         `json:"IncludeFuture"`
	Apps          []ignoredApp `json:"Apps,omitempty"`
}

type appIgnoreMatcher struct {
	pattern string
	regex   *regexp.Regexp
}

func parseIgnoreRules(value string) ([]appIgnoreRule, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	// Persistence is rewritten by migration/m20260904_app_ignore_rules_include_future.go.
	// Keep an in-memory fallback so unread legacy JSON still behaves as dynamic rules.
	if ignoreRulesNeedMigration(value) {
		return migrateLegacyIgnoreRules(value)
	}

	var rules []appIgnoreRule
	if err := json.Unmarshal([]byte(value), &rules); err != nil {
		return nil, err
	}
	return normalizeAppIgnoreRules(rules), nil
}

// ignoreRulesNeedMigration reports whether any object is pre-IncludeFuture JSON.
func ignoreRulesNeedMigration(value string) bool {
	var objects []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(value), &objects); err != nil {
		return false
	}
	for _, object := range objects {
		if _, ok := object["IncludeFuture"]; !ok {
			return true
		}
	}
	return false
}

// migrateLegacyIgnoreRules turns old Pattern-only rows into dynamic rules.
func migrateLegacyIgnoreRules(value string) ([]appIgnoreRule, error) {
	var objects []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(value), &objects); err != nil {
		return nil, err
	}

	rules := make([]appIgnoreRule, 0, len(objects))
	for _, object := range objects {
		encoded, err := json.Marshal(object)
		if err != nil {
			continue
		}
		var rule appIgnoreRule
		if err := json.Unmarshal(encoded, &rule); err != nil {
			continue
		}
		if _, ok := object["IncludeFuture"]; !ok {
			rule.IncludeFuture = true
			rule.Apps = nil
		}
		rules = append(rules, rule)
	}
	return normalizeAppIgnoreRules(rules), nil
}

func normalizeAppIgnoreRules(rules []appIgnoreRule) []appIgnoreRule {
	normalized := make([]appIgnoreRule, 0, len(rules))
	seenPatterns := make(map[string]bool)

	for _, rule := range rules {
		rule.Pattern = strings.TrimSpace(rule.Pattern)
		if rule.IncludeFuture {
			if rule.Pattern == "" || seenPatterns[strings.ToLower(rule.Pattern)] {
				continue
			}
			seenPatterns[strings.ToLower(rule.Pattern)] = true
			rule.Apps = nil
			normalized = append(normalized, rule)
			continue
		}

		apps := normalizeIgnoreRuleApps(rule.Apps)
		if len(apps) == 0 {
			continue
		}
		if rule.Pattern == "" {
			rule.Pattern = apps[0].Name
		}
		rule.Apps = apps
		normalized = append(normalized, rule)
	}

	return normalized
}

// normalizeIgnoreRuleApps drops invalid entries and dedupes by path or identity.
func normalizeIgnoreRuleApps(apps []ignoredApp) []ignoredApp {
	normalized := make([]ignoredApp, 0, len(apps))
	seen := make(map[string]bool)
	for index := range apps {
		app, ok := normalizeIgnoreRuleApp(&apps[index])
		if !ok {
			continue
		}
		key := ignoredAppMatchKey(app.Path, app.Identity)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, app)
	}
	return normalized
}

func normalizeIgnoreRuleApp(app *ignoredApp) (ignoredApp, bool) {
	if app == nil {
		return ignoredApp{}, false
	}

	normalized := ignoredApp{
		Name:     strings.TrimSpace(app.Name),
		Identity: strings.TrimSpace(app.Identity),
		Path:     strings.TrimSpace(app.Path),
		Icon:     app.Icon,
	}
	if normalized.Name == "" && normalized.Identity == "" && normalized.Path == "" {
		return ignoredApp{}, false
	}
	if ignoredAppMatchKey(normalized.Path, normalized.Identity) == "" {
		return ignoredApp{}, false
	}
	return normalized, true
}

func compileAppIgnorePattern(pattern string) (*regexp.Regexp, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, fmt.Errorf("empty ignore pattern")
	}
	if !strings.Contains(pattern, "*") {
		pattern = "*" + pattern + "*"
	}
	escaped := regexp.QuoteMeta(pattern)
	escaped = strings.ReplaceAll(escaped, "\\*", ".*")
	return regexp.Compile("(?i)^" + escaped + "$")
}

// AppMatchesIgnorePattern reports whether any candidate matches a wildcard ignore rule.
func AppMatchesIgnorePattern(pattern string, candidates ...string) bool {
	compiled, err := compileAppIgnorePattern(pattern)
	if err != nil {
		return false
	}
	for _, candidate := range candidates {
		if candidate = strings.TrimSpace(candidate); candidate != "" && compiled.MatchString(candidate) {
			return true
		}
	}
	return false
}

func splitIgnoreRules(rules []appIgnoreRule) ([]appIgnoreMatcher, []ignoredApp) {
	normalized := normalizeAppIgnoreRules(rules)
	matchers := make([]appIgnoreMatcher, 0, len(normalized))
	apps := make([]ignoredApp, 0, len(normalized))
	for _, rule := range normalized {
		if !rule.IncludeFuture {
			apps = append(apps, rule.Apps...)
			continue
		}
		compiled, err := compileAppIgnorePattern(rule.Pattern)
		if err != nil {
			continue
		}
		matchers = append(matchers, appIgnoreMatcher{
			pattern: rule.Pattern,
			regex:   compiled,
		})
	}
	return matchers, apps
}

func (a *ApplicationPlugin) rebuildIgnoreRuleMatchers(ctx context.Context) {
	rawRules := strings.TrimSpace(a.api.GetSetting(ctx, ignoreRulesSettingKey))
	if rawRules == "" {
		a.queryEntriesMutex.Lock()
		a.ignoreMatchers = nil
		a.ignoredApps = nil
		a.queryEntriesMutex.Unlock()
		return
	}

	rules, err := parseIgnoreRules(rawRules)
	if err != nil {
		a.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to parse %s: %s", ignoreRulesSettingKey, err.Error()))
		return
	}

	matchers, apps := splitIgnoreRules(rules)
	a.queryEntriesMutex.Lock()
	a.ignoreMatchers = matchers
	a.ignoredApps = apps
	a.queryEntriesMutex.Unlock()
}

func (a *ApplicationPlugin) getIgnoreRuleMatchersSnapshot() []appIgnoreMatcher {
	a.queryEntriesMutex.RLock()
	matchers := a.ignoreMatchers
	a.queryEntriesMutex.RUnlock()
	return matchers
}

func buildIgnoreRuleCandidates(info appInfo, displayName string) []string {
	candidates := info.GetSearchCandidates(displayName)
	candidates = append(candidates, strings.TrimSpace(info.Path), strings.TrimSpace(info.Identity))

	filtered := make([]string, 0, len(candidates))
	for _, candidate := range util.UniqueStrings(candidates) {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		filtered = append(filtered, candidate)
	}

	return filtered
}

func (a *ApplicationPlugin) matchIgnoreRuleCandidates(candidates []string, matchers []appIgnoreMatcher) (string, bool) {
	if len(matchers) == 0 {
		return "", false
	}

	for _, candidate := range candidates {
		for _, matcher := range matchers {
			if matcher.regex.MatchString(candidate) {
				return matcher.pattern, true
			}
		}
	}

	return "", false
}
