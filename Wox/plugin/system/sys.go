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

var sysIcon = plugin.NewWoxImageSvg(`<svg t="1700799439280" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="5120" width="200" height="200"><path d="M914.032 867.569h-799.58c-25.398 0-45.969 20.683-45.969 46.216 0 25.534 20.571 46.215 45.968 46.215h799.581C939.428 960 960 939.319 960 913.785c0-25.415-20.572-46.216-45.968-46.216z" fill="#FA8B14" p-id="5121"></path><path d="M102.73 791.32h814.175c21.38 0 38.73-17.45 38.73-38.939V102.938c0-21.495-17.35-38.938-38.73-38.938H102.73C81.35 64 64 81.443 64 102.938v649.443c0.112 21.489 17.35 38.938 38.73 38.938z m224.708-379.778l-34.129-46.91c-6.323-8.784-7.013-20.45-1.61-29.81l30.223-52.686c5.404-9.359 15.745-14.674 26.435-13.517l57.572 6.24c11.834 1.269 22.984-5.203 27.81-16.175l23.331-53.262c4.366-9.94 14.135-16.292 24.82-16.292h60.563c10.8 0 20.57 6.352 24.824 16.292l23.326 53.262c4.832 10.86 16.094 17.444 27.816 16.174l57.572-6.239c10.689-1.157 21.144 4.158 26.429 13.517l30.228 52.687c5.398 9.358 4.709 21.025-1.61 29.809l-34.017 46.91c-7.007 9.59-7.007 22.644 0 32.348l34.129 46.91c6.325 8.784 7.014 20.45 1.61 29.81l-30.223 52.686c-5.402 9.359-15.745 14.674-26.428 13.517l-57.578-6.24c-11.834-1.269-22.984 5.201-27.81 16.175l-23.332 53.267c-4.366 9.935-14.135 16.287-24.819 16.287h-60.45c-10.803 0-20.57-6.352-24.819-16.287l-23.333-53.267c-4.825-10.861-16.087-17.444-27.809-16.174l-57.572 6.239c-10.69 1.157-21.15-4.158-26.434-13.517l-30.224-52.687c-5.403-9.359-4.713-21.025 1.612-29.809l34.126-46.91c6.78-9.704 6.78-22.758-0.23-32.348z m100.613 16.174c0 48.178 38.842 87.229 86.763 87.229 47.92 0 86.762-39.05 86.762-87.23 0-48.179-38.842-87.23-86.762-87.23-47.921 0-86.763 39.051-86.763 87.23z" fill="#2075CC" p-id="5122"></path></svg>`)
var sysSettingIcon = plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" width="48" height="48" viewBox="0 0 48 48"><circle cx="24" cy="24" r="20" fill="#616161"></circle><path fill="#d1d1d1" d="M34.139,17.887l-0.955,0.265c-0.453,0.126-0.812-0.388-0.539-0.77l0.576-0.807 c0.123-0.2-0.08-0.441-0.298-0.355l-0.838,0.4c-0.443,0.211-0.906-0.251-0.694-0.694l0.4-0.838 c0.086-0.218-0.155-0.421-0.355-0.298l-0.807,0.576c-0.383,0.273-0.896-0.086-0.77-0.539l0.265-0.955 c0.047-0.23-0.226-0.387-0.401-0.232l-0.695,0.707c-0.329,0.335-0.897,0.07-0.852-0.397l0.095-0.987 c0.006-0.235-0.29-0.342-0.436-0.159l-0.562,0.817c-0.266,0.387-0.871,0.225-0.908-0.243l-0.078-0.988 c-0.034-0.232-0.345-0.287-0.456-0.08l-0.411,0.902c-0.195,0.428-0.819,0.373-0.937-0.082l-0.248-0.96 c-0.074-0.223-0.389-0.223-0.463,0l-0.248,0.96c-0.118,0.455-0.742,0.51-0.937,0.082l-0.411-0.902 c-0.112-0.206-0.422-0.152-0.456,0.08l-0.078,0.988c-0.037,0.469-0.642,0.631-0.908,0.243l-0.562-0.817 c-0.146-0.184-0.442-0.076-0.436,0.158l0.095,0.987c0.045,0.468-0.523,0.733-0.852,0.397l-0.695-0.707 c-0.176-0.156-0.448,0.002-0.401,0.232l0.265,0.955c0.126,0.453-0.388,0.812-0.77,0.539l-0.807-0.576 c-0.2-0.123-0.441,0.08-0.355,0.298l0.427,0.895c0.202,0.424-0.241,0.867-0.665,0.665l-0.895-0.427 c-0.218-0.086-0.421,0.155-0.298,0.355l0.576,0.807c0.273,0.383-0.086,0.896-0.539,0.77l-0.955-0.265 c-0.23-0.047-0.387,0.226-0.232,0.401l0.707,0.695c0.335,0.329,0.07,0.897-0.397,0.852l-0.987-0.095 c-0.235-0.006-0.342,0.29-0.159,0.435l0.817,0.562c0.387,0.266,0.225,0.871-0.243,0.908l-0.988,0.078 c-0.232,0.034-0.287,0.345-0.08,0.456l0.902,0.411c0.428,0.195,0.373,0.819-0.082,0.937l-0.96,0.248 c-0.223,0.074-0.223,0.389,0,0.463l0.96,0.248c0.455,0.118,0.51,0.742,0.082,0.937l-0.902,0.411 c-0.206,0.112-0.152,0.422,0.08,0.456l0.988,0.078c0.469,0.037,0.631,0.642,0.243,0.908l-0.817,0.562 c-0.184,0.146-0.076,0.442,0.158,0.436l0.987-0.095c0.468-0.045,0.733,0.523,0.397,0.852l-0.707,0.695 c-0.156,0.176,0.002,0.448,0.232,0.401l0.955-0.265c0.453-0.126,0.812,0.388,0.539,0.77l-0.576,0.807 c-0.123,0.2,0.08,0.441,0.298,0.355l0.895-0.427c0.424-0.202,0.867,0.241,0.665,0.665l-0.427,0.895 c-0.086,0.218,0.155,0.421,0.355,0.298l0.807-0.576c0.383-0.273,0.896,0.086,0.77,0.539l-0.265,0.956 c-0.047,0.23,0.226,0.387,0.401,0.232l0.695-0.707c0.329-0.335,0.897-0.07,0.852,0.397l-0.095,0.987 c-0.006,0.235,0.29,0.342,0.436,0.159l0.562-0.817c0.266-0.387,0.871-0.225,0.908,0.243l0.078,0.988 c0.034,0.232,0.345,0.287,0.456,0.08l0.411-0.902c0.195-0.428,0.819-0.373,0.937,0.082l0.248,0.96 c0.074,0.223,0.389,0.223,0.463,0l0.248-0.96c0.118-0.455,0.742-0.51,0.937-0.082l0.411,0.902c0.112,0.206,0.422,0.152,0.456-0.08 l0.078-0.988c0.037-0.469,0.642-0.631,0.908-0.243l0.562,0.817c0.146,0.184,0.442,0.076,0.436-0.158l-0.095-0.987 c-0.045-0.468,0.523-0.733,0.852-0.397l0.695,0.707c0.176,0.156,0.448-0.002,0.401-0.232l-0.265-0.956 c-0.126-0.453,0.388-0.812,0.77-0.539l0.807,0.576c0.2,0.123,0.441-0.08,0.355-0.298l-0.427-0.895 c-0.202-0.424,0.241-0.867,0.665-0.665l0.895,0.427c0.218,0.086,0.421-0.155,0.298-0.355l-0.576-0.807 c-0.273-0.383,0.086-0.896,0.539-0.77l0.955,0.265c0.23,0.047,0.387-0.226,0.232-0.401l-0.707-0.695 c-0.335-0.329-0.07-0.897,0.397-0.852l0.987,0.095c0.235,0.006,0.342-0.29,0.159-0.436l-0.817-0.562 c-0.387-0.266-0.225-0.871,0.243-0.908l0.988-0.078c0.232-0.034,0.287-0.345,0.08-0.456l-0.902-0.411 c-0.428-0.195-0.373-0.819,0.082-0.937l0.96-0.248c0.223-0.074,0.223-0.389,0-0.463l-0.96-0.248 c-0.455-0.118-0.51-0.742-0.082-0.937l0.902-0.411c0.206-0.112,0.152-0.422-0.08-0.456l-1.054-0.083 c-0.447-0.035-0.601-0.612-0.232-0.866l0.871-0.599c0.184-0.146,0.076-0.442-0.158-0.436l-0.987,0.095 c-0.468,0.045-0.733-0.523-0.397-0.852l0.707-0.695C34.527,18.113,34.369,17.84,34.139,17.887z M23.496,23.135 c0.48-0.277,1.093-0.113,1.37,0.367c0.277,0.48,0.113,1.093-0.367,1.37c-0.48,0.277-1.093,0.113-1.37-0.367 C22.852,24.025,23.016,23.412,23.496,23.135z M17.477,30.691c-0.601-0.582-1.133-1.254-1.572-2.015 c-2.181-3.778-1.418-8.441,1.57-11.352c0.616-0.6,1.642-0.438,2.073,0.307l3.022,5.234c0.407,0.705,0.407,1.573,0,2.277 l-3.024,5.237C19.116,31.123,18.094,31.288,17.477,30.691z M33.047,26.328c-0.603,2.348-2.113,4.46-4.378,5.768 c-2.265,1.308-4.849,1.559-7.184,0.907c-0.835-0.233-1.218-1.204-0.785-1.955l2.797-4.845c0.547-0.948,1.559-1.532,2.653-1.532 l5.595,0C32.613,24.671,33.263,25.488,33.047,26.328z M33.049,21.701c0.209,0.833-0.445,1.635-1.304,1.635l-6.047,0 c-0.814,0-1.565-0.434-1.972-1.139l-3.022-5.234c-0.43-0.745-0.058-1.715,0.77-1.949c4.015-1.132,8.435,0.538,10.616,4.317 C32.529,20.092,32.845,20.889,33.049,21.701z"></path><path fill="#bdbdbd" d="M39.191,14.837l-1.431,0.397c-0.679,0.188-1.217-0.581-0.808-1.154l0.863-1.209 c0.184-0.3-0.119-0.661-0.446-0.532l-1.256,0.599c-0.664,0.317-1.357-0.377-1.04-1.041l0.599-1.256 c0.129-0.327-0.232-0.63-0.532-0.446l-1.21,0.863c-0.573,0.409-1.342-0.13-1.154-0.808l0.397-1.432 c0.07-0.345-0.338-0.581-0.601-0.347l-1.041,1.06c-0.494,0.502-1.344,0.106-1.277-0.595l0.142-1.479 c0.01-0.352-0.434-0.513-0.653-0.237L28.9,8.442c-0.399,0.58-1.306,0.337-1.361-0.365l-0.117-1.481 c-0.052-0.348-0.516-0.43-0.684-0.121l-0.616,1.352c-0.292,0.641-1.227,0.559-1.403-0.123l-0.372-1.438 c-0.111-0.334-0.583-0.334-0.694,0l-0.372,1.438c-0.176,0.682-1.111,0.764-1.403,0.123l-0.616-1.352 c-0.167-0.309-0.632-0.227-0.684,0.121l-0.117,1.481C20.405,8.78,19.499,9.023,19.1,8.442l-0.841-1.224 c-0.219-0.276-0.662-0.114-0.653,0.237l0.142,1.479c0.067,0.701-0.783,1.098-1.277,0.595L15.43,8.47 c-0.263-0.233-0.672,0.003-0.601,0.347l0.397,1.432c0.188,0.678-0.581,1.217-1.154,0.808l-1.209-0.863 c-0.3-0.184-0.661,0.119-0.532,0.446l0.639,1.341c0.303,0.636-0.361,1.299-0.996,0.996l-1.341-0.639 c-0.327-0.129-0.63,0.232-0.446,0.532l0.863,1.21c0.409,0.573-0.13,1.342-0.808,1.154l-1.432-0.397 c-0.345-0.07-0.581,0.338-0.347,0.601l1.059,1.041c0.502,0.494,0.106,1.344-0.595,1.277l-1.479-0.142 c-0.352-0.01-0.513,0.434-0.238,0.652l1.224,0.841c0.58,0.399,0.337,1.306-0.365,1.361l-1.481,0.117 c-0.348,0.052-0.43,0.516-0.121,0.684l1.352,0.616c0.641,0.292,0.559,1.227-0.123,1.403l-1.438,0.372 c-0.334,0.111-0.334,0.583,0,0.694l1.438,0.372c0.682,0.176,0.764,1.111,0.123,1.403l-1.352,0.616 c-0.309,0.167-0.227,0.632,0.121,0.684l1.481,0.117c0.702,0.055,0.945,0.962,0.365,1.361L7.21,29.75 c-0.276,0.219-0.114,0.662,0.237,0.653l1.479-0.142c0.701-0.067,1.098,0.783,0.595,1.277l-1.059,1.041 c-0.233,0.263,0.003,0.672,0.347,0.601l1.431-0.397c0.679-0.188,1.217,0.581,0.808,1.154l-0.863,1.209 c-0.184,0.3,0.119,0.661,0.446,0.532l1.341-0.639c0.636-0.303,1.299,0.361,0.996,0.996l-0.639,1.341 c-0.129,0.327,0.232,0.63,0.532,0.446l1.209-0.863c0.573-0.409,1.342,0.13,1.154,0.808l-0.397,1.432 c-0.071,0.345,0.338,0.581,0.601,0.347l1.041-1.06c0.494-0.502,1.344-0.106,1.277,0.595l-0.142,1.479 c-0.01,0.352,0.434,0.513,0.653,0.237l0.841-1.224c0.399-0.58,1.306-0.337,1.361,0.365l0.117,1.481 c0.052,0.348,0.516,0.43,0.684,0.121l0.616-1.352c0.292-0.641,1.227-0.559,1.403,0.123l0.372,1.438 c0.111,0.334,0.583,0.334,0.694,0l0.372-1.438c0.176-0.682,1.111-0.763,1.403-0.123l0.616,1.352 c0.168,0.309,0.632,0.227,0.684-0.121l0.117-1.481c0.055-0.702,0.962-0.945,1.361-0.365l0.842,1.224 c0.219,0.276,0.662,0.114,0.653-0.237l-0.142-1.479c-0.067-0.701,0.783-1.098,1.277-0.595l1.041,1.06 c0.263,0.233,0.672-0.003,0.601-0.347l-0.397-1.432c-0.188-0.678,0.581-1.217,1.154-0.808l1.21,0.863 c0.3,0.184,0.661-0.119,0.532-0.446l-0.639-1.341c-0.303-0.636,0.361-1.299,0.996-0.996l1.341,0.639 c0.327,0.129,0.63-0.232,0.446-0.532l-0.863-1.21c-0.409-0.573,0.13-1.342,0.808-1.154l1.432,0.397 c0.345,0.07,0.581-0.338,0.347-0.601l-1.06-1.041c-0.502-0.494-0.105-1.344,0.595-1.277l1.479,0.142 c0.352,0.01,0.513-0.434,0.237-0.653l-1.224-0.842c-0.58-0.399-0.337-1.306,0.365-1.361l1.481-0.117 c0.348-0.052,0.43-0.516,0.121-0.684l-1.352-0.616c-0.641-0.292-0.559-1.227,0.123-1.403l1.438-0.372 c0.334-0.111,0.334-0.583,0-0.694l-1.438-0.372c-0.682-0.176-0.764-1.111-0.123-1.403l1.352-0.616 c0.309-0.167,0.227-0.632-0.121-0.684l-1.579-0.124c-0.669-0.053-0.901-0.917-0.348-1.297l1.306-0.897 c0.276-0.219,0.114-0.662-0.237-0.653l-1.479,0.142c-0.701,0.067-1.098-0.783-0.595-1.277l1.06-1.041 C39.771,15.175,39.536,14.766,39.191,14.837z M23.496,23.135c0.478-0.276,1.09-0.112,1.366,0.366 c0.276,0.478,0.112,1.09-0.366,1.366s-1.09,0.112-1.366-0.366C22.854,24.023,23.018,23.411,23.496,23.135z M14.227,34.02 c-0.9-0.872-1.698-1.879-2.356-3.019c-3.268-5.661-2.125-12.647,2.352-17.009c0.924-0.9,2.461-0.656,3.105,0.461l4.528,7.842 c0.61,1.056,0.61,2.356,0,3.412l-4.53,7.847C16.683,34.668,15.151,34.915,14.227,34.02z M37.555,27.484 c-0.903,3.517-3.165,6.683-6.559,8.642s-7.266,2.336-10.763,1.359c-1.251-0.349-1.825-1.804-1.176-2.929l4.191-7.259 c0.82-1.42,2.335-2.295,3.975-2.295l8.383,0C36.905,25.001,37.878,26.225,37.555,27.484z M37.557,20.551 c0.313,1.247-0.667,2.45-1.953,2.45l-9.06,0c-1.219,0-2.346-0.65-2.955-1.706l-4.528-7.842c-0.645-1.117-0.087-2.57,1.154-2.919 c6.016-1.697,12.637,0.806,15.906,6.468C36.778,18.141,37.252,19.335,37.557,20.551z"></path></svg>`)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &SysPlugin{})
}

