package main

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"

	"sugarpet/core"
)

// Where the app is published.
const (
	releasesURL = "https://github.com/lijiaxiang63/sugar-pet/releases"
	guideURL    = "https://lijiaxiang63.github.io/sugar-pet/"
	sourceURL   = "https://github.com/lijiaxiang63/sugar-pet"
)

// The update plugin's check and preferences, which tests and snapshots
// swap: they don't use the plugin, whose functions panic without it.
var (
	checkForUpdates    = updater.CheckForUpdates
	updateChecks       = updater.AutomaticChecks
	setUpdateChecks    = updater.SetAutomaticChecks
	updateDownloads    = updater.AutomaticDownloads
	setUpdateDownloads = updater.SetAutomaticDownloads
	lastUpdateCheck    = updater.LastCheck
	availableVersion   = updater.AvailableVersion
	canUpdate          = func() bool { return mygo.Updater.Enabled() }
)

// useUpdater checks the releases on GitHub for a new version soon after
// launch and then once a day, and offers it in an update window of native
// UI, in the language the app shows (App.SetLocale).  It must run before
// App.Run.
func (a *app) useUpdater() {
	mygo.Use(native.New(updater.Options{Icon: iconPNG}))
	a.updates = true
	// A check or an answer in the update window changes what the settings
	// and the sidebar show.
	updater.OnChange(a.redraw)
}

// appVersion is the version of the app: that of the package when it is
// built as one, else that of the code.
func appVersion() string {
	if v := mygo.App.Version(); v != "" && v != "0.0.0" {
		return v
	}
	return core.Version
}

// ago says how long ago a time was, roughly.
func ago(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return T("just now", "刚刚")
	case d < time.Hour:
		return Tf("%d min ago", "%d 分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return Tf("%d h ago", "%d 小时前", int(d.Hours()))
	case d < 48*time.Hour:
		return T("yesterday", "昨天")
	}
	return t.Format("2006-01-02")
}
