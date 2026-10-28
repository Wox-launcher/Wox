package sys

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/util"
)

const (
	omarchyCommandPrefix        = "omarchy:"
	omarchyAppsMenuID           = "apps"
	omarchyGuardRefreshInterval = 60 * time.Second
	omarchyGuardTimeout         = 20 * time.Second
)

// omarchyGuardHelpers answers package and command checks inside one shell.
// The shipped menu calls them often enough that forking pacman once per row
// makes a refresh take over a second. These shadows match omarchy-pkg-present
// and omarchy-cmd-present for this batch only; see guardHelpers in Omarchy's
// shell/plugins/menu/MenuModel.js.
const omarchyGuardHelpers = `declare -A __omarchy_pkgs=()
mapfile -t __omarchy_pkg_names < <({ pacman -Qq; LC_ALL=C pacman -Qi | awk '/^[A-Za-z]/ { provides = ($0 ~ /^Provides/); sub(/^[^:]*: /, "") } provides && $0 != "None" { n = split($0, p, " "); for (i = 1; i <= n; i++) { sub(/[<>=].*/, "", p[i]); print p[i] } }'; } 2>/dev/null)
for __omarchy_pkg in "${__omarchy_pkg_names[@]}"; do __omarchy_pkgs[$__omarchy_pkg]=1; done
__omarchy_pkg_has() { [[ -n ${__omarchy_pkgs[$1]-} ]] && return 0; [[ $1 == *[\<\>=]* ]] && { pacman -Q "$1" &>/dev/null; return; }; return 1; }
omarchy-pkg-present() { local p; for p in "$@"; do __omarchy_pkg_has "$p" || return 1; done; return 0; }
omarchy-pkg-missing() { local p; for p in "$@"; do __omarchy_pkg_has "$p" || return 0; done; return 1; }
omarchy-cmd-present() { local c; for c in "$@"; do command -v "$c" &>/dev/null || return 1; done; return 0; }
omarchy-cmd-missing() { local c; for c in "$@"; do command -v "$c" &>/dev/null || return 0; done; return 1; }
`

var (
	omarchyFullLineComment = regexp.MustCompile(`(?m)^\s*//.*(?:\n|$)`)
	omarchyTrailingComma   = regexp.MustCompile(`,(\s*[}\]])`)
	omarchyMenuIDPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
)

// omarchyMenuItem is one row from Omarchy's menu JSONC after defaults and the
// user extension have been merged.
type omarchyMenuItem struct {
	ID          string
	Parent      string
	Label       string
	Action      string
	Provider    string
	Description string
	When        string
	Checked     string
	Aliases     []string
}

type omarchyRawItem struct {
	ID     string
	Fields map[string]json.RawMessage
}

// omarchyWhenState stores the latest `when` results. Queries read it while a
// refresh writes it.
type omarchyWhenState struct {
	ready  atomic.Bool
	values *util.HashMap[string, bool]
}

func newOmarchyWhenState() *omarchyWhenState {
	return &omarchyWhenState{values: util.NewHashMap[string, bool]()}
}

// allows reports whether a menu row should be searchable. Rows without a
// condition are always shown. Conditional rows stay hidden until the first
// successful evaluation, then follow that result.
func (s *omarchyWhenState) allows(id, when string) bool {
	if strings.TrimSpace(when) == "" {
		return true
	}
	if s == nil || !s.ready.Load() {
		return false
	}
	allowed, ok := s.values.Load(id)
	return ok && allowed
}

// replace publishes a complete guard result. Missing ids are dropped so a row
// whose condition disappeared cannot stay visible.
func (s *omarchyWhenState) replace(results map[string]bool) {
	if s == nil {
		return
	}
	present := make(map[string]struct{}, len(results))
	for id, allowed := range results {
		s.values.Store(id, allowed)
		present[id] = struct{}{}
	}
	for _, id := range s.values.Keys() {
		if _, ok := present[id]; !ok {
			s.values.Delete(id)
		}
	}
	s.ready.Store(true)
}

