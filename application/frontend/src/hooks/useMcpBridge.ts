import { useEffect, useRef } from 'react';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import * as BridgeAPI from '../../wailsjs/go/mcpserver/BridgeService';
import * as AppAPI from '../../wailsjs/go/app/App';
import * as WorkspaceManagerAPI from '../../wailsjs/go/app/WorkspaceManager';
import { FileOptions } from '../types/FileOptions';
import { showWorkspaceOpened, showWorkspaceClosed } from '../utils/workspaceView';
import { annotateRowsByHash, deleteRowAnnotationsByHash } from '../utils/annotations';
import { LogEntry } from './useConsoleLogger';

// A command dispatched by the MCP server for the visible window to perform.
interface McpCommand {
  id: string;
  action: string;
  params: any;
}

// The handlers the bridge needs from App to drive the real UI. These are the
// same handlers a human triggers, so AI-driven and human-driven work share one
// session.
export interface McpBridgeDeps {
  openFile: (filePath: string, fileOptions?: FileOptions) => Promise<void>;
  applyQuery: (query: string) => Promise<void>;
  changeTab: (tabId: string) => Promise<void> | void;
  closeTab: (tabId: string) => Promise<void> | void;
  tabState: any;
  setIsWorkspaceOpen: (open: boolean) => void;
  setWorkspaceKey: (updater: (prev: number) => number) => void;
  addLog: (level: 'info' | 'warn' | 'error', message: string) => void;
  // Change the active tab's timestamp column (same flow as the header menu), so
  // AI-driven timestamp changes re-sort and refresh the grid + histogram.
  setTimestampColumn: (columnName: string) => Promise<{ success: boolean; message?: string }>;
  // Read the current console log buffer (backend + UI events) for get_console_log.
  getConsoleLogs: () => LogEntry[];
}

// Map the MCP options payload (from list_workspace_files) to a FileOptions.
function toFileOptions(o: any): FileOptions {
  o = o || {};
  return {
    jpath: o.jpath || '',
    noHeaderRow: !!o.noHeaderRow,
    ingestTimezoneOverride: o.ingestTimezoneOverride || '',
    isDirectory: !!o.isDirectory,
    filePattern: o.filePattern || '',
    includeSourceColumn: !!o.includeSourceColumn,
  };
}

// Look up the backend tab for a path (newest match wins) and return its summary.
async function findBackendTab(filePath: string) {
  const tabs = await AppAPI.GetTabs();
  const matches = (tabs || []).filter((t: any) => t.filePath === filePath);
  return matches.length ? matches[matches.length - 1] : null;
}

