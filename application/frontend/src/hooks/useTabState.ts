import { useState, useRef, useCallback } from 'react';
import { TabState, createEmptyTabState, SearchResult, ResultsPanelTab } from '../types/TabState';
import { FileOptions, createDefaultFileOptions, fileOptionsKey } from '../types/FileOptions';

export interface TabInfo {
  id: string;
  filePath: string;
  fileOptions?: import('../types/FileOptions').FileOptions;
}

export interface UseTabStateReturn {
  // Tab management
  tabs: TabInfo[];
  activeTabId: string | null;
  currentTab: TabState | null;
  stateVersion: number;
  
  // Tab operations
  createTab: (tabId: string, filePath: string, fileHash?: string, fileOptions?: FileOptions) => void;
  switchTab: (tabId: string) => void;
  closeTab: (tabId: string) => void;
  findTabByFilePath: (filePath: string, fileOptions?: FileOptions) => string | null;
  
  // Current tab state getters
  getTabState: (tabId: string) => TabState | null;
  getCurrentTabState: () => TabState | null;
  
  // Current tab state setters
  updateCurrentTab: (updater: (prev: TabState) => TabState) => void;
  setColumnDefs: (defs: any[]) => void;
  setHeader: (header: string[]) => void;
  setOriginalHeader: (header: string[]) => void;
  setQuery: (query: string) => void;
  setAppliedQuery: (query: string) => void;
  setHistBuckets: (buckets: { start: number; count: number }[]) => void;
  setHistBucketsForTab: (tabId: string, buckets: { start: number; count: number }[]) => void;
  setHistogramVersionForTab: (tabId: string, version: string) => void;
  setRowAnnotationsForTab: (tabId: string, annotations: Map<number, { color: string }>) => void;
  setTimeField: (field: string) => void;
  setFileHashForTab: (tabId: string, fileHash: string) => void;
  setHeaderForTab: (tabId: string, header: string[]) => void;
  setOriginalHeaderForTab: (tabId: string, header: string[]) => void;
  setColumnDefsForTab: (tabId: string, defs: any[]) => void;
  setTimeFieldForTab: (tabId: string, field: string) => void;
  setFileOptionsForTab: (tabId: string, options: FileOptions) => void;
  setTotalRowsForTab: (tabId: string, rows: number | null) => void;
  setDatasourceForTab: (tabId: string, ds: any) => void;
  incrementGenerationForTab: (tabId: string) => void;
  incrementFileTokenForTab: (tabId: string) => void;
  setTotalRows: (rows: number | null) => void;
  setVirtualSelectAll: (val: boolean) => void;
  setSelectionRange: (range: { start: number; end: number } | null) => void;
  setGridApi: (api: any) => void;
  setColumnApi: (api: any) => void;
  setGridApiForTab: (tabId: string, api: any) => void;
  setColumnApiForTab: (tabId: string, api: any) => void;
  setDatasource: (ds: any) => void;
  setGridPending: (delta: number) => void;
  setGridPendingForTab: (tabId: string, delta: number) => void;
  setHistPendingForTab: (tabId: string, delta: number) => void;
  setScrollPositionForTab: (tabId: string, position: { firstDisplayedRow: number; lastDisplayedRow: number }) => void;
  setGridInitializedForTab: (tabId: string, initialized: boolean) => void;
  setQueryError: (error: string | null) => void;
  setHighlightTerms: (terms: string[]) => void;
  setHighlightTermsForTab: (tabId: string, terms: string[]) => void;
  setRowMetadataForTab: (tabId: string, metadata: { originalIndices: number[]; displayIndices: number[] } | undefined) => void;
  
  // Annotation panel methods
  setShowAnnotationPanelForTab: (tabId: string, show: boolean) => void;
  setAnnotationPanelHeightForTab: (tabId: string, height: number) => void;
  toggleAnnotationPanelForTab: (tabId: string) => void;
  toggleResultsPanelForTab: (tabId: string) => void; // Toggles entire results panel (annotations + search)
  toggleSearchPanelForTab: (tabId: string) => void; // Toggles search panel specifically
  toggleAnnotationsPanelForTab: (tabId: string) => void; // Toggles annotations panel specifically
  
