package plugin

import (
	"context"
	"strings"
	"wox/common"
)

// MatchQueryHint prefers a complete command, then a complete trigger; the source is returned solely
// for translating placeholders and is never attached to the hint or used for routing.
func MatchQueryHint(text string, instances []*Instance) (*common.QueryHint, *Instance) {
	// Both command and trigger templates require an explicit context separator.
	if !strings.HasSuffix(text, " ") {
		return nil, nil
	}
	// Even a plugin without a hint owns its keyword. Never decorate an ambiguous route.
	prefix := strings.SplitN(text, " ", 2)[0]
	owners := 0
	var triggerOwner *Instance
	for _, instance := range instances {
		for _, keyword := range instance.GetTriggerKeywords() {
			if keyword != "*" && keyword == prefix {
				owners++
				triggerOwner = instance
				break
			}
		}
	}
	if owners > 1 {
		return nil, nil
	}
	var matched *common.QueryHint
	var source *Instance
	for _, instance := range instances {
		if triggerOwner != nil && instance != triggerOwner {
			continue
		}
		for _, command := range instance.GetQueryCommands() {
			if command.QueryHint == nil || command.QueryHint.Validate() != nil {
				continue
			}
			aliases := append([]string{command.Command}, command.Aliases...)
			found := false
			for _, trigger := range instance.GetTriggerKeywords() {
				if triggerOwner != nil && trigger == "*" {
					continue
				}
				for _, alias := range aliases {
					prefix := alias
					if trigger != "*" {
						prefix = trigger + " " + alias
					}
					if strings.EqualFold(strings.TrimRight(text, " "), prefix) {
						found = true
					}
				}
			}
			if !found {
				continue
			}
			if matched != nil {
				return nil, nil
			}
			matched = command.QueryHint.Clone()
			source = instance
			matched.Elements = append([]common.QueryElement{{Id: "command", Kind: common.QueryElementText, Text: strings.TrimRight(text, " ") + " "}}, matched.Elements...)
			if matched.Validate() != nil {
				return nil, nil
			}
		}
	}
	if matched != nil {
		return matched, source
	}
	for _, instance := range instances {
		for _, keyword := range instance.GetTriggerKeywords() {
			if keyword == "*" || strings.TrimRight(text, " ") != keyword {
				continue
			}
			hint := instance.triggerQueryHint(keyword)
			if hint == nil {
				continue
			}
			if hint.Validate() != nil {
				return nil, nil
			}
			hint.Elements = append([]common.QueryElement{{Id: "command", Kind: common.QueryElementText, Text: keyword + " "}}, hint.Elements...)
			if hint.Validate() != nil {
				return nil, nil
			}
			return hint, instance
		}
	}
	return nil, nil
}

// ResolveQueryHint uses currently available plugins and translates the template once.
func (m *Manager) ResolveQueryHint(ctx context.Context, text string) *common.QueryHint {
	var available []*Instance
	for _, instance := range m.GetPluginInstances() {
		if instance.Setting != nil && !instance.Setting.Disabled.Get() {
			available = append(available, instance)
		}
	}
	structure, instance := MatchQueryHint(text, available)
	if structure != nil {
		for i := range structure.Elements {
			structure.Elements[i].Placeholder = common.I18nString(instance.TranslateMetadataText(ctx, structure.Elements[i].Placeholder))
		}
	}
	return structure
}
