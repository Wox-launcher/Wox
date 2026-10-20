package raycast

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"wox/plugin"
	"wox/plugin/thirdparty/migrate"
	"wox/plugin/thirdparty/raycast/brand"
	"wox/plugin/thirdparty/raycast/extension"
	"wox/util"
)

// localExtension retains a verified source path separately from the wizard's display model.
type localExtension struct {
	migrate.Plugin
	directory   string
	preferences map[string]json.RawMessage
}

// mapExtensionPreferences joins explicit values by export identity; the installer filters them against the local manifest.
func mapExtensionPreferences(payload Payload) []ExportExtension {
	refs := append([]ExportExtension(nil), payload.NodeExtensions.Extensions...)
	values := map[string]map[string]json.RawMessage{}
	for _, ref := range refs {
		values["e:n:"+strings.ToLower(ref.UUID)] = map[string]json.RawMessage{}
	}
	for _, settings := range payload.Settings.NodeExtensions {
		if target := values[strings.ToLower(settings.ID)]; target != nil {
			for key, value := range settings.Meta {
				target["preference:"+key] = value
			}
		}
	}
	for _, command := range payload.Settings.Commands {
		id := strings.ToLower(command.ExtensionID)
		target := values[id]
		prefix := "c:n:" + strings.TrimPrefix(id, "e:n:") + "::-::"
		if target == nil || !strings.HasPrefix(command.ID, prefix) {
			continue
		}
		name := strings.TrimPrefix(command.ID, prefix)
		for key, value := range command.Meta {
			target["command:"+name+":"+key] = value
		}
	}
	for index := range refs {
		refs[index].Preferences = values["e:n:"+strings.ToLower(refs[index].UUID)]
	}
	return refs
}

// localExtensions joins export identities with local code, without downloading or executing anything.
// Missing and unsupported entries stay visible so an empty selection never conceals why migration is unavailable.
func (i *installation) localExtensions(ctx context.Context, refs []ExportExtension) ([]localExtension, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	root := i.extensionRoot
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		root = filepath.Join(home, ".config", "raycast", "extensions")
	}
	seen := map[string]bool{}
	var candidates []localExtension
	for _, ref := range refs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(ref.UUID)
		// UUIDs are directory names, never arbitrary paths supplied by a backup.
		if err != nil || !strings.EqualFold(id.String(), ref.UUID) || seen[id.String()] {
			continue
		}
		seen[id.String()] = true
		candidate := localExtension{Plugin: migrate.Plugin{ID: id.String(), Name: ref.Name, Version: ref.Version,
			Icon: brand.Image(), Status: migrate.PluginUnsupported, Detail: "raycast_extension_missing"}}
		directory := filepath.Join(root, id.String())
		info, statErr := os.Lstat(directory)
		if statErr == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			candidate.Detail = "raycast_extension_unsupported"
			d, parseErr := extension.Parse(directory)
			if parseErr == nil {
				candidate.Name = d.Metadata.GetName(ctx)
				candidate.Icon = d.Metadata.GetIconOrDefault(directory, brand.Image())
				owner, localOwner := ref.Owner, d.Manifest.Owner
				if owner == "" {
					owner = ref.Author
				}
				if localOwner == "" {
					localOwner = d.Manifest.Author
				}
				matches := strings.EqualFold(ref.Name, d.Manifest.Name) && strings.EqualFold(owner, localOwner) && strings.EqualFold(ref.Author, d.Manifest.Author)
				supported := len(d.Metadata.SupportedOS) == 0 || plugin.IsAnySupportedInCurrentOS(d.Metadata.SupportedOS)
				if !matches {
					candidate.Detail = "raycast_extension_mismatch"
				} else if supported {
					candidate.directory = directory
					candidate.preferences = ref.Preferences
					candidate.Description = d.Metadata.GetDescription(ctx)
					candidate.Keywords = d.Metadata.TriggerKeywords
					candidate.Status, candidate.Selectable, candidate.Detail = migrate.PluginReady, true, "raycast_extension_local"
					if _, err := os.Stat(filepath.Join(extension.CollectionDirectory(), d.Metadata.Id)); err == nil {
						candidate.Status, candidate.Selectable, candidate.Detail = migrate.PluginImported, false, ""
					}
				}
			} else {
				util.GetLogger().Debug(ctx, fmt.Sprintf("Raycast local extension %s: %v", ref.Name, parseErr))
			}
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

// importExtensions revalidates local files and calls the Raycast importer directly.
// Per-plugin failures are returned to the wizard while other selected items continue.
func (i *installation) importExtensions(ctx context.Context, refs []ExportExtension, itemIDs []string) ([]migrate.PluginResult, error) {
	var selected []string
	seen := map[string]bool{}
	for _, id := range itemIDs {
		if strings.HasPrefix(id, "plugin:") && !seen[id] {
			selected = append(selected, id)
			seen[id] = true
		}
	}
	if len(selected) == 0 {
		return nil, nil
	}
	candidates, err := i.localExtensions(ctx, refs)
	if err != nil {
		return nil, err
	}
	byID := map[string]localExtension{}
	for _, candidate := range candidates {
		byID["plugin:"+candidate.ID] = candidate
	}
	var results []migrate.PluginResult
	for _, id := range selected {
		outcome := migrate.PluginResult{ID: id}
		candidate, found := byID[id]
		switch {
		case ctx.Err() != nil:
			outcome.Error = ctx.Err().Error()
		case found && candidate.Status == migrate.PluginImported:
			// Retrying an import is a no-op and must not replace an existing Wox plugin.
		case !found || !candidate.Selectable:
			outcome.Error = "Raycast plugin cannot be imported from its local installation"
		default:
			if err := extension.Import(ctx, candidate.directory, candidate.preferences); err != nil {
				outcome.Error = err.Error()
			}
		}
		results = append(results, outcome)
	}
	return results, nil
}
