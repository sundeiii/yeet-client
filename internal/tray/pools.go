package tray

import (
	"errors"
	"github.com/sundeiii/yeet-client/internal/i18n"
	"log"

	"fyne.io/fyne/v2"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

// RefreshPools loads the account's pools for the "Upload To" menu. Servers
// without pool support leave the menu out.
func (m *TrayManager) RefreshPools() {
	if !m.api.Account.Credentials.HasApiKey() {
		return
	}
	pools, err := m.api.Pools()
	if err != nil {
		if !errors.Is(err, puush.ErrNotSupported) {
			log.Printf("Could not load pools: %v", err)
		}
		return
	}

	m.mu.Lock()
	m.pools = pools
	m.mu.Unlock()
	m.stateChanged()
}

// Pools returns the pools loaded by RefreshPools.
func (m *TrayManager) Pools() []*puush.Pool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pools
}

// UploadPool returns the pool new uploads go into, if the pools are known.
func (m *TrayManager) UploadPool() *puush.Pool {
	pools := m.Pools()
	for _, pool := range pools {
		if pool.Id == m.config.General.UploadPoolId {
			return pool
		}
	}
	// No choice, or the chosen pool is gone: the server uses the default one
	for _, pool := range pools {
		if pool.Default {
			return pool
		}
	}
	return nil
}

// SetUploadPool picks the pool new uploads go into.
func (m *TrayManager) SetUploadPool(pool *puush.Pool) {
	if pool == nil || pool.Default {
		// Following the account's default keeps working if the default changes
		m.config.General.UploadPoolId = 0
	} else {
		m.config.General.UploadPoolId = pool.Id
	}
	m.stateChanged()
}

func (m *TrayManager) buildPoolMenu() *fyne.MenuItem {
	pools := m.Pools()
	if len(pools) == 0 {
		return nil
	}
	current := m.UploadPool()

	items := make([]*fyne.MenuItem, 0, len(pools))
	for _, pool := range pools {
		pool := pool
		label := pool.Name
		if pool.Default {
			label += " (default)"
		}
		item := fyne.NewMenuItem(escapeMenuLabel(label), func() { m.SetUploadPool(pool) })
		item.Checked = current != nil && pool.Id == current.Id
		items = append(items, item)
	}

	menu := fyne.NewMenuItem(i18n.T("Upload To"), nil)
	if current != nil {
		menu.Label = i18n.T("Upload To: %s", escapeMenuLabel(current.Name))
	}
	menu.ChildMenu = fyne.NewMenu("", items...)
	return menu
}