// useMcpBridge listens for MCP commands and performs them in the live UI, then
// reports the structured result back to the waiting tool call.
export function useMcpBridge(deps: McpBridgeDeps) {
  // Keep the latest handlers in a ref so the single event subscription always
  // runs against current tab/query state rather than a stale first-render closure.
  const depsRef = useRef(deps);
  depsRef.current = deps;

  useEffect(() => {
    const off = EventsOn('mcp:command', async (cmd: McpCommand) => {
      if (!cmd || !cmd.id) return;
      const p = cmd.params || {};
      const current = depsRef.current;
      try {
        const result = await dispatch(cmd.action, p, current);
        BridgeAPI.MCPResolve(cmd.id, JSON.stringify(result ?? {}), '');
      } catch (e: any) {
        const msg = e?.message || String(e) || 'command failed';
        current.addLog('error', `MCP ${cmd.action} failed: ${msg}`);
        BridgeAPI.MCPResolve(cmd.id, '', msg);
      }
    });
    return () => { try { off(); } catch { /* ignore */ } };
    // Subscribe once; latest handlers are read via depsRef.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
}

async function dispatch(action: string, p: any, deps: McpBridgeDeps): Promise<any> {
  const { tabState } = deps;
  switch (action) {
    case 'open_file': {
      const options: FileOptions = {
        jpath: p.jpath || '',
        noHeaderRow: !!p.noHeaderRow,
        ingestTimezoneOverride: p.ingestTimezone || '',
      };
      await deps.openFile(p.path, options);
      const tab = await findBackendTab(p.path);
      if (!tab) throw new Error('the file could not be opened');
      return { tabId: tab.id, fileName: tab.fileName, columns: tab.headers || [] };
    }
    case 'open_directory': {
      if (!p.filePattern) throw new Error('filePattern is required for a directory (e.g. **/*.json.gz)');
      const options: FileOptions = {
        jpath: p.jpath || '',
        isDirectory: true,
        filePattern: p.filePattern,
        includeSourceColumn: !!p.includeSourceColumn,
      };
      await deps.openFile(p.path, options);
      const tab = await findBackendTab(p.path);
      if (!tab) throw new Error('the directory could not be opened (no matching files?)');
      // Surface truncation so the agent knows the dataset is incomplete and can
      // tell the user to raise the file limit (0 = unlimited) or narrow the pattern.
      return {
        tabId: tab.id,
        fileName: tab.fileName,
        columns: tab.headers || [],
        truncated: !!tab.truncated,
        filesLoaded: tab.filesLoaded || 0,
      };
    }
    case 'apply_query': {
      const st = tabState.getTabState(p.tabId);
      if (!st) throw new Error(`no open tab with id ${p.tabId}`);
      if (tabState.activeTabId !== p.tabId) {
        await deps.changeTab(p.tabId);
      }
      tabState.setQuery(p.query || '');
      await deps.applyQuery(p.query || '');
      return { tabId: p.tabId, appliedQuery: p.query || '' };
    }
    case 'set_active_tab': {
      const st = tabState.getTabState(p.tabId);
      if (!st) throw new Error(`no open tab with id ${p.tabId}`);
      await deps.changeTab(p.tabId);
      return { ok: true };
    }
    case 'close_tab': {
      const st = tabState.getTabState(p.tabId);
      if (!st) throw new Error(`no open tab with id ${p.tabId}`);
      await deps.closeTab(p.tabId);
      return { ok: true };
    }
    case 'open_workspace': {
      await WorkspaceManagerAPI.OpenWorkspace(p.path);
      showWorkspaceOpened(deps);
      return { ok: true };
    }
    case 'close_workspace': {
      await AppAPI.CloseWorkspace();
      showWorkspaceClosed(deps);
      return { ok: true };
    }
    case 'add_file_to_workspace': {
      const options: FileOptions = {
        jpath: p.jpath || '',
        isDirectory: !!p.filePattern,
        filePattern: p.filePattern || '',
      };
      await AppAPI.AddFileToWorkspace(p.path, options);
      return { ok: true };
    }
    case 'annotate_rows': {
      const st = tabState.getTabState(p.tabId);
      if (!st) throw new Error(`no open tab with id ${p.tabId}`);
      // Same operation the annotate button performs for an explicit row
      // selection. By-hash (not by-path) so it works on directory-backed tabs.
      if (!st.fileHash) throw new Error(`tab ${p.tabId} has no file hash yet`);
      const rows: number[] = p.rowIndices || [];
      await annotateRowsByHash(
        { fileHash: st.fileHash, fileOptions: st.fileOptions, timeField: st.timeField, appliedQuery: st.appliedQuery },
        rows,
        p.note || '',
        p.color || 'grey',
      );
      st.gridApi?.refreshInfiniteCache?.();
      return { ok: true, count: rows.length };
    }
    case 'delete_annotations': {
      const st = tabState.getTabState(p.tabId);
      if (!st) throw new Error(`no open tab with id ${p.tabId}`);
      if (!st.fileHash) throw new Error(`tab ${p.tabId} has no file hash yet`);
      const rows: number[] = p.rowIndices || [];
      await deleteRowAnnotationsByHash(
        { fileHash: st.fileHash, fileOptions: st.fileOptions, timeField: st.timeField, appliedQuery: st.appliedQuery },
        rows,
      );
      st.gridApi?.refreshInfiniteCache?.();
      return { ok: true, count: rows.length };
    }
    case 'export_workspace_timeline': {
      await AppAPI.ExportWorkspaceTimeline();
      return { ok: true };
    }
    case 'create_local_workspace': {
      if (!p.path) throw new Error('path is required');
      // Create then open, mirroring the human "Create Local Workspace" flow (which
      // differs only in that it picks the path via a save dialog).
      await WorkspaceManagerAPI.CreateLocalWorkspace(p.path);
      await WorkspaceManagerAPI.OpenWorkspace(p.path);
      showWorkspaceOpened(deps);
      return { ok: true };
    }
    case 'create_remote_workspace': {
      if (!p.name) throw new Error('name is required');
      await AppAPI.CreateRemoteWorkspace(p.name);
      showWorkspaceOpened(deps);
      return { ok: true };
    }
    case 'remove_file_from_workspace': {
      if (!p.fileHash) throw new Error('fileHash is required');
      await AppAPI.RemoveFileFromWorkspaceByHash(p.fileHash, toFileOptions(p.options));
      // Remount the dashboard so the file list and annotation gutters refresh.
      deps.setWorkspaceKey((prev) => prev + 1);
      return { ok: true };
    }
    case 'update_file_description': {
      if (!p.fileHash) throw new Error('fileHash is required');
      await AppAPI.UpdateFileDescription(p.fileHash, toFileOptions(p.options), p.description || '');
      deps.setWorkspaceKey((prev) => prev + 1);
      return { ok: true };
    }
    case 'set_timestamp_column': {
      const st = tabState.getTabState(p.tabId);
      if (!st) throw new Error(`no open tab with id ${p.tabId}`);
      if (!p.columnName) throw new Error('columnName is required');
      // The set-timestamp flow operates on the active tab, so focus it first.
      if (tabState.activeTabId !== p.tabId) {
        await deps.changeTab(p.tabId);
      }
      const res = await deps.setTimestampColumn(p.columnName);
      return { success: !!res?.success, message: res?.message || '' };
    }
    case 'get_console_log': {
      // Backend also emits 'debug' (below 'info'), so rank it explicitly; an
      // unspecified min shows everything, an unknown entry level ranks as info.
      const order: Record<string, number> = { debug: 0, info: 1, warn: 2, error: 3 };
      const min = p.level in order ? order[p.level] : 0;
      const limit = p.limit && p.limit > 0 ? p.limit : 100;
      const entries = deps
        .getConsoleLogs()
        .filter((e) => (order[e.level] ?? 1) >= min)
        .slice(-limit)
        .map((e) => ({ ts: e.ts, level: e.level, message: e.message }));
      return { entries };
    }
    default:
      throw new Error(`unknown command: ${action}`);
  }
}
