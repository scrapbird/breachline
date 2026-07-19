import { useEffect, useRef } from 'react';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import * as BridgeAPI from '../../wailsjs/go/mcpserver/BridgeService';
import * as AppAPI from '../../wailsjs/go/app/App';
import * as WorkspaceManagerAPI from '../../wailsjs/go/app/WorkspaceManager';
import { FileOptions } from '../types/FileOptions';
import { showWorkspaceOpened, showWorkspaceClosed } from '../utils/workspaceView';
import { annotateRowsByHash, deleteRowAnnotationsByHash } from '../utils/annotations';

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
      return { tabId: tab.id, fileName: tab.fileName, columns: tab.headers || [] };
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
    default:
      throw new Error(`unknown command: ${action}`);
  }
}
