package main

import (
	"context"
	"log"

	"github.com/sundeiii/yeet-client/internal/config"
	"github.com/sundeiii/yeet-client/internal/desktop"
	"github.com/sundeiii/yeet-client/internal/updater"
)

func updaterLoop(cfg *config.Config, ui *desktop.UI) {
	// We don't want to run the updater in dev builds
	if desktop.IsDevelopmentBuild() {
		return
	}

	// Check if we can even update, and inform the user if not
	if !updater.CanUpdate() {
		log.Printf("puush cannot update itself, no write permission to the executable path")
		ui.ShowNotification("puush cannot update itself!", "puush has no write permission to the installation folder. Please move it somewhere else!")
		return
	}

	currentBranch := updater.BranchStable
	currentVersion, err := currentUpdateVersion(currentBranch)
	if err != nil {
		log.Printf("Failed to determine current update version: %v", err)
		return
	}

	// Check if an old version is next to the current executable
	wasUpdated := updater.Cleanup()
	if wasUpdated {
		log.Printf("puush was updated to version %s", currentVersion)
		ui.ShowNotification("puush was updated!", "You are now running "+currentVersion.String())
		// TODO: Add button for opening changelog page on github
	}

	controller, err := updater.NewController(currentUpdateVersion, updater.DefaultCheckInterval)
	if err != nil {
		log.Printf("Failed to initialize updater: %v", err)
		return
	}
	controller.
		WithBranch(func() updater.Branch {
			return currentBranch
		}).
		WithAutomaticChecksEnabled(func() bool {
			return cfg.General.AutoUpdate
		}).
		WithCallback(func(result updater.CheckResult) bool {
			return handleUpdateResult(result, cfg, ui)
		})
	ui.SetUpdateCheckCallback(func(branch *updater.Branch) bool {
		request := updater.CheckRequest{Manual: true, Branch: branch}
		return controller.RequestCheck(request)
	})
	controller.Run(context.Background())
}

func handleUpdateResult(result updater.CheckResult, cfg *config.Config, ui *desktop.UI) bool {
	cfg.Misc.LastUpdate = result.CheckedAt
	config.NewStore().Save(cfg) // ensure we update this before restarting
	defer ui.FinishUpdateCheck(result.CheckedAt)

	if result.Error != nil {
		log.Printf("Failed to check for updates: %v", result.Error)
		ui.ShowNotification("Update check failed!", "You may have to check for updates manually :(")
		return false
	}
	if result.Candidate == nil {
		log.Printf(
			"No update available, current version: %s",
			result.CurrentVersion,
		)
		if result.Manual {
			ui.ShowNotification(
				"No updates available",
				"You're already running the latest version ("+result.CurrentVersion.String()+").",
			)
		}
		return false
	}

	log.Printf("Update available: %s -> %s", result.CurrentVersion, result.Candidate.Version())
	ui.ShowNotification("Downloading update...", "puush will automatically restart when done!")

	updatedExecutable, err := updater.Perform(result.Candidate)
	if err != nil {
		ui.ShowNotification("Update failed!", "You may have to install this update manually :(")
		log.Printf("Failed to perform update: %v", err)
		return false
	}

	// Stop the IPC server to let the new process start its own server
	ui.CloseIPCServer()

	log.Printf("Update downloaded, restarting...")
	if err := restart(updatedExecutable); err != nil {
		ui.ShowNotification("Update restart failed!", "Please restart puush manually to finish the update.")
		log.Printf("Failed to restart after update: %v", err)
		return false
	}
	ui.Quit()
	return true
}

func currentUpdateVersion(branch updater.Branch) (updater.Version, error) {
	return updater.NewSemanticVersionFromString(AppVersion)
}
