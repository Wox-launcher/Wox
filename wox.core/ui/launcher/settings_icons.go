package launcher

import "sync"

var (
	settingNavIconPaths         map[string]string
	settingNavIconPathsOnce     sync.Once
	settingControlIconPaths     map[string]string
	settingControlIconPathsOnce sync.Once
)

// settingNavIconSource maps the Flutter rail's line-icon semantics onto portable monochrome SVGs.
func settingNavIconSource(id string) woxImage {
	const start = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">`
	const end = `</svg>`
	settingNavIconPathsOnce.Do(func() {
		settingNavIconPaths = map[string]string{
			"general":           `<path d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7z"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-2.83 2.83-.06-.06A1.7 1.7 0 0 0 15 19.4a1.7 1.7 0 0 0-1 .6 1.7 1.7 0 0 0-.4 1.1V21H9.6v-.1A1.7 1.7 0 0 0 8.5 19.4a1.7 1.7 0 0 0-1.88.34l-.06.06-2.83-2.83.06-.06A1.7 1.7 0 0 0 4.6 15a1.7 1.7 0 0 0-.6-1 1.7 1.7 0 0 0-1.1-.4H3V9.6h.1A1.7 1.7 0 0 0 4.6 8.5a1.7 1.7 0 0 0-.34-1.88l-.06-.06 2.83-2.83.06.06A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-.6 1.7 1.7 0 0 0 .4-1.1V3h4v.1A1.7 1.7 0 0 0 15.5 4.6a1.7 1.7 0 0 0 1.88-.34l.06-.06 2.83 2.83-.06.06A1.7 1.7 0 0 0 19.4 9c.4.28.75.62 1 .99.25.38.39.82.4 1.27v1.48c-.01.45-.15.9-.4 1.27-.25.37-.6.71-1 .99z"/>`,
			"ui":                `<path d="M12 3a9 9 0 1 0 0 18h1.5a1.5 1.5 0 0 0 0-3H12a1.5 1.5 0 0 1 0-3h2a7 7 0 0 0 7-7c0-2.76-4.03-5-9-5z"/><path d="M7.5 10.5h.01M9.5 6.5h.01M14.5 6.5h.01M17 10h.01"/>`,
			"ai":                `<path d="M9.5 18H8a4 4 0 0 1-4-4 3.5 3.5 0 0 1 2-3.15V9a4 4 0 0 1 7.5-1.94A3.5 3.5 0 0 1 20 9v1.85A3.5 3.5 0 0 1 19.5 17H18"/><path d="M9 13h6M10 17h4M11 21h2"/>`,
			"network":           `<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18"/>`,
			"data":              `<path d="M3 7.5h6l2-2h10v13H3z"/>`,
			"data.backup":       `<path d="M7 18h10a4 4 0 0 0 .6-7.96A6 6 0 0 0 6.2 8.6 4.7 4.7 0 0 0 7 18z"/><path d="m9 13 3-3 3 3M12 10v6"/>`,
			"data.cloudsync":    `<path d="M7 18h10a4 4 0 0 0 .6-7.96A6 6 0 0 0 6.2 8.6 4.7 4.7 0 0 0 7 18z"/>`,
			"plugins":           `<path d="M8.5 3v4H5a2 2 0 0 0-2 2v3.5h4a2 2 0 1 1 0 4H3V21h6a2 2 0 0 0 2-2v-3.5h3.5a2 2 0 1 0 4 0H21V9a2 2 0 0 0-2-2h-3.5V3a2 2 0 1 0-4 0z"/>`,
			"plugins.store":     `<path d="M6 8h12l1 13H5zM9 8V6a3 3 0 0 1 6 0v2"/>`,
			"plugins.installed": `<rect x="4" y="4" width="6" height="6"/><rect x="14" y="4" width="6" height="6"/><rect x="4" y="14" width="6" height="6"/><path d="M17 14v6M14 17h6"/>`,
			"plugins.runtime":   `<rect x="3" y="5" width="18" height="14" rx="2"/><path d="m7 10 2 2-2 2M12 15h4"/>`,
			"themes":            `<path d="M12 3a9 9 0 1 0 0 18h1.5a1.5 1.5 0 0 0 0-3H12a1.5 1.5 0 0 1 0-3h2a7 7 0 0 0 7-7c0-2.76-4.03-5-9-5z"/><path d="M7.5 10.5h.01M9.5 6.5h.01M14.5 6.5h.01M17 10h.01"/>`,
			"themes.store":      `<path d="M6 8h12l1 13H5zM9 8V6a3 3 0 0 1 6 0v2"/>`,
			"themes.installed":  `<path d="m4 20 5-5M14 4l6 6-9 9-6-6z"/>`,
			"themes.edit":       `<path d="M4 20h6M7 17v-7h10v7M9 10V6h6v4"/>`,
			"usage":             `<path d="M4 19V9M9 19V5M14 19v-7M19 19V3"/>`,
			"debug":             `<path d="M8 9h8M9 4h6l1 3H8zM6 12h12v5a6 6 0 0 1-12 0zM3 14h3M18 14h3M4 20l3-2M20 20l-3-2"/>`,
			"update":            `<path d="M20 11a8 8 0 1 0-2.34 5.66M20 4v7h-7"/>`,
			"privacy":           `<path d="M12 3 5 6v5c0 4.8 2.9 8.2 7 10 4.1-1.8 7-5.2 7-10V6z"/>`,
			"about":             `<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7h.01"/>`,
		}
	})
	path := settingNavIconPaths[id]
	if path == "" {
		return woxImage{}
	}
	return woxImage{ImageType: "svg", ImageData: start + path + end}
}

