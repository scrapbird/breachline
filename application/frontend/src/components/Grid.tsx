import React, { useEffect, useRef, useState } from 'react';
import { AgGridReact } from 'ag-grid-react';
import LogoUniversal from '../assets/images/logo-universal.png';
import { applyJPathToCell } from '../utils/jpathUtils';

export type LogLevel = 'info' | 'warn' | 'error';

interface GridProps {
  columnDefs: any[];
  pageSize: number;
  datasource: any;
  theme: any;
  totalRows: number | null;
  gridPending: number;
  copyPending: number;
  onForceHideSpinner: () => void;
  onGridReady: (params: any) => void;
  gridApi: any | null;
  onCopySelected: () => void;
  onVirtualSelectAll: (enabled: boolean) => void; // synchronous setter from parent
  selectAllDisplayed: () => void;
  selectRange: (start: number, end: number, preserveSelection?: boolean) => void;
  clearSelectionRange: () => void;
  addLog: (level: LogLevel, msg: string) => void;
  containerRef: React.RefObject<HTMLDivElement>;
  rowAnnotations?: Map<number, { color: string }>; // Map of row index to annotation data
  onAnnotateRow?: (rowIndices?: number[]) => void; // Callback when user wants to annotate rows
  isLicensed: boolean; // Whether the user has a valid license
  isWorkspaceOpen: boolean; // Whether a workspace file is currently open
  columnJPathExpressions?: Record<string, string>; // Column name -> JPath expression for cell copy
  onJumpToOriginal?: (rowIndex: number) => void; // Callback to jump to original file position
  hasActiveQuery: boolean; // Whether there is an active query filter
  searchTerms?: string[]; // Search terms for highlighting in cell viewer dialog
  onShowCellViewer?: (columnName: string, cellValue: string) => void; // Callback to show cell viewer dialog
}