// omarchyGuardCache runs menu visibility checks away from the query path and
// keeps the previous answers while a new check is in flight.
type omarchyGuardCache struct {
	state    *omarchyWhenState
	evaluate func(context.Context, string) (map[string]bool, error)
	script   string
	running  atomic.Bool
	epoch    atomic.Uint64
	last     atomic.Int64
}

func (c *omarchyGuardCache) refreshIfStale(ctx context.Context) {
	c.refresh(ctx, false)
}

// refreshSoon runs another check after the current one so an action can update
// the rows that depend on what it changed.
func (c *omarchyGuardCache) refreshSoon(ctx context.Context) {
	c.refresh(ctx, true)
}

func (c *omarchyGuardCache) refresh(ctx context.Context, force bool) {
	if c == nil || c.evaluate == nil || strings.TrimSpace(c.script) == "" {
		return
	}
	if !force {
		last := c.last.Load()
		if last != 0 && time.Since(time.Unix(0, last)) < omarchyGuardRefreshInterval {
			return
		}
	}
	// epoch records that a run is wanted. A check already in flight notices the
	// newer value when it finishes and starts the follow-up itself.
	c.epoch.Add(1)
	c.kick(ctx)
}

// kick starts one guard run. Overlapping requests leave epoch ahead of the
// run that is in flight, and that run starts the next one before it exits.
func (c *omarchyGuardCache) kick(ctx context.Context) {
	if !c.running.CompareAndSwap(false, true) {
		return
	}
	seen := c.epoch.Load()
	c.last.Store(time.Now().UnixNano())
	script := c.script
	util.Go(ctx, "refresh omarchy menu guards", func() {
		results, err := c.evaluate(context.Background(), script)
		if err != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("omarchy menu guards: %v", err))
		} else if c.state != nil {
			c.state.replace(results)
		}
		c.running.Store(false)
		if c.epoch.Load() != seen {
			c.kick(ctx)
		}
	})
}

// stripOmarchyJSONC removes the comment and trailing-comma forms Omarchy's
// menu parser accepts. Comments are full-line only, matching that parser.
func stripOmarchyJSONC(raw string) string {
	withoutComments := omarchyFullLineComment.ReplaceAllString(raw, "")
	return omarchyTrailingComma.ReplaceAllString(withoutComments, "$1")
}

// parseOmarchyMenuJSONC reads one menu document. A top-level "items" object
// replaces the document, which is the wrapper Omarchy already accepts.
func parseOmarchyMenuJSONC(raw string) ([]omarchyRawItem, error) {
	stripped := strings.TrimSpace(stripOmarchyJSONC(raw))
	if stripped == "" {
		return nil, nil
	}
	fields, err := decodeOmarchyObject([]byte(stripped))
	if err != nil {
		return nil, err
	}
	source := fields
	for _, field := range fields {
		if field.Key != "items" {
			continue
		}
		// A catalog wrapper is an object of menu rows. A normal row named
		// "items" has string fields, and must stay in the top-level document.
		if nested, ok := omarchyMenuCatalog(field.Raw); ok {
			source = nested
		}
		break
	}

	items := make([]omarchyRawItem, 0, len(source))
	for _, field := range source {
		if !omarchyMenuIDPattern.MatchString(field.Key) || field.Key == "root" {
			continue
		}
		var entry map[string]json.RawMessage
		if json.Unmarshal(field.Raw, &entry) != nil || entry == nil {
			continue
		}
		items = append(items, omarchyRawItem{ID: field.Key, Fields: entry})
	}
	return items, nil
}

type omarchyObjectField struct {
	Key string
	Raw json.RawMessage
}

// omarchyMenuCatalog reports whether raw is a map of menu rows rather than one row.
func omarchyMenuCatalog(raw json.RawMessage) ([]omarchyObjectField, bool) {
	fields, err := decodeOmarchyObject(raw)
	if err != nil || len(fields) == 0 {
		return nil, false
	}
	for _, field := range fields {
		var entry map[string]json.RawMessage
		if json.Unmarshal(field.Raw, &entry) == nil && entry != nil {
			return fields, true
		}
	}
	return nil, false
}

