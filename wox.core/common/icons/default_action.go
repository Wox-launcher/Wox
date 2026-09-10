package icons

import "wox/common"

// Action verbs opt into the appearance variable. The Action Panel then tints
// only these SVGs to the row label color; brand and plugin icons stay authored.
func newActionIcon(paths string) common.WoxImage {
	return common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="var(--wox-theme-icon-color)" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`)
}

func newActionFillIcon(paths string) common.WoxImage {
	return common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="var(--wox-theme-icon-color)">` + paths + `</svg>`)
}

var defaultActionIcons = map[string]common.WoxImage{
	ActionInstall:              newActionIcon(`<path d="M12 4v12M8 12l4 4 4-4M5 20h14"/>`),
	ActionPin:                  newActionIcon(`<path d="m9 3 6 0-1 6 4 4H6l4-4zM12 13v6"/>`),
	ActionUnpin:                newActionIcon(`<path d="m9 3 6 0-1 6 4 4H6l4-4zM12 13v6M5 5l14 14"/>`),
	ActionRevertRanking:        newActionIcon(`<path d="M9 7 5 11l4 4"/><path d="M5 11h8a6 6 0 0 1 6 6v1"/>`),
	ActionQueryShortcut:        newActionIcon(`<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M6 9h.01M9 9h.01M12 9h.01M15 9h.01M18 9h.01M6 13h.01M9 13h.01M12 13h6"/>`),
	ActionOpenContainingFolder: newActionIcon(`<path d="M3 7h6l2 2h10v10H3z"/>`),
	ActionContextMenu:          newActionIcon(`<path d="M8 7h12M8 12h12M8 17h12M4 7h.01M4 12h.01M4 17h.01"/>`),
	ActionPreview:              newActionIcon(`<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>`),
	ActionHide:                 newActionIcon(`<path d="M3 3l18 18M10.6 10.6A3 3 0 0 0 13.4 13.4M9.9 5.1A11 11 0 0 1 12 5c6.5 0 10 7 10 7a18 18 0 0 1-3.2 4.1M6.1 6.1A18 18 0 0 0 2 12s3.5 7 10 7a11 11 0 0 0 3.1-.4"/>`),
	ActionAirdrop:              newActionIcon(`<circle cx="12" cy="12" r="2"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="10"/>`),
	ActionCopy:                 newActionIcon(`<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/>`),
	ActionOpen:                 newActionIcon(`<path d="M14 5h5v5M19 5l-9 9"/><path d="M13 7H6a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h9a2 2 0 0 0 2-2v-7"/>`),
	ActionTerminate:            newActionIcon(`<circle cx="12" cy="12" r="9"/><path d="m9 9 6 6M15 9l-6 6"/>`),
	ActionText:                 newActionIcon(`<path d="M6 3h8l4 4v14H6z"/><path d="M14 3v5h5M9 12h6M9 16h6"/>`),
	ActionError:                newActionIcon(`<circle cx="12" cy="12" r="9"/><path d="M12 8v5M12 16h.01"/>`),
	ActionCorrect:              newActionIcon(`<circle cx="12" cy="12" r="9"/><path d="m8 12 3 3 5-6"/>`),
	// The gear path is full-bleed in a 24 box. A wider viewBox insets it to the
	// optical size of other action verbs; stroke is scaled so line weight stays 1.8.
	ActionSettings: common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="-3 -3 30 30" fill="none" stroke="var(--wox-theme-icon-color)" stroke-width="2.25" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>`),
	ActionStar:                 newActionIcon(`<path d="m12 3 2.6 5.4 6 .9-4.3 4.2 1 5.9L12 16.8 6.7 19.4l1-5.9L3.4 9.3l6-.9z"/>`),
	ActionLock:                 newActionIcon(`<rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/>`),
	ActionDelete:               newActionIcon(`<path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"/>`),
	ActionExit:                 newActionIcon(`<path d="M12 4v7"/><path d="M8.2 6.8a6.5 6.5 0 1 0 7.6 0"/>`),
	ActionCPUProfile:           newActionIcon(`<rect x="7" y="7" width="10" height="10" rx="1"/><path d="M9 3v4M15 3v4M9 17v4M15 17v4M3 9h4M3 15h4M17 9h4M17 15h4"/>`),
	ActionSearch:               newActionIcon(`<circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/>`),
	ActionUpdate:               newActionIcon(`<path d="M20 11a8 8 0 1 0-2.34 5.66M20 4v7h-7"/>`),
	ActionRunAsAdministrator:   newActionIcon(`<path d="M12 3 5 6v5c0 4.8 2.9 8.2 7 10 4.1-1.8 7-5.2 7-10V6z"/>`),
	ActionExecute:              newActionIcon(`<path d="M13 2 4.5 13.5h5.5L9 22l10-13h-6z"/>`),
	ActionRun:                  newActionFillIcon(`<path d="M8 5.5v13l11-6.5z"/>`),
	ActionEdit:                 newActionIcon(`<path d="M13.5 6.5l4 4M4 20h4l10.5-10.5a2.83 2.83 0 1 0-4-4L4 16v4z"/>`),
	ActionUpgrade:              newActionIcon(`<circle cx="12" cy="12" r="9"/><path d="M12 16V8M8 11l4-4 4 4"/>`),
	ActionTooltip:              newActionIcon(`<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7h.01"/>`),
	ActionMultipleFiles:        newActionIcon(`<path d="M8 5h8l4 4v10H8z"/><path d="M16 5v4h4M4 9v12h12"/>`),
	ActionVolume:               newActionIcon(`<path d="M4 9h4l5-4v14l-5-4H4z"/><path d="M16 8.5a5 5 0 0 1 0 7"/>`),
	ActionVolumeUp:             newActionIcon(`<path d="M4 9h4l5-4v14l-5-4H4z"/><path d="M16 8.5a5 5 0 0 1 0 7M19 10v4M17 12h4"/>`),
	ActionVolumeDown:           newActionIcon(`<path d="M4 9h4l5-4v14l-5-4H4z"/><path d="M16 12h5"/>`),
	ActionMute:                 newActionIcon(`<path d="M4 9h4l5-4v14l-5-4H4z"/><path d="m16 9 5 5M21 9l-5 5"/>`),
	ActionPause:                newActionIcon(`<path d="M8 5v14M16 5v14"/>`),
	ActionSkipNext:             newActionIcon(`<path d="M5 6v12l9-6zM18 6v12"/>`),
	ActionSkipPrevious:         newActionIcon(`<path d="M19 6v12l-9-6zM6 6v12"/>`),
	ActionPaste:                newActionIcon(`<rect x="6" y="7" width="12" height="14" rx="2"/><path d="M9 7V5h6v2M9 13h6M9 17h4"/>`),
	ActionAdd:                  newActionIcon(`<path d="M12 5v14M5 12h14"/>`),
}
