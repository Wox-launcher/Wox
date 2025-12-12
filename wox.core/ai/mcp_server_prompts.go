package ai

import (
	"bytes"
	"io/fs"
	"sync"
	"text/template"
	"wox/resource"
)

var mcpPromptTemplateCache sync.Map // map[string]*template.Template

func renderMcpPrompt(templateFile string, data any) (string, error) {
	cacheKey := "prompts/" + templateFile
	if cached, ok := mcpPromptTemplateCache.Load(cacheKey); ok {
		var buf bytes.Buffer
		if err := cached.(*template.Template).Execute(&buf, data); err != nil {
			return "", err
		}
		return buf.String(), nil
	}

	b, err := resource.PromptsFS.ReadFile(cacheKey)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New(templateFile).Option("missingkey=zero").Parse(string(b))
	if err != nil {
		return "", err
	}

	mcpPromptTemplateCache.Store(cacheKey, tmpl)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func mustRenderMcpPrompt(templateFile string, data any, fallback string) string {
	out, err := renderMcpPrompt(templateFile, data)
	if err != nil {
		_ = err
		return fallback
	}
	return out
}

func renderMcpTemplateFromFS(cacheKey string, templatePath string, templateFS fs.FS, data any) (string, error) {
	if cached, ok := mcpPromptTemplateCache.Load(cacheKey); ok {
		var buf bytes.Buffer
		if err := cached.(*template.Template).Execute(&buf, data); err != nil {
			return "", err
		}
		return buf.String(), nil
	}

	b, err := fs.ReadFile(templateFS, templatePath)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New(templatePath).Option("missingkey=zero").Parse(string(b))
	if err != nil {
		return "", err
	}

	mcpPromptTemplateCache.Store(cacheKey, tmpl)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func mustRenderMcpTemplateFromScriptTemplates(templateFile string, data any, fallback string) string {
	cacheKey := "script_plugin_templates/" + templateFile
	templatePath := "script_plugin_templates/" + templateFile
	out, err := renderMcpTemplateFromFS(cacheKey, templatePath, resource.ScriptPluginTemplatesFS, data)
	if err != nil {
		_ = err
		return fallback
	}
	return out
}