func decodeOmarchyObject(raw []byte) ([]omarchyObjectField, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("omarchy menu must be a JSON object")
	}

	var fields []omarchyObjectField
	for dec.More() {
		keyToken, keyErr := dec.Token()
		if keyErr != nil {
			return nil, keyErr
		}
		key, keyOK := keyToken.(string)
		if !keyOK {
			return nil, fmt.Errorf("omarchy menu key is not a string")
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		fields = append(fields, omarchyObjectField{Key: key, Raw: value})
	}
	end, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := end.(json.Delim); !ok || delim != '}' {
		return nil, fmt.Errorf("omarchy menu object did not close")
	}
	return fields, nil
}

// mergeOmarchyMenu overlays the user extension onto the default menu. A field
// is replaced only when the extension sets it, so a label override keeps the
// original action.
func mergeOmarchyMenu(base, extra []omarchyRawItem) []omarchyMenuItem {
	order := make([]string, 0, len(base)+len(extra))
	merged := make(map[string]map[string]json.RawMessage, len(base)+len(extra))
	apply := func(items []omarchyRawItem) {
		for _, item := range items {
			if item.ID == "" {
				continue
			}
			fields := merged[item.ID]
			if fields == nil {
				fields = make(map[string]json.RawMessage, len(item.Fields))
				merged[item.ID] = fields
				order = append(order, item.ID)
			}
			for key, value := range item.Fields {
				fields[key] = value
			}
		}
	}
	apply(base)
	apply(extra)

	items := make([]omarchyMenuItem, 0, len(order))
	for _, id := range order {
		items = append(items, normalizeOmarchyItem(id, merged[id]))
	}
	return items
}

func normalizeOmarchyItem(id string, fields map[string]json.RawMessage) omarchyMenuItem {
	label, hasLabel := omarchyJSONString(fields, "label")
	if !hasLabel || strings.TrimSpace(label) == "" {
		label = id
	}
	parent, hasParent := omarchyJSONString(fields, "parent")
	if !hasParent {
		if dot := strings.LastIndex(id, "."); dot >= 0 {
			parent = id[:dot]
		} else {
			parent = "root"
		}
	}
	action, _ := omarchyJSONString(fields, "action")
	provider, _ := omarchyJSONString(fields, "provider")
	description, _ := omarchyJSONString(fields, "description")
	when, _ := omarchyJSONString(fields, "when")
	checked, _ := omarchyJSONString(fields, "checked")
	return omarchyMenuItem{
		ID:          id,
		Parent:      parent,
		Label:       label,
		Action:      strings.TrimSpace(action),
		Provider:    strings.TrimSpace(provider),
		Description: strings.TrimSpace(description),
		When:        strings.TrimSpace(when),
		Checked:     strings.TrimSpace(checked),
		Aliases:     omarchyJSONAliases(fields["aliases"]),
	}
}

func omarchyJSONString(fields map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := fields[key]
	if !ok {
		return "", false
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", true
	}
	return value, true
}

func omarchyJSONAliases(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		return compactOmarchyAliases(many)
	}
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return compactOmarchyAliases([]string{one})
	}
	return nil
}

func compactOmarchyAliases(values []string) []string {
	aliases := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		aliases = append(aliases, value)
	}
	return aliases
}

// omarchyParentPath is the ancestor breadcrumb, such as "Install › Browser".
func omarchyParentPath(byID map[string]omarchyMenuItem, id string) string {
	item, ok := byID[id]
	if !ok {
		return ""
	}
	labels := make([]string, 0, 4)
	current := item.Parent
	for guard := 0; current != "" && current != "root" && guard < 32; guard++ {
		parent, found := byID[current]
		if !found {
			break
		}
		labels = append(labels, parent.Label)
		current = parent.Parent
	}
	for left, right := 0, len(labels)-1; left < right; left, right = left+1, right-1 {
		labels[left], labels[right] = labels[right], labels[left]
	}
	return strings.Join(labels, " › ")
}

