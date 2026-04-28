//go:build !windows

package main

// TrayController is a no-op stub on non-Windows platforms. The system tray
// relies on getlantern/systray, which conflicts with Wails v2's macOS/Linux
// runtime (duplicate AppDelegate on macOS, extra GTK deps on Linux).
type TrayController struct{}

// NewTrayController returns nil so every tray.* call in app.go / main.go is
// skipped by the existing `if a.tray != nil` guards.
func NewTrayController(app *App) *TrayController { return nil }

func (t *TrayController) Register() {}
func (t *TrayController) Shutdown() {}
func (t *TrayController) Refresh()  {}
