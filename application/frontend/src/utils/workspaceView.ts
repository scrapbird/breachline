// Shared UI side effects for opening and closing a workspace. Both the
// human-driven flows (workspace menu, local/remote create, sync select/logout)
// and the MCP bridge call these, so AI-driven and hand-driven sessions render
// identically: an open workspace shows a sticky Dashboard tab and live
// annotations, a closed one removes them. Backend calls stay at each call site
// (they differ per flow); only the view sync is shared here.

export const DASHBOARD_TAB_ID = '__dashboard__';

// The slice of the tab store these helpers touch.
export interface WorkspaceViewTabState {
  tabs: { id: string }[];
  createTab: (id: string, name: string) => void;
  switchTab: (id: string) => void;
  closeTab: (id: string) => void;
  getTabState: (id: string) => any;
}

export interface WorkspaceViewDeps {
  tabState: WorkspaceViewTabState;
  setIsWorkspaceOpen: (open: boolean) => void;
  // Bumped to force the Dashboard to remount.
  setWorkspaceKey: (updater: (prev: number) => number) => void;
}

// Repaint every open data grid so annotation gutters update. Skips the sticky
// Dashboard tab, which has no grid of its own.
function refreshDataGrids(tabState: WorkspaceViewTabState) {
  tabState.tabs.forEach((tabInfo) => {
    if (tabInfo.id === DASHBOARD_TAB_ID) return;
    const tab = tabState.getTabState(tabInfo.id);
    tab?.gridApi?.refreshInfiniteCache?.();
  });
}

// Reflect a freshly opened workspace: flag it open, remount the dashboard,
// create and focus the sticky Dashboard tab, and repaint grids so annotations
// show immediately.
export function showWorkspaceOpened({ tabState, setIsWorkspaceOpen, setWorkspaceKey }: WorkspaceViewDeps) {
  setIsWorkspaceOpen(true);
  setWorkspaceKey((prev) => prev + 1);

  if (!tabState.tabs.some((t) => t.id === DASHBOARD_TAB_ID)) {
    tabState.createTab(DASHBOARD_TAB_ID, 'Dashboard');
  }
  tabState.switchTab(DASHBOARD_TAB_ID);

  refreshDataGrids(tabState);
}

// Reflect a closed workspace: clear the flag, remount, drop the Dashboard tab,
// and repaint grids so annotations disappear.
export function showWorkspaceClosed({ tabState, setIsWorkspaceOpen, setWorkspaceKey }: WorkspaceViewDeps) {
  setIsWorkspaceOpen(false);
  setWorkspaceKey((prev) => prev + 1);

  if (tabState.tabs.some((t) => t.id === DASHBOARD_TAB_ID)) {
    tabState.closeTab(DASHBOARD_TAB_ID);
  }

  refreshDataGrids(tabState);
}