  // Search panel methods
  setSearchTermForTab: (tabId: string, term: string) => void;
  setSearchIsRegexForTab: (tabId: string, isRegex: boolean) => void;
  setSearchResultsForTab: (tabId: string, results: SearchResult[], totalCount: number, resetPage?: boolean) => void;
  setSearchCurrentPageForTab: (tabId: string, page: number) => void;
  setShowSearchPanelForTab: (tabId: string, show: boolean) => void;
  setIsSearchingForTab: (tabId: string, searching: boolean) => void;
  setActiveResultsTabForTab: (tabId: string, tab: ResultsPanelTab) => void;
  clearSearchForTab: (tabId: string) => void;
  
  // Column JPath expression methods
  setColumnJPathExpressionForTab: (tabId: string, columnName: string, expression: string) => void;
  clearColumnJPathExpressionForTab: (tabId: string, columnName: string) => void;
  getColumnJPathExpressionsForTab: (tabId: string) => Record<string, string>;
  
  // Cell viewer methods
  showCellViewerForTab: (tabId: string, columnName: string, cellValue: string) => void;
  hideCellViewerForTab: (tabId: string) => void;
  
  // Generation and token management
  incrementGeneration: () => void;
  incrementFileToken: () => void;
  getGeneration: () => number;
  getFileToken: () => number;
  
  // Refs for current tab (for performance-critical code that needs sync access)
  appliedQueryRef: React.MutableRefObject<string>;
  totalRowsRef: React.MutableRefObject<number | null>;
  virtualSelectAllRef: React.MutableRefObject<boolean>;
  selectionRangeRef: React.MutableRefObject<{ start: number; end: number } | null>;
  generationRef: React.MutableRefObject<number>;
  fileTokenRef: React.MutableRefObject<number>;
}

