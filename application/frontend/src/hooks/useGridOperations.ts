import { useCallback, useRef } from 'react';
import { UseTabStateReturn } from './useTabState';
import { resolveDataLoad, rejectDataLoad } from '../utils/dataLoadTracker';

export interface UseGridOperationsProps {
  tabState: UseTabStateReturn;
  pageSize: number;
  buildColumnDefs: (header: string[], timeField: string, overrideDisplayTZ?: string, columnJPathExpressions?: Record<string, string>, searchTerms?: string[]) => any[];
  addLog: (level: 'info' | 'warn' | 'error', message: string) => void;
  useUnifiedFetch?: boolean; // Enable unified data + histogram fetching
  showErrorDialog?: (message: string) => void; // Show error dialog to user
}

export interface UseGridOperationsReturn {
  ensureRowVisible: (index: number) => void;
  selectAllDisplayed: () => void;
  selectRange: (start: number, end: number, preserveSelection?: boolean) => void;
  getSelectedRowIndexes: () => number[];
  indexesToRanges: (idxs: number[]) => { start: number; end: number }[];
  mapRows: (hdr: string[], rows: string[][]) => any[];
  createDataSource: (tabId: string, query: string, headerSnapshot: string[], timeField?: string) => any;
  selectingProgrammaticallyRef: React.MutableRefObject<boolean>;
  getOriginalIndexForRow: (tabId: string, displayIndex: number) => number | undefined;
  getDisplayIndexForRow: (tabId: string, displayIndex: number) => number | undefined;
}

