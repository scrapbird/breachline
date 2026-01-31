package main

import (
	"context"
	"embed"
	"runtime"

	"breachline/app"
	"breachline/app/settings"
	"breachline/app/sync"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// MenuManager implements the MenuUpdater interface
type MenuManager struct{}

// UpdateSyncMenuLabel updates the sync menu item label
// Note: Wails v2 doesn't support dynamic menu updates, so this is a no-op
func (m *MenuManager) UpdateSyncMenuLabel(label string) {
	// Wails v2 menus are immutable after creation
	// This method exists to satisfy the interface but does nothing
}

func main() {
	// Create an instance of the app structure
	appInstance := app.NewApp()
	settingsService := settings.NewSettingsService()
	// Inject cache manager (app) so settings service can clear caches when needed
	settingsService.SetCacheManager(appInstance)
	license := app.NewLicenseService()
	license.SetApp(appInstance)
	workspace := app.NewWorkspaceService()
	workspace.SetApp(appInstance)
	appInstance.SetWorkspaceService(workspace)
	syncService := sync.NewSyncService()

	// Set sync client on workspace manager for remote workspace support
	workspace.SetSyncClient(syncService)
	// Set workspace manager on sync service so it can open remote workspaces
	syncService.SetWorkspaceManager(workspace)

	// Set up menu updater for sync service (no-op in Wails v2)
	menuManager := &MenuManager{}
	sync.SetMenuUpdater(menuManager)

	AppMenu := menu.NewMenu()
	if runtime.GOOS == "darwin" {
		AppMenu.Append(menu.AppMenu())
	}

	FileMenu := AppMenu.AddSubmenu("File")
	FileMenu.AddText("Open File", keys.CmdOrCtrl("o"), func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:open")
		}
	})
	FileMenu.AddText("Open File with Options", keys.Combo("o", keys.CmdOrCtrlKey, keys.ShiftKey), func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:openWithOptions")
		}
	})
	fuzzyFinderMenuItem := FileMenu.AddText("Go to File", keys.CmdOrCtrl("p"), nil)
	fuzzyFinderMenuItem.OnClick(func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:fuzzyFinder")
		}
	})
	FileMenu.AddSeparator()
	FileMenu.AddText("Copy Selected Rows", keys.CmdOrCtrl("c"), func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:copySelected")
		}
	})
	FileMenu.AddSeparator()
	FileMenu.AddText("Import License File", nil, func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:importLicense")
		}
	})
	FileMenu.AddText("Settings", keys.CmdOrCtrl(","), func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:settings")
		}
	})

	WorkspaceMenu := AppMenu.AddSubmenu("Workspace")
	WorkspaceMenu.AddText("Sync", nil, func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:sync")
		}
	})
	WorkspaceMenu.AddText("Purchase License", nil, func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.BrowserOpenURL(appInstance.Ctx(), "https://breachline.app/pricing")
		}
	})

	WorkspaceMenu.AddSeparator()
	CreateWorkspaceMenu := WorkspaceMenu.AddSubmenu("Create Workspace")
	CreateWorkspaceMenu.AddText("Local", nil, func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:createLocalWorkspace")
		}
	})
	CreateWorkspaceMenu.AddText("Sync", nil, func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:createRemoteWorkspace")
		}
	})
	WorkspaceMenu.AddSeparator()
	WorkspaceMenu.AddText("Local Workspaces", nil, func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:openWorkspace")
		}
	})
	WorkspaceMenu.AddText("Close Workspace", nil, func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:closeWorkspace")
		}
	})
	WorkspaceMenu.AddSeparator()
	WorkspaceMenu.AddText("Add File to Workspace", keys.CmdOrCtrl("i"), func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:addFileToWorkspace")
		}
	})
	WorkspaceMenu.AddText("Add File to Workspace with Options", keys.Combo("i", keys.CmdOrCtrlKey, keys.ShiftKey), func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:addFileToWorkspaceWithOptions")
		}
	})
	WorkspaceMenu.AddSeparator()
	WorkspaceMenu.AddText("Export Workspace Timeline", nil, func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:exportWorkspaceTimeline")
		}
	})

	ViewMenu := AppMenu.AddSubmenu("View")
	histogramMenuItem := ViewMenu.AddText("Toggle Histogram", keys.CmdOrCtrl("h"), nil)
	histogramMenuItem.OnClick(func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:toggleHistogram")
		}
	})
	annotationsMenuItem := ViewMenu.AddText("Toggle Annotations", keys.CmdOrCtrl("b"), nil)
	annotationsMenuItem.OnClick(func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:toggleAnnotations")
		}
	})
	searchMenuItem := ViewMenu.AddText("Toggle Search", keys.CmdOrCtrl("f"), nil)
	searchMenuItem.OnClick(func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:toggleSearch")
		}
	})
	consoleMenuItem := ViewMenu.AddText("Toggle Console", keys.CmdOrCtrl("`"), nil)
	consoleMenuItem.OnClick(func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:toggleConsole")
		}
	})
	ViewMenu.AddSeparator()
	cacheIndicatorMenuItem := ViewMenu.AddText("Toggle Cache Indicator", nil, nil)
	cacheIndicatorMenuItem.OnClick(func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:toggleCacheIndicator")
		}
	})

	HelpMenu := AppMenu.AddSubmenu("Help")
	HelpMenu.AddText("Syntax", nil, func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:syntax")
		}
	})
	HelpMenu.AddText("Shortcuts", nil, func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:shortcuts")
		}
	})
	HelpMenu.AddSeparator()
	HelpMenu.AddText("About", nil, func(_ *menu.CallbackData) {
		if appInstance != nil {
			wruntime.EventsEmit(appInstance.Ctx(), "menu:about")
		}
	})

	// Get saved window size or use defaults
	width, height, err := appInstance.GetSavedWindowSize()
	if err != nil {
		println("Warning: Failed to get saved window size, using defaults:", err.Error())
		width, height = 1024, 768
	}

	// Create application with options
	err = wails.Run(&options.App{
		Title:  "BreachLine",
		Width:  width,
		Height: height,
		Menu:   AppMenu,
		// Window sizing options for ultrawide monitor support
		MinWidth:  400,
		MinHeight: 300,
		MaxWidth:  7680, // Support up to 8K ultrawide (7680x4320)
		MaxHeight: 4320,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			appInstance.Startup(ctx)
			settingsService.Startup(ctx)
			// Ensure instance ID is generated on first startup
			if err := settingsService.EnsureInstanceID(); err != nil {
				println("Warning: Failed to generate instance ID:", err.Error())
			}
			license.Startup(ctx)
			workspace.Startup(ctx)
			syncService.Startup(ctx)
		},
		Bind: []interface{}{
			appInstance,
			settingsService,
			license,
			workspace,
			syncService,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
