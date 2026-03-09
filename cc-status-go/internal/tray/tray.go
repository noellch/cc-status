package tray

import (
	"fmt"
	"path/filepath"
	"sync"

	"fyne.io/systray"
	"github.com/anthropics/cc-status-go/internal/session"
	"github.com/anthropics/cc-status-go/pkg/model"
)

// maxSessionGroups is the maximum number of sessions we can display.
// Each session uses 3 pre-allocated menu item slots: main row, summary row, spacer.
const maxSessionGroups = 10

// sessionGroup represents the 3 menu items for a single session.
type sessionGroup struct {
	mainItem    *systray.MenuItem // clickable: emoji + repo + branch
	summaryItem *systray.MenuItem // disabled: indented summary text
	spacerItem  *systray.MenuItem // disabled: visual spacing between sessions
}

// Tray manages the system tray icon and menu for cc-status.
type Tray struct {
	store       *session.Store
	emptyItem   *systray.MenuItem // "—" shown when no sessions
	groups      []sessionGroup
	dismissItem *systray.MenuItem
	quitItem    *systray.MenuItem

	// slotSessions maps each slot index to the session ID it currently displays.
	// Protected by slotMu to avoid race between refresh and click handler.
	slotMu       sync.RWMutex
	slotSessions [maxSessionGroups]string
}

// NewTray creates a Tray bound to the given session store.
func NewTray(store *session.Store) *Tray {
	return &Tray{store: store}
}

// OnReady is called by systray.Run when the tray is initialised.
func (t *Tray) OnReady() {
	systray.SetIcon(iconTransparent)
	systray.SetTooltip("cc-status")
	systray.SetTitle("○")

	// Empty state item (matches Swift's "—" disabled item).
	t.emptyItem = systray.AddMenuItem("—", "No active sessions")
	t.emptyItem.Disable()

	// Pre-allocate session group slots (hidden by default).
	t.groups = make([]sessionGroup, maxSessionGroups)
	for i := 0; i < maxSessionGroups; i++ {
		g := sessionGroup{
			mainItem:    systray.AddMenuItem("", ""),
			summaryItem: systray.AddMenuItem("", ""),
			spacerItem:  systray.AddMenuItem("", ""),
		}
		g.mainItem.Hide()
		g.summaryItem.Hide()
		g.summaryItem.Disable()
		g.spacerItem.Hide()
		g.spacerItem.Disable()
		t.groups[i] = g

		// Handle clicks on the main item — look up session by stored ID (not index).
		go func(idx int) {
			for range t.groups[idx].mainItem.ClickedCh {
				t.slotMu.RLock()
				sessionID := t.slotSessions[idx]
				t.slotMu.RUnlock()
				if sessionID == "" {
					continue
				}
				all := t.store.All()
				if info, ok := all[sessionID]; ok {
					FocusTerminal(info.TerminalID)
				}
			}
		}(i)
	}

	// Bottom items (matches Swift: separator → dismiss all → quit).
	systray.AddSeparator()
	t.dismissItem = systray.AddMenuItem("dismiss all", "Dismiss all sessions")
	t.dismissItem.Hide()
	t.quitItem = systray.AddMenuItem("quit", "Quit cc-status")

	t.store.SetOnChange(func() {
		t.refresh()
	})
	t.refresh()

	// Handle clicks on bottom items.
	go func() {
		for {
			select {
			case <-t.dismissItem.ClickedCh:
				t.store.DismissAll()
			case <-t.quitItem.ClickedCh:
				systray.Quit()
			}
		}
	}()
}

// OnExit is called by systray.Run when the tray is shutting down.
func (t *Tray) OnExit() {}

func (t *Tray) refresh() {
	sorted := t.store.Sorted()

	// Update menu bar title (matches Swift's updateIcon).
	t.updateTitle(sorted)

	// Empty state: show "—" when no sessions (matches Swift).
	if len(sorted) == 0 {
		t.emptyItem.Show()
	} else {
		t.emptyItem.Hide()
	}

	// Record which session ID each slot displays (for click handler).
	t.slotMu.Lock()
	for i := range t.slotSessions {
		if i < len(sorted) {
			t.slotSessions[i] = sorted[i].SessionID
		} else {
			t.slotSessions[i] = ""
		}
	}
	t.slotMu.Unlock()

	// Update session groups.
	for i, g := range t.groups {
		if i < len(sorted) {
			s := sorted[i]

			// Main row: emoji + repo + " · " + branch
			// Matches Swift: dotImage(color) + repo (medium) + " · " + branch (light)
			emoji := statusEmoji(s.Status)
			repo := filepath.Base(s.Cwd)
			title := fmt.Sprintf("%s %s", emoji, repo)
			if s.Branch != "" {
				title += " · " + s.Branch
			}
			g.mainItem.SetTitle(title)
			g.mainItem.Show()
			g.mainItem.Enable()

			// Summary row: indented sub-item (matches Swift's indentationLevel=1, disabled).
			if s.Summary != "" {
				summary := truncateRunes(s.Summary, 50)
				// Leading spaces simulate indentation (systray has no indentationLevel).
				g.summaryItem.SetTitle("    " + summary)
				g.summaryItem.Show()
			} else {
				g.summaryItem.Hide()
			}

			// Spacer between sessions (matches Swift's spacerItem between groups).
			if i < len(sorted)-1 {
				g.spacerItem.SetTitle(" ")
				g.spacerItem.Show()
			} else {
				g.spacerItem.Hide()
			}
		} else {
			g.mainItem.Hide()
			g.summaryItem.Hide()
			g.spacerItem.Hide()
		}
	}

	// Show "dismiss all" only when sessions exist (matches Swift).
	if len(sorted) > 0 {
		t.dismissItem.Show()
	} else {
		t.dismissItem.Hide()
	}
}

// updateTitle sets the menu bar title text to reflect session states.
// Matches Swift's updateIcon(): empty → "○", sessions → colored dots with counts.
func (t *Tray) updateTitle(sorted []model.SessionInfo) {
	if len(sorted) == 0 {
		systray.SetTitle("○")
		return
	}

	// Count sessions by status.
	var waiting, done, active int
	for _, s := range sorted {
		switch s.Status {
		case model.StatusWaiting:
			waiting++
		case model.StatusDone:
			done++
		case model.StatusActive:
			active++
		}
	}

	// Build title with colored emoji dots + counts.
	// Order: waiting (orange), done (green), active (blue) — matches Swift.
	// Swift uses "●" + " \(count)" with space before count.
	type segment struct {
		count int
		emoji string
	}
	segments := []segment{
		{waiting, "🟠"},
		{done, "🟢"},
		{active, "🔵"},
	}

	title := ""
	for _, seg := range segments {
		if seg.count == 0 {
			continue
		}
		if title != "" {
			title += "  "
		}
		title += seg.emoji
		if seg.count > 1 {
			title += fmt.Sprintf(" %d", seg.count)
		}
	}
	systray.SetTitle(title)
}

func statusEmoji(s model.SessionStatus) string {
	switch s {
	case model.StatusWaiting:
		return "🟠"
	case model.StatusDone:
		return "🟢"
	case model.StatusActive:
		return "🔵"
	default:
		return "⚪"
	}
}

// truncateRunes truncates a string to maxRunes characters, appending "…" if truncated.
// Uses rune count (not byte count) to handle multi-byte UTF-8 correctly.
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}
