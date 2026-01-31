import React, { useRef, useEffect, useCallback, useState } from 'react';
import Grid from './Grid';
import ResultsPanel from './ResultsPanel';
import RowCountIndicator from './RowCountIndicator';
import { AnnotationInfo } from './AnnotationPanel';
import { UseTabStateReturn } from '../hooks/useTabState';
import { UseGridOperationsReturn } from '../hooks/useGridOperations';
import { ResultsPanelTab } from '../types/TabState';
// @ts-ignore - Wails generated bindings
import * as AppAPI from '../../wailsjs/go/app/App';

export type LogLevel = 'info' | 'warn' | 'error';

interface GridManagerProps {
  tabState: UseTabStateReturn;
  gridOps: UseGridOperationsReturn;
  pageSize: number;
  theme: any;
  onCopySelected: () => void;
  onAnnotateRow?: (rowIndices?: number[]) => void;
  isLicensed: boolean;
  isWorkspaceOpen: boolean;
  addLog: (level: LogLevel, msg: string) => void;
  copyPending: number;
  onJumpToOriginal?: (tabId: string, rowIndex: number) => void;
  onClearSearch?: (tabId: string) => void;
  onSearch?: (tabId: string, term: string, isRegex: boolean) => void;
  onCancelSearch?: (tabId: string) => void;
}

