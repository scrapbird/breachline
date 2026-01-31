import { useCallback } from 'react';
import { UseTabStateReturn } from './useTabState';
import { applyJPathToCell } from '../utils/jpathUtils';
import HighlightCellRenderer from '../components/HighlightCellRenderer';
import { FileOptions } from '../types/FileOptions';
// @ts-ignore - Wails generated bindings
import * as SettingsAPI from "../../wailsjs/go/settings/SettingsService";

export interface TimeHeaderProps {
  displayName: string;
  column: any;
}

export interface UseFileOperationsProps {
  tabState: UseTabStateReturn;
  appliedDisplayTZ: string;
  pinTimestampColumn: boolean;
  TimeHeader: React.FC<any>;
  RegularHeader: React.FC<any>;
  JPathHeader: React.FC<any>;
  onHeaderContextMenu: (e: React.MouseEvent, columnName: string) => void;
  addLog: (level: 'info' | 'warn' | 'error', message: string) => void;
  onHeadersReady?: (tabId: string, headers: string[], timeField: string) => void;
}

export const useFileOperations = ({ 
  tabState, 
  appliedDisplayTZ, 
  pinTimestampColumn,
  TimeHeader,
  RegularHeader,
  JPathHeader,
  onHeaderContextMenu,
  addLog,
  onHeadersReady,
}: UseFileOperationsProps) => {
  
  const detectTimestampField = useCallback((hdr: string[], current?: string): string => {
    if (current && hdr.includes(current)) return current;
    const norm = hdr.map(h => (h || '').trim());
    const lower = norm.map(h => h.toLowerCase());
    
    const exacts = ['@timestamp', 'timestamp', 'time'];
    for (const ex of exacts) {
      const idx = lower.indexOf(ex);
      if (idx >= 0) return norm[idx];
    }
    
    const containsSeq = ['@timestamp', 'timestamp', 'datetime', 'date', 'time', 'ts'];
    for (const key of containsSeq) {
      const idx = lower.findIndex(h => h.includes(key));
      if (idx >= 0) return norm[idx];
    }
    return '';
  }, []);
  
  const buildColumnDefs = useCallback((
    header: string[], 
    timeField: string, 
    overrideDisplayTZ?: string,
    columnJPathExpressions?: Record<string, string>,
    searchTerms?: string[]
  ) => {
    const lcHdr = header.map((x: string) => (x || '').trim().toLowerCase());
    const chosenIdx = timeField ? lcHdr.indexOf((timeField || '').trim().toLowerCase()) : -1;
    const displayTZ = overrideDisplayTZ !== undefined ? overrideDisplayTZ : appliedDisplayTZ;
    const jpathExprs = columnJPathExpressions || {};
    
    // Determine the order of columns - if pinTimestampColumn is enabled and we have a timestamp column,
    // reorder so the timestamp column comes first
    let columnOrder = header.map((_, i) => i);
    if (pinTimestampColumn && chosenIdx >= 0 && chosenIdx !== 0) {
      // Move timestamp column to the front
      columnOrder = [chosenIdx, ...header.map((_, i) => i).filter(i => i !== chosenIdx)];
    }
    
    // Debug logging for unnamed columns
    if (header.some(h => (h || '').trim().match(/^unnamed_[a-z]$/i))) {
      console.log('buildColumnDefs - received header:', header);
    }
    
    // Track empty columns to generate sequential names
    let emptyColCount = 0;
    
    // Map using columnOrder to support pinning timestamp column
    return columnOrder.map((originalIdx: number) => {
      const h = header[originalIdx];
      const i = originalIdx; // Keep original index for timestamp detection
      const trimmedHeader = (h || '').trim();
      const isEmpty = trimmedHeader === '';
      
      // Check if this is a normalized unnamed column from backend (e.g., "unnamed_a", "unnamed_b")
      const unnamedMatch = trimmedHeader.match(/^unnamed_([a-z])$/i);
      const isUnnamedColumn = isEmpty || unnamedMatch !== null;
      
      // Generate display name and field name for empty/unnamed headers
      let displayName = h;
      let fieldName = h;
      let currentEmptyIndex = -1; // Track this column's empty index
      
      if (isEmpty) {
        currentEmptyIndex = emptyColCount; // Capture the current index before incrementing
        emptyColCount++;
        const letter = String.fromCharCode('A'.charCodeAt(0) + currentEmptyIndex);
        displayName = `Unnamed_${letter}`;
        // Use unique field name for ag-Grid data mapping
        fieldName = `__empty_col_${i}__`;
      } else if (unnamedMatch) {
        // Backend has already normalized this - format for display
        const letter = unnamedMatch[1].toUpperCase();
        displayName = `Unnamed_${letter}`;
        // Use the backend's normalized name as the field
        fieldName = trimmedHeader;
      }
      
      // Use a custom valueGetter for columns with dots, empty headers, or normalized unnamed columns
      const needsValueGetter = isEmpty || unnamedMatch !== null || (h && h.includes('.'));
      
      // Capture the original header value for the closure (not trimmed, to match mapRows exactly)
      const headerKey = h;
      const emptyKey = currentEmptyIndex >= 0 ? `__empty_${currentEmptyIndex}__` : null;
      
      const fieldConfig = needsValueGetter ? {
        colId: fieldName,
        valueGetter: (params: any) => {
          if (!params.data) return undefined;
          // For empty headers, the data is stored with __empty_N__ key
          if (emptyKey !== null) {
            return params.data[emptyKey];
          }
          // For normalized unnamed columns or columns with dots, use the header name directly
          // Use the original header value to match what mapRows uses as the key
          return params.data[headerKey];
        },
      } : {
        field: h,
      };
      
      // Add styling for empty/unnamed column headers (greyed out)
      const headerClass = isUnnamedColumn ? 'empty-column-header' : undefined;
      
      // Check if this column has a JPath expression
      const jpathExpr = jpathExprs[h] || jpathExprs[trimmedHeader];
      const hasJPath = !!jpathExpr && jpathExpr !== '$';
      
      // Value formatter for JPath transformation (display-only)
      const jpathValueFormatter = hasJPath ? (params: any) => {
        const rawValue = params.value;
        if (rawValue === undefined || rawValue === null || rawValue === '') {
          return rawValue;
        }
        return applyJPathToCell(String(rawValue), jpathExpr);
      } : undefined;
      
      // Determine if we need to use the highlight cell renderer
      const useHighlightRenderer = searchTerms && searchTerms.length > 0;
      
      // Cell renderer configuration for highlighting
      const cellRendererConfig = useHighlightRenderer ? {
        cellRenderer: HighlightCellRenderer,
        cellRendererParams: { searchTerms },
      } : {};
      
      if (chosenIdx >= 0 && i === chosenIdx) {
        // Timestamp column - also check for JPath
        return {
          headerName: displayName,
          ...fieldConfig,
          headerComponent: hasJPath ? JPathHeader : TimeHeader,
          headerComponentParams: { displayTZ, appliedDisplayTZ: displayTZ, onHeaderContextMenu, hasJPath, jpathExpression: jpathExpr },
          headerTooltip: isUnnamedColumn ? 'Empty column header' : (hasJPath ? `JPath: ${jpathExpr}` : 'Timestamp column'),
          headerClass,
          sortable: false,
          valueFormatter: jpathValueFormatter,
          ...cellRendererConfig,
        };
      }

      return {
        headerName: displayName,
        ...fieldConfig,
        headerComponent: hasJPath ? JPathHeader : RegularHeader,
        headerComponentParams: { onHeaderContextMenu, hasJPath, jpathExpression: jpathExpr },
        headerTooltip: isUnnamedColumn ? 'Empty column header' : (hasJPath ? `JPath: ${jpathExpr}` : undefined),
        headerClass,
        sortable: false,
        valueFormatter: jpathValueFormatter,
        ...cellRendererConfig,
      };
    });
  }, [appliedDisplayTZ, pinTimestampColumn, TimeHeader, RegularHeader, JPathHeader, onHeaderContextMenu]);
  
  // Helper function to detect if a file is JSON (including compressed JSON files)
  // This matches the backend's DetectFileTypeAndCompression logic
  const isJsonFile = (filePath: string): boolean => {
    const lower = filePath.toLowerCase();
    
    // Strip compression extensions (.gz, .bz2, .xz) to check inner file type
    let innerPath = lower;
    const compressionExtensions = ['.gz', '.bz2', '.xz'];
    for (const ext of compressionExtensions) {
      if (lower.endsWith(ext)) {
        innerPath = lower.slice(0, -ext.length);
        break;
      }
    }
    
    // Check if the inner file is JSON
    return innerPath.endsWith('.json');
  };

  const openCsvWithDialog = useCallback(async () => {
    console.log('Opening file with dialog...');
    const AppAPI = await import('../../wailsjs/go/app/App');
    
    // First, just get the file path using the dialog
    const filePath = await AppAPI.OpenFileDialog();
    if (!filePath) {
      console.log('No file selected');
      return null;
    }
    
    console.log('File selected:', filePath);
    
    // Check if this is a JSON file (including compressed) BEFORE creating the tab
    if (isJsonFile(filePath)) {
      console.log('JSON file detected (possibly compressed), showing ingest expression dialog');
      // Return special flag to trigger the ingest expression dialog in App.tsx
      return {
        filePath: filePath,
        needsIngestExpression: true,
      };
    }
    
    // For non-JSON files (CSV/XLSX), open directly with default options
    const response = await AppAPI.OpenFileTabWithOptions(filePath, {});
    console.log('OpenFileTabWithOptions response:', response);
      
    if (!response || !response.id) {
      console.log('No response or no ID');
      return null;
    }
    
    // Check if file is already open in another tab (no jpath for non-JSON files)
    const jpathOpts: FileOptions = {};
    const existingTabId = tabState.findTabByFilePath(response.filePath || '', jpathOpts);
    if (existingTabId) {
      console.log('File already open in tab:', existingTabId, '- switching to it');
      tabState.switchTab(existingTabId);
      addLog('info', `File already open, switched to existing tab`);
      
      // Return the existing tab info
      const existingTab = tabState.getTabState(existingTabId);
      if (existingTab) {
        return {
          tabId: existingTabId,
          header: existingTab.header,
          timeField: existingTab.timeField,
        };
      }
      return null;
    }
    
    // Create new tab
    console.log('Creating tab:', response.id, response.filePath);
    tabState.createTab(response.id, response.filePath || '', response.fileHash || '');
    
    // Headers are already included in response
    const hdr = response.headers || [];
    console.log('Headers:', hdr);
    
    if (hdr && hdr.length > 0) {
      // Use tab-specific setters to ensure we update the correct tab
      // even if the user switches tabs during file loading
      const tabId = response.id;
      
      tabState.setHeaderForTab(tabId, hdr);
      tabState.setOriginalHeaderForTab(tabId, hdr);
      
      // Fetch current display timezone from settings to ensure we use the up-to-date value
      // This is important because the file might be opened before the settings state has
      // been initialized or propagated through React's state updates
      let currentDisplayTZ = appliedDisplayTZ;
      try {
        const currentSettings = await SettingsAPI.GetSettings();
        if (currentSettings && currentSettings.display_timezone) {
          currentDisplayTZ = currentSettings.display_timezone;
        }
      } catch (e) {
        console.warn('Failed to fetch current settings, using appliedDisplayTZ:', e);
      }
      
      const detectedTimeField = detectTimestampField(hdr);
      const defs = buildColumnDefs(hdr, detectedTimeField, currentDisplayTZ);
      
      tabState.setColumnDefsForTab(tabId, defs);
      tabState.setTimeFieldForTab(tabId, detectedTimeField);
      tabState.setHistBucketsForTab(tabId, []);
      // Initialize histogram version so the first async histogram event will be accepted
      tabState.setHistogramVersionForTab(tabId, `${tabId}:1`);
      tabState.incrementFileTokenForTab(tabId);
      tabState.incrementGenerationForTab(tabId);
      
      addLog('info', `Opened new file: ${response.filePath || 'Unknown'}. Columns: ${hdr.length}`);
      
      // Check for decompression warning (incomplete decompression of compressed files)
      if (response.decompressionWarning) {
        addLog('warn', `Decompression warning: ${response.decompressionWarning}`);
        // Show alert dialog to notify user
        alert(`Warning: ${response.decompressionWarning}\n\nSome data may be missing from this file.`);
      }
      
      // Fetch total row count
      try {
        const total = await AppAPI.GetCSVRowCount();
        tabState.setTotalRowsForTab(tabId, total || 0);
      } catch (e) {
        console.warn('Failed to get total row count:', e);
      }
      
      // Notify caller that headers are ready
      if (onHeadersReady) {
        console.log('Calling onHeadersReady callback');
        onHeadersReady(response.id, hdr, detectedTimeField);
      }
      
      // Return info for caller to set up datasource and histogram
      return {
        tabId: response.id,
        header: hdr,
        timeField: detectedTimeField,
      };
    }
      
    return null;
  }, [tabState, detectTimestampField, buildColumnDefs, addLog, appliedDisplayTZ, onHeadersReady]);
  
  const loadJsonFile = useCallback(async (filePath: string, fileOptions: FileOptions) => {
    try {
      const expression = fileOptions.jpath || '';
      console.log('Loading JSON file:', filePath, 'options:', fileOptions);
      
      const AppAPI = await import('../../wailsjs/go/app/App');
      
      // Use file options directly (jpath should already be included)
      const existingTabId = tabState.findTabByFilePath(filePath, fileOptions);
      if (existingTabId) {
        console.log('File with this JSONPath already open in tab:', existingTabId, '- switching to it');
        tabState.switchTab(existingTabId);
        addLog('info', `File with this JSONPath already open, switched to existing tab`);
        
        // Return the existing tab info
        const existingTab = tabState.getTabState(existingTabId);
        if (existingTab) {
          return {
            tabId: existingTabId,
            header: existingTab.header,
            timeField: existingTab.timeField,
          };
        }
        return null;
      }
      
      // Open file with options upfront, backend returns complete tab with headers
      const openOpts = {
        jpath: expression,
        noHeaderRow: fileOptions.noHeaderRow || false,
        ingestTimezoneOverride: fileOptions.ingestTimezoneOverride || '',
      };
      const response = await AppAPI.OpenFileTabWithOptions(filePath, openOpts);
      console.log('OpenFileTabWithOptions response:', response);
        
      if (!response || !response.id) {
        console.log('No response or no ID');
        return null;
      }
      
      // Create new tab with file options - same as CSV flow
      console.log('Creating tab:', response.id, response.filePath, 'with options:', fileOptions);
      tabState.createTab(response.id, response.filePath || '', response.fileHash || '', fileOptions);
      
      // Headers are already included in response - same as CSV!
      const hdr = response.headers || [];
      console.log('Headers:', hdr);
      
      if (hdr && hdr.length > 0) {
        const tabId = response.id;
        
        tabState.setHeaderForTab(tabId, hdr);
        tabState.setOriginalHeaderForTab(tabId, hdr);
        // jpath expression is already set via createTab's fileOptions parameter
        
        // Fetch current display timezone
        let currentDisplayTZ = appliedDisplayTZ;
        try {
          const SettingsAPI = await import('../../wailsjs/go/settings/SettingsService');
          const currentSettings = await SettingsAPI.GetSettings();
          if (currentSettings && currentSettings.display_timezone) {
            currentDisplayTZ = currentSettings.display_timezone;
          }
        } catch (e) {
          console.warn('Failed to fetch current settings, using appliedDisplayTZ:', e);
        }
        
        const detectedTimeField = detectTimestampField(hdr);
        const defs = buildColumnDefs(hdr, detectedTimeField, currentDisplayTZ);
        
        tabState.setColumnDefsForTab(tabId, defs);
        tabState.setTimeFieldForTab(tabId, detectedTimeField);
        tabState.setHistBucketsForTab(tabId, []);
        // Initialize histogram version so the first async histogram event will be accepted
        tabState.setHistogramVersionForTab(tabId, `${tabId}:1`);
        tabState.incrementFileTokenForTab(tabId);
        tabState.incrementGenerationForTab(tabId);
        
        addLog('info', `Opened JSON file: ${filePath}. Columns: ${hdr.length}`);
        
        // Check for decompression warning (incomplete decompression of compressed files)
        if (response.decompressionWarning) {
          addLog('warn', `Decompression warning: ${response.decompressionWarning}`);
          // Show alert dialog to notify user
          alert(`Warning: ${response.decompressionWarning}\n\nSome data may be missing from this file.`);
        }
        
        // Fetch total row count
        try {
          const total = await AppAPI.GetCSVRowCountForTab(tabId);
          tabState.setTotalRowsForTab(tabId, total || 0);
        } catch (e) {
          console.warn('Failed to get total row count:', e);
        }
        
        // Notify caller that headers are ready
        if (onHeadersReady) {
          console.log('Calling onHeadersReady callback for JSON file');
          onHeadersReady(response.id, hdr, detectedTimeField);
        }
        
        return {
          tabId: response.id,
          header: hdr,
          timeField: detectedTimeField,
        };
      }
      
      return null;
    } catch (err: any) {
      addLog('error', "Failed to load JSON file: " + (err?.message || err));
      return null;
    }
  }, [tabState, detectTimestampField, buildColumnDefs, addLog, appliedDisplayTZ, onHeadersReady]);
  
  return {
    detectTimestampField,
    buildColumnDefs,
    openCsvWithDialog,
    loadJsonFile,
  };
};