const Grid: React.FC<GridProps> = (props) => {
  const {
    columnDefs,
    pageSize,
    datasource,
    theme,
    totalRows,
    gridPending,
    copyPending,
    onForceHideSpinner,
    onGridReady,
    gridApi,
    onCopySelected,
    onVirtualSelectAll,
    selectAllDisplayed,
    selectRange,
    clearSelectionRange,
    addLog,
    containerRef,
    rowAnnotations,
    onAnnotateRow,
    isLicensed,
    isWorkspaceOpen,
    columnJPathExpressions,
    onJumpToOriginal,
    hasActiveQuery,
    searchTerms,
    onShowCellViewer,
  } = props;

  // Local state for custom context menu
  const [contextMenu, setContextMenu] = useState<{ visible: boolean; x: number; y: number; cellText: string; colField: string; rowIndex: number | null }>(
    { visible: false, x: 0, y: 0, cellText: '', colField: '', rowIndex: null }
  );

  // Drag-to-select rows state local to the grid view
  const isSelectingRef = useRef<boolean>(false);
  const anchorIndexRef = useRef<number | null>(null);
  // Local state controlling custom No Rows overlay visibility
  const [noRowsVisible, setNoRowsVisible] = useState<boolean>(false);

  // Centralized overlay state updater
  const updateNoRowsOverlay = (api: any) => {
    try {
      const displayed = api?.getDisplayedRowCount ? api.getDisplayedRowCount() : 0;
      const want = (typeof totalRows === 'number' && totalRows === 0 && displayed === 0);
      setNoRowsVisible(want);
      if (want) onForceHideSpinner();
    } catch {}
  };

  // Keep the NoRows overlay state in sync across container resizes (e.g., console panel open/close)
  useEffect(() => {
    const el = containerRef?.current as HTMLElement | null;
    // If no ResizeObserver support or no element, bail
    if (!el || !('ResizeObserver' in window)) return;
    const ro = new ResizeObserver(() => {
      try {
        updateNoRowsOverlay(gridApi);
      } catch {}
    });
    try { ro.observe(el); } catch {}
    return () => {
      try { ro.disconnect(); } catch {}
    };
  }, [containerRef, gridApi, totalRows, gridPending, onForceHideSpinner]);

  // React to totalRows or gridApi changes to keep the custom NoRows overlay accurate
  useEffect(() => {
    try {
      updateNoRowsOverlay(gridApi);
    } catch {}
  }, [totalRows, gridApi]);

  // Also react to datasource swaps and pending state changes to avoid stale overlays between cache purges
  useEffect(() => {
    try {
      updateNoRowsOverlay(gridApi);
    } catch {}
  }, [datasource, gridPending]);

  useEffect(() => {
    // Close the custom context menu on Escape, scroll, or outside interactions
    if (!contextMenu.visible) return;
    const onKey = (ev: KeyboardEvent) => {
      if (ev.key === 'Escape' || ev.key === 'Esc') {
        ev.preventDefault();
        ev.stopPropagation();
        setContextMenu(m => ({ ...m, visible: false }));
      }
    };
    const onWheel = () => setContextMenu(m => ({ ...m, visible: false }));
    const onScroll = () => setContextMenu(m => ({ ...m, visible: false }));
    window.addEventListener('keydown', onKey, true);
    window.addEventListener('wheel', onWheel, true);
    window.addEventListener('scroll', onScroll, true);
    return () => {
      window.removeEventListener('keydown', onKey, true);
      window.removeEventListener('wheel', onWheel, true);
      window.removeEventListener('scroll', onScroll, true);
    };
  }, [contextMenu.visible]);

  // Mouseup handler to end drag-to-select
  const onGlobalMouseUp = React.useCallback(() => {
    if (isSelectingRef.current) {
      isSelectingRef.current = false;
      anchorIndexRef.current = null;
      window.removeEventListener('mouseup', onGlobalMouseUp, true);
    }
  }, []);

  // Clipboard fallback for copying single cell text
  const copyPlainText = async (text: string): Promise<boolean> => {
    try {
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(text ?? '');
        return true;
      }
    } catch {}
    try {
      const ta = document.createElement('textarea');
      ta.value = text ?? '';
      ta.setAttribute('readonly', '');
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.focus();
      ta.select();
      const ok = document.execCommand('copy');
      document.body.removeChild(ta);
      return ok;
    } catch {
      return false;
    }
  };

  const NoRowsOverlay: React.FC = () => (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', opacity: 0.9, gap: 12 }}>
      <img src={LogoUniversal} alt="App logo" style={{ maxWidth: 160, width: '30%', height: 'auto' }} />
      <div style={{ fontSize: 14, color: '#bbb' }}>No data</div>
    </div>
  );

  return (
    <div
      ref={containerRef}
      style={{ width: '100%', height: '100%', position: 'relative' }}
      onClick={() => { if (contextMenu.visible) setContextMenu(m => ({ ...m, visible: false })); }}
    >
      <AgGridReact
        columnDefs={columnDefs}
        defaultColDef={{
          cellStyle: { textAlign: 'left' },
          sortable: false,
        }}
        overlayNoRowsTemplate=""
        rowSelection={"multiple"}
        rowMultiSelectWithClick={true}
        rowModelType="infinite"
        suppressCopyRowsToClipboard={true}
        suppressRowClickSelection={true}
        cacheBlockSize={pageSize}
        maxBlocksInCache={5}
        cacheOverflowSize={0}
        maxConcurrentDatasourceRequests={1}
        blockLoadDebounceMillis={30}
        datasource={datasource}
        theme={theme}
        domLayout="normal"
        getRowStyle={(params: any) => {
          const rowIndex = params?.node?.rowIndex;
          if (typeof rowIndex === 'number' && rowAnnotations) {
            const annotation = rowAnnotations.get(rowIndex);
            if (annotation) {
              // Map color name to hex value
              const colorMap: { [key: string]: string } = {
                white: '#6b675d',
                grey: '#3a3a3a',
                blue: '#2a4a7c',
                green: '#2a5a3a',
                yellow: '#6b5d2a',
                orange: '#6b4a2a',
                red: '#6b2a2a',
              };
              return { background: colorMap[annotation.color] || '#3a3a3a' };
            }
          }
          return undefined;
        }}
        onGridSizeChanged={(params: any) => {
          const api = params?.api;
          if (!api) return;
          updateNoRowsOverlay(api);
        }}
        onCellContextMenu={(e: any) => {
          e.event.preventDefault();
          const rect = containerRef?.current?.getBoundingClientRect();
          let x = (e.event.clientX ?? 0) - (rect?.left ?? 0);
          let y = (e.event.clientY ?? 0) - (rect?.top ?? 0);
          const val = (e && e.value != null) ? String(e.value) : '';
          // Use __originalIndex from row data instead of node.rowIndex to avoid index mismatch
          // node.rowIndex is global but metadata arrays are local to current page
          const originalIdx = e?.data?.__originalIndex;
          const colField = e?.colDef?.field || e?.colDef?.headerName || '';
          
          // Calculate menu dimensions to prevent overflow
          const menuWidth = 180; // minWidth from menu style
          const itemHeight = 40; // padding + text height per item
          const separatorHeight = 9; // height + margin
          
          // Count menu items
          let itemCount = 2; // Always: Copy cell, Copy selected rows
          if (onAnnotateRow && typeof originalIdx === 'number') itemCount++;
          if (onJumpToOriginal && typeof originalIdx === 'number' && hasActiveQuery) itemCount++;
          
          const separatorCount = itemCount - 1; // Separators between items
          const menuHeight = (itemCount * itemHeight) + (separatorCount * separatorHeight) + 12; // +12 for padding
          
          // Adjust position to keep menu within container bounds
          const containerWidth = rect?.width ?? 0;
          const containerHeight = rect?.height ?? 0;
          
          // Flip horizontally if overflowing right
          if (x + menuWidth > containerWidth) {
            x = Math.max(0, x - menuWidth);
          }
          
          // Flip vertically if overflowing bottom
          if (y + menuHeight > containerHeight) {
            y = Math.max(0, y - menuHeight);
          }
          
          setContextMenu({ visible: true, x, y, cellText: val, colField, rowIndex: typeof originalIdx === 'number' ? originalIdx : null });
        }}
        onCellDoubleClicked={(e: any) => {
          const colName = e?.colDef?.headerName || e?.colDef?.field || 'Cell';
          const value = e?.value != null ? String(e.value) : '';
          onShowCellViewer?.(colName, value);
        }}
        onGridReady={(params: any) => {
          onGridReady(params);
          updateNoRowsOverlay(params?.api);
        }}
        onCellKeyDown={(e: any) => {
          const ev: KeyboardEvent | undefined = e?.event;
          if (!ev) return;
          const isCmdOrCtrl = ev.ctrlKey || ev.metaKey;
          
          // Handle Ctrl/Cmd shortcuts
          if (isCmdOrCtrl) {
            if (ev.key === 'a' || ev.key === 'A') {
              ev.preventDefault();
              ev.stopPropagation();
              onVirtualSelectAll(true);
              clearSelectionRange();
              selectAllDisplayed();
              addLog('info', 'All rows for current query selected for copy');
            } else if (ev.key === 'c' || ev.key === 'C') {
              ev.preventDefault();
              ev.stopPropagation();
              onCopySelected();
            }
            return;
          }
          
          // Handle Enter key to open cell viewer dialog
          if (ev.key === 'Enter' && !ev.shiftKey && !ev.ctrlKey && !ev.metaKey && !ev.altKey) {
            ev.preventDefault();
            ev.stopPropagation();
            const colName = e?.colDef?.headerName || e?.colDef?.field || 'Cell';
            const value = e?.value != null ? String(e.value) : '';
            onShowCellViewer?.(colName, value);
            return;
          }
          
          // Handle 'a' key without modifiers for annotation
          if (ev.key === 'a' && !ev.shiftKey && !ev.altKey && onAnnotateRow) {
            ev.preventDefault();
            ev.stopPropagation();
            
            // Annotate all selected rows, not just the focused one
            // Let App.tsx handle getting the selection and validation
            onAnnotateRow();
          }
        }}
        onCellMouseDown={(e: any) => {
          const ev: MouseEvent | undefined = e?.event;
          if (!ev || ev.button !== 0) return;
          if (ev.shiftKey || ev.ctrlKey || ev.metaKey) return; // let row click handler cover these
          onVirtualSelectAll(false);
          const idx = e?.node?.rowIndex;
          if (typeof idx !== 'number') return;
          isSelectingRef.current = true;
          anchorIndexRef.current = idx;
          selectRange(idx, idx);
          // Attach global mouseup listener to end drag selection
          window.addEventListener('mouseup', onGlobalMouseUp, true);
        }}
        onCellMouseOver={(e: any) => {
          if (!isSelectingRef.current) return;
          const idx = e?.node?.rowIndex;
          const anchor = anchorIndexRef.current;
          if (typeof idx === 'number' && typeof anchor === 'number') {
            selectRange(anchor, idx);
          }
        }}
        onRowClicked={(e: any) => {
          const evt: MouseEvent | undefined = e?.event;
          const idx = e?.node?.rowIndex;
          if (typeof idx !== 'number') return;
          if (!evt || evt.button !== 0) return; // preserve selection on right-click
          if (evt && (evt.ctrlKey || evt.metaKey)) {
            onVirtualSelectAll(false);
            clearSelectionRange();
            const node = gridApi?.getDisplayedRowAtIndex?.(idx);
            if (node) {
              const currentlySelected = !!node.isSelected?.();
              node.setSelected?.(!currentlySelected);
              anchorIndexRef.current = idx;
            }
            return;
          }
          if (evt && evt.shiftKey && anchorIndexRef.current != null) {
            onVirtualSelectAll(false);
            selectRange(anchorIndexRef.current, idx, true);
            return;
          }
          if (gridApi) {
            onVirtualSelectAll(false);
            if (gridApi.deselectAll) gridApi.deselectAll();
            const node = gridApi.getDisplayedRowAtIndex?.(idx);
            node?.setSelected?.(true);
            anchorIndexRef.current = idx;
            // parent maintains the contiguous range via selectRange()
            selectRange(idx, idx);
          }
        }}
        onSelectionChanged={() => {
          try {
            const count = gridApi?.getSelectedNodes ? gridApi.getSelectedNodes().length : 0;
            if (!count) clearSelectionRange();
          } catch {}
        }}
        onModelUpdated={(params: any) => {
          const api = params.api;
          if (!api) return;
          updateNoRowsOverlay(api);
        }}
      />

      {(gridPending > 0 && (totalRows == null || totalRows !== 0)) && (
        <div
          style={{
            position: 'absolute',
            inset: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            background: 'rgba(0,0,0,0.25)',
            borderRadius: 8,
            pointerEvents: 'none',
          }}
        >
          <i className="fas fa-spinner fa-spin" style={{ color: '#ddd', fontSize: 24 }} aria-label="Loading grid" />
        </div>
      )}

      {noRowsVisible && (
        <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', pointerEvents: 'none' }}>
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 12, opacity: 0.95 }}>
            <img src={LogoUniversal} alt="App logo" style={{ maxWidth: 160, width: '30%', height: 'auto' }} />
            <div style={{ fontSize: 14, color: '#bbb' }}>No data</div>
          </div>
        </div>
      )}

      {contextMenu.visible && (
        <div
          style={{ position: 'absolute', left: contextMenu.x, top: contextMenu.y, zIndex: 3001 }}
          onClick={(e) => { e.stopPropagation(); }}
        >
          <div
            style={{
              minWidth: 180,
              background: '#222',
              color: '#eee',
              border: '1px solid #444',
              borderRadius: 6,
              boxShadow: '0 8px 20px rgba(0,0,0,0.45)',
              padding: 6,
              textAlign: 'left',
            }}
          >
            <div
              role="menuitem"
              tabIndex={0}
              onClick={async () => {
                // Apply JPath expression if one is set for this column
                const jpathExpr = contextMenu.colField && columnJPathExpressions?.[contextMenu.colField];
                const displayValue = jpathExpr ? applyJPathToCell(contextMenu.cellText || '', jpathExpr) : (contextMenu.cellText || '');
                const ok = await copyPlainText(displayValue);
                if (!ok) addLog('warn', 'Unable to access clipboard');
                setContextMenu(m => ({ ...m, visible: false }));
              }}
              onKeyDown={async (ev) => {
                if (ev.key === 'Enter' || ev.key === ' ') {
                  ev.preventDefault();
                  // Apply JPath expression if one is set for this column
                  const jpathExpr = contextMenu.colField && columnJPathExpressions?.[contextMenu.colField];
                  const displayValue = jpathExpr ? applyJPathToCell(contextMenu.cellText || '', jpathExpr) : (contextMenu.cellText || '');
                  const ok = await copyPlainText(displayValue);
                  if (!ok) addLog('warn', 'Unable to access clipboard');
                  setContextMenu(m => ({ ...m, visible: false }));
                }
              }}
              style={{ padding: '8px 10px', borderRadius: 4, cursor: 'pointer' }}
              onMouseDown={(e) => e.stopPropagation()}
              onMouseEnter={(e) => { (e.currentTarget as HTMLElement).style.background = '#2a2a2a'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLElement).style.background = 'transparent'; }}
            >
              Copy cell
            </div>
            <div style={{ height: 1, background: '#333', margin: '4px 0' }} />
            <div
              role="menuitem"
              tabIndex={0}
              onClick={async () => {
                await onCopySelected();
                setContextMenu(m => ({ ...m, visible: false }));
              }}
              onKeyDown={async (ev) => {
                if (ev.key === 'Enter' || ev.key === ' ') {
                  ev.preventDefault();
                  await onCopySelected();
                  setContextMenu(m => ({ ...m, visible: false }));
                }
              }}
              style={{ padding: '8px 10px', borderRadius: 4, cursor: 'pointer' }}
              onMouseDown={(e) => e.stopPropagation()}
              onMouseEnter={(e) => { (e.currentTarget as HTMLElement).style.background = '#2a2a2a'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLElement).style.background = 'transparent'; }}
            >
              Copy selected rows
            </div>
            {onAnnotateRow && typeof contextMenu.rowIndex === 'number' && (
              <>
                <div style={{ height: 1, background: '#333', margin: '4px 0' }} />
                <div
                  role="menuitem"
                  tabIndex={0}
                  onClick={() => {
                    if (!isLicensed) {
                      addLog('warn', 'Annotations require a valid license');
                      setContextMenu(m => ({ ...m, visible: false }));
                      return;
                    }
                    if (!isWorkspaceOpen) {
                      addLog('warn', 'Please open a workspace file first (File → Open workspace)');
                      setContextMenu(m => ({ ...m, visible: false }));
                      return;
                    }
                    // Right-click should annotate all selected rows (or just the clicked row if none selected)
                    onAnnotateRow();
                    setContextMenu(m => ({ ...m, visible: false }));
                  }}
                  onKeyDown={(ev) => {
                    if (ev.key === 'Enter' || ev.key === ' ') {
                      ev.preventDefault();
                      if (!isLicensed) {
                        addLog('warn', 'Annotations require a valid license');
                        setContextMenu(m => ({ ...m, visible: false }));
                        return;
                      }
                      if (!isWorkspaceOpen) {
                        addLog('warn', 'Please open a workspace file first (File → Open workspace)');
                        setContextMenu(m => ({ ...m, visible: false }));
                        return;
                      }
                      // Right-click should annotate all selected rows (or just the clicked row if none selected)
                      onAnnotateRow();
                      setContextMenu(m => ({ ...m, visible: false }));
                    }
                  }}
                  style={{ 
                    padding: '8px 10px', 
                    borderRadius: 4, 
                    cursor: (isLicensed && isWorkspaceOpen) ? 'pointer' : 'not-allowed',
                    opacity: (isLicensed && isWorkspaceOpen) ? 1 : 0.5 
                  }}
                  onMouseDown={(e) => e.stopPropagation()}
                  onMouseEnter={(e) => { if (isLicensed && isWorkspaceOpen) (e.currentTarget as HTMLElement).style.background = '#2a2a2a'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLElement).style.background = 'transparent'; }}
                >
                  Annotate
                </div>
              </>
            )}
            {onJumpToOriginal && typeof contextMenu.rowIndex === 'number' && hasActiveQuery && (
              <>
                <div style={{ height: 1, background: '#333', margin: '4px 0' }} />
                <div
                  role="menuitem"
                  tabIndex={0}
                  onClick={() => {
                    onJumpToOriginal(contextMenu.rowIndex!);
                    setContextMenu(m => ({ ...m, visible: false }));
                  }}
                  onKeyDown={(ev) => {
                    if (ev.key === 'Enter' || ev.key === ' ') {
                      ev.preventDefault();
                      onJumpToOriginal(contextMenu.rowIndex!);
                      setContextMenu(m => ({ ...m, visible: false }));
                    }
                  }}
                  style={{ 
                    padding: '8px 10px', 
                    borderRadius: 4, 
                    cursor: 'pointer'
                  }}
                  onMouseDown={(e) => e.stopPropagation()}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLElement).style.background = '#2a2a2a'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLElement).style.background = 'transparent'; }}
                >
                  Jump to original position
                </div>
              </>
            )}
          </div>
        </div>
      )}

      {copyPending > 0 && (
        <div
          style={{
            position: 'absolute',
            inset: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            background: 'rgba(0,0,0,0.35)',
            borderRadius: 8,
            zIndex: 10,
            pointerEvents: 'none',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, color: '#ddd' }}>
            <i className="fas fa-spinner fa-spin" style={{ color: '#ddd', fontSize: 20 }} aria-label="Copying selection" />
            <span style={{ fontSize: 14, opacity: 0.95 }}>Copying to clipboard…</span>
          </div>
        </div>
      )}
    </div>
  );
};

export default Grid;
