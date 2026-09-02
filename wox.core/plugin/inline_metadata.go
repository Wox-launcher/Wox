package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wox/common"
	"wox/util"

	"github.com/Masterminds/semver/v3"
)

type inlineMetadataResult struct {
	Metadata      Metadata
	Keys          map[string]bool
	JSONStartLine int
}

func (r inlineMetadataResult) hasKey(name string) bool {
	if r.Keys[name] {
		return true
	}
	for key := range r.Keys {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}

// parseInlineMetadata reads a plugin source file and decodes the first JSON
// object from the leading comment block. json.Decoder is used so braces inside
// strings and escape sequences cannot break parsing.
func parseInlineMetadata(filePath string) (inlineMetadataResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return inlineMetadataResult{}, fmt.Errorf("failed to read plugin file: %w", err)
	}
	return parseInlineMetadataContent(filePath, content)
}

func parseInlineMetadataContent(filePath string, content []byte) (inlineMetadataResult, error) {
	header, headerStartLine := extractInlineCommentHeader(string(content))
	if strings.TrimSpace(header) == "" {
		return inlineMetadataResult{}, fmt.Errorf("no JSON metadata block found in %s. Plugins must define metadata as a JSON object in comments", filePath)
	}

	jsonOffset := strings.Index(header, "{")
	if jsonOffset < 0 {
		return inlineMetadataResult{}, fmt.Errorf("no JSON metadata block found in %s. Plugins must define metadata as a JSON object in comments", filePath)
	}

	jsonStartLine := headerStartLine + strings.Count(header[:jsonOffset], "\n")
	decoder := json.NewDecoder(bytes.NewReader([]byte(header[jsonOffset:])))
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return inlineMetadataResult{}, fmt.Errorf("failed to parse JSON metadata block (starting at line %d): %w", jsonStartLine, err)
	}

	keys := map[string]bool{}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return inlineMetadataResult{}, fmt.Errorf("failed to parse JSON metadata block (starting at line %d): %w", jsonStartLine, err)
	}
	for key := range object {
		keys[key] = true
	}

	var metadata Metadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return inlineMetadataResult{}, fmt.Errorf("failed to parse JSON metadata block (starting at line %d): %w", jsonStartLine, err)
	}

	return inlineMetadataResult{
		Metadata:      metadata,
		Keys:          keys,
		JSONStartLine: jsonStartLine,
	}, nil
}

// extractInlineCommentHeader collects the leading shebang plus consecutive
// `#` or `//` comments. Blank lines are allowed inside that header region.
func extractInlineCommentHeader(content string) (string, int) {
	lines := strings.Split(content, "\n")
	var builder strings.Builder
	headerStartLine := 1
	started := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isShebangLine(trimmed) {
			if started {
				break
			}
			headerStartLine++
			continue
		}
		if trimmed == "" {
			if started {
				builder.WriteByte('\n')
			} else {
				headerStartLine++
			}
			continue
		}
		if !isInlineMetadataComment(trimmed) {
			break
		}
		started = true
		builder.WriteString(stripInlineMetadataComment(trimmed))
		builder.WriteByte('\n')
	}

	return builder.String(), headerStartLine
}

func isShebangLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, "#!")
}

func isInlineMetadataComment(trimmed string) bool {
	return strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//")
}

func stripInlineMetadataComment(trimmed string) string {
	cleaned := trimmed
	switch {
	case strings.HasPrefix(cleaned, "//"):
		cleaned = strings.TrimPrefix(cleaned, "//")
	case strings.HasPrefix(cleaned, "#"):
		cleaned = strings.TrimPrefix(cleaned, "#")
	}
	return strings.TrimSpace(cleaned)
}

