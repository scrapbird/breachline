import { useCallback, useRef, useEffect } from 'react';
import { UseTabStateReturn } from './useTabState';
import { extractSearchTerms } from '../utils/searchHighlight';

// Global ref to track expected histogram versions per tab
// This is updated synchronously when we pre-increment the version, ensuring
// the event handler can always check against the correct expected version
// even before React state updates are flushed.
export const expectedHistogramVersions = new Map<string, string>();

export interface UseSearchOperationsProps {
  tabState: UseTabStateReturn;
  buildColumnDefs: (header: string[], timeField: string, overrideDisplayTZ?: string, columnJPathExpressions?: Record<string, string>, searchTerms?: string[]) => any[];
  createDataSource: (tabId: string, query: string, headerSnapshot: string[], timeField?: string) => any;
  addLog: (level: 'info' | 'warn' | 'error', message: string) => void;
}

export const useSearchOperations = ({
  tabState,
  buildColumnDefs,
  createDataSource,
  addLog,
}: UseSearchOperationsProps) => {
  const isApplyingRef = useRef<boolean>(false);
  const lastApplyTsRef = useRef<number>(0);
  
  // Note: Header change detection moved to useGridOperations to avoid infinite loops
  
  const splitPipesTopLevel = useCallback((s: string): string[] => {
    const out: string[] = [];
    let cur = "";
    let inQuote: string | null = null;
    for (let i = 0; i < s.length; i++) {
      const ch = s[i];
      if (ch === '"' || ch === '\'') {
        if (!inQuote) inQuote = ch; else if (inQuote === ch) inQuote = null;
        cur += ch;
        continue;
      }
      if (!inQuote && ch === '|') {
        const seg = cur.trim();
        if (seg) out.push(seg);
        cur = "";
        continue;
      }
      cur += ch;
    }
    if (cur.trim()) out.push(cur.trim());
    return out;
  }, []);
  
  const unquoteIfQuoted = useCallback((s: string): string => {
    const t = s.trim();
    if (t.length >= 2 && ((t.startsWith('"') && t.endsWith('"')) || (t.startsWith('\'') && t.endsWith('\'')))) {
      return t.slice(1, -1);
    }
    return t;
  }, []);
  
  const splitRespectingQuotes = useCallback((s: string): string[] => {
    const out: string[] = [];
    let cur = "";
    let inQuote: string | null = null;
    for (let i = 0; i < s.length; i++) {
      const ch = s[i];
      if (ch === '"' || ch === '\'') {
        if (!inQuote) inQuote = ch; else if (inQuote === ch) inQuote = null;
        cur += ch;
        continue;
      }
      if (!inQuote && (ch === '|' || /\s/.test(ch))) {
        if (cur) out.push(cur);
        cur = "";
        continue;
      }
      cur += ch;
    }
    if (cur) out.push(cur);
    return out;
  }, []);
  
  const parseColumnsStage = useCallback((q: string, hdr: string[]): string[] | null => {
    const stages = splitPipesTopLevel(q || "");
    if (!stages.length) return null;
    
    // Build index map with normalized names for empty headers
    const idxMap: Record<string, number> = {};
    let emptyCount = 0;
    hdr.forEach((h, i) => {
      const trimmed = (h || "").trim();
      let key = trimmed.toLowerCase();
      if (key === "") {
        // Generate sequential names for empty columns: "unnamed_a", "unnamed_b", etc.
        const letter = String.fromCharCode('a'.charCodeAt(0) + emptyCount);
        key = `unnamed_${letter}`;
        emptyCount++;
      }
      // Also map the header as-is (for normalized names from backend like "unnamed_a")
      idxMap[key] = i;
    });
    
    for (const st of stages) {
      const toks = splitRespectingQuotes(st);
      if (toks.length && toks[0].toLowerCase() === "columns") {
        const spec = toks.slice(1).join(" ").trim();
        const parts = spec.split(',');
        const cols: string[] = [];
        for (const p of parts) {
          const name = unquoteIfQuoted(p).trim().toLowerCase();
          if (!name) continue;
          const idx = idxMap[name];
          if (idx !== undefined) {
            cols.push(hdr[idx]); // Push the original header value (normalized or empty)
          }
        }
        return cols.length ? cols : [];
      }
    }
    return null;
  }, [splitPipesTopLevel, splitRespectingQuotes, unquoteIfQuoted]);

  
  const applySearch = useCallback(async (queryOverride?: string, overrideDisplayTZ?: string, timeFieldOverride?: string) => {
    const currentTab = tabState.currentTab;
    if (!currentTab) return;
    
    const q = queryOverride ?? currentTab.query;
    const currentTimeField = currentTab.timeField || '';
    const isTimeFieldChanging = timeFieldOverride !== undefined && timeFieldOverride !== currentTimeField;
    // Force refresh if timeField is changing, even if query is the same
    const isSameApplied = (q === currentTab.appliedQuery) && !isTimeFieldChanging;
    
    // Simple cooldown to avoid rapid overlapping apply
    const nowTs = Date.now();
    if (isApplyingRef.current && nowTs - lastApplyTsRef.current < 200) {
      return;
    }
    isApplyingRef.current = true;
    lastApplyTsRef.current = nowTs;
    
    // Increment generation token so stale async work is ignored
    tabState.incrementGeneration();
    
    // Set histogram loading state and pre-increment version for ALL queries
    // This is CRITICAL because a new datasource is ALWAYS created (line ~221), which triggers
    // a backend call that ALWAYS increments the histogram version. If we don't pre-increment
    // the frontend version, there will be a version mismatch and histogram events will be ignored.
    if (currentTab) {
      // Always set loading state - backend will generate/return histogram
      tabState.setHistPendingForTab(currentTab.tabId, 1);
      
      // CRITICAL: ALWAYS pre-increment histogram version to match what backend will generate
      // The backend increments the version on every GetDataAndHistogram call, regardless of
      // whether the query is cached or not. If we don't match, histogram events get ignored.
      const currentVersion = currentTab.histogramVersion || `${currentTab.tabId}:0`;
      const versionParts = currentVersion.split(':');
      const nextVersionNumber = parseInt(versionParts[1] || '0') + 1;
      const nextVersion = `${currentTab.tabId}:${nextVersionNumber}`;
      
      // Update global Map SYNCHRONOUSLY before React state update
      // This ensures the event handler can check against the correct version
      // even if the histogram event arrives before React flushes the state update
      expectedHistogramVersions.set(currentTab.tabId, nextVersion);
      
      tabState.setHistogramVersionForTab(currentTab.tabId, nextVersion);
      console.log('[HISTOGRAM_VERSION_PREINCREMENT] Updated version from', currentVersion, 'to', nextVersion, '(isSameApplied:', isSameApplied, ')');
    }
    
    // Reset virtual select-all, contiguous range, and annotations on new search
    // (annotations need to be cleared because row indices change when filtering)
    tabState.setVirtualSelectAll(false);
    tabState.setSelectionRange(null);
    
    // Clear annotations if query is changing (row indices will be different)
    if (!isSameApplied) {
      tabState.updateCurrentTab(prev => ({
        ...prev,
        rowAnnotations: new Map(),
      }));
    }
    
    // Apply query immediately
    if (!isSameApplied) {
      tabState.setAppliedQuery(q);
    }
    
    // Adjust header and columns based on query operations
    const originalHeader = currentTab.originalHeader.length ? currentTab.originalHeader : currentTab.header;
    
    // Check for columns operation (frontend-handled)
    const projected = parseColumnsStage(q, originalHeader);
    
    let effectiveHeader: string[];
    if (projected && projected.length) {
      // Columns operation found - use frontend filtering
      effectiveHeader = projected;
      tabState.setHeader(effectiveHeader);
    } else if (q.includes('strip')) {
      // Strip operation - use current header (updated by backend via useGridOperations)
      // The backend provides the filtered header, so use it if available
      effectiveHeader = currentTab.header.length ? currentTab.header : originalHeader;
    } else {
      // No special operations - use original header
      effectiveHeader = originalHeader;
      tabState.setHeader(effectiveHeader);
    }
    
    // Rebuild columnDefs to match effectiveHeader
    // IMPORTANT: Never change timeField here - it should only be set on file load or manual user change
    // Use the timeField that's already in tab state, or the override if provided
    const timeFieldToUse = timeFieldOverride !== undefined ? timeFieldOverride : (currentTab.timeField || '');
    
    // Extract free-text search terms for highlighting in grid cells
    const searchTerms = extractSearchTerms(q);
    tabState.setHighlightTerms(searchTerms);
    
    // Get column JPath expressions to preserve them when rebuilding column defs
    const columnJPathExprs = currentTab.columnJPathExpressions || {};
    const defs = buildColumnDefs(effectiveHeader, timeFieldToUse, overrideDisplayTZ, columnJPathExprs, searchTerms);
    
    tabState.setColumnDefs(defs);
    // DO NOT call setTimeField here - it should only be set on file load or manual change
    
    const gridApi = currentTab.gridApi;
    if (gridApi) {
      // Apply the new column definitions to the grid immediately
      if (gridApi.setColumnDefs) {
        try { gridApi.setColumnDefs(defs); } catch {}
      }
      // Then refresh headers to ensure they're updated
      if (gridApi.refreshHeader) {
        try { gridApi.refreshHeader(); } catch {}
      }
    }
    
    // Create and set fresh datasource with current timeField
    const dsImmediate = createDataSource(currentTab.tabId, q, effectiveHeader, timeFieldToUse);
    tabState.setDatasource(dsImmediate);
    
    if (gridApi) {
      // Note: Datasource is set via tabState, not gridApi.setDatasource()
      // The Grid component will automatically use the updated datasource from props
      try { gridApi.hideOverlay?.(); } catch {}
      try { gridApi.ensureIndexVisible?.(0); } catch {}
      try { gridApi.purgeInfiniteCache?.(); } catch {}
    }
    
    // Handle filtering vs clearing
    const isFiltering = (q || "").trim().length > 0;
    if (isFiltering) {
      tabState.setTotalRows(null);
      if (gridApi) {
        try { gridApi.hideOverlay?.(); } catch {}
        try { gridApi.ensureIndexVisible?.(0); } catch {}
        try { gridApi.purgeInfiniteCache?.(); } catch {}
      }
    }
    
    setTimeout(() => { isApplyingRef.current = false; }, 300);
    
    addLog('info', isFiltering ? 'Applied search query' : 'Cleared search filters');
  }, [tabState, parseColumnsStage, buildColumnDefs, createDataSource, addLog]);
  
  const replaceExistingTimeRange = useCallback((q: string, afterStr: string, beforeStr: string): string | null => {
    if (!q || !q.trim()) return null;
    const rxAfter = /\bafter\s+("[^"]*"|'[^']*'|\S+)/gi;
    const rxBefore = /\bbefore\s+("[^"]*"|'[^']*'|\S+)/gi;
    const hasAfter = rxAfter.test(q);
    rxAfter.lastIndex = 0;
    const hasBefore = rxBefore.test(q);
    rxBefore.lastIndex = 0;
    if (!(hasAfter && hasBefore)) return null;
    return q.replace(rxAfter, `after ${afterStr}`).replace(rxBefore, `before ${beforeStr}`);
  }, []);
  
  // Apply search to a specific tab by ID (used for refreshing all tabs when settings change)
  const applySearchForTab = useCallback(async (tabId: string, overrideDisplayTZ?: string) => {
    const targetTab = tabState.getTabState(tabId);
    if (!targetTab) return;
    
    // Skip non-file tabs (like dashboard)
    if (tabId === '__dashboard__' || !targetTab.filePath) return;
    
    const q = targetTab.appliedQuery || '';
    const currentTimeField = targetTab.timeField || '';
    
    // Increment generation token so stale async work is ignored
    tabState.incrementGenerationForTab(tabId);
    
    // Set histogram loading state and pre-increment version
    tabState.setHistPendingForTab(tabId, 1);
    
    // Pre-increment histogram version to match what backend will generate
    const currentVersion = targetTab.histogramVersion || `${tabId}:0`;
    const versionParts = currentVersion.split(':');
    const nextVersionNumber = parseInt(versionParts[1] || '0') + 1;
    const nextVersion = `${tabId}:${nextVersionNumber}`;
    
    // Update global Map SYNCHRONOUSLY before React state update
    expectedHistogramVersions.set(tabId, nextVersion);
    tabState.setHistogramVersionForTab(tabId, nextVersion);
    
    // Adjust header based on query operations
    const originalHeader = targetTab.originalHeader.length ? targetTab.originalHeader : targetTab.header;
    const projected = parseColumnsStage(q, originalHeader);
    
    let effectiveHeader: string[];
    if (projected && projected.length) {
      effectiveHeader = projected;
      tabState.setHeaderForTab(tabId, effectiveHeader);
    } else if (q.includes('strip')) {
      effectiveHeader = targetTab.header.length ? targetTab.header : originalHeader;
    } else {
      effectiveHeader = originalHeader;
      tabState.setHeaderForTab(tabId, effectiveHeader);
    }
    
    // Extract search terms for highlighting
    const searchTerms = extractSearchTerms(q);
    tabState.setHighlightTermsForTab(tabId, searchTerms);
    
    // Rebuild column defs with new settings (timezone, etc.)
    const columnJPathExprs = targetTab.columnJPathExpressions || {};
    const defs = buildColumnDefs(effectiveHeader, currentTimeField, overrideDisplayTZ, columnJPathExprs, searchTerms);
    tabState.setColumnDefsForTab(tabId, defs);
    
    // Create and set fresh datasource
    const dsImmediate = createDataSource(tabId, q, effectiveHeader, currentTimeField);
    tabState.setDatasourceForTab(tabId, dsImmediate);
    
    // Purge grid cache if the grid API is available
    const gridApi = targetTab.gridApi;
    if (gridApi) {
      try {
        if (gridApi.setColumnDefs) gridApi.setColumnDefs(defs);
        if (gridApi.refreshHeader) gridApi.refreshHeader();
        gridApi.hideOverlay?.();
        gridApi.purgeInfiniteCache?.();
      } catch {}
    }
    
    console.log(`[SETTINGS_REFRESH] Refreshed tab ${tabId} with new settings`);
  }, [tabState, parseColumnsStage, buildColumnDefs, createDataSource]);
  
  // Apply search to all open tabs (used when settings like ingest timezone change)
  const applySearchToAllTabs = useCallback(async (overrideDisplayTZ?: string) => {
    const allTabs = tabState.tabs;
    console.log(`[SETTINGS_REFRESH] Refreshing ${allTabs.length} tabs due to settings change`);
    
    for (const tab of allTabs) {
      await applySearchForTab(tab.id, overrideDisplayTZ);
    }
    
    console.log(`[SETTINGS_REFRESH] Finished refreshing all tabs`);
  }, [tabState.tabs, applySearchForTab]);

  return {
    applySearch,
    applySearchForTab,
    applySearchToAllTabs,
    parseColumnsStage,
    replaceExistingTimeRange,
    splitPipesTopLevel,
    unquoteIfQuoted,
    splitRespectingQuotes,
    isApplyingRef,
  };
};
