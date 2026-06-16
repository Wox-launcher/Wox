package sys

import "wox/common"

var (
	sysVolumeIcon       = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#4f7cff" d="M4 9v6h4l5 4V5L8 9z"/><path fill="#4f7cff" d="M16.5 8.5a5 5 0 0 1 0 7l1.4 1.4a7 7 0 0 0 0-9.8z"/></svg>`)
	sysVolumeUpIcon     = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#21bf4b" d="M4 9v6h4l5 4V5L8 9z"/><path fill="#21bf4b" d="M18 10V7h-2v3h-3v2h3v3h2v-3h3v-2z"/></svg>`)
	sysVolumeDownIcon   = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#ff9800" d="M4 9v6h4l5 4V5L8 9z"/><path fill="#ff9800" d="M15 10h6v2h-6z"/></svg>`)
	sysMuteIcon         = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#8b95a5" d="M4 9v6h4l5 4V5L8 9z"/><path stroke="#d94b4b" stroke-linecap="round" stroke-width="2" d="m16 9 5 5m0-5-5 5"/></svg>`)
	sysSleepIcon        = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#8b95a5" d="M12 3a9 9 0 1 0 8.4 12.2A7 7 0 0 1 8.8 3.6A9 9 0 0 0 12 3z"/></svg>`)
	sysDisplaySleepIcon = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="13" rx="2" fill="#8b95a5"/><path fill="#fff" d="M8 19h8v2H8z"/><path fill="#fff" d="M13 8h4l-5 7v-5H8l5-6z"/></svg>`)
	sysLogoutIcon       = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#da0e0e" d="M5 3h9v2H7v14h7v2H5z"/><path fill="#da0e0e" d="m16 7 5 5-5 5v-4H10v-2h6z"/></svg>`)
	sysEjectIcon        = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#4f7cff" d="m12 4 8 10H4z"/><path fill="#4f7cff" d="M4 18h16v2H4z"/></svg>`)
	sysDesktopIcon      = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="13" rx="2" fill="#4f7cff"/><path fill="#dbe6ff" d="M8 19h8v2H8z"/></svg>`)
	sysScreenSaverIcon  = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="16" rx="3" fill="#191919"/><path fill="#8b5cf6" d="M7 14c4-8 6 8 10 0 1.2-2.4-.4-4.4-2.7-3.4-1.7.7-2.6 3.6-4.3 4.2-1.7.6-3.2-.2-3-2z"/></svg>`)
	sysQuitAppsIcon     = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect x="4" y="4" width="7" height="7" rx="2" fill="#da0e0e"/><rect x="13" y="4" width="7" height="7" rx="2" fill="#da0e0e"/><rect x="4" y="13" width="7" height="7" rx="2" fill="#da0e0e"/><path stroke="#da0e0e" stroke-linecap="round" stroke-width="2" d="m15 15 4 4m0-4-4 4"/></svg>`)
	sysHideAppsIcon     = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#8b95a5" d="M3 5h18v14H3z"/><path fill="#fff" d="M6 8h8v2H6zm0 4h12v2H6z"/><path stroke="#da0e0e" stroke-linecap="round" stroke-width="2" d="m16 8 5 5m0-5-5 5"/></svg>`)
	sysUnhideAppsIcon   = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#21bf4b" d="M3 5h18v14H3z"/><path fill="#fff" d="M6 8h8v2H6zm0 4h12v2H6z"/><path fill="#fff" d="m10 17 2 2 5-5 1.4 1.4L12 21.8l-3.4-3.4z"/></svg>`)
	sysAppearanceIcon   = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#111827" d="M12 3a9 9 0 1 0 0 18z"/><path fill="#f8fafc" d="M12 3a9 9 0 0 1 0 18z"/></svg>`)
	sysHiddenFilesIcon  = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#8b95a5" d="M4 5h16v14H4z"/><path fill="#fff" d="M7 8h10v2H7zm0 4h6v2H7z"/><path stroke="#4f7cff" stroke-linecap="round" stroke-width="2" d="M4 20 20 4"/></svg>`)
	sysAttentionIcon    = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#4f7cff" d="M4 5.5A2.5 2.5 0 0 1 6.5 3h11A2.5 2.5 0 0 1 20 5.5v13A2.5 2.5 0 0 1 17.5 21h-11A2.5 2.5 0 0 1 4 18.5z"/><path fill="#fff" d="M6.4 13.2h3.1c.5 0 .9.3 1.1.7l.5 1c.2.4.6.7 1.1.7h1.6c.5 0 .9-.3 1.1-.7l.5-1c.2-.4.6-.7 1.1-.7h3.1v5.3c0 .7-.6 1.3-1.3 1.3H7.7c-.7 0-1.3-.6-1.3-1.3z" opacity=".95"/><path fill="#dbe6ff" d="M7.8 6.2h8.4a.8.8 0 0 1 0 1.6H7.8a.8.8 0 1 1 0-1.6m0 3.2h8.4a.8.8 0 0 1 0 1.6H7.8a.8.8 0 1 1 0-1.6"/></svg>`)
)
