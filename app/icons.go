package main

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

//go:embed assets/logo.png
var iconPNG []byte

var appLogo, _ = ui.DecodeBitmap(iconPNG)

func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	iconWarp    = icon(`<path d="M2 5V3a1 1 0 0 1 1-1h2M11 2h2a1 1 0 0 1 1 1v2M14 11v2a1 1 0 0 1-1 1h-2M5 14H3a1 1 0 0 1-1-1v-2"/><ellipse cx="8" cy="8" rx="3" ry="3.7"/>`)
	iconSliders = icon(`<path d="M2 4.5h5.5M11 4.5h3M2 11.5h2.5M8 11.5h6"/><circle cx="9.25" cy="4.5" r="1.75"/><circle cx="6.25" cy="11.5" r="1.75"/>`)
	iconLayers  = icon(`<path d="M8 2 14 5 8 8 2 5z"/><path d="M2 8l6 3 6-3"/><path d="M2 11l6 3 6-3"/>`)
	iconCurve   = icon(`<path d="M1.5 12c3 0 3.6-8.5 6.5-8.5S11.5 12 14.5 12"/><path d="M1.5 14h13"/>`)
	iconReport  = icon(`<rect x="3" y="2.8" width="10" height="11.4" rx="1.4"/><path d="M6 2.8V1.6h4v1.2"/><path d="M5 9.2h1.5l1-2.4 1.5 4.2 1-1.8h1"/>`)
	iconCompare = icon(`<circle cx="6" cy="8" r="4.2"/><circle cx="10" cy="8" r="4.2"/>`)
	iconTarget  = icon(`<circle cx="8" cy="8" r="5.8"/><circle cx="8" cy="8" r="2.2"/><path d="M8 .8v2M8 13.2v2M.8 8h2M13.2 8h2"/>`)
	iconGrid    = icon(`<rect x="2" y="2" width="5" height="5" rx="1.2"/><rect x="9" y="2" width="5" height="5" rx="1.2"/><rect x="2" y="9" width="5" height="5" rx="1.2"/><rect x="9" y="9" width="5" height="5" rx="1.2"/>`)
	iconDoc     = icon(`<path d="M4 1.8h5l3 3v9.4H4z"/><path d="M9 1.8v3h3"/>`)
	iconFolder  = icon(`<path d="M1.8 4.3v7.9a1 1 0 0 0 1 1h10.4a1 1 0 0 0 1-1V6.1a1 1 0 0 0-1-1H8.2L6.7 3.3H2.8a1 1 0 0 0-1 1z"/>`)
	iconPlus    = icon(`<path d="M8 3v10M3 8h10"/>`)
	iconClose   = icon(`<path d="M4 4l8 8M12 4l-8 8"/>`)
	iconCheck   = icon(`<circle cx="8" cy="8" r="6.2"/><path d="M5.2 8.2l2 2 3.6-4"/>`)
	iconWarn    = icon(`<path d="M8 2.2 14.3 13H1.7z"/><path d="M8 6.4v3.2M8 11.4v.1"/>`)
	iconInfo    = icon(`<circle cx="8" cy="8" r="6.2"/><path d="M8 7.3v3.8M8 4.9v.1"/>`)
	iconReveal  = icon(`<path d="M6.5 3.5h-3v9h9v-3"/><path d="M9 3h4v4M13 3 7.5 8.5"/>`)
	iconTray    = icon(`<path d="M8 2v7.5M5 6.8l3 3 3-3"/><path d="M2.5 10.5v2a1 1 0 0 0 1 1h9a1 1 0 0 0 1-1v-2"/>`)
	iconTable   = icon(`<rect x="1.8" y="2.8" width="12.4" height="10.4" rx="1.6"/><path d="M1.8 6.3h12.4M6 6.3v6.9"/>`)
)