export const useGridOperations = ({ 
  tabState, 
  pageSize, 
  buildColumnDefs, 
  addLog,
  useUnifiedFetch = false,
  showErrorDialog
}: UseGridOperationsProps): UseGridOperationsReturn => {
  const selectingProgrammaticallyRef = useRef<boolean>(false);
  
  const ensureRowVisible = useCallback((index: number) => {
    const currentTab = tabState.getCurrentTabState();
    const gridApi = currentTab?.gridApi;
    if (!gridApi || !gridApi.ensureIndexVisible) return;
    const idx = Math.max(0, index || 0);
    gridApi.ensureIndexVisible(idx);
  }, [tabState]);
  
  const selectAllDisplayed = useCallback(() => {
    const currentTab = tabState.getCurrentTabState();
    const gridApi = currentTab?.gridApi;
    if (!gridApi) return;
    
    selectingProgrammaticallyRef.current = true;
    if (gridApi.deselectAll) gridApi.deselectAll();
    
    // Only select rows that are currently loaded in cache, not all rows
    // This avoids iterating through 32k+ row indices which triggers data fetches
    // The virtualSelectAll flag handles the logical "all selected" state for backend operations
    if (gridApi.forEachNode) {
      gridApi.forEachNode((node: any) => {
        if (node.data) {  // Only select nodes that have data loaded
          node.setSelected(true);
        }
      });
    }
    setTimeout(() => { selectingProgrammaticallyRef.current = false; }, 150);
  }, [tabState]);
  
  const selectRange = useCallback((start: number, end: number, preserveSelection: boolean = false) => {
    const currentTab = tabState.getCurrentTabState();
    const gridApi = currentTab?.gridApi;
    if (!gridApi) return;
    
    const a = Math.max(0, Math.min(start || 0, end || 0));
    const b = Math.max(start || 0, end || 0);
    
    if (!preserveSelection && gridApi.deselectAll) gridApi.deselectAll();
    
    const getAt = gridApi.getDisplayedRowAtIndex?.bind(gridApi) || gridApi.getDisplayedRowAtIndex;
    for (let i = a; i <= b; i++) {
      const node = getAt ? getAt(i) : null;
      if (node && node.setSelected) node.setSelected(true);
    }
    
    tabState.setSelectionRange({ start: a, end: b });
  }, [tabState]);
  
  const getSelectedRowIndexes = useCallback((): number[] => {
    const currentTab = tabState.getCurrentTabState();
    const gridApi = currentTab?.gridApi;
    if (!gridApi) return [];
    
    try {
      const nodes: any[] = gridApi.getSelectedNodes ? gridApi.getSelectedNodes() : [];
      let idxs: number[] = nodes
        .map((n: any) => (typeof n?.rowIndex === 'number' ? n.rowIndex : null))
        .filter((v: number | null): v is number => typeof v === 'number');
      
      if (!idxs.length && gridApi.getDisplayedRowCount && gridApi.getDisplayedRowAtIndex) {
        const displayedCount = gridApi.getDisplayedRowCount();
        const fallback: number[] = [];
        for (let i = 0; i < displayedCount; i++) {
          const node: any = gridApi.getDisplayedRowAtIndex(i);
          if (node?.isSelected?.()) {
            const ri = (typeof node.rowIndex === 'number') ? node.rowIndex : i;
            fallback.push(ri);
          }
        }
        if (fallback.length) idxs = fallback;
      }
      
      idxs.sort((a: number, b: number) => a - b);
      return idxs;
    } catch {
      return [];
    }
  }, [tabState]);
  
  const indexesToRanges = useCallback((idxs: number[]): { start: number; end: number }[] => {
    if (!idxs || idxs.length === 0) return [];
    const out: { start: number; end: number }[] = [];
    let s = idxs[0];
    let e = idxs[0];
    for (let i = 1; i < idxs.length; i++) {
      const v = idxs[i];
      if (v === e + 1) {
        e = v;
      } else {
        // Backend expects exclusive ranges (like Go slices), so end = last_index + 1
        out.push({ start: s, end: e + 1 });
        s = v;
        e = v;
      }
    }
    // Backend expects exclusive ranges (like Go slices), so end = last_index + 1
    out.push({ start: s, end: e + 1 });
    return out;
  }, []);
  
  const mapRows = useCallback((hdr: string[], rows: string[][]) => {
    return rows.map((row) => {
      const obj: any = {};
      // Track empty column count for sequential numbering
      let emptyCount = 0;
      hdr.forEach((h: string, idx: number) => {
        let key: string | number;
        if (h === '') {
          // For empty headers, use a generated key based on empty column count
          // This ensures consistency even after column filtering
          key = `__empty_${emptyCount}__`;
          emptyCount++;
        } else {
          // Use the header as-is (including normalized names like "unnamed_a")
          key = h;
        }
        // Handle rows with fewer columns than headers
        obj[key] = idx < row.length ? row[idx] : '';
      });
      // Debug logging for unnamed columns
      if (hdr.some(h => h.match(/^unnamed_[a-z]$/i))) {
        console.log('mapRows - header:', hdr, 'mapped object keys:', Object.keys(obj), 'first row sample:', obj);
      }
      return obj;
    });
  }, []);
  
  const createDataSource = useCallback((tabId: string, query: string, headerSnapshot: string[], timeField: string = '') => {
    return {
      getRows: async (params: any) => {
        const startRow: number = (params.startRow ?? 0);
        const endRow: number = (params.endRow ?? (startRow + pageSize));
        const thisGen = tabState.getGeneration();
        
        try {
          // Update loading state for the specific tab, not the current tab
          tabState.setGridPendingForTab(tabId, 1);
          
          // Import AppAPI dynamically
          const AppAPI = await import('../../wailsjs/go/app/App');
          
          let page: any;
          
          // Use unified endpoint that fetches both grid data and histogram in one call
          if (AppAPI.GetDataAndHistogram) {
            addLog('info', '[ASYNC_FETCH] Fetching grid data (histogram will be generated async)');
            
            // Fetch both grid data and histogram in one unified call
            const unifiedResult = await AppAPI.GetDataAndHistogram(tabId, startRow, endRow, query || "", timeField || "", 300);
            
            // DEBUG: Log the entire unifiedResult to see what we're getting
            console.log('[UNIFIED_RESULT_DEBUG] Full response:', unifiedResult);
            console.log('[UNIFIED_RESULT_DEBUG] histogramVersion:', unifiedResult?.histogramVersion);
            console.log('[UNIFIED_RESULT_DEBUG] histogramCached:', unifiedResult?.histogramCached);
            console.log('[UNIFIED_RESULT_DEBUG] histogramBuckets length:', unifiedResult?.histogramBuckets?.length);
            
            // Extract grid data from unified response
            page = {
              originalHeader: unifiedResult?.originalHeader || [],
              header: unifiedResult?.header || [],
              displayColumns: unifiedResult?.displayColumns || [],
              rows: unifiedResult?.rows || [],
              reachedEnd: unifiedResult?.reachedEnd || false,
              total: unifiedResult?.total || 0,
              annotations: unifiedResult?.annotations || [],
              annotationColors: unifiedResult?.annotationColors || [],
              originalIndices: unifiedResult?.originalIndices || [],
              displayIndices: unifiedResult?.displayIndices || []
            };
            
            // Verify histogram version matches what we pre-incremented
            if (unifiedResult?.histogramVersion) {
              const currentVersion = tabState.getTabState(tabId)?.histogramVersion;
              if (currentVersion !== unifiedResult.histogramVersion) {
                console.warn('[HISTOGRAM_VERSION_MISMATCH] Frontend version:', currentVersion, 'Backend version:', unifiedResult.histogramVersion);
                // Update to backend version as fallback
                tabState.setHistogramVersionForTab(tabId, unifiedResult.histogramVersion);
              } else {
                console.log('[HISTOGRAM_VERSION_MATCH] Versions match:', currentVersion);
              }
            }
            
            // Update histogram data from response (may be cached or empty)
            // Only set buckets if they're not empty or if no buckets exist yet
            if (unifiedResult?.histogramBuckets && unifiedResult.histogramBuckets.length > 0) {
              console.log('[GRID_OPERATIONS_DEBUG] Setting histogram buckets from API response:', unifiedResult.histogramBuckets.length);
              tabState.setHistBucketsForTab(tabId, unifiedResult.histogramBuckets);
            } else {
              const currentTabState = tabState.getTabState(tabId);
              const existingBuckets = currentTabState?.histBuckets?.length || 0;
              console.log('[GRID_OPERATIONS_DEBUG] Skipping empty histogram buckets from API, existing buckets:', existingBuckets);
            }
            
            // Store row metadata (index mappings)
            if (unifiedResult?.originalIndices && unifiedResult?.displayIndices) {
              tabState.setRowMetadataForTab(tabId, {
                originalIndices: unifiedResult.originalIndices,
                displayIndices: unifiedResult.displayIndices,
              });
            } else {
              // Clear metadata if not provided
              tabState.setRowMetadataForTab(tabId, undefined);
            }
            
            // Set histogram loading state based on whether it's cached
            // If not cached, histogram will arrive via event with the version we just set above
            if (unifiedResult?.histogramCached) {
              tabState.setHistPendingForTab(tabId, 0);
              addLog('info', '[ASYNC_FETCH] Histogram loaded from cache');
            } else {
              // Check if histogram has already been loaded by event (race condition)
              const currentTabState = tabState.getTabState(tabId);
              const currentPending = currentTabState?.histPending || 0;
              
              if (currentPending === 0) {
                console.log('[ASYNC_FETCH] Histogram already loaded by event, keeping loading state cleared');
                addLog('info', '[ASYNC_FETCH] Histogram already loaded by event');
              } else {
                tabState.setHistPendingForTab(tabId, 1);
                addLog('info', `[ASYNC_FETCH] Histogram generation in progress (async) - expecting version ${unifiedResult?.histogramVersion}`);
              }
            }
          } else {
            throw new Error("Unified endpoint not available. Run 'wails dev' to regenerate bindings.");
          }
          
          // Check if backend returned updated header (from strip/columns/dedup operations)
          const backendHeader: string[] = page?.header || [];
          if (backendHeader.length > 0 && JSON.stringify(backendHeader) !== JSON.stringify(headerSnapshot)) {
            // Backend provided a filtered/normalized header
            // Update the tab's header
            tabState.setHeaderForTab(tabId, backendHeader);
            
            // Rebuild column definitions immediately when header changes
            // This is needed for strip, columns, dedup, and any operation that normalizes unnamed columns
            if (tabId === tabState.getCurrentTabState()?.tabId) {
              const currentTabState = tabState.getTabState(tabId);
              if (currentTabState) {
                const timeFieldToUse = currentTabState.timeField || '';
                // Preserve column JPath expressions when rebuilding column defs
                const columnJPathExprs = currentTabState.columnJPathExpressions || {};
                // Preserve highlight terms when rebuilding column defs
                const highlightTerms = currentTabState.highlightTerms || [];
                const defs = buildColumnDefs(backendHeader, timeFieldToUse, undefined, columnJPathExprs, highlightTerms);
                
                tabState.setColumnDefsForTab(tabId, defs);
                
                // Force grid to use new column definitions
                if (params.api) {
                  params.api.setColumnDefs(defs);
                }
              }
            }
          }

          const rowsArr: any[] = page?.rows || [];
          const effectiveHeader = backendHeader.length > 0 ? backendHeader : headerSnapshot;
          const rows = mapRows(effectiveHeader, rowsArr);
          
          // Include original and display indices in each row object for context menu access
          // This solves the issue where node.rowIndex is global but metadata arrays are local to the page
          const originalIndices = page?.originalIndices || [];
          const displayIndices = page?.displayIndices || [];
          if (originalIndices.length > 0 && displayIndices.length > 0) {
            // DEBUG: Log first few index mappings to verify
            console.log(`[INDEX_DEBUG] Received indices for rows ${startRow}-${endRow}, first 5 mappings:`);
            for (let i = 0; i < Math.min(5, originalIndices.length); i++) {
              console.log(`  [${i}] originalIndex=${originalIndices[i]}, displayIndex=${displayIndices[i]}`);
            }
            
            rows.forEach((row, i) => {
              if (i < originalIndices.length) {
                row.__originalIndex = originalIndices[i];
                row.__displayIndex = displayIndices[i];
              }
            });
          }
          
          const reportedTotal = (page?.total ?? page?.Total);
          const requested = Math.max(0, (endRow - startRow));
          
          // Extract and store annotations using absolute row indices
          const annotations: boolean[] = page?.annotations || [];
          const annotationColors: string[] = page?.annotationColors || [];

          // Update annotations map with absolute row indices
          // Get current tab state to access existing annotations
          const currentTabState = tabState.getTabState(tabId);
          if (currentTabState) {
            const updatedAnnotations = new Map(currentTabState.rowAnnotations || new Map());

            // Add annotations for this page using absolute row indices
            for (let i = 0; i < annotations.length; i++) {
              const absoluteRowIndex = startRow + i;
              if (annotations[i]) {
                updatedAnnotations.set(absoluteRowIndex, {
                  color: annotationColors[i] || 'grey'
                });
              } else {
                // Remove annotation if the row is no longer annotated
                updatedAnnotations.delete(absoluteRowIndex);
              }
            }

            // Update the tab's annotations for the specific tab (not just current tab)
            // This ensures annotations update when workspace is switched even for non-active tabs
            tabState.setRowAnnotationsForTab(tabId, updatedAnnotations);
          }
          
          let lastRow: number = -1;
          if (typeof reportedTotal === 'number' && reportedTotal >= 0) {
            lastRow = reportedTotal;
          } else {
            if (Array.isArray(rowsArr) && rowsArr.length < requested) {
              lastRow = startRow + rowsArr.length;
            } else if ((Array.isArray(rowsArr) ? rowsArr.length : 0) === 0 && startRow === 0) {
              lastRow = 0;
            } else {
              lastRow = -1;
            }
          }
          
          if (lastRow >= 0 && tabState.getGeneration() === thisGen) {
            tabState.setTotalRows(lastRow);
            const currentTab = tabState.getCurrentTabState();
            const gridApi = currentTab?.gridApi;
            if (gridApi && gridApi.setRowCount) {
              try { gridApi.setRowCount(lastRow, true); } catch {}
            }
          }
          
          // Clear any previous query error on successful execution
          tabState.setQueryError(null);
          
          params.successCallback(rows, lastRow);
          
          // Signal data load success (only for first page to avoid multiple resolves)
          if (startRow === 0) {
            resolveDataLoad(tabId);
          }
        } catch (e) {
          const errorMessage = e instanceof Error ? e.message : String(e);
          addLog("error", "Failed to load rows: " + errorMessage);
          
          // Set query error for the specific tab to show red outline
          tabState.setQueryError(errorMessage);
          
          // Show user-friendly error dialog for specific error types
          if (showErrorDialog) {
            if (errorMessage.includes("no plugin registered for extension")) {
              // Extract file extension from the error message
              const extensionMatch = errorMessage.match(/no plugin registered for extension: (\.[\w]+)/);
              const extension = extensionMatch ? extensionMatch[1] : "this file type";
              showErrorDialog(
                `Plugin Not Available\n\nThe plugin required to open files with extension "${extension}" is no longer available. ` +
                `This may have been caused by uninstalling the plugin while the file was still open.\n\n` +
                `Please reinstall the required plugin or close this file tab.`
              );
            } else if (errorMessage.includes("plugin")) {
              // Generic plugin-related error
              showErrorDialog(
                `Plugin Error\n\nThere was an error loading this file using a plugin:\n\n${errorMessage}\n\n` +
                `Please check that the required plugin is properly installed and configured.`
              );
            } else {
              // Other data loading errors
              showErrorDialog(
                `Data Loading Error\n\nFailed to load file data:\n\n${errorMessage}`
              );
            }
          }
          
          params.failCallback();
          // Signal data load failure
          rejectDataLoad(tabId, errorMessage);
        } finally {
          // Clear loading state for the specific tab
          tabState.setGridPendingForTab(tabId, -1);
        }
      }
    };
  }, [tabState, pageSize, buildColumnDefs, addLog, mapRows, useUnifiedFetch, showErrorDialog]);
  
  const getOriginalIndexForRow = useCallback((tabId: string, displayIndex: number): number | undefined => {
    const tab = tabState.getTabState(tabId);
    if (!tab?.rowMetadata?.originalIndices) return undefined;
    if (displayIndex < 0 || displayIndex >= tab.rowMetadata.originalIndices.length) return undefined;
    return tab.rowMetadata.originalIndices[displayIndex];
  }, [tabState]);
  
  const getDisplayIndexForRow = useCallback((tabId: string, displayIndex: number): number | undefined => {
    const tab = tabState.getTabState(tabId);
    if (!tab?.rowMetadata?.displayIndices) return undefined;
    if (displayIndex < 0 || displayIndex >= tab.rowMetadata.displayIndices.length) return undefined;
    return tab.rowMetadata.displayIndices[displayIndex];
  }, [tabState]);
  
  return {
    ensureRowVisible,
    selectAllDisplayed,
    selectRange,
    getSelectedRowIndexes,
    indexesToRanges,
    mapRows,
    createDataSource,
    selectingProgrammaticallyRef,
    getOriginalIndexForRow,
    getDisplayIndexForRow,
  };
};
