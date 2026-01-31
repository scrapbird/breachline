import { FileOptions, createDefaultFileOptions } from './FileOptions';

// Search result from backend
export interface SearchResult {
  rowIndex: number;
  columnIndex: number;
  columnName: string;
  matchStart: number;
  matchEnd: number;
  snippet: string;
}

// Results panel tab type
export type ResultsPanelTab = 'annotations' | 'search';

// Per-tab state interface
export interface TabState {
  tabId: string;
  filePath: string;
  fileHash: string;
  columnDefs: any[];
  header: string[];
  originalHeader: string[];
  query: string;
  appliedQuery: string;
  histBuckets: { start: number; count: number }[];
  histogramVersion: string; // Format: "tab_id:version_number"
  timeField: string;
  totalRows: number | null;
  virtualSelectAll: boolean;
  selectionRange: { start: number; end: number } | null;
  generation: number;
  fileToken: number;
  gridApi: any | null;
  columnApi: any | null;
  datasource: any | null;
  gridPending: number;
  histPending: number;
  rowAnnotations?: Map<number, { color: string }>; // Map of row index to annotation data
  fileOptions: FileOptions; // File options (jpath, noHeaderRow, ingestTimezoneOverride)
  queryError: string | null; // Error message from query execution
  columnJPathExpressions: Record<string, string>; // Column name → JPath expression for display transformation
  highlightTerms: string[]; // Free-text search terms to highlight in grid cells
  rowMetadata?: {
    originalIndices: number[]; // Maps display position -> original file position
    displayIndices: number[];  // Maps display position -> display index
  };
  
  // Annotation panel state
  showAnnotationPanel: boolean;
  annotationPanelHeight: number;
  
  // Search state
  searchTerm: string;
  searchIsRegex: boolean;
  searchResults: SearchResult[];
  searchTotalCount: number;
  searchCurrentPage: number;
  showSearchPanel: boolean;
  isSearching: boolean;
  activeResultsTab: ResultsPanelTab; // Which sub-tab is active in the results panel
  
  // Grid persistence fields
  scrollPosition?: {
    firstDisplayedRow: number;
    lastDisplayedRow: number;
  };
  gridContainerRef?: React.RefObject<HTMLDivElement>;
  isGridInitialized: boolean;
  
  // Cell viewer dialog state (per-tab)
  cellViewer: {
    visible: boolean;
    columnName: string;
    cellValue: string;
  };
}

export const createEmptyTabState = (tabId: string, filePath: string, fileHash: string = '', fileOptions: FileOptions = createDefaultFileOptions()): TabState => ({
  tabId,
  filePath,
  fileHash,
  columnDefs: [],
  header: [],
  originalHeader: [],
  query: '',
  appliedQuery: '',
  histBuckets: [],
  histogramVersion: `${tabId}:0`,
  timeField: '',
  totalRows: null,
  virtualSelectAll: false,
  selectionRange: null,
  generation: 0,
  fileToken: 0,
  gridApi: null,
  columnApi: null,
  datasource: null,
  gridPending: 0,
  histPending: 0,
  rowAnnotations: new Map(),
  fileOptions,
  queryError: null,
  columnJPathExpressions: {},
  highlightTerms: [],
  
  // Annotation panel state
  showAnnotationPanel: false,
  annotationPanelHeight: 250,
  
  // Search state
  searchTerm: '',
  searchIsRegex: false,
  searchResults: [],
  searchTotalCount: 0,
  searchCurrentPage: 0,
  showSearchPanel: false,
  isSearching: false,
  activeResultsTab: 'annotations',
  
  // Grid persistence fields
  scrollPosition: undefined,
  gridContainerRef: undefined,
  isGridInitialized: false,
  
  // Cell viewer dialog state
  cellViewer: {
    visible: false,
    columnName: '',
    cellValue: '',
  },
});