// ParseScriptMetadata parses metadata from script plugin file comments.
func (m *Manager) ParseScriptMetadata(ctx context.Context, scriptPath string) (Metadata, error) {
	parsed, err := parseInlineMetadata(scriptPath)
	if err != nil {
		return Metadata{}, err
	}

	if parsed.hasKey("Runtime") {
		runtime := ConvertToRuntime(parsed.Metadata.Runtime)
		switch runtime {
		case PLUGIN_RUNTIME_SCRIPT, "":
		case PLUGIN_RUNTIME_PYTHON, PLUGIN_RUNTIME_NODEJS:
			return Metadata{}, fmt.Errorf("script plugin %s declares Runtime %s; move this file to %s to load it as a single-file SDK plugin", filepath.Base(scriptPath), parsed.Metadata.Runtime, util.GetLocation().GetUserSingleFilePluginsDirectory())
		default:
			return Metadata{}, fmt.Errorf("unsupported runtime in script plugin %s: %s", filepath.Base(scriptPath), parsed.Metadata.Runtime)
		}
	}

	metadata := parsed.Metadata
	metadata.Runtime = string(PLUGIN_RUNTIME_SCRIPT)
	metadata.Entry = filepath.Base(scriptPath)
	metadata.Directory = filepath.Dir(scriptPath)
	metadata.LoadPluginI18nFromDirectory(ctx)
	return m.validateAndSetScriptMetadataDefaults(metadata)
}

// ParseSingleFilePluginMetadata parses a Python or Node.js single-file SDK plugin.
func (m *Manager) ParseSingleFilePluginMetadata(ctx context.Context, filePath string) (Metadata, error) {
	parsed, err := parseInlineMetadata(filePath)
	if err != nil {
		return Metadata{}, err
	}
	return validateSingleFilePluginMetadata(ctx, filePath, parsed)
}

func validateSingleFilePluginMetadata(_ context.Context, filePath string, parsed inlineMetadataResult) (Metadata, error) {

	if parsed.hasKey("Entry") {
		return Metadata{}, fmt.Errorf("single-file plugin metadata must not declare Entry")
	}
	if parsed.hasKey("Directory") {
		return Metadata{}, fmt.Errorf("single-file plugin metadata must not declare Directory")
	}

	metadata := parsed.Metadata
	if metadata.Id == "" {
		return Metadata{}, fmt.Errorf("missing required field: Id")
	}
	if metadata.GetName(context.Background()) == "" {
		return Metadata{}, fmt.Errorf("missing required field: Name")
	}
	if metadata.Version == "" {
		return Metadata{}, fmt.Errorf("missing required field: Version")
	}
	if strings.TrimSpace(metadata.MinWoxVersion) == "" {
		return Metadata{}, fmt.Errorf("missing required field: MinWoxVersion")
	}
	if _, err := semver.NewVersion(metadata.MinWoxVersion); err != nil {
		return Metadata{}, fmt.Errorf("invalid MinWoxVersion %q: %w", metadata.MinWoxVersion, err)
	}
	if !parsed.hasKey("Runtime") || strings.TrimSpace(metadata.Runtime) == "" {
		return Metadata{}, fmt.Errorf("missing required field: Runtime")
	}
	if len(metadata.TriggerKeywords) == 0 {
		return Metadata{}, fmt.Errorf("missing required field: TriggerKeywords")
	}

	expectedRuntime, err := singleFileRuntimeForPath(filePath)
	if err != nil {
		return Metadata{}, err
	}
	if ConvertToRuntime(metadata.Runtime) != expectedRuntime {
		return Metadata{}, fmt.Errorf("runtime %s does not match file extension %s", metadata.Runtime, filepath.Ext(filePath))
	}
	metadata.Runtime = string(expectedRuntime)

	if err := validateSingleFileMetadataIcon(metadata.Icon); err != nil {
		return Metadata{}, err
	}
	if err := metadata.ValidateGlances(); err != nil {
		return Metadata{}, fmt.Errorf("invalid glances: %w", err)
	}

	metadata.Entry = filepath.Base(filePath)
	metadata.Directory = util.GetLocation().GetUserSingleFilePluginsDirectory()
	return metadata, nil
}

func validateSingleFileMetadataIcon(icon string) error {
	if strings.TrimSpace(icon) == "" {
		return nil
	}
	image, err := common.ParseWoxImage(icon)
	if err != nil {
		return fmt.Errorf("invalid Icon: %w", err)
	}
	if image.ImageType == common.WoxImageTypeRelativePath {
		return fmt.Errorf("single-file plugins cannot use relative metadata icons; use emoji, url, svg, base64, or an absolute path")
	}
	return nil
}
