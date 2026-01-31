import { useCallback, useState, useRef } from 'react';
import { UseTabStateReturn } from './useTabState';

export interface UseClipboardOperationsProps {
  tabState: UseTabStateReturn;
  getSelectedRowIndexes: () => number[];
  indexesToRanges: (idxs: number[]) => { start: number; end: number }[];
  gridContainerRef: React.RefObject<HTMLDivElement>;
  addLog: (level: 'info' | 'warn' | 'error', message: string) => void;
  showErrorDialog?: (title: string, message: string) => void;
}

export const useClipboardOperations = ({
  tabState,
  getSelectedRowIndexes,
  indexesToRanges,
  gridContainerRef,
  addLog,
  showErrorDialog,
}: UseClipboardOperationsProps) => {
  const [copyPending, setCopyPending] = useState<number>(0);
  
  const getSelectedRowIndexesFromDom = useCallback((): number[] => {
    const container = gridContainerRef.current;
    if (!container) return [];
    try {
      const nodes = container.querySelectorAll('.ag-center-cols-container .ag-row.ag-row-selected');
      const idxs: number[] = [];
      nodes.forEach((el) => {
        const attr = (el as HTMLElement).getAttribute('row-index');
        if (!attr) return;
        const n = parseInt(attr, 10);
        if (!Number.isNaN(n)) idxs.push(n);
      });
      return Array.from(new Set(idxs)).sort((a, b) => a - b);
    } catch {
      return [];
    }
  }, [gridContainerRef]);
  
  const copySelectedToClipboard = useCallback(async () => {
    const currentTab = tabState.getCurrentTabState();
    if (!currentTab) {
      addLog('info', 'No active tab');
      return;
    }
    
    const effectiveQueryForCopy = tabState.appliedQueryRef.current;
    
    if (currentTab.query !== currentTab.appliedQuery) {
      addLog('info', 'Search text changed but not applied; copying with last applied query. Press Enter/Apply to use the new search.');
    }
    
    // Determine displayed column order and headers
    // Note: AG Grid v31+ merged columnApi into api, so use gridApi.getAllDisplayedColumns()
    let fields: string[] = [];
    let headersOut: string[] = [];
    
    if (currentTab.gridApi?.getAllDisplayedColumns) {
      const cols = currentTab.gridApi.getAllDisplayedColumns();
      fields = cols.map((c: any) => c.getColDef()?.field || c.getColId());
      headersOut = cols.map((c: any) => c.getColDef()?.headerName || (c.getColDef()?.field || c.getColId()));
    } else {
      fields = currentTab.header.slice();
      headersOut = currentTab.header.slice();
    }
    
    // Build selection payload
    const payload: any = {
      ranges: [] as { start: number; end: number }[],
      virtualSelectAll: Boolean(tabState.virtualSelectAllRef.current),
      fields,
      headers: headersOut,
      query: effectiveQueryForCopy || "",
      timeField: currentTab.timeField || "",
      // Include column JPath expressions so copied data uses the displayed/extracted values
      columnJPathExpressions: currentTab.columnJPathExpressions || {},
    };
    
    if (payload.virtualSelectAll) {
      addLog('info', 'Copying all rows (virtual select-all active)...');
    } else {
      const idxs = getSelectedRowIndexes();
      const vr = tabState.selectionRangeRef.current;
      
      if (vr && typeof vr.start === 'number' && typeof vr.end === 'number') {
        const a = Math.max(0, Math.min(vr.start || 0, vr.end || 0));
        const b = Math.max(vr.start || 0, vr.end || 0);
        // Backend expects exclusive ranges (like Go slices), so end = last_index + 1
        payload.ranges.push({ start: a, end: b + 1 });
      } else if (idxs.length > 0) {
        payload.ranges = indexesToRanges(idxs);
      } else {
        // Timing race: wait and re-check
        await new Promise<void>((resolve) => setTimeout(resolve, 50));
        const idxs2 = getSelectedRowIndexes();
        const vr2 = tabState.selectionRangeRef.current;
        
        if (vr2 && typeof vr2.start === 'number' && typeof vr2.end === 'number') {
          const a2 = Math.max(0, Math.min(vr2.start || 0, vr2.end || 0));
          const b2 = Math.max(vr2.start || 0, vr2.end || 0);
          // Backend expects exclusive ranges (like Go slices), so end = last_index + 1
          payload.ranges.push({ start: a2, end: b2 + 1 });
        } else if (idxs2.length > 0) {
          payload.ranges = indexesToRanges(idxs2);
        } else {
          // Final fallback: DOM-based
          const domIdxs = getSelectedRowIndexesFromDom();
          if (domIdxs.length > 0) {
            payload.ranges = indexesToRanges(domIdxs);
          } else {
            addLog('info', 'No rows selected to copy');
            return;
          }
        }
      }
    }
    
    try {
      setCopyPending((n) => n + 1);
      const AppAPI = await import('../../wailsjs/go/app/App');
      const resp = await AppAPI.CopySelectionToClipboard(payload);
      const rowsCopied: number = resp?.rowsCopied ?? 0;
      
      if (payload.virtualSelectAll) {
        addLog('info', `Copied ${rowsCopied.toLocaleString()} row${rowsCopied === 1 ? '' : 's'} (current query) to clipboard`);
      } else if (payload.ranges.length === 1) {
        const r = payload.ranges[0];
        addLog('info', `Copied ${rowsCopied.toLocaleString()} rows from range [${r.start}..${r.end}] to clipboard`);
      } else {
        addLog('info', `Copied ${rowsCopied.toLocaleString()} selected row${rowsCopied === 1 ? '' : 's'} to clipboard`);
      }
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : String(err);
      addLog('error', 'Failed to copy via backend: ' + errorMessage);
      
      // Show error dialog for clipboard-specific errors
      if (showErrorDialog) {
        // Check if it's a size-related error
        if (errorMessage.includes('too large') || errorMessage.includes('clipboard')) {
          showErrorDialog('Clipboard Error', errorMessage);
        } else {
          showErrorDialog('Copy Failed', `Failed to copy data to clipboard: ${errorMessage}`);
        }
      }
    } finally {
      setCopyPending((n) => Math.max(0, n - 1));
    }
  }, [tabState, getSelectedRowIndexes, indexesToRanges, getSelectedRowIndexesFromDom, addLog, showErrorDialog]);
  
  return {
    copyPending,
    copySelectedToClipboard,
  };
};
