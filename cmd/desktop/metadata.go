package main

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/sundeiii/yeet-client/assets"
)

const (
	AppName     = "puush"
	AppID       = "ee.tupsujumal.yeet"
	AppVersion  = "1.1.1"
	AppIconName = "puush.png"
)

// Set by the build system through -ldflags -X
var (
	AppBuild     = "0"   // Number of current commits
	AppCommit    = "dev" // Current commit sha
	AppTimestamp = "0"   // Unix timestamp of the current commit
)

func init() {
	buildNumber, _ := strconv.Atoi(AppBuild)

	app.SetMetadata(fyne.AppMetadata{
		ID:      AppID,
		Name:    AppName,
		Version: AppVersion,
		Build:   buildNumber,
		Icon: &fyne.StaticResource{
			StaticName:    AppIconName,
			StaticContent: assets.PuushIconData,
		},
		Release:    true,
		Custom:     map[string]string{"commit": AppCommit},
		Migrations: map[string]bool{"fyneDo": true},
	})
}
