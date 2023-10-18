package util

import "regexp"

func FindRegexGroups(regexExpression, raw string) (groups []map[string]string) {
	var compRegEx = regexp.MustCompile(regexExpression)
	matches := compRegEx.FindAllStringSubmatch(raw, -1)

	for _, match := range matches {
		subGroup := make(map[string]string)
		for i, name := range compRegEx.SubexpNames() {
			if i > 0 && i <= len(match) {
				subGroup[name] = match[i]
			}
		}
		groups = append(groups, subGroup)
	}

	return groups
}

func FindRegexGroup(regexExpression, raw string) (group map[string]string) {
	groups := FindRegexGroups(regexExpression, raw)
	if len(groups) > 0 {
		return groups[0]
	}

	return make(map[string]string)
}

func FindRegexLines(regexExpression, raw string) []string {
	var compRegEx = regexp.MustCompile(regexExpression)
	return compRegEx.FindAllString(raw, -1)
}
