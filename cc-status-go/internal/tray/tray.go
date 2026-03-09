package tray

import (
	"fmt"
	"path/filepath"

	"fyne.io/systray"
	"github.com/anthropics/cc-status-go/internal/session"
	"github.com/anthropics/cc-status-go/pkg/model"
)

const maxSessionSlots = 20

// Tray manages the system tray icon and menu for cc-status.
type Tray struct {
	store        *session.Store
	sessionItems []*systray.MenuItem
	dismissItem  *systray.MenuItem
	quitItem     *systray.MenuItem
}

// NewTray creates a Tray bound to the given session store.
func NewTray(store *session.Store) *Tray {
	return &Tray{store: store}
}

// OnReady is called by systray.Run when the tray is initialised.
func (t *Tray) OnReady() {
	systray.SetIcon(iconIdle)
	systray.SetTooltip("cc-status")

	// Pre-allocate session item slots (hidden by default).
	t.sessionItems = make([]*systray.MenuItem, maxSessionSlots)
	for i := 0; i < maxSessionSlots; i++ {
		item := systray.AddMenuItem("", "")
		item.Hide()
		t.sessionItems[i] = item
		go func(idx int) {
			for range t.sessionItems[idx].ClickedCh {
				sessions := t.store.Sorted()
				if idx < len(sessions) {
					FocusTerminal(sessions[idx].TerminalID)
				}
			}
		}(i)
	}

	// Bottom items.
	t.dismissItem = systray.AddMenuItem("Dismiss All", "Dismiss all sessions")
	t.dismissItem.Hide()
	systray.AddSeparator()
	t.quitItem = systray.AddMenuItem("Quit", "Quit cc-status")

	t.store.SetOnChange(func() {
		t.refresh()
	})
	t.refresh()

	// Handle clicks.
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

	// Update tray icon based on highest-priority state.
	if len(sorted) == 0 {
		systray.SetIcon(iconIdle)
	} else {
		hasWaiting, hasDone := false, false
		for _, s := range sorted {
			if s.Status == model.StatusWaiting {
				hasWaiting = true
			}
			if s.Status == model.StatusDone {
				hasDone = true
			}
		}
		switch {
		case hasWaiting:
			systray.SetIcon(iconWaiting)
		case hasDone:
			systray.SetIcon(iconDone)
		default:
			systray.SetIcon(iconActive)
		}
	}

	// Update visible items (slots are pre-allocated in OnReady).
	for i, item := range t.sessionItems {
		if i < len(sorted) {
			s := sorted[i]
			emoji := statusEmoji(s.Status)
			repo := filepath.Base(s.Cwd)
			title := fmt.Sprintf("%s %s", emoji, repo)
			if s.Branch != "" {
				title += fmt.Sprintf(" \u00b7 %s", s.Branch)
			}
			if s.Summary != "" {
				summary := s.Summary
				if len(summary) > 50 {
					summary = summary[:50] + "\u2026"
				}
				title += fmt.Sprintf(" \u2014 %s", summary)
			}
			item.SetTitle(title)
			item.Show()
		} else {
			item.Hide()
		}
	}

	// Show/hide dismiss.
	if len(sorted) > 0 {
		t.dismissItem.Show()
	} else {
		t.dismissItem.Hide()
	}
}

func statusEmoji(s model.SessionStatus) string {
	switch s {
	case model.StatusWaiting:
		return "\U0001f7e0" // orange circle
	case model.StatusDone:
		return "\U0001f7e2" // green circle
	case model.StatusActive:
		return "\U0001f535" // blue circle
	default:
		return "\u26aa" // white circle
	}
}