type SysPlugin struct {
	api      plugin.API
	commands []SysCommand
}

type SysCommand struct {
	Title                  string
	SubTitle               string
	Icon                   plugin.WoxImage
	PreventHideAfterAction bool
	Action                 func(ctx context.Context, actionContext plugin.ActionContext)
}

func (r *SysPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "227f7d64-df08-4e35-ad05-98a26d540d06",
		Name:          "System Commands",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Provide System related commands. e.g. shutdown,lock,setting etc.",
		Icon:          sysIcon.String(),
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
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if util.IsMacOS() {
					util.ShellRun("osascript", "-e", "tell application \"System Events\" to keystroke \"q\" using {control down, command down}")
				}
			},
		},
		{
			Title: "i18n:plugin_sys_empty_trash",
			Icon:  plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAABKklEQVR4nO2XMW7CMBiFfY+uFXcpc+kJuEAr/1FHhiKxN1YXzI4QS0J3DhCrnQkTrEWCExglVRYUA5LzGxG9T3rK8GTpf/bL8AsBAACtQZEZKDL2nGKZjcS9Dq9uFSKdrLuJzrepzm0IJTrfJnr91FiAkMOnlcarTWMBgg+v/4UAp1z7k/pKcIEAhBfwo3UV+ugvbfTybYf9pa3zqLco5fIjx9lgAYrh5HNafl3eJZ9qvGABqgELnfMu+QoBrgQvQKiQH6gQoUJ+oEKECvmBChEq5AcqRKiQH62rEN37SjksF/NFuaDXedVS7/Ijx9lgAbgkGAMcuIePpdmzBYhlNmd/AZnN2AJ8vv12lDR/bLdPZhe/m0fBydfrz4MiM224Tofi5tmHB6CFHAGn3ZbcU2hBbwAAAABJRU5ErkJggg==`),
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if util.IsMacOS() {
					util.ShellRun("osascript", "-e", "tell application \"Finder\" to empty trash")
				}
			},
		},
		{
			Title: "i18n:plugin_sys_quit_wox",
			Icon:  plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADIAAAAyCAYAAAAeP4ixAAAACXBIWXMAAAsTAAALEwEAmpwYAAABgklEQVR4nO2ZTU7DQAyF5zYgxE0AET+pJ0KVOEA5DtDTIFhCu+i8LkBTdVFVpfMTTxzCWLKajSt/85yxIzvXrFmzItt03b0Hll5kTeDb0r3I2ou8boC7LAgPPFonz9+ggHmyEtbJMuLbrrtNUWNpnSjjpfYSBxFZWSfKeHl9RUGsk2SiNxCOQAU2RWB/8myKIPsqfPLAe+nJhtjwH6aKeGCxmwZms6sSmBATYveNeGEJ8uFFrvcwFx54K4oFLnNiWaOPlMD0hWCthpgDowHBmp09BUYLgrVHlHMwmhAcYtY6BaMNwaGGxsNrNfwePvfpORwa5FgZTSVoDpLZZzgGkEmUlj/xYveZAGgBcu520oZxtUBSrlhNGFcDJKdPaME4bZCSZqcB4zRB+nTsvjBOGWQaH1acyqcuDd01ENirwKYI7E+e/14R/xcWPSKf0VsrbE9HDwI8R0HCCtg6UUZ8K3ITBdmpAsxHXFYPLsfCCjhsT8fwzniRVSinZCWaNWvmju0Hs8M8e1QllsgAAAAASUVORK5CYII=`),
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				share.ExitApp(ctx)
			},
		},
		{
			Title:                  "i18n:plugin_sys_open_wox_preferences",
			PreventHideAfterAction: true,
			Icon:                   sysSettingIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				plugin.GetPluginManager().GetUI().OpenSettingWindow(ctx)
			},
		},
		{
			Title:                  "i18n:plugin_sys_open_wox_settings",
			PreventHideAfterAction: true,
			Icon:                   sysSettingIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				plugin.GetPluginManager().GetUI().OpenSettingWindow(ctx)
			},
		},
		{
			Title: "i18n:plugin_sys_open_system_settings",
			Icon:  sysSettingIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if util.IsMacOS() {
					util.ShellRun("open", "-a", "System Preferences")
				}
				if util.IsWindows() {
					util.ShellRun("desk.cpl")
				}
			},
		},
	}

	if util.IsDev() {
		r.commands = append(r.commands, SysCommand{
			Title: "Performance CPU profiling",
			Icon:  plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAGAAAABgCAYAAADimHc4AAAACXBIWXMAAAsTAAALEwEAmpwYAAACvUlEQVR4nO2cPW4UQRCFO4Gz8BM6GBNAOrKndruE5CsgxFXAixGJyRwQIdQNIXAMm4MAzoxUqDbjx6zk1eyrmX6fVOFqe96rqp6eGVVKhBBCCCGEEDIydprsf5GC/37yoAU0GkADoKAz2FgBNAAKOoONFUADoKAz2OZeAYuPi/tS9OVQ9EKKXkpVm3QUvfRrGYqulh+W91JUjt4d3ZaaXw81/4SLVscJv7ah5ld7b57cSuHEL/oFLZDszojPoUzwzEeLIjuPfJKi9Pw5tx25LopeHRa9GyH7T+BiVEwMNR+j9U9D0a9oIQQW+Rytfxpq/tFsBRT9jtY/oUUQcKD1pwFo0BkorAC8CDLnFrTp4RVaAAHH6A/3aIDSAAmQ6ayAihebLajiBeceUGPFpDfhw7dqj55l25dsXb/b8P98+FTt4Cy3aYCLv7/cvfB/GbHM67WENWATN124Z37Xxwhfy02vY3SBxzIA0Xa666pg0aABaNG7P4IG9DSAFVDZgowtiHuAcRPuuQmPfhCby12QTfUkjBa8owF40VkBPV54tqA+RnAP6GkAN+HKuyBjC+JJ2HgS7qezCSc0rR/EEhoaAIYGgKEBEzXAX4R3fYxo8qW8fxTVBRA//GcpYz2OPjhTe5BjfJjla2nuWdBvnyYuMG3H/3sb8SdvgMwgaEClAfAsFFYAXghhC8KLIYDgHlBnbsAm0AIIOEYXmAYoDZAAmc4KqHixQ7YgH1qEFkFQUfRbBAMu4ELUlkeWFV3hhVBIDEWfo/VPPs631bGV8v7xnRQBH+fbYPavUhR8jK+P821G/Jo/hRpd7PiC1kNcvTQDiCRjRNErz/xw4v9jTzj2O4Q5zBQd1teQz6XoixCjirdl24dXBv795EELaDSABkBBZ7CxAmgAFHQGGyuABkBBZ7C1XgGEEEIIIYSQhOYXgxYY872M2ekAAAAASUVORK5CYII=`),
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
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
						Name:                   "i18n:plugin_sys_execute",
						Action:                 command.Action,
						PreventHideAfterAction: command.PreventHideAfterAction,
					},
				},
			})
		}
	}
	return
}
