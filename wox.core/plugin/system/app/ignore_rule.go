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

type appIgnoreRule struct {
	Pattern string `json:"Pattern"`
}

type appIgnoreMatcher struct {
	pattern string
	regex   *regexp.Regexp
}

func normalizeAppIgnoreRules(rules []appIgnoreRule) []appIgnoreRule {
	normalized := make([]appIgnoreRule, 0, len(rules))
	seen := make(map[string]bool)

	for _, rule := range rules {
		rule.Pattern = strings.TrimSpace(rule.Pattern)
		if rule.Pattern == "" {
			continue
		}

		key := strings.ToLower(rule.Pattern)
		if seen[key] {
			continue
		}

		seen[key] = true
		normalized = append(normalized, rule)
	}

	return normalized
}

func compileAppIgnorePattern(pattern string) (*regexp.Regexp, error) {
	escaped := regexp.QuoteMeta(strings.TrimSpace(pattern))
	escaped = strings.ReplaceAll(escaped, "\\*", ".*")
	return regexp.Compile("(?i)^" + escaped + "$")
}

func (a *ApplicationPlugin) rebuildIgnoreRuleMatchers(ctx context.Context) {
	rawRules := strings.TrimSpace(a.api.GetSetting(ctx, "IgnoreRules"))
	if rawRules == "" {
		a.queryEntriesMutex.Lock()
		a.ignoreMatchers = nil
		a.queryEntriesMutex.Unlock()
		return
	}

	var rules []appIgnoreRule
	if err := json.Unmarshal([]byte(rawRules), &rules); err != nil {
		a.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to parse IgnoreRules: %s", err.Error()))
		return
	}

	normalizedRules := normalizeAppIgnoreRules(rules)
	matchers := make([]appIgnoreMatcher, 0, len(normalizedRules))
	for _, rule := range normalizedRules {
		compiled, err := compileAppIgnorePattern(rule.Pattern)
		if err != nil {
			a.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to compile ignore rule %q: %s", rule.Pattern, err.Error()))
			continue
		}

		matchers = append(matchers, appIgnoreMatcher{
			pattern: rule.Pattern,
			regex:   compiled,
		})
	}

	a.queryEntriesMutex.Lock()
	a.ignoreMatchers = matchers
	a.queryEntriesMutex.Unlock()
}

func (a *ApplicationPlugin) getIgnoreRuleMatchersSnapshot() []appIgnoreMatcher {
	a.queryEntriesMutex.RLock()
	matchers := a.ignoreMatchers
	a.queryEntriesMutex.RUnlock()
	return matchers
}

func buildIgnoreRuleCandidates(info appInfo, displayName string) []string {
	candidates := []string{
		strings.TrimSpace(displayName),
		strings.TrimSpace(info.Path),
	}

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