func omarchyMenuIndex(items []omarchyMenuItem) map[string]omarchyMenuItem {
	byID := make(map[string]omarchyMenuItem, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	return byID
}

// omarchyInvocation is the shell command for a row. Static actions run as
// written. Dynamic provider menus open that Omarchy submenu, except Apps,
// which the app plugin already lists.
func omarchyInvocation(item omarchyMenuItem) (string, bool) {
	if item.Action != "" {
		return item.Action, true
	}
	if item.Provider != "" && item.ID != omarchyAppsMenuID {
		return "omarchy menu summon " + item.ID, true
	}
	return "", false
}

func omarchyCommandAliases(item omarchyMenuItem, parentPath string) []string {
	aliases := append([]string{}, item.Aliases...)
	if parentPath != "" {
		aliases = append(aliases, parentPath+" "+item.Label, strings.ReplaceAll(parentPath, " › ", " ")+" "+item.Label)
	}
	aliases = append(aliases, strings.ReplaceAll(item.ID, ".", " "))
	if item.Description != "" {
		aliases = append(aliases, item.Description)
	}
	return compactOmarchyAliases(aliases)
}

func omarchyCommandSubtitle(ctx context.Context, parentPath string) string {
	if parentPath == "" {
		return "i18n:plugin_sys_omarchy_command_subtitle"
	}
	template := i18n.GetI18nManager().TranslateWox(ctx, "plugin_sys_omarchy_command_path")
	if !strings.Contains(template, "%s") {
		return "Omarchy › " + parentPath
	}
	return fmt.Sprintf(template, parentPath)
}

// buildOmarchySysCommands turns merged menu rows into system commands. run is
// invoked with the menu id and the shell command; a nil run leaves the action
// in place for tests that only inspect the catalog.
func buildOmarchySysCommands(items []omarchyMenuItem, state *omarchyWhenState, icon common.WoxImage, run func(context.Context, string, string)) []SysCommand {
	byID := omarchyMenuIndex(items)
	commands := make([]SysCommand, 0, len(items))
	for _, item := range items {
		invocation, ok := omarchyInvocation(item)
		if !ok {
			continue
		}
		menuID := item.ID
		when := item.When
		action := invocation
		parentPath := omarchyParentPath(byID, item.ID)
		label := item.Label
		commands = append(commands, SysCommand{
			ID:             omarchyCommandPrefix + menuID,
			Title:          label,
			Icon:           icon,
			Aliases:        omarchyCommandAliases(item, parentPath),
			SupportedOS:    []string{util.PlatformLinux},
			SkipEmptyQuery: true,
			IsAvailable: func() bool {
				return state.allows(menuID, when)
			},
			BuildSubTitle: func(ctx context.Context, _ plugin.Query) string {
				subtitle := omarchyCommandSubtitle(ctx, parentPath)
				if checked, ok := state.values.Load(menuID + ":c"); ok && checked {
					subtitle += "  ✓"
				}
				return subtitle
			},
			Action: func(ctx context.Context, _ plugin.ActionContext) {
				if run != nil {
					run(ctx, menuID, action)
				}
			},
		})
	}
	return commands
}

// omarchyGuardScript evaluates visibility and checked conditions in one shell.
// The package helpers are prepended by the runner.
func omarchyGuardScript(items []omarchyMenuItem) string {
	var body strings.Builder
	for _, item := range items {
		if !omarchyMenuIDPattern.MatchString(item.ID) {
			continue
		}
		for _, condition := range []struct{ kind, expression string }{{"w", item.When}, {"c", item.Checked}} {
			if condition.expression == "" {
				continue
			}
			expression := strings.ReplaceAll(condition.expression, "\n", " ")
			fmt.Fprintf(&body, "if { %s; } >/dev/null 2>&1; then echo %s:%s:1; else echo %s:%s:0; fi\n", expression, item.ID, condition.kind, item.ID, condition.kind)
		}
	}
	return body.String()
}

func omarchyGuardProgram(body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	return omarchyGuardHelpers + body
}

func parseOmarchyGuardOutput(output string) map[string]bool {
	results := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasSuffix(line, ":c:1"):
			results[strings.TrimSuffix(line, ":1")] = true
		case strings.HasSuffix(line, ":c:0"):
			results[strings.TrimSuffix(line, ":0")] = false
		case strings.HasSuffix(line, ":w:1"):
			results[strings.TrimSuffix(line, ":w:1")] = true
		case strings.HasSuffix(line, ":w:0"):
			results[strings.TrimSuffix(line, ":w:0")] = false
		}
	}
	return results
}
