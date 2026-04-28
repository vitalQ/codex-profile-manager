//go:build windows

package main

import (
	_ "embed"
	"fmt"
	"sync"
	"time"

	"github.com/getlantern/systray"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

type TrayController struct {
	app *App

	mu           sync.Mutex
	quitOnce     sync.Once
	appQuitOnce  sync.Once
	ready        bool
	currentItem  *systray.MenuItem
	switchRoot   *systray.MenuItem
	emptyItem    *systray.MenuItem
	showItem     *systray.MenuItem
	hideItem     *systray.MenuItem
	refreshItem  *systray.MenuItem
	quitItem     *systray.MenuItem
	profileItems map[string]*systray.MenuItem
}

func NewTrayController(app *App) *TrayController {
	return &TrayController{
		app:          app,
		profileItems: make(map[string]*systray.MenuItem),
	}
}

func (t *TrayController) Register() {
	systray.Register(t.onReady, t.onExit)
}

func (t *TrayController) Shutdown() {
	t.quitOnce.Do(func() {
		systray.Quit()
	})
}

func (t *TrayController) onReady() {
	if len(trayIcon) > 0 {
		systray.SetIcon(trayIcon)
	}
	systray.SetTooltip("Codex Profile Manager")

	t.mu.Lock()
	t.currentItem = systray.AddMenuItem("当前: 初始化中...", "")
	t.currentItem.Disable()
	t.switchRoot = systray.AddMenuItem("快速切换", "快速切换托管资料")
	t.emptyItem = t.switchRoot.AddSubMenuItem("暂无资料", "")
	t.emptyItem.Disable()
	systray.AddSeparator()
	t.showItem = systray.AddMenuItem("显示主窗口", "")
	t.hideItem = systray.AddMenuItem("隐藏主窗口", "")
	t.refreshItem = systray.AddMenuItem("刷新状态", "")
	systray.AddSeparator()
	t.quitItem = systray.AddMenuItem("退出 Codex Profile Manager", "")
	t.ready = true
	t.mu.Unlock()

	go t.listenFixedActions()
	go t.Refresh()
}

func (t *TrayController) onExit() {
	t.appQuitOnce.Do(func() {
		go func() {
			time.Sleep(60 * time.Millisecond)
			t.app.quitApplication()
		}()
	})
}

func (t *TrayController) listenFixedActions() {
	for {
		select {
		case <-t.showItem.ClickedCh:
			t.app.showMainWindow()
		case <-t.hideItem.ClickedCh:
			t.app.hideMainWindow()
		case <-t.refreshItem.ClickedCh:
			t.Refresh()
		case <-t.quitItem.ClickedCh:
			t.Shutdown()
			return
		}
	}
}

func (t *TrayController) Refresh() {
	t.mu.Lock()
	if !t.ready {
		t.mu.Unlock()
		return
	}
	t.mu.Unlock()

	if err := t.app.ensureReady(); err != nil {
		t.setCurrentTitle("当前: 初始化失败")
		return
	}

	settings, err := t.app.config.Load()
	if err != nil {
		t.setCurrentTitle("当前: 设置读取失败")
		return
	}
	profiles, err := t.app.profiles.List()
	if err != nil {
		t.setCurrentTitle("当前: 资料读取失败")
		return
	}
	current, err := t.app.detector.Current(settings.TargetAuthPath, settings.ActiveProfileID)
	if err != nil {
		t.setCurrentTitle("当前: 状态读取失败")
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if current.Managed && current.ProfileName != "" {
		t.currentItem.SetTitle("当前: " + current.ProfileName)
	} else if current.Exists {
		t.currentItem.SetTitle("当前: 未托管 auth.json")
	} else {
		t.currentItem.SetTitle("当前: 未检测到 auth.json")
	}

	if len(profiles) == 0 {
		t.emptyItem.Show()
	} else {
		t.emptyItem.Hide()
	}

	seen := make(map[string]struct{}, len(profiles))
	for _, record := range profiles {
		seen[record.ID] = struct{}{}

		item, exists := t.profileItems[record.ID]
		if !exists {
			item = t.switchRoot.AddSubMenuItemCheckbox(record.Name, record.Fingerprint, current.ProfileID == record.ID)
			t.profileItems[record.ID] = item
			go t.listenProfileItem(record.ID, item)
		}

		item.Show()
		item.SetTitle(record.Name)
		item.SetTooltip(fmt.Sprintf("%s · %s", shortFingerprint(record.Fingerprint), record.Note))
		if current.ProfileID == record.ID {
			item.Check()
		} else {
			item.Uncheck()
		}
	}

	for id, item := range t.profileItems {
		if _, ok := seen[id]; ok {
			continue
		}
		item.Hide()
		delete(t.profileItems, id)
	}
}

func (t *TrayController) listenProfileItem(profileID string, item *systray.MenuItem) {
	for {
		<-item.ClickedCh
		_, err := t.app.switchProfileInternal(profileID)
		if err != nil {
			t.app.showTrayError("切换失败", err.Error())
			t.Refresh()
			continue
		}
		t.Refresh()
	}
}

func (t *TrayController) setCurrentTitle(title string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.currentItem != nil {
		t.currentItem.SetTitle(title)
	}
}

func shortFingerprint(value string) string {
	if len(value) <= 22 {
		return value
	}
	return value[:22] + "..."
}
