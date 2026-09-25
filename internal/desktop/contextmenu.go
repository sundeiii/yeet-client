package desktop

import (
	"github.com/sundeiii/yeet-client/internal/i18n"
	"log"

	"github.com/sundeiii/yeet-client/internal/contextmenu"
)

// UpdateContextMenuConfiguration applies and remembers the requested state.
func (ui *UI) UpdateContextMenuConfiguration(enabled bool) {
	ui.config.General.ContextMenu = enabled

	if IsDevelopmentBuild() {
		return
	}
	if err := contextmenu.Apply(enabled); err != nil {
		log.Printf("Failed to update context-menu integration: %v", err)
		if ui.tray != nil {
			ui.tray.ShowErrorNotification(i18n.T("Could not update the context menu. puush will try again the next time it starts."))
		}
	}
}

// ReconcileContextMenuConfiguration applies the configured/remembered context menu state.
func (ui *UI) ReconcileContextMenuConfiguration() {
	if IsDevelopmentBuild() {
		return
	}
	if err := contextmenu.Apply(ui.config.General.ContextMenu); err != nil {
		log.Printf("Failed to reconcile context menu integration: %v", err)
	}
}
