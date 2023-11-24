package system

import (
	"context"
	"strings"
	"wox/plugin"
	"wox/util"
	"wox/util/clipboard"
)

var selectionIcon = plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" width="48" height="48" viewBox="0 0 48 48"><path fill="#388e3c" d="M43,38.833C43,41.135,41.135,43,38.833,43H17.167C14.866,43,13,41.135,13,38.833V17.167 C13,14.865,14.866,13,17.167,13h21.667C41.135,13,43,14.865,43,17.167V38.833z"></path><path fill="#c8e6c9" d="M35,30.833C35,33.135,33.135,35,30.833,35H9.167C6.866,35,5,33.135,5,30.833V9.167 C5,6.865,6.866,5,9.167,5h21.667C33.135,5,35,6.865,35,9.167V30.833z"></path><path fill="#4caf50" d="M18 28.121L11.064 21.186 13.186 19.064 18 23.879 28.814 13.064 30.936 15.186z"></path></svg>`)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &SelectionPlugin{})
}

type SelectionPlugin struct {
	api plugin.API
}

func (i *SelectionPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "d9e557ed-89bd-4b8b-bd64-2a7632cf3483",
		Name:          "Selection",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Wox default actions for selection query",
		Icon:          selectionIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureQuerySelection,
			},
		},
	}
}

func (i *SelectionPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	i.api = initParams.API
}

func (i *SelectionPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if query.Type != plugin.QueryTypeSelection {
		return []plugin.QueryResult{}
	}

	if query.Selection.Type == util.SelectionTypeText {
		return i.queryForSelectionText(ctx, query.Selection.Text)
	}
	if query.Selection.Type == util.SelectionTypeFile {
		return i.queryForSelectionFile(ctx, query.Selection.FilePaths)
	}

	return []plugin.QueryResult{}
}

func (i *SelectionPlugin) queryForSelectionText(ctx context.Context, text string) []plugin.QueryResult {
	var results []plugin.QueryResult
	results = append(results, plugin.QueryResult{
		Title: "Copy",
		Icon:  plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAABRElEQVR4nO2ZzUpCQRiG5wJa63gFQfaziLqFEhfp7RzbtiloUwg6Bi26FEFy00ItMUHPHPm6gFauvhijIKgo5puxn/eBd/89zHNWRykAwLeoPnC5QjyrELPPdlqTtloGB8SZ7/Fua4e95UhIHO9WrPWeJS6mnV8rUIwhkW9Ny9qkM31+x7nja5Hj3dZr/bcSZtxVSq2IC2iTZgVjWVpgI+l/JCGLOz6EwGYyeF9COqdQAlvJII5EKIHto/tPJXYv0/aPFtjrPMaRCCXw1SkIGLwAIyEfkBAhIT+QECEhP5AQISE/kBAhIT+QECEhP5AQISE//lBCQ86ddLk0nscUsHIC9RHnT2949WoYS8JWiffFBAqNCeuz24WEewmXU4gpaV4FXiTqo8X3EGoq1A+OGNPN1MoLNLJSDAndTK021r95AP4ZT0uTPkQe0ydSAAAAAElFTkSuQmCC`),
		Actions: []plugin.QueryResultAction{
			{
				Name: "Copy to clipboard",
				Action: func(actionContext plugin.ActionContext) {
					clipboard.WriteText(text)
				},
			},
		},
	})
	return results
}

func (i *SelectionPlugin) queryForSelectionFile(ctx context.Context, filePaths []string) []plugin.QueryResult {
	var results []plugin.QueryResult
	results = append(results, plugin.QueryResult{
		Title: "Copy path to clipboard",
		Icon:  plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAABRElEQVR4nO2ZzUpCQRiG5wJa63gFQfaziLqFEhfp7RzbtiloUwg6Bi26FEFy00ItMUHPHPm6gFauvhijIKgo5puxn/eBd/89zHNWRykAwLeoPnC5QjyrELPPdlqTtloGB8SZ7/Fua4e95UhIHO9WrPWeJS6mnV8rUIwhkW9Ny9qkM31+x7nja5Hj3dZr/bcSZtxVSq2IC2iTZgVjWVpgI+l/JCGLOz6EwGYyeF9COqdQAlvJII5EKIHto/tPJXYv0/aPFtjrPMaRCCXw1SkIGLwAIyEfkBAhIT+QECEhP5AQISE/kBAhIT+QECEhP5AQISE//lBCQ86ddLk0nscUsHIC9RHnT2949WoYS8JWiffFBAqNCeuz24WEewmXU4gpaV4FXiTqo8X3EGoq1A+OGNPN1MoLNLJSDAndTK021r95AP4ZT0uTPkQe0ydSAAAAAElFTkSuQmCC`),
		Actions: []plugin.QueryResultAction{
			{
				Name: "Copy to clipboard",
				Action: func(actionContext plugin.ActionContext) {
					clipboard.WriteText(strings.Join(filePaths, "\n"))
				},
			},
		},
	})
	return results
}