// settingControlIconSource returns Tabler line icons used by settings controls.
func settingControlIconSource(id string) woxImage {
	const start = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">`
	const end = `</svg>`
	settingControlIconPathsOnce.Do(func() {
		settingControlIconPaths = map[string]string{
			"add":                `<path d="M12 5v14M5 12h14"/>`,
			"undo":               `<path d="M9 7 5 11l4 4"/><path d="M5 11h8a6 6 0 0 1 6 6v1"/>`,
			"save":               `<path d="M5 4h12l2 2v14H5z"/><path d="M8 4v6h8V4M8 20v-6h8v6"/>`,
			"save-edit":          `<path d="M5 4h12l2 2v6M8 4v6h8V4"/><path d="m13 19 6-6 2 2-6 6h-2z"/>`,
			"search":             `<circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/>`,
			"locate":             `<circle cx="12" cy="12" r="3"/><circle cx="12" cy="12" r="8"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2"/>`,
			"check-circle":       `<circle cx="12" cy="12" r="9"/><path d="m8 12 3 3 5-6"/>`,
			"external":           `<path d="M14 5h5v5M19 5l-9 9"/><path d="M13 7H6a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h9a2 2 0 0 0 2-2v-7"/>`,
			"filter":             `<path d="M4 5h16l-6 7v6l-4 2v-8z"/>`,
			"inbox":              `<path d="M4 4h16v13a3 3 0 0 1-3 3H7a3 3 0 0 1-3-3V4z"/><path d="M4 13h3l3 3h4l3-3h3"/>`,
			"edit":               `<path d="M13.5 6.5l4 4M4 20h4l10.5-10.5a2.83 2.83 0 1 0-4-4L4 16v4z"/>`,
			"delete":             `<path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"/>`,
			"emoji":              `<circle cx="12" cy="12" r="9"/><path d="M8 14s1.5 2 4 2 4-2 4-2M9 9h.01M15 9h.01"/>`,
			"upload":             `<path d="M12 16V4M8 8l4-4 4 4M5 14v5h14v-5"/>`,
			"refresh":            `<path d="M20 11a8 8 0 1 0-2.34 5.66M20 4v7h-7"/>`,
			"key":                `<circle cx="8" cy="15" r="4"/><path d="m11 12 8-8M15 8l3 3M17 6l3 3"/>`,
			"onboarding":         `<path d="M4 5.5A3.5 3.5 0 0 1 7.5 2H11v18H7.5A3.5 3.5 0 0 0 4 23zM20 5.5A3.5 3.5 0 0 0 16.5 2H13v18h3.5a3.5 3.5 0 0 1 3.5 3z"/>`,
			"document":           `<path d="M6 2h8l4 4v16H6zM14 2v5h5M9 12h6M9 16h6"/>`,
			"code":               `<path d="m8 9-3 3 3 3M16 9l3 3-3 3"/>`,
			"checkbox.checked":   `<rect x="3" y="3" width="18" height="18" rx="2"/><path d="m8 12 3 3 5-6"/>`,
			"checkbox.unchecked": `<rect x="3" y="3" width="18" height="18" rx="2"/>`,
		}
	})
	path := settingControlIconPaths[id]
	if path == "" {
		return woxImage{}
	}
	return woxImage{ImageType: "svg", ImageData: start + path + end}
}