export const useTabState = (): UseTabStateReturn => {
  const [tabStates, setTabStates] = useState<Map<string, TabState>>(new Map());
  const [activeTabId, setActiveTabId] = useState<string | null>(null);
  const [stateVersion, setStateVersion] = useState<number>(0);
  
  // Refs to always access the latest state without closure issues
  const tabStatesRef = useRef<Map<string, TabState>>(new Map());
  const activeTabIdRef = useRef<string | null>(null);
  
  // Refs that track the current active tab's state for sync access
  const appliedQueryRef = useRef<string>('');
  const totalRowsRef = useRef<number | null>(null);
  const virtualSelectAllRef = useRef<boolean>(false);
  const selectionRangeRef = useRef<{ start: number; end: number } | null>(null);
  const generationRef = useRef<number>(0);
  const fileTokenRef = useRef<number>(0);
  
  // Sync refs with current tab state whenever active tab changes
  const syncRefsWithCurrentTab = useCallback((state: TabState | null) => {
    if (state) {
      appliedQueryRef.current = state.appliedQuery;
      totalRowsRef.current = state.totalRows;
      virtualSelectAllRef.current = state.virtualSelectAll;
      selectionRangeRef.current = state.selectionRange;
      generationRef.current = state.generation;
      fileTokenRef.current = state.fileToken;
    }
  }, []);
  
  const getCurrentTabState = useCallback((): TabState | null => {
    const currentActiveId = activeTabIdRef.current;
    if (!currentActiveId) return null;
    return tabStatesRef.current.get(currentActiveId) || null;
  }, []);
  
  const getTabState = useCallback((tabId: string): TabState | null => {
    return tabStatesRef.current.get(tabId) || null;
  }, []);
  
  const findTabByFilePath = useCallback((filePath: string, fileOptions?: FileOptions): string | null => {
    if (!filePath) return null;
    
    // Normalize fileOptions to defaults if not provided
    const normalizedOptions = fileOptions || createDefaultFileOptions();
    const searchKey = fileOptionsKey(normalizedOptions);
    
    for (const [tabId, state] of tabStatesRef.current.entries()) {
      // Match if filepath matches AND fileOptions match
      if (state.filePath === filePath) {
        // If fileOptions was provided, compare using keys
        if (fileOptions !== undefined) {
          const stateKey = fileOptionsKey(state.fileOptions);
          if (stateKey !== searchKey) {
            continue; // Skip this tab, fileOptions don't match
          }
        }
        return tabId;
      }
    }
    return null;
  }, []);
  
  const createTab = useCallback((tabId: string, filePath: string, fileHash?: string, fileOptions?: FileOptions) => {
    const newState = createEmptyTabState(tabId, filePath, fileHash || '', fileOptions || createDefaultFileOptions());
    
    // Update refs synchronously BEFORE setState
    const next = new Map(tabStatesRef.current);
    next.set(tabId, newState);
    tabStatesRef.current = next;
    activeTabIdRef.current = tabId;
    
    // Then update React state
    setTabStates(next);
    setActiveTabId(tabId);
    syncRefsWithCurrentTab(newState);
    
    console.log('createTab: Created tab', tabId, 'in ref, can access:', tabStatesRef.current.has(tabId));
  }, [syncRefsWithCurrentTab]);
  
  const switchTab = useCallback((tabId: string) => {
    const state = tabStatesRef.current.get(tabId);
    if (state) {
      setActiveTabId(tabId);
      activeTabIdRef.current = tabId; // Update ref immediately
      syncRefsWithCurrentTab(state);
    }
  }, [syncRefsWithCurrentTab]);
  
  const closeTab = useCallback((tabId: string) => {
    // Update refs synchronously BEFORE setState
    const next = new Map(tabStatesRef.current);
    next.delete(tabId);
    tabStatesRef.current = next;
    
    // Determine new active tab
    let newActiveId: string | null = null;
    if (tabId === activeTabIdRef.current) {
      const remainingTabs = Array.from(next.keys());
      if (remainingTabs.length > 0) {
        newActiveId = remainingTabs[0];
      }
      activeTabIdRef.current = newActiveId;
      syncRefsWithCurrentTab(newActiveId ? next.get(newActiveId) || null : null);
    }
    
    // Then update React state
    setTabStates(next);
    if (tabId === activeTabId) {
      setActiveTabId(newActiveId);
    }
  }, [activeTabId, syncRefsWithCurrentTab]);
  
  const updateCurrentTab = useCallback((updater: (prev: TabState) => TabState) => {
    const currentActiveId = activeTabIdRef.current;
    if (!currentActiveId) return;
    
    // Read from ref to avoid React batching issues where prev is stale
    const current = tabStatesRef.current.get(currentActiveId);
    if (!current) {
      console.warn('updateCurrentTab: No current tab state found for', currentActiveId);
      return;
    }
    
    const updated = updater(current);
    
    // Update ref synchronously BEFORE setState
    const next = new Map(tabStatesRef.current);
    next.set(currentActiveId, updated);
    tabStatesRef.current = next;
    
    // Then update React state
    setTabStates(next);
    
    // Sync refs with the updated state
    syncRefsWithCurrentTab(updated);
    
    // Increment version to trigger useEffects that depend on tab state
    setStateVersion(v => v + 1);
  }, [syncRefsWithCurrentTab]);
  
  // Convenience setters for common operations
  const setColumnDefs = useCallback((defs: any[]) => {
    updateCurrentTab(prev => ({ ...prev, columnDefs: defs }));
  }, [updateCurrentTab]);
  
  const setHeader = useCallback((header: string[]) => {
    updateCurrentTab(prev => ({ ...prev, header }));
  }, [updateCurrentTab]);
  
  const setOriginalHeader = useCallback((header: string[]) => {
    updateCurrentTab(prev => ({ ...prev, originalHeader: header }));
  }, [updateCurrentTab]);
  
  const setQuery = useCallback((query: string) => {
    updateCurrentTab(prev => ({ ...prev, query }));
  }, [updateCurrentTab]);
  
  const setAppliedQuery = useCallback((query: string) => {
    updateCurrentTab(prev => {
      const updated = { ...prev, appliedQuery: query };
      appliedQueryRef.current = query;
      return updated;
    });
  }, [updateCurrentTab]);
  
  const setHistBuckets = useCallback((buckets: { start: number; count: number }[]) => {
    updateCurrentTab(prev => ({ ...prev, histBuckets: buckets }));
  }, [updateCurrentTab]);
  
  const setHistBucketsForTab = useCallback((tabId: string, buckets: { start: number; count: number }[]) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) {
      console.warn('[HIST_BUCKETS_DEBUG] Tab not found:', tabId);
      return;
    }
    
    console.log('[HIST_BUCKETS_DEBUG] Setting histBuckets for tab', tabId, 'buckets:', buckets.length);
    const updated = { ...current, histBuckets: buckets };
    
    // Update ref synchronously
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    
    // Update React state
    setTabStates(next);
    
    // If this is the current tab, sync refs
    if (tabId === activeTabIdRef.current) {
      syncRefsWithCurrentTab(updated);
    }
  }, [syncRefsWithCurrentTab]);
  
  const setHistogramVersionForTab = useCallback((tabId: string, version: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;

    const updated = { ...current, histogramVersion: version };

    // Update ref synchronously
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;

    // Update React state
    setTabStates(next);

    // If this is the current tab, sync refs
    if (tabId === activeTabIdRef.current) {
      syncRefsWithCurrentTab(updated);
    }
  }, [syncRefsWithCurrentTab]);

  const setRowAnnotationsForTab = useCallback((tabId: string, annotations: Map<number, { color: string }>) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;

    const updated = { ...current, rowAnnotations: annotations };

    // Update ref synchronously
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;

    // Update React state
    setTabStates(next);

    // Increment version to trigger re-renders
    setStateVersion(v => v + 1);

    // If this is the current tab, sync refs
    if (tabId === activeTabIdRef.current) {
      syncRefsWithCurrentTab(updated);
    }
  }, [syncRefsWithCurrentTab]);

  const setTimeField = useCallback((field: string) => {
    updateCurrentTab(prev => ({ ...prev, timeField: field }));
  }, [updateCurrentTab]);
  
  const setTotalRows = useCallback((rows: number | null) => {
    updateCurrentTab(prev => {
      const updated = { ...prev, totalRows: rows };
      totalRowsRef.current = rows;
      return updated;
    });
  }, [updateCurrentTab]);
  
  const setVirtualSelectAll = useCallback((val: boolean) => {
    updateCurrentTab(prev => {
      const updated = { ...prev, virtualSelectAll: val };
      virtualSelectAllRef.current = val;
      return updated;
    });
  }, [updateCurrentTab]);
  
  const setSelectionRange = useCallback((range: { start: number; end: number } | null) => {
    updateCurrentTab(prev => {
      const updated = { ...prev, selectionRange: range };
      selectionRangeRef.current = range;
      return updated;
    });
  }, [updateCurrentTab]);
  
  const setGridApi = useCallback((api: any) => {
    updateCurrentTab(prev => ({ ...prev, gridApi: api }));
  }, [updateCurrentTab]);
  
  const setColumnApi = useCallback((api: any) => {
    updateCurrentTab(prev => ({ ...prev, columnApi: api }));
  }, [updateCurrentTab]);
  
  const setGridApiForTab = useCallback((tabId: string, api: any) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, gridApi: api };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setColumnApiForTab = useCallback((tabId: string, api: any) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, columnApi: api };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setDatasource = useCallback((ds: any) => {
    updateCurrentTab(prev => ({ ...prev, datasource: ds }));
  }, [updateCurrentTab]);
  
  const setGridPending = useCallback((delta: number) => {
    updateCurrentTab(prev => ({ ...prev, gridPending: Math.max(0, prev.gridPending + delta) }));
  }, [updateCurrentTab]);
  
  const setGridPendingForTab = useCallback((tabId: string, delta: number) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    
    const updated = { ...current, gridPending: Math.max(0, current.gridPending + delta) };
    
    // Update ref synchronously
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    
    // Update React state
    setTabStates(next);
    
    // If this is the current tab, sync refs
    if (tabId === activeTabIdRef.current) {
      syncRefsWithCurrentTab(updated);
    }
  }, [syncRefsWithCurrentTab]);
  
  const setHistPendingForTab = useCallback((tabId: string, value: number) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) {
      console.warn('[HIST_PENDING_DEBUG] Tab not found:', tabId);
      return;
    }
    
    console.log('[HIST_PENDING_DEBUG] Setting histPending for tab', tabId, 'from', current.histPending, 'to', value);
    
    // Changed from delta to absolute value to prevent accumulation bugs
    // value should be 0 (not loading) or 1 (loading)
    const updated = { ...current, histPending: Math.max(0, value) };
    
    // Update ref synchronously
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    
    // Update React state
    setTabStates(next);
    
    // If this is the current tab, sync refs
    if (tabId === activeTabIdRef.current) {
      syncRefsWithCurrentTab(updated);
    }
  }, [syncRefsWithCurrentTab]);
  
  const incrementGeneration = useCallback(() => {
    updateCurrentTab(prev => {
      const updated = { ...prev, generation: prev.generation + 1 };
      generationRef.current = updated.generation;
      return updated;
    });
  }, [updateCurrentTab]);
  
  const incrementFileToken = useCallback(() => {
    updateCurrentTab(prev => {
      const updated = { ...prev, fileToken: prev.fileToken + 1 };
      fileTokenRef.current = updated.fileToken;
      return updated;
    });
  }, [updateCurrentTab]);
  
  // Tab-specific setters for file loading operations
  const setHeaderForTab = useCallback((tabId: string, header: string[]) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, header };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setOriginalHeaderForTab = useCallback((tabId: string, header: string[]) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, originalHeader: header };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setColumnDefsForTab = useCallback((tabId: string, defs: any[]) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, columnDefs: defs };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setTimeFieldForTab = useCallback((tabId: string, field: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, timeField: field };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);

  const setFileHashForTab = useCallback((tabId: string, fileHash: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, fileHash };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setFileOptionsForTab = useCallback((tabId: string, options: FileOptions) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, fileOptions: options };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setTotalRowsForTab = useCallback((tabId: string, rows: number | null) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, totalRows: rows };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) {
      totalRowsRef.current = rows;
      syncRefsWithCurrentTab(updated);
    }
  }, [syncRefsWithCurrentTab]);
  
  const setDatasourceForTab = useCallback((tabId: string, ds: any) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, datasource: ds };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const incrementGenerationForTab = useCallback((tabId: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, generation: current.generation + 1 };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) {
      generationRef.current = updated.generation;
      syncRefsWithCurrentTab(updated);
    }
  }, [syncRefsWithCurrentTab]);
  
  const incrementFileTokenForTab = useCallback((tabId: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, fileToken: current.fileToken + 1 };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) {
      fileTokenRef.current = updated.fileToken;
      syncRefsWithCurrentTab(updated);
    }
  }, [syncRefsWithCurrentTab]);
  
  const getGeneration = useCallback((): number => {
    return generationRef.current;
  }, []);
  
  const getFileToken = useCallback((): number => {
    return fileTokenRef.current;
  }, []);

  const setScrollPositionForTab = useCallback((tabId: string, position: { firstDisplayedRow: number; lastDisplayedRow: number }) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, scrollPosition: position };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);

  const setGridInitializedForTab = useCallback((tabId: string, initialized: boolean) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, isGridInitialized: initialized };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setQueryError = useCallback((error: string | null) => {
    updateCurrentTab(prev => ({ ...prev, queryError: error }));
  }, [updateCurrentTab]);
  
  const setHighlightTerms = useCallback((terms: string[]) => {
    updateCurrentTab(prev => ({ ...prev, highlightTerms: terms }));
  }, [updateCurrentTab]);
  
  const setHighlightTermsForTab = useCallback((tabId: string, terms: string[]) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, highlightTerms: terms };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setRowMetadataForTab = useCallback((tabId: string, metadata: { originalIndices: number[]; displayIndices: number[] } | undefined) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, rowMetadata: metadata };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  // Annotation panel methods
  const setShowAnnotationPanelForTab = useCallback((tabId: string, show: boolean) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, showAnnotationPanel: show };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    setStateVersion(v => v + 1);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setAnnotationPanelHeightForTab = useCallback((tabId: string, height: number) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, annotationPanelHeight: height };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const toggleAnnotationPanelForTab = useCallback((tabId: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, showAnnotationPanel: !current.showAnnotationPanel };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    setStateVersion(v => v + 1);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  // Toggle the entire results panel (both annotations and search)
  const toggleResultsPanelForTab = useCallback((tabId: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    
    // Panel is visible if either is showing
    const isPanelVisible = current.showAnnotationPanel || current.showSearchPanel;
    
    // If visible, hide both. If hidden, show annotations panel.
    const updated = isPanelVisible 
      ? { ...current, showAnnotationPanel: false, showSearchPanel: false }
      : { ...current, showAnnotationPanel: true, activeResultsTab: 'annotations' as ResultsPanelTab };
    
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    setStateVersion(v => v + 1);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  // Toggle search panel specifically (Ctrl+F)
  // Behavior:
  // - If search is open and active → close it (switch to annotations if open, else close panel)
  // - If search is open but not active → switch to it
  // - If search is not open → open it and switch to it
  const toggleSearchPanelForTab = useCallback((tabId: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    
    let updated: typeof current;
    
    if (current.showSearchPanel && current.activeResultsTab === 'search') {
      // Search is open and active - close it
      if (current.showAnnotationPanel) {
        // Annotations tab is also open - switch to it
        updated = { ...current, showSearchPanel: false, activeResultsTab: 'annotations' as ResultsPanelTab };
      } else {
        // Search was the only tab - close the panel
        updated = { ...current, showSearchPanel: false };
      }
    } else if (current.showSearchPanel && current.activeResultsTab !== 'search') {
      // Search is open but not active - just switch to it
      updated = { ...current, activeResultsTab: 'search' as ResultsPanelTab };
    } else {
      // Search is not open - open it and switch to it
      updated = { ...current, showSearchPanel: true, activeResultsTab: 'search' as ResultsPanelTab };
    }
    
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    setStateVersion(v => v + 1);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  // Toggle annotations panel specifically (Ctrl+B)
  // Behavior:
  // - If annotations is open and active → close it (switch to search if open, else close panel)
  // - If annotations is open but not active → switch to it
  // - If annotations is not open → open it and switch to it
  const toggleAnnotationsPanelForTab = useCallback((tabId: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    
    let updated: typeof current;
    
    if (current.showAnnotationPanel && current.activeResultsTab === 'annotations') {
      // Annotations is open and active - close it
      if (current.showSearchPanel) {
        // Search tab is also open - switch to it
        updated = { ...current, showAnnotationPanel: false, activeResultsTab: 'search' as ResultsPanelTab };
      } else {
        // Annotations was the only tab - close the panel
        updated = { ...current, showAnnotationPanel: false };
      }
    } else if (current.showAnnotationPanel && current.activeResultsTab !== 'annotations') {
      // Annotations is open but not active - just switch to it
      updated = { ...current, activeResultsTab: 'annotations' as ResultsPanelTab };
    } else {
      // Annotations is not open - open it and switch to it
      updated = { ...current, showAnnotationPanel: true, activeResultsTab: 'annotations' as ResultsPanelTab };
    }
    
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    setStateVersion(v => v + 1);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  // Column JPath expression methods
  const setColumnJPathExpressionForTab = useCallback((tabId: string, columnName: string, expression: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const newExpressions = { ...current.columnJPathExpressions };
    if (expression && expression !== '$') {
      newExpressions[columnName] = expression;
    } else {
      delete newExpressions[columnName];
    }
    const updated = { ...current, columnJPathExpressions: newExpressions };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    setStateVersion(v => v + 1);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const clearColumnJPathExpressionForTab = useCallback((tabId: string, columnName: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const newExpressions = { ...current.columnJPathExpressions };
    delete newExpressions[columnName];
    const updated = { ...current, columnJPathExpressions: newExpressions };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    setStateVersion(v => v + 1);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const getColumnJPathExpressionsForTab = useCallback((tabId: string): Record<string, string> => {
    const current = tabStatesRef.current.get(tabId);
    return current?.columnJPathExpressions || {};
  }, []);
  
  // Search panel methods
  const setSearchTermForTab = useCallback((tabId: string, term: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, searchTerm: term };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setSearchIsRegexForTab = useCallback((tabId: string, isRegex: boolean) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, searchIsRegex: isRegex };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setSearchResultsForTab = useCallback((tabId: string, results: SearchResult[], totalCount: number, resetPage: boolean = false) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { 
      ...current, 
      searchResults: results, 
      searchTotalCount: totalCount,
      // Only reset page when explicitly requested (e.g., new search, not pagination)
      ...(resetPage ? { searchCurrentPage: 0 } : {}),
    };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    setStateVersion(v => v + 1);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setSearchCurrentPageForTab = useCallback((tabId: string, page: number) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, searchCurrentPage: page };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setShowSearchPanelForTab = useCallback((tabId: string, show: boolean) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, showSearchPanel: show };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    setStateVersion(v => v + 1);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setIsSearchingForTab = useCallback((tabId: string, searching: boolean) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, isSearching: searching };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    setStateVersion(v => v + 1);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const setActiveResultsTabForTab = useCallback((tabId: string, tab: ResultsPanelTab) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { ...current, activeResultsTab: tab };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    setStateVersion(v => v + 1);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const clearSearchForTab = useCallback((tabId: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { 
      ...current, 
      searchTerm: '',
      searchIsRegex: false,
      searchResults: [],
      searchTotalCount: 0,
      searchCurrentPage: 0,
      isSearching: false,
      // Keep the search tab open - just clear the results
    };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    setStateVersion(v => v + 1);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  // Cell viewer methods
  const showCellViewerForTab = useCallback((tabId: string, columnName: string, cellValue: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { 
      ...current, 
      cellViewer: { visible: true, columnName, cellValue }
    };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    setStateVersion(v => v + 1);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  const hideCellViewerForTab = useCallback((tabId: string) => {
    const current = tabStatesRef.current.get(tabId);
    if (!current) return;
    const updated = { 
      ...current, 
      cellViewer: { visible: false, columnName: '', cellValue: '' }
    };
    const next = new Map(tabStatesRef.current);
    next.set(tabId, updated);
    tabStatesRef.current = next;
    setTabStates(next);
    setStateVersion(v => v + 1);
    if (tabId === activeTabIdRef.current) syncRefsWithCurrentTab(updated);
  }, [syncRefsWithCurrentTab]);
  
  // Compute tabs list for TabBar
  // Ensure dashboard tab is always first (leftmost) if it exists
  const tabs: TabInfo[] = Array.from(tabStates.entries())
    .map(([id, state]) => ({
      id,
      filePath: state.filePath,
      fileOptions: state.fileOptions,
    }))
    .sort((a, b) => {
      // Dashboard tab always comes first
      if (a.id === '__dashboard__') return -1;
      if (b.id === '__dashboard__') return 1;
      // Maintain original order for other tabs
      return 0;
    });
  
  const currentTab = getCurrentTabState();
  
  return {
    tabs,
    activeTabId,
    currentTab,
    stateVersion,
    
    createTab,
    switchTab,
    closeTab,
    findTabByFilePath,
    
    getTabState,
    getCurrentTabState,
    
    updateCurrentTab,
    setColumnDefs,
    setHeader,
    setOriginalHeader,
    setQuery,
    setAppliedQuery,
    setHistBuckets,
    setHistBucketsForTab,
    setHistogramVersionForTab,
    setRowAnnotationsForTab,
    setTimeField,
    setFileHashForTab,
    setHeaderForTab,
    setOriginalHeaderForTab,
    setColumnDefsForTab,
    setTimeFieldForTab,
    setFileOptionsForTab,
    setTotalRowsForTab,
    setDatasourceForTab,
    incrementGenerationForTab,
    incrementFileTokenForTab,
    setTotalRows,
    setVirtualSelectAll,
    setSelectionRange,
    setGridApi,
    setColumnApi,
    setGridApiForTab,
    setColumnApiForTab,
    setDatasource,
    setGridPending,
    setGridPendingForTab,
    setHistPendingForTab,
    setScrollPositionForTab,
    setGridInitializedForTab,
    setQueryError,
    setHighlightTerms,
    setHighlightTermsForTab,
    setRowMetadataForTab,
    
    setShowAnnotationPanelForTab,
    setAnnotationPanelHeightForTab,
    toggleAnnotationPanelForTab,
    toggleResultsPanelForTab,
    toggleSearchPanelForTab,
    toggleAnnotationsPanelForTab,
    
    setSearchTermForTab,
    setSearchIsRegexForTab,
    setSearchResultsForTab,
    setSearchCurrentPageForTab,
    setShowSearchPanelForTab,
    setIsSearchingForTab,
    setActiveResultsTabForTab,
    clearSearchForTab,
    
    setColumnJPathExpressionForTab,
    clearColumnJPathExpressionForTab,
    getColumnJPathExpressionsForTab,
    
    showCellViewerForTab,
    hideCellViewerForTab,
    
    incrementGeneration,
    incrementFileToken,
    getGeneration,
    getFileToken,
    
    appliedQueryRef,
    totalRowsRef,
    virtualSelectAllRef,
    selectionRangeRef,
    generationRef,
    fileTokenRef,
  };
};