const GridManager: React.FC<GridManagerProps> = ({
  tabState,
  gridOps,
  pageSize,
  theme,
  onCopySelected,
  onAnnotateRow,
  isLicensed,
  isWorkspaceOpen,
  addLog,
  copyPending,
  onJumpToOriginal,
  onClearSearch,
  onSearch,
  onCancelSearch,
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  
  // Per-tab annotation data for the annotation panel
  const [tabAnnotations, setTabAnnotations] = useState<Map<string, AnnotationInfo[]>>(new Map());
  
  // Fetch annotations for a tab when its panel is shown
  const fetchAnnotationsForTab = useCallback(async (tabId: string) => {
    const tab = tabState.getTabState(tabId);
    if (!tab || !tab.fileHash) return;
    
    try {
      const result = await AppAPI.GetFileAnnotations(tab.fileHash, tab.fileOptions || {});
      
      const annotations: AnnotationInfo[] = (result || []).map((a: any) => ({
        originalRowIndex: a.originalRowIndex,
        displayRowIndex: a.displayRowIndex >= 0 ? a.displayRowIndex : -1,
        note: a.note || '',
        color: a.color || 'grey',
      }));
      
      setTabAnnotations(prev => {
        const next = new Map(prev);
        next.set(tabId, annotations);
        return next;
      });
    } catch (e) {
      console.error('Failed to fetch annotations for tab:', tabId, e);
    }
  }, [tabState]);
  
  // Track the applied query for the active tab to refresh annotations when it changes
  const activeTab = tabState.activeTabId ? tabState.getTabState(tabState.activeTabId) : null;
  const activeAppliedQuery = activeTab?.appliedQuery || '';
  
  // Fetch annotations when panel is shown, annotations change, or query changes
  useEffect(() => {
    const activeTabId = tabState.activeTabId;
    if (!activeTabId) return;
    
    const tab = tabState.getTabState(activeTabId);
    if (tab?.showAnnotationPanel) {
      fetchAnnotationsForTab(activeTabId);
    }
  }, [tabState.activeTabId, tabState.stateVersion, activeAppliedQuery, fetchAnnotationsForTab]);
  
  // Handle annotation click - navigate to the row
  const handleAnnotationClick = useCallback(async (tabId: string, displayIndex: number) => {
    if (displayIndex < 0) return;
    
    const tab = tabState.getTabState(tabId);
    if (!tab?.gridApi) return;
    
    // Get the first column's ID for cell focus
    const allColumns = tab.gridApi.getColumns?.() || [];
    const firstColId = allColumns.length > 0 ? allColumns[0]?.getColId?.() : undefined;
    
    // Scroll to the row first
    gridOps.ensureRowVisible(displayIndex);
    
    // Wait for the row to be loaded (infinite scroll may need time to fetch data)
    const maxAttempts = 5;
    const delayMs = 100;
    
    for (let attempt = 0; attempt < maxAttempts; attempt++) {
      await new Promise(resolve => setTimeout(resolve, delayMs));
      
      const rowNode = tab.gridApi.getDisplayedRowAtIndex(displayIndex);
      if (rowNode?.data) {
        gridOps.selectRange(displayIndex, displayIndex);
        // Focus the first column of the row
        if (firstColId) {
          tab.gridApi.setFocusedCell?.(displayIndex, firstColId);
        }
        return;
      }
    }
    
    // Last attempt
    gridOps.selectRange(displayIndex, displayIndex);
    // Focus the first column of the row
    if (firstColId) {
      tab.gridApi.setFocusedCell?.(displayIndex, firstColId);
    }
  }, [tabState, gridOps]);
  
  // Handle results panel height change (shared by both annotation and search panels)
  const handleResultsPanelHeightChange = useCallback((tabId: string, height: number) => {
    tabState.setAnnotationPanelHeightForTab(tabId, height);
  }, [tabState]);
  
  // Handle results panel tab change
  const handleResultsTabChange = useCallback((tabId: string, tab: ResultsPanelTab) => {
    tabState.setActiveResultsTabForTab(tabId, tab);
  }, [tabState]);
  
  // Handle search result click - navigate to the specific cell
  const handleSearchResultClick = useCallback(async (tabId: string, rowIndex: number, columnIndex: number, columnName?: string) => {
    if (rowIndex < 0) return;
    
    const tab = tabState.getTabState(tabId);
    if (!tab?.gridApi) return;
    
    // Get the column ID - we need to find the column by name because the grid may have
    // reordered columns (e.g., timestamp column pinned to front). The columnIndex from
    // the search is based on original data order, not the grid's visual order.
    const allColumns = tab.gridApi.getColumns?.() || [];
    let targetColId: string | undefined;
    
    if (columnName) {
      // Find column by matching headerName or field/colId to the column name
      const targetColumn = allColumns.find((col: any) => {
        const colDef = col.getColDef?.();
        if (!colDef) return false;
        // Match against headerName (display name) or field/colId (data key)
        return colDef.headerName === columnName || 
               colDef.field === columnName || 
               col.getColId?.() === columnName;
      });
      targetColId = targetColumn?.getColId?.();
    }
    
    // Fallback to index-based lookup if name matching failed
    if (!targetColId && columnIndex >= 0 && columnIndex < allColumns.length) {
      // Use the original header to find the correct column
      // The header array is in original data order, so header[columnIndex] gives us the column name
      const originalColumnName = tab.header?.[columnIndex];
      if (originalColumnName) {
        const targetColumn = allColumns.find((col: any) => {
          const colDef = col.getColDef?.();
          if (!colDef) return false;
          return colDef.headerName === originalColumnName || 
                 colDef.field === originalColumnName || 
                 col.getColId?.() === originalColumnName;
        });
        targetColId = targetColumn?.getColId?.();
      }
      
      // Last resort: use index directly (may be wrong if columns are reordered)
      if (!targetColId) {
        targetColId = allColumns[columnIndex]?.getColId?.();
      }
    }
    
    // Scroll to the row first
    gridOps.ensureRowVisible(rowIndex);
    
    // Also ensure the column is visible
    if (targetColId) {
      tab.gridApi.ensureColumnVisible?.(targetColId);
    }
    
    // Wait for the row to be loaded (infinite scroll may need time to fetch data)
    const maxAttempts = 5;
    const delayMs = 100;
    
    for (let attempt = 0; attempt < maxAttempts; attempt++) {
      await new Promise(resolve => setTimeout(resolve, delayMs));
      
      // Check if the row is now loaded
      const rowNode = tab.gridApi.getDisplayedRowAtIndex(rowIndex);
      if (rowNode?.data) {
        // Row is loaded, select the row and focus the cell
        gridOps.selectRange(rowIndex, rowIndex);
        
        // Focus the specific cell
        if (targetColId) {
          tab.gridApi.setFocusedCell?.(rowIndex, targetColId);
        }
        return;
      }
    }
    
    // Last attempt - try to select anyway
    gridOps.selectRange(rowIndex, rowIndex);
    if (targetColId) {
      tab.gridApi.setFocusedCell?.(rowIndex, targetColId);
    }
  }, [tabState, gridOps]);
  
  // Handle search page change
  const handleSearchPageChange = useCallback(async (tabId: string, page: number) => {
    try {
      const result = await AppAPI.GetSearchResultsPage(tabId, page);
      if (result) {
        tabState.setSearchCurrentPageForTab(tabId, page);
        tabState.setSearchResultsForTab(tabId, result.results || [], result.totalCount);
      }
    } catch (e) {
      console.error('Failed to fetch search results page:', e);
      addLog('error', 'Failed to load search results page');
    }
  }, [tabState, addLog]);
  
  // Handle clear search - delegate to parent for column def rebuild
  const handleClearSearch = useCallback((tabId: string) => {
    if (onClearSearch) {
      onClearSearch(tabId);
    }
  }, [onClearSearch]);
  
  // Handle search from the search panel
  const handleSearch = useCallback((tabId: string, term: string, isRegex: boolean) => {
    if (onSearch) {
      onSearch(tabId, term, isRegex);
    }
  }, [onSearch]);
  
  // Handle cancel search
  const handleCancelSearch = useCallback((tabId: string) => {
    if (onCancelSearch) {
      onCancelSearch(tabId);
    }
  }, [onCancelSearch]);
  
  // Handle edit annotation from panel - navigate to row and open annotation dialog
  const handleEditAnnotation = useCallback((tabId: string, displayIndex: number) => {
    if (displayIndex < 0) return;
    
    const tab = tabState.getTabState(tabId);
    if (!tab?.gridApi) return;
    
    // Scroll to the row and select it
    gridOps.ensureRowVisible(displayIndex);
    gridOps.selectRange(displayIndex, displayIndex);
    
    // Open annotation dialog for this row
    if (onAnnotateRow) {
      onAnnotateRow([displayIndex]);
    }
  }, [tabState, gridOps, onAnnotateRow]);

  // No need for complex tab switching logic - display:none preserves scroll positions naturally

  // Handle grid ready for each tab
  const handleGridReady = useCallback((tabId: string, params: any) => {
    const api = params.api;
    const columnApi = params.columnApi;
    
    // Update tab state with grid APIs
    tabState.setGridApiForTab(tabId, api);
    tabState.setColumnApiForTab(tabId, columnApi);
    
    // Get the tab state by ID to ensure we're working with the right tab
    const tab = tabState.getTabState(tabId);
    if (tab && tab.header.length > 0) {
      // Reuse existing datasource if available, otherwise create new one
      // This prevents the grid from being cleared when switching tabs
      let ds = tab.datasource;
      if (!ds) {
        ds = gridOps.createDataSource(
          tabId,
          tab.appliedQuery || '',
          tab.header,
          tab.timeField
        );
        tabState.setDatasourceForTab(tabId, ds);
      }
      
      try {
        // Note: Datasource is set via props, not API
        // The Grid component will use the datasource from tabData.datasource
        api.hideOverlay?.();
        // Don't call ensureIndexVisible(0) as it resets scroll position to top
      } catch (e) {
        console.warn('Failed to initialize grid in onGridReady:', e);
      }
    }
    
    // Mark grid as initialized
    tabState.setGridInitializedForTab(tabId, true);
  }, [tabState, gridOps]);

  // Handle virtual select all
  const handleVirtualSelectAll = useCallback((enabled: boolean) => {
    tabState.setVirtualSelectAll(enabled);
  }, [tabState]);

  // Clear selection range
  const clearSelectionRange = useCallback(() => {
    tabState.setSelectionRange(null);
  }, [tabState]);

  // Force hide spinner
  const onForceHideSpinner = useCallback(() => {
    // This can be implemented if needed for specific spinner logic
  }, []);

  // Handle keyboard shortcuts for grid navigation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Handle grid navigation shortcuts
      const activeTabId = tabState.activeTabId;
      if (!activeTabId) return;
      
      const activeTab = tabState.getTabState(activeTabId);
      const gridApi = activeTab?.gridApi;
      if (!gridApi) return;

      const isMac = /Mac|iPod|iPhone|iPad/.test(navigator.userAgent);
      const cmdOrCtrl = isMac ? e.metaKey : e.ctrlKey;
      const keyLower = (e.key || '').toLowerCase();

      // Cmd/Ctrl+G: focus the grid
      if (cmdOrCtrl && !e.shiftKey && !e.altKey && keyLower === 'g') {
        e.preventDefault();
        e.stopPropagation();

        const focusedCell = gridApi.getFocusedCell?.();
        const allColumns = gridApi.getColumns?.();
        const defaultColId = allColumns && allColumns.length > 0 ? allColumns[0]?.getColId?.() : undefined;
        const colKey = focusedCell?.column?.getColId?.() || defaultColId;

        let totalRows = typeof activeTab.totalRows === 'number' ? activeTab.totalRows : undefined;
        if (totalRows == null && gridApi.getDisplayedRowCount) {
          totalRows = gridApi.getDisplayedRowCount();
        }
        const safeTotalRows = typeof totalRows === 'number' && totalRows > 0 ? totalRows : 0;
        const currentRow = focusedCell?.rowIndex ?? 0;
        const targetRow = safeTotalRows > 0 ? Math.max(0, Math.min(safeTotalRows - 1, currentRow)) : 0;

        if (colKey && safeTotalRows > 0) {
          gridApi.setFocusedCell?.(targetRow, colKey);
          gridApi.ensureIndexVisible?.(targetRow);
          gridApi.ensureColumnVisible?.(colKey);
        } else if (colKey) {
          gridApi.ensureColumnVisible?.(colKey);
        }

        // Attempt to focus the grid wrapper element for accessibility
        const gridWrapper = containerRef.current?.querySelector<HTMLElement>(`[data-tab-id="${activeTabId}"] .ag-root-wrapper`);
        gridWrapper?.focus?.();
        return;
      }

      if (!document.activeElement?.closest('.ag-root-wrapper')) {
        return;
      }

      // g: jump to top
      if (e.key === 'g' && !cmdOrCtrl && !e.shiftKey && !e.altKey) {
        const allColumns = gridApi.getColumns?.();
        e.preventDefault();
        e.stopPropagation();
        if (gridApi.ensureIndexVisible) {
          const targetColumn = allColumns[0];
          const targetColId = targetColumn?.getColId?.();
          gridApi.ensureIndexVisible(0, 'top');
          gridApi.setFocusedCell?.(0, targetColId);
        }
        return;
      }
      
      // G (Shift+g): jump to bottom
      if (e.key === 'G' && !cmdOrCtrl && !e.altKey) {
        e.preventDefault();
        e.stopPropagation();
        const allColumns = gridApi.getColumns?.();
        if (gridApi.ensureIndexVisible && activeTab.totalRows != null) {
          const targetColumn = allColumns[0];
          const targetColId = targetColumn?.getColId?.();
          const lastIndex = Math.max(0, activeTab.totalRows - 1);
          gridApi.ensureIndexVisible(lastIndex, 'bottom');
          gridApi.setFocusedCell?.(lastIndex, targetColId);
        }
        return;
      }
      
      // 0 or ^: scroll all the way left
      if ((e.key === '0' && !cmdOrCtrl && !e.shiftKey && !e.altKey) || (e.key === '^' && !cmdOrCtrl && !e.altKey)) {
        e.preventDefault();
        e.stopPropagation();
        const focusedCell = gridApi.getFocusedCell?.();
        const allColumns = gridApi.getColumns?.();
        if (allColumns && allColumns.length > 0) {
          const targetColumn = allColumns[0];
          const targetColId = targetColumn?.getColId?.();
          gridApi.ensureColumnVisible?.(targetColumn);
          gridApi.setFocusedCell?.(focusedCell.rowIndex, targetColId);
        }
        return;
      }
      
      // $: scroll all the way right
      if (e.key === '$' && !cmdOrCtrl && !e.altKey) {
        e.preventDefault();
        e.stopPropagation();
        const focusedCell = gridApi.getFocusedCell?.();
        const allColumns = gridApi.getColumns?.();
        if (allColumns && allColumns.length > 0) {
          const targetColumn = allColumns[allColumns.length - 1];
          const targetColId = targetColumn?.getColId?.();
          gridApi.ensureColumnVisible?.(targetColumn);
          gridApi.setFocusedCell?.(focusedCell.rowIndex, targetColId);
        }
        return;
      }
      
      // k: move cell focus up one row (like up arrow)
      if (e.key === 'k' && !cmdOrCtrl && !e.shiftKey && !e.altKey) {
        e.preventDefault();
        e.stopPropagation();
        const focusedCell = gridApi.getFocusedCell?.();
        const currentRow = focusedCell?.rowIndex ?? 0;
        const targetIndex = Math.max(0, currentRow - 1);
        const allColumns = gridApi.getColumns?.();
        const colKey = focusedCell?.column?.getColId?.() || (allColumns?.[0]?.getColId?.());
        if (colKey) {
          gridApi.setFocusedCell?.(targetIndex, colKey);
          gridApi.ensureIndexVisible?.(targetIndex);
        }
        return;
      }
      
      // j: move cell focus down one row (like down arrow)
      if (e.key === 'j' && !cmdOrCtrl && !e.shiftKey && !e.altKey) {
        e.preventDefault();
        e.stopPropagation();
        const focusedCell = gridApi.getFocusedCell?.();
        const currentRow = focusedCell?.rowIndex ?? 0;
        const totalRows = activeTab.totalRows ?? 0;
        const targetIndex = Math.max(0, Math.min(totalRows - 1, currentRow + 1));
        const allColumns = gridApi.getColumns?.();
        const colKey = focusedCell?.column?.getColId?.() || (allColumns?.[0]?.getColId?.());
        if (colKey) {
          gridApi.setFocusedCell?.(targetIndex, colKey);
          gridApi.ensureIndexVisible?.(targetIndex);
        }
        return;
      }

      // h: move cell focus left one column (like left arrow)
      if (e.key === 'h' && !cmdOrCtrl && !e.shiftKey && !e.altKey) {
        e.preventDefault();
        e.stopPropagation();
        const focusedCell = gridApi.getFocusedCell?.();
        const currentRow = focusedCell?.rowIndex ?? 0;
        const allColumns = gridApi.getColumns?.();
        if (allColumns && allColumns.length > 0) {
          const currentColId = focusedCell?.column?.getColId?.();
          let targetColIndex = 0;
          if (currentColId) {
            const idx = allColumns.findIndex((col: any) => col?.getColId?.() === currentColId);
            targetColIndex = idx > 0 ? idx - 1 : 0;
          }
          const targetColId = allColumns[targetColIndex]?.getColId?.();
          if (targetColId) {
            gridApi.setFocusedCell?.(currentRow, targetColId);
            gridApi.ensureColumnVisible?.(targetColId);
          }
        }
        return;
      }

      // l: move cell focus right one column (like right arrow)
      if (e.key === 'l' && !cmdOrCtrl && !e.shiftKey && !e.altKey) {
        e.preventDefault();
        e.stopPropagation();
        const focusedCell = gridApi.getFocusedCell?.();
        const currentRow = focusedCell?.rowIndex ?? 0;
        const allColumns = gridApi.getColumns?.();
        if (allColumns && allColumns.length > 0) {
          const currentColId = focusedCell?.column?.getColId?.();
          let targetColIndex = allColumns.length - 1;
          if (currentColId) {
            const idx = allColumns.findIndex((col: any) => col?.getColId?.() === currentColId);
            targetColIndex = idx >= 0 ? Math.min(allColumns.length - 1, idx + 1) : 0;
          }
          const targetColId = allColumns[targetColIndex]?.getColId?.();
          if (targetColId) {
            gridApi.setFocusedCell?.(currentRow, targetColId);
            gridApi.ensureColumnVisible?.(targetColId);
          }
        }
        return;
      }
    };

    // Add event listeners
    document.addEventListener('keydown', handleKeyDown);
    
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [tabState]);

  // Subscribe to stateVersion to re-render when tab state changes (e.g., rowAnnotations)
  // This is necessary because tabState.getTabState reads from a ref, not React state
  const _ = tabState.stateVersion;

  return (
    <div 
      ref={containerRef}
      style={{ 
        position: 'relative', 
        width: '100%', 
        height: '100%',
        overflow: 'hidden'
      }}
    >
      {tabState.tabs.map(tab => {
        const tabData = tabState.getTabState(tab.id);
        if (!tabData || tab.id === '__dashboard__') return null;

        const isActive = tab.id === tabState.activeTabId;
        
        // Calculate grid height based on results panel visibility (annotations or search)
        const showResultsPanel = tabData.showAnnotationPanel || tabData.showSearchPanel;
        const resultsPanelHeight = showResultsPanel ? tabData.annotationPanelHeight : 0;
        const gridHeight = showResultsPanel ? `calc(100% - ${resultsPanelHeight}px - 4px)` : '100%';
        
        // Get search results for this tab
        const searchResults = tabData.searchResults.map(r => ({
          rowIndex: r.rowIndex,
          columnIndex: r.columnIndex,
          columnName: r.columnName,
          matchStart: r.matchStart,
          matchEnd: r.matchEnd,
          snippet: r.snippet,
        }));
        
        return (
          <div
            key={tab.id}
            data-tab-id={tab.id}
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              width: '100%',
              height: '100%',
              visibility: isActive ? 'visible' : 'hidden',
              pointerEvents: isActive ? 'auto' : 'none', // Prevent interaction with hidden grids
              zIndex: isActive ? 1 : 0,
              display: 'flex',
              flexDirection: 'column',
            }}
          >
            <div style={{ flex: showResultsPanel ? 'none' : 1, height: gridHeight, minHeight: 0, position: 'relative' }}>
              <Grid
                columnDefs={tabData.columnDefs}
                pageSize={pageSize}
                datasource={tabData.datasource}
                theme={theme}
                totalRows={tabData.totalRows}
                gridPending={tabData.gridPending}
                copyPending={copyPending}
                onForceHideSpinner={onForceHideSpinner}
                onGridReady={(params) => handleGridReady(tab.id, params)}
                gridApi={tabData.gridApi}
                onCopySelected={onCopySelected}
                onVirtualSelectAll={handleVirtualSelectAll}
                selectAllDisplayed={gridOps.selectAllDisplayed}
                selectRange={gridOps.selectRange}
                clearSelectionRange={clearSelectionRange}
                addLog={addLog}
                containerRef={containerRef}
                rowAnnotations={tabData.rowAnnotations}
                onAnnotateRow={onAnnotateRow}
                isLicensed={isLicensed}
                isWorkspaceOpen={isWorkspaceOpen}
                columnJPathExpressions={tabData.columnJPathExpressions}
                onJumpToOriginal={onJumpToOriginal ? (rowIndex) => onJumpToOriginal(tab.id, rowIndex) : undefined}
                hasActiveQuery={!!tabData.appliedQuery && tabData.appliedQuery.trim().length > 0}
                searchTerms={tabData.highlightTerms}
                onShowCellViewer={(columnName, cellValue) => {
                  tabState.showCellViewerForTab(tab.id, columnName, cellValue);
                }}
              />
              
              {/* Row count indicator - attached to grid */}
              {isActive && (
                <RowCountIndicator
                  visible={true}
                  totalRows={tabData.totalRows}
                />
              )}
            </div>
            
            {/* Results Panel (Annotations + Search) */}
            <ResultsPanel
              show={showResultsPanel}
              showAnnotationTab={tabData.showAnnotationPanel}
              showSearchTab={tabData.showSearchPanel}
              activeTab={tabData.activeResultsTab}
              onTabChange={(t) => handleResultsTabChange(tab.id, t)}
              height={tabData.annotationPanelHeight}
              onHeightChange={(h: number) => handleResultsPanelHeightChange(tab.id, h)}
              annotations={tabAnnotations.get(tab.id) || []}
              onAnnotationClick={(displayIndex: number) => handleAnnotationClick(tab.id, displayIndex)}
              onEditAnnotation={(displayIndex: number) => handleEditAnnotation(tab.id, displayIndex)}
              searchResults={searchResults}
              searchTotalCount={tabData.searchTotalCount}
              searchCurrentPage={tabData.searchCurrentPage}
              searchPageSize={1000}
              searchTerm={tabData.searchTerm}
              searchIsRegex={tabData.searchIsRegex}
              isSearching={tabData.isSearching}
              onSearchResultClick={(rowIndex: number, columnIndex: number, columnName: string) => handleSearchResultClick(tab.id, rowIndex, columnIndex, columnName)}
              onSearchPageChange={(page: number) => handleSearchPageChange(tab.id, page)}
              onClearSearch={() => handleClearSearch(tab.id)}
              onSearch={(term: string, isRegex: boolean) => handleSearch(tab.id, term, isRegex)}
              onCancelSearch={() => handleCancelSearch(tab.id)}
            />
          </div>
        );
      })}
    </div>
  );
};

export default GridManager;
