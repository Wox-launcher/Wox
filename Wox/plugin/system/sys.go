package system

import (
	"context"
	"os"
	"path"
	"runtime/pprof"
	"time"
	"wox/i18n"
	"wox/plugin"
	"wox/share"
	"wox/util"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &SysPlugin{})
}

type SysPlugin struct {
	api      plugin.API
	commands []SysCommand
}

type SysCommand struct {
	Title    string
	SubTitle string
	Icon     plugin.WoxImage
	Action   func(actionContext plugin.ActionContext)
}

func (r *SysPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "227f7d64-df08-4e35-ad05-98a26d540d06",
		Name:          "System Commands",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Provide System related commands. e.g. shutdown,lock,setting etc.",
		Icon:          "",
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (r *SysPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API
	r.commands = []SysCommand{
		{
			Title: "i18n:plugin_sys_lock_computer",
			Icon:  plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAABzElEQVR4nO2Yv0oDQRDGPwQrfQMt1VY0oHZ3MyGtNnkOIX0gnX+4tfDPG2gj4nPYmFqxSaOdARMF4x64MnhoXDVG73KXM/vBwLHswPfbmdnjDnByckpMhUJhnJnLRHTEzBdE9MDM99GzrJVlD4ZRvu+vEdEVM5teIXtkL4ZFtVptjJk3fzJuxTMzb0hu1v7xB/PdsZGpeWmF6DS7W6TDzHu+7y+XSqUJCc/zVohon4ie7EoQ0Wom5mUY7Z4nomsimv8up1gsLhDRjT0ThSwGW24U++R7me/KW/yiEmWkreha7Dax+4vcAwv+EGmLmS8tgKV+c2UmrNxLpK3oBfVmwvO8yX5zZa8FcI+0ZV+JaefHlgNgV4H+ZbYwrRVOtEI7VDCDDK3Q1gqnnW3MJmc+QHPQxkMbJEDT7GAqNoCcfNrmw/dqHCcB0M4Q4C42QFbmwygcQOgqANdCsTSSLaQVTKMKc16BqVden3WeABpVmLP1jyFruQE4r3wGkLXcANS/AKjnCaCR9xbSEUQ9r0McJhhwAMpVwLgWGukW0gFauf6k1AqnGQKcxAboKMxphdvUzQdoPu5gBklI/s/IL4402kkHaMnJJ2beyemf6wXITRX/xbt3RgAAAABJRU5ErkJggg==`),
			Action: func(actionContext plugin.ActionContext) {
				if util.IsMacOS() {
					util.ShellRun("osascript", "-e", "tell application \"System Events\" to keystroke \"q\" using {control down, command down}")
				}
			},
		},
		{
			Title: "i18n:plugin_sys_empty_trash",
			Icon:  plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAABKklEQVR4nO2XMW7CMBiFfY+uFXcpc+kJuEAr/1FHhiKxN1YXzI4QS0J3DhCrnQkTrEWCExglVRYUA5LzGxG9T3rK8GTpf/bL8AsBAACtQZEZKDL2nGKZjcS9Dq9uFSKdrLuJzrepzm0IJTrfJnr91FiAkMOnlcarTWMBgg+v/4UAp1z7k/pKcIEAhBfwo3UV+ugvbfTybYf9pa3zqLco5fIjx9lgAYrh5HNafl3eJZ9qvGABqgELnfMu+QoBrgQvQKiQH6gQoUJ+oEKECvmBChEq5AcqRKiQH62rEN37SjksF/NFuaDXedVS7/Ijx9lgAbgkGAMcuIePpdmzBYhlNmd/AZnN2AJ8vv12lDR/bLdPZhe/m0fBydfrz4MiM224Tofi5tmHB6CFHAGn3ZbcU2hBbwAAAABJRU5ErkJggg==`),
			Action: func(actionContext plugin.ActionContext) {
				if util.IsMacOS() {
					util.ShellRun("osascript", "-e", "tell application \"Finder\" to empty trash")
				}
			},
		},
		{
			Title: "i18n:plugin_sys_quit_wox",
			Icon:  plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADIAAAAyCAYAAAAeP4ixAAAACXBIWXMAAAsTAAALEwEAmpwYAAABgklEQVR4nO2ZTU7DQAyF5zYgxE0AET+pJ0KVOEA5DtDTIFhCu+i8LkBTdVFVpfMTTxzCWLKajSt/85yxIzvXrFmzItt03b0Hll5kTeDb0r3I2ou8boC7LAgPPFonz9+ggHmyEtbJMuLbrrtNUWNpnSjjpfYSBxFZWSfKeHl9RUGsk2SiNxCOQAU2RWB/8myKIPsqfPLAe+nJhtjwH6aKeGCxmwZms6sSmBATYveNeGEJ8uFFrvcwFx54K4oFLnNiWaOPlMD0hWCthpgDowHBmp09BUYLgrVHlHMwmhAcYtY6BaMNwaGGxsNrNfwePvfpORwa5FgZTSVoDpLZZzgGkEmUlj/xYveZAGgBcu520oZxtUBSrlhNGFcDJKdPaME4bZCSZqcB4zRB+nTsvjBOGWQaH1acyqcuDd01ENirwKYI7E+e/14R/xcWPSKf0VsrbE9HDwI8R0HCCtg6UUZ8K3ITBdmpAsxHXFYPLsfCCjhsT8fwzniRVSinZCWaNWvmju0Hs8M8e1QllsgAAAAASUVORK5CYII=`),
			Action: func(actionContext plugin.ActionContext) {
				share.ExitApp(ctx)
			},
		},
	}

	r.commands = append(r.commands, SysCommand{
		Title: "Performance CPU profiling",
		Icon:  plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAGAAAABgCAYAAADimHc4AAAACXBIWXMAAAsTAAALEwEAmpwYAAACvUlEQVR4nO2cPW4UQRCFO4Gz8BM6GBNAOrKndruE5CsgxFXAixGJyRwQIdQNIXAMm4MAzoxUqDbjx6zk1eyrmX6fVOFqe96rqp6eGVVKhBBCCCGEEDIydprsf5GC/37yoAU0GkADoKAz2FgBNAAKOoONFUADoKAz2OZeAYuPi/tS9OVQ9EKKXkpVm3QUvfRrGYqulh+W91JUjt4d3ZaaXw81/4SLVscJv7ah5ld7b57cSuHEL/oFLZDszojPoUzwzEeLIjuPfJKi9Pw5tx25LopeHRa9GyH7T+BiVEwMNR+j9U9D0a9oIQQW+Rytfxpq/tFsBRT9jtY/oUUQcKD1pwFo0BkorAC8CDLnFrTp4RVaAAHH6A/3aIDSAAmQ6ayAihebLajiBeceUGPFpDfhw7dqj55l25dsXb/b8P98+FTt4Cy3aYCLv7/cvfB/GbHM67WENWATN124Z37Xxwhfy02vY3SBxzIA0Xa666pg0aABaNG7P4IG9DSAFVDZgowtiHuAcRPuuQmPfhCby12QTfUkjBa8owF40VkBPV54tqA+RnAP6GkAN+HKuyBjC+JJ2HgS7qezCSc0rR/EEhoaAIYGgKEBEzXAX4R3fYxo8qW8fxTVBRA//GcpYz2OPjhTe5BjfJjla2nuWdBvnyYuMG3H/3sb8SdvgMwgaEClAfAsFFYAXghhC8KLIYDgHlBnbsAm0AIIOEYXmAYoDZAAmc4KqHixQ7YgH1qEFkFQUfRbBAMu4ELUlkeWFV3hhVBIDEWfo/VPPs631bGV8v7xnRQBH+fbYPavUhR8jK+P821G/Jo/hRpd7PiC1kNcvTQDiCRjRNErz/xw4v9jTzj2O4Q5zBQd1teQz6XoixCjirdl24dXBv795EELaDSABkBBZ7CxAmgAFHQGGyuABkBBZ7C1XgGEEEIIIYSQhOYXgxYY872M2ekAAAAASUVORK5CYII=`),
		Action: func(actionContext plugin.ActionContext) {
			cpuProfPath := path.Join(util.GetLocation().GetWoxDataDirectory(), "cpu.prof")
			f, err := os.Create(cpuProfPath)
			if err != nil {
				util.GetLogger().Info(ctx, "failed to create cpu profile file: "+err.Error())
				return
			}

			util.GetLogger().Info(ctx, "start cpu profile")
			profileErr := pprof.StartCPUProfile(f)
			if profileErr != nil {
				util.GetLogger().Info(ctx, "failed to start cpu profile: "+profileErr.Error())
				return
			}

			time.AfterFunc(30*time.Second, func() {
				pprof.StopCPUProfile()
				util.GetLogger().Info(ctx, "cpu profile saved to "+cpuProfPath)
			})
		},
	})
}

func (r *SysPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	for _, command := range r.commands {
		translatedTitle := i18n.GetI18nManager().TranslateWox(ctx, command.Title)
		isTitleMatch, titleScore := IsStringMatchScore(ctx, translatedTitle, query.Search)
		if !isTitleMatch {
			translatedTitleEnUs := i18n.GetI18nManager().TranslateWoxEnUs(ctx, command.Title)
			isTitleMatch, titleScore = IsStringMatchScore(ctx, translatedTitleEnUs, query.Search)
		}

		if isTitleMatch {
			results = append(results, plugin.QueryResult{
				Title:    command.Title,
				SubTitle: command.SubTitle,
				Score:    titleScore,
				Icon:     command.Icon,
				Actions: []plugin.QueryResultAction{
					{
						Name:   "i18n:plugin_sys_execute",
						Action: command.Action,
					},
				},
			})
		}
	}
	return
}
