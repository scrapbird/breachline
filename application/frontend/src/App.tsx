import React, { useState, useEffect, useRef, useMemo, useCallback } from 'react';
import { themeQuartz, ModuleRegistry } from 'ag-grid-community';
import { colorSchemeDark } from 'ag-grid-community';
import { InfiniteRowModelModule } from 'ag-grid-community';
import './App.css';
// @ts-ignore - Wails generated bindings
import * as AppAPI from "../wailsjs/go/app/App";
// @ts-ignore - Wails generated bindings
import * as SettingsAPI from "../wailsjs/go/settings/SettingsService";
// @ts-ignore - Wails generated bindings
import * as LicenseAPI from "../wailsjs/go/app/LicenseService";
// @ts-ignore - Wails generated bindings
import * as WorkspaceManagerAPI from "../wailsjs/go/app/WorkspaceManager";
import { EventsOn } from "../wailsjs/runtime";
import { useDialogState } from './hooks/useDialogState';
import { useConsoleLogger, LogLevel } from './hooks/useConsoleLogger';
import LogoUniversal from './assets/images/logo-universal.png';
import HistogramManager from './components/HistogramManager';
import SearchBar from './components/SearchBar';
import ConsolePanel from './components/ConsolePanel';
import GridManager from './components/GridManager';
import { TabBar } from './components/TabBar';
import Settings from './components/Settings';
import AnnotationDialog from './components/AnnotationDialog';
import ColumnJPathDialog from './components/ColumnJPathDialog';
import Dashboard from './components/Dashboard';
import AuthPinEntry from './components/AuthPinEntry';
import SyncStatus from './components/SyncStatus';
import FuzzyFinderDialog from './components/FuzzyFinderDialog';
import CellViewerDialog from './components/CellViewerDialog';
import FileOptionsDialog, { FileOpenOptions } from './components/FileOptionsDialog';
// DirectoryIngestDialog removed - directories now use FileOptionsDialog
import { FileOptions, createDefaultFileOptions, fileOptionsKey, fileOptionsEqual } from './types/FileOptions';
import WorkspaceNameDialog from './components/WorkspaceNameDialog';
import PluginSelectionDialog from './components/PluginSelectionDialog';
import PluginWarningDialog from './components/PluginWarningDialog';
import { usePluginHandlers } from './hooks/usePluginHandlers';
import AboutDialog from './components/dialogs/AboutDialog';
import SyntaxHelpDialog from './components/dialogs/SyntaxHelpDialog';
import ShortcutsDialog from './components/dialogs/ShortcutsDialog';
import MessageDialog from './components/dialogs/MessageDialog';
import ErrorDialog from './components/ErrorDialog';
import DirectoryHashWarningDialog from './components/DirectoryHashWarningDialog';
import LoadingOverlay from './components/dialogs/LoadingOverlay';
import TimeHeader from './components/grid/TimeHeader';
import RegularHeader from './components/grid/RegularHeader';
import JPathHeader from './components/grid/JPathHeader';
import HeaderContextMenu from './components/grid/HeaderContextMenu';
import { loadQueryHistory, addToQueryHistory } from './utils/queryHistory';
import { useAnnotationHandlers } from './hooks/useAnnotationHandlers';
import { useSettingsHandlers, AppSettings, defaultSettings } from './hooks/useSettingsHandlers';
import { FileItem } from './components/FuzzyFinderDialog';
import { useWorkspaceHandlers } from './hooks/useWorkspaceHandlers';
import { useMenuEvents } from './hooks/useMenuEvents';
import { useSyncHandlers } from './hooks/useSyncHandlers';

// Import custom hooks
import { useTabState } from './hooks/useTabState';
import { useGridOperations } from './hooks/useGridOperations';
import { useHistogram } from './hooks/useHistogram';
import { useFileOperations } from './hooks/useFileOperations';
import { useSearchOperations, expectedHistogramVersions } from './hooks/useSearchOperations';
import { useClipboardOperations } from './hooks/useClipboardOperations';

const gridTheme = themeQuartz.withPart(colorSchemeDark);
ModuleRegistry.registerModules([InfiniteRowModelModule]);

type LogEntry = { ts: number; level: 'info' | 'warn' | 'error'; message: string };

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

function App() {
    // Multi-tab state management
    const tabState = useTabState();

    // Consolidated dialog state management
    const [dialogState, dialogActions] = useDialogState();

    // Consolidated console logger
    const [consoleState, consoleActions] = useConsoleLogger();
    const { logs, consoleHeight } = consoleState;
    const { addLog, clearLogs, setConsoleHeight } = consoleActions;

    // Destructure dialog state for compatibility with existing code
    const {
        showAbout, showSyntax, showShortcuts, showSyncStatus, showFuzzyFinder,
        showWorkspaceNameDialog, showConsole, showHistogram,
        showMessageDialog, messageDialogTitle, messageDialogMessage, messageDialogIsError,
        showAnnotationDialog, annotationRowIndices, annotationNote, annotationColor, isAnnotationLoading,
        showIngestExpression, ingestTabId, ingestFilePath,
        showAddToWorkspaceDialog, addToWorkspaceFilePath, addToWorkspaceIsJson,
        showAuthPinDialog, authPinMessage, isAuthPinLoading,
        showColumnJPathDialog, columnJPathTarget,
        headerContextMenu,
        isCreatingWorkspace, showExportLoading, isOpeningFile, isOpeningWorkspace, isChangingTimestamp,
    } = dialogState;

    // Destructure dialog actions for compatibility
    const {
        setShowSettings: setShowSettingsDialog,
        setShowAbout, setShowSyntax, setShowShortcuts, setShowSyncStatus, setShowFuzzyFinder,
        setShowWorkspaceNameDialog,
        setShowConsole: setShowConsoleAction,
        setShowHistogram: setShowHistogramAction,
        showMessageDialog: showMessageDialogAction, hideMessageDialog,
        showAnnotationDialogWithData, hideAnnotationDialog,
        setAnnotationLoading: setIsAnnotationLoading,
        setAnnotationNote, setAnnotationColor, setAnnotationRowIndices,
        showIngestExpressionDialog, hideIngestExpressionDialog,
        showAddToWorkspaceDialog: showAddToWorkspaceDialogAction,
        hideAddToWorkspaceDialog,
        showAuthPinDialogWithMessage, hideAuthPinDialog,
        setAuthPinLoading: setIsAuthPinLoading, setAuthPinMessage,
        showColumnJPathDialogWithData, hideColumnJPathDialog,
        showHeaderContextMenuAt, hideHeaderContextMenu,
        setCreatingWorkspace: setIsCreatingWorkspace,
        setExportLoading: setShowExportLoading,
        setOpeningFile: setIsOpeningFile,
        setOpeningWorkspace: setIsOpeningWorkspace,
        setChangingTimestamp: setIsChangingTimestamp,
    } = dialogActions;

    // Global app state (not tab-specific)
    const [error, setError] = useState<string>("");
    const [pageSize] = useState<number>(1000);
    const [appliedDisplayTZ, setAppliedDisplayTZ] = useState<string>('Local');

    // Settings modal state
    const [showSettings, setShowSettings] = useState<boolean>(false);
    const [settings, setSettings] = useState<AppSettings>(defaultSettings);

    // About modal data (separate from show state)
    const [licenseEmail, setLicenseEmail] = useState<string | null>(null);
    const [licenseEndDate, setLicenseEndDate] = useState<Date | null>(null);

    // License status state
    const [isLicensed, setIsLicensed] = useState<boolean>(false);

    // Workspace status state
    const [isWorkspaceOpen, setIsWorkspaceOpen] = useState<boolean>(false);
    const [workspaceKey, setWorkspaceKey] = useState<number>(0);

    // Cache indicator visibility state (persisted to settings)
    const [showCacheIndicator, setShowCacheIndicator] = useState<boolean>(true);

    // Fuzzy finder files data (separate from show state)
    const [fuzzyFinderFiles, setFuzzyFinderFiles] = useState<FileItem[]>([]);

    // File options dialog state
    const [showFileOptionsDialog, setShowFileOptionsDialog] = useState<boolean>(false);
    const [fileOptionsFilePath, setFileOptionsFilePath] = useState<string>('');
    const [fileOptionsMode, setFileOptionsMode] = useState<'open' | 'addToWorkspace'>('open');
    const [fileOptionsFileType, setFileOptionsFileType] = useState<'json' | 'csv' | 'xlsx' | 'directory'>('csv');
    const [fileOptionsShowTimezone, setFileOptionsShowTimezone] = useState<boolean>(true);
    const [fileOptionsExistingTabId, setFileOptionsExistingTabId] = useState<string>(''); // Tab ID for already-created tabs needing jpath

    // Directory loading now uses FileOptionsDialog with fileType='directory'

    // File or Directory choice dialog state
    const [showFileOrDirChoice, setShowFileOrDirChoice] = useState<boolean>(false);
    const [fileOrDirChoiceMode, setFileOrDirChoiceMode] = useState<'open' | 'addToWorkspace'>('open');

    // Error dialog state
    const [showErrorDialog, setShowErrorDialog] = useState<boolean>(false);
    const [errorDialogMessage, setErrorDialogMessage] = useState<string>('');

    // Directory hash warning dialog state
    const [showDirHashWarning, setShowDirHashWarning] = useState<boolean>(false);
    const [dirHashWarningPath, setDirHashWarningPath] = useState<string>('');
    const [dirHashWarningOptions, setDirHashWarningOptions] = useState<FileOptions | null>(null);

    // Query history
    const [queryHistory, setQueryHistory] = useState<string[]>([]);

    // Refs
    const searchInputRef = useRef<HTMLInputElement | null>(null);
    const gridContainerRef = useRef<HTMLDivElement | null>(null);

    // Load query history from localStorage
    useEffect(() => {
        setQueryHistory(loadQueryHistory());
    }, []);

    const addToHistory = (q: string) => {
        const next = addToQueryHistory(queryHistory, q);
        setQueryHistory(next);
    };


    // Listen for histogram ready events from async generation
    useEffect(() => {
        console.log('[HISTOGRAM_EVENT_SETUP] Setting up histogram:ready event listener');
        const unsub = EventsOn("histogram:ready", (event: any) => {
            console.log('[HISTOGRAM_EVENT] Received histogram:ready', event);

            // Get current tab state
            const tab = tabState.getTabState(event.tabId);
            if (!tab) {
                console.warn('[HISTOGRAM_EVENT] Tab not found:', event.tabId);
                return;
            }

            // Check if this histogram matches the current query version
            // Use both React state and the synchronous global Map for version checking
            // The Map is updated synchronously before React state, handling race conditions
            const stateVersion = tab.histogramVersion || `${event.tabId}:0`;
            const expectedVersion = expectedHistogramVersions.get(event.tabId) || stateVersion;
            const eventVersion = event.version;

            // Version matches if it matches EITHER the state version OR the expected version
            // This handles race conditions where the event arrives before React state is updated
            const versionMatches = eventVersion === stateVersion || eventVersion === expectedVersion;

            console.log('[HISTOGRAM_EVENT] Version check - state:', stateVersion, 'expected:', expectedVersion, 'event:', eventVersion, 'matches:', versionMatches);
            console.log('[HISTOGRAM_EVENT] Tab state at event time:', {
                tabId: event.tabId,
                histogramVersion: tab.histogramVersion,
                histPending: tab.histPending,
                bucketsCount: tab.histBuckets?.length || 0
            });

            if (event.error) {
                // Handle error - show error in histogram area
                console.error('[HISTOGRAM_EVENT] Histogram generation failed:', event.error);
                addLog('error', `Histogram generation failed: ${event.error}`);
                // Only clear loading if this matches current version
                if (versionMatches) {
                    tabState.setHistPendingForTab(event.tabId, 0);
                }
            } else if (versionMatches) {
                // This matches our current version - update histogram
                console.log('[HISTOGRAM_EVENT] Updating histogram for matching version', eventVersion);
                const buckets = event.buckets?.map((b: any) => ({
                    start: b.start,
                    count: b.count
                })) || [];
                console.log('[HISTOGRAM_EVENT] Setting buckets:', buckets.length, 'buckets');
                console.log('[HISTOGRAM_EVENT] Buckets data:', buckets.slice(0, 3)); // Show first 3 buckets
                tabState.setHistBucketsForTab(event.tabId, buckets);

                // Verify buckets were set
                const updatedTab = tabState.getTabState(event.tabId);
                console.log('[HISTOGRAM_EVENT] Buckets after setting:', updatedTab?.histBuckets?.length || 0);
                console.log('[HISTOGRAM_EVENT] Clearing loading state (setHistPending to 0)');
                tabState.setHistPendingForTab(event.tabId, 0);
                console.log('[HISTOGRAM_EVENT] Histogram update complete');
                addLog('info', `Histogram updated for tab ${event.tabId} (${buckets.length} buckets)`);
            } else {
                // This is from an old query - ignore it
                console.log('[HISTOGRAM_EVENT] Ignoring stale histogram - version mismatch');
                console.log('[HISTOGRAM_EVENT] Expected (state):', stateVersion, 'Expected (map):', expectedVersion, 'Got:', eventVersion);
            }
        });
        return () => { if (unsub) unsub(); };
    }, [tabState]);

    // Load settings on mount
    useEffect(() => {
        (async () => {
            try {
                const s = await SettingsAPI.GetSettings();
                if (s) {
                    const loaded = {
                        sort_by_time: s.sort_by_time ?? true,
                        sort_ascending: !s.sort_descending,
                        enable_query_cache: s.enable_query_cache ?? true,
                        cache_size_limit_mb: s.cache_size_limit_mb ?? 100,
                        default_ingest_timezone: s.default_ingest_timezone ?? 'Local',
                        display_timezone: s.display_timezone ?? 'Local',
                        timestamp_display_format: s.timestamp_display_format ?? 'yyyy-MM-dd HH:mm:ss',
                        pin_timestamp_column: s.pin_timestamp_column ?? false,
                        max_directory_files: s.max_directory_files ?? 500,
                        enable_plugins: s.enable_plugins ?? false,
                    };
                    setSettings(loaded);
                    setAppliedDisplayTZ(loaded.display_timezone);
                }
            } catch (e) {
                console.warn('Failed to load settings:', e);
            }
        })();
    }, []);

    // Check license status on mount and when license changes
    useEffect(() => {
        (async () => {
            try {
                await LicenseAPI.GetLicenseDetails();
                setIsLicensed(true);
            } catch (e) {
                setIsLicensed(false);
            }
        })();
    }, [licenseEmail]);

    // Custom hooks for operations
    const histogram = useHistogram(tabState);

    // Handler for header context menu
    const handleHeaderContextMenu = (e: React.MouseEvent, columnName: string) => {
        e.preventDefault();
        e.stopPropagation();
        showHeaderContextMenuAt(e.clientX, e.clientY, columnName);
    };

    // Handler for annotating rows (single or multiple)
    const fileOps = useFileOperations({
        tabState,
        appliedDisplayTZ,
        pinTimestampColumn: settings.pin_timestamp_column,
        TimeHeader,
        RegularHeader,
        JPathHeader,
        onHeaderContextMenu: handleHeaderContextMenu,
        addLog,
    });

    const gridOps = useGridOperations({
        tabState,
        pageSize,
        buildColumnDefs: fileOps.buildColumnDefs,
        addLog,
        useUnifiedFetch: true,  // Enable unified query optimization
        showErrorDialog: (message: string) => {
            setErrorDialogMessage(message);
            setShowErrorDialog(true);
        },
    });

    const searchOps = useSearchOperations({
        tabState,
        buildColumnDefs: fileOps.buildColumnDefs,
        createDataSource: gridOps.createDataSource,
        addLog,
    });

    const clipboardOps = useClipboardOperations({
        tabState,
        getSelectedRowIndexes: gridOps.getSelectedRowIndexes,
        indexesToRanges: gridOps.indexesToRanges,
        gridContainerRef,
        addLog,
        showErrorDialog: (title: string, message: string) => {
            showMessageDialogAction(title, message, true);
        },
    });

    const { handleAnnotateRow, handleSubmitAnnotation, handleDeleteAnnotation } = useAnnotationHandlers({
        tabState,
        gridOps,
        isLicensed,
        isWorkspaceOpen,
        annotationRowIndices,
        dialogActions,
        addLog,
    });

    // Plugin handlers hook
    const pluginHandlers = usePluginHandlers({
        tabState,
        fileOps,
        appliedDisplayTZ,
        addLog,
        setIsOpeningFile,
        showMessageDialogAction,
        gridOps,
        histogram,
    });

    // Handler for jumping to original row position when query is active
    // Note: displayRowIndex is actually the original file index passed directly from row data
    const handleJumpToOriginal = useCallback(async (tabId: string, originalFileIndex: number) => {
        const tab = tabState.getTabState(tabId);
        if (!tab) {
            addLog('warn', 'Tab not found');
            return;
        }

        // originalFileIndex is now passed directly from the row data's __originalIndex field
        if (originalFileIndex === undefined || originalFileIndex === null) {
            addLog('warn', 'Could not determine original row position');
            return;
        }

        addLog('info', `Jumping to original file position: row ${originalFileIndex + 1}`);

        // Call backend to find the display index for this original file index
        // This executes an empty query and searches through all rows efficiently
        const AppAPI = await import('../wailsjs/go/app/App');
        let targetDisplayIndex: number;

        try {
            // @ts-ignore - FindDisplayIndexForOriginalRow will be available after Wails bindings regeneration
            targetDisplayIndex = await AppAPI.FindDisplayIndexForOriginalRow(
                tabId,
                originalFileIndex,
                tab.timeField || ''
            );
            addLog('info', `Backend found display position ${targetDisplayIndex + 1} for original row ${originalFileIndex + 1}`);
        } catch (e) {
            addLog('error', `Failed to find display position: ${e instanceof Error ? e.message : String(e)}`);
            return;
        }

        // Clear the query in state
        tabState.setQuery('');

        // Import the data load tracker for promise-based waiting
        const { createDataLoadPromise } = await import('./utils/dataLoadTracker');

        // Create promise BEFORE triggering the search so we capture the load completion
        const dataLoadPromise = createDataLoadPromise(tabId);

        // Apply empty search to actually clear the grid and histogram
        await searchOps.applySearch('');

        // Refresh histogram with no filter
        await histogram.refreshHistogram(
            tabState.currentTab?.histogramVersion || `${tabId}:0`,
            tabState.currentTab?.appliedQuery || '',
            tabState.currentTab?.timeField || '',
            false // clearExisting = false (will update with new data)
        );

        // Wait for the grid data to actually load using promise-based approach
        // This is more efficient than polling as it resolves exactly when data loading completes
        addLog('info', 'Waiting for grid to load unfiltered data...');
        try {
            await dataLoadPromise;
            addLog('info', 'Grid data loaded successfully');
        } catch (e) {
            addLog('warn', `Data load failed or timed out: ${e instanceof Error ? e.message : String(e)}`);
            return;
        }

        // Give the grid a moment to render the loaded data
        await new Promise(resolve => setTimeout(resolve, 100));

        // Get the refreshed tab state
        const refreshedTab = tabState.getTabState(tabId);

        // Scroll to and select the row
        if (refreshedTab?.gridApi) {
            try {
                addLog('info', `Scrolling to display position ${targetDisplayIndex + 1}...`);

                // Ensure the row is visible - this forces AG Grid to load the row if not already loaded
                refreshedTab.gridApi.ensureIndexVisible(targetDisplayIndex, 'middle');

                // Wait for the scroll and data load to complete
                await new Promise(resolve => setTimeout(resolve, 300));

                // DEBUG: Check what row is actually at this position
                const rowNode = refreshedTab.gridApi.getDisplayedRowAtIndex(targetDisplayIndex);
                if (rowNode?.data) {
                    const actualOriginalIndex = rowNode.data.__originalIndex;
                    const actualDisplayIndex = rowNode.data.__displayIndex;
                    addLog('info', `[DEBUG] Row at position ${targetDisplayIndex}: originalIndex=${actualOriginalIndex}, displayIndex=${actualDisplayIndex}, expected originalIndex=${originalFileIndex}`);

                    if (actualOriginalIndex !== originalFileIndex) {
                        addLog('warn', `[DEBUG] Index mismatch! Expected original index ${originalFileIndex} but found ${actualOriginalIndex} at display position ${targetDisplayIndex}`);
                    }
                } else {
                    addLog('warn', `[DEBUG] No row data found at position ${targetDisplayIndex}`);
                }

                // Select the row
                gridOps.selectRange(targetDisplayIndex, targetDisplayIndex);

                addLog('info', `Successfully jumped to row at display position ${targetDisplayIndex + 1}`);
            } catch (e) {
                addLog('error', `Failed to jump to row: ${e instanceof Error ? e.message : String(e)}`);
            }
        }
    }, [tabState, gridOps, searchOps, histogram, addLog]);

    // Handler for executing search (called from GridManager/SearchPanel)
    const handleSearchForTab = useCallback(async (tabId: string, searchTerm: string, isRegex: boolean) => {
        if (!tabId || tabId === '__dashboard__') {
            return;
        }

        // Get the current applied query to search within filtered results
        const tab = tabState.getTabState(tabId);
        const appliedQuery = tab?.appliedQuery || '';

        // Mark search as in progress
        tabState.setIsSearchingForTab(tabId, true);
        tabState.setSearchTermForTab(tabId, searchTerm);
        tabState.setSearchIsRegexForTab(tabId, isRegex);

        try {
            // Call backend search API with the applied query to search within filtered results
            const result = await AppAPI.SearchInFile(tabId, searchTerm, isRegex, 0, appliedQuery);

            if (result && !result.cancelled) {
                // Update search results (resetPage=true for new searches)
                tabState.setSearchResultsForTab(tabId, result.results || [], result.totalCount, true);

                // Set highlight terms for grid highlighting
                const highlightTerms = [searchTerm];
                tabState.setHighlightTermsForTab(tabId, highlightTerms);

                // Rebuild column definitions with new highlight terms so HighlightCellRenderer gets them
                const tab = tabState.getTabState(tabId);
                if (tab) {
                    const columnJPathExprs = tab.columnJPathExpressions || {};
                    const newDefs = fileOps.buildColumnDefs(
                        tab.header,
                        tab.timeField,
                        appliedDisplayTZ,
                        columnJPathExprs,
                        highlightTerms
                    );
                    tabState.setColumnDefsForTab(tabId, newDefs);

                    // Refresh grid cells to show highlights
                    if (tab.gridApi) {
                        tab.gridApi.refreshCells({ force: true });
                    }
                }

                addLog('info', `Search completed: ${result.totalCount} matches for "${searchTerm}"`);
            } else if (result?.cancelled) {
                addLog('info', 'Search was cancelled');
            }
        } catch (e) {
            console.error('Search failed:', e);
            addLog('error', `Search failed: ${e instanceof Error ? e.message : String(e)}`);
        } finally {
            tabState.setIsSearchingForTab(tabId, false);
        }
    }, [tabState, fileOps, appliedDisplayTZ, addLog]);

    // Handler for cancelling search (called from GridManager/SearchPanel)
    const handleCancelSearchForTab = useCallback((tabId: string) => {
        if (!tabId) return;

        AppAPI.CancelSearch(tabId).catch(console.error);
        tabState.setIsSearchingForTab(tabId, false);
    }, [tabState]);

    // Handler for clearing search (from GridManager)
    const handleClearSearchForTab = useCallback((tabId: string) => {
        // Clear search state
        tabState.clearSearchForTab(tabId);
        tabState.setHighlightTermsForTab(tabId, []);

        // Call backend to clear cached search results
        AppAPI.ClearSearchResults(tabId).catch(console.error);

        // Rebuild column definitions without highlight terms to remove highlighting
        const tab = tabState.getTabState(tabId);
        if (tab) {
            const columnJPathExprs = tab.columnJPathExpressions || {};
            const newDefs = fileOps.buildColumnDefs(
                tab.header,
                tab.timeField,
                appliedDisplayTZ,
                columnJPathExprs,
                [] // No highlight terms
            );
            tabState.setColumnDefsForTab(tabId, newDefs);

            // Refresh grid cells to remove highlights
            if (tab.gridApi) {
                tab.gridApi.refreshCells({ force: true });
            }
        }
    }, [tabState, fileOps, appliedDisplayTZ]);

    // Settings handlers hook
    const { openSettings, cancelSettings, saveSettings } = useSettingsHandlers({
        settings,
        setSettings,
        setShowSettings,
        appliedDisplayTZ,
        setAppliedDisplayTZ,
        tabState,
        searchOps,
        histogram,
        addLog,
    });

    // Sync handlers hook
    const {
        handlePinSubmit,
        handleSyncLogin,
        handleSyncLogout,
        handleSelectRemoteWorkspace,
    } = useSyncHandlers({
        tabState,
        setIsWorkspaceOpen,
        setWorkspaceKey,
        dialogActions,
        addLog,
    });

    // Workspace handlers hook
    const {
        handleOpenWorkspace,
        handleCloseWorkspace,
        handleCreateLocalWorkspace,
        handleOpenCreateRemoteWorkspaceDialog,
        handleCreateRemoteWorkspace,
    } = useWorkspaceHandlers({
        tabState,
        setIsWorkspaceOpen,
        setWorkspaceKey,
        dialogActions,
        addLog,
    });

    // Handler for when a workspace is deleted - close it if it's the currently open one
    const handleWorkspaceDeleted = useCallback(async (deletedWorkspaceId: string) => {
        try {
            const WorkspaceAPI = await import('../wailsjs/go/app/WorkspaceManager');
            const isRemote = await WorkspaceAPI.IsRemoteWorkspace();
            if (!isRemote) {
                return; // Not a remote workspace, nothing to do
            }

            const currentWorkspaceId = await WorkspaceAPI.GetWorkspaceIdentifier();
            if (currentWorkspaceId === deletedWorkspaceId) {
                addLog('info', 'Currently open workspace was deleted, closing it');
                await handleCloseWorkspace();
            }
        } catch (e: any) {
            addLog('error', 'Error checking workspace after deletion: ' + (e?.message || String(e)));
        }
    }, [handleCloseWorkspace, addLog]);

    // Fuzzy finder file fetcher and handler
    const getFuzzyFinderFiles = useCallback(async (): Promise<FileItem[]> => {
        const files: FileItem[] = [];

        // Add all open tabs (skip dashboard tab)
        for (const tab of tabState.tabs) {
            if (tab.filePath && tab.id !== '__dashboard__') {
                files.push({
                    id: tab.id,
                    path: tab.filePath,
                    isOpen: true,
                    fileOptions: tab.fileOptions,
                });
            }
        }

        // Add workspace files if workspace is open
        if (isWorkspaceOpen) {
            try {
                const workspaceFiles = await WorkspaceManagerAPI.GetWorkspaceFiles();

                for (const wf of workspaceFiles) {
                    // Skip if already in open tabs (compare both path AND options for virtual file variants)
                    const wfOptions = wf.options || createDefaultFileOptions();
                    const alreadyOpen = files.some(f =>
                        f.path === wf.filePath &&
                        fileOptionsEqual(f.fileOptions || createDefaultFileOptions(), wfOptions)
                    );
                    if (!alreadyOpen && wf.filePath) {
                        // Include all options in ID for uniqueness across virtual file variants
                        files.push({
                            id: `workspace-${wf.fileHash}-${fileOptionsKey(wfOptions)}`,
                            path: wf.filePath,
                            isOpen: false,
                            fileOptions: wfOptions,
                        });
                    }
                }
            } catch (e) {
                console.error('Failed to fetch workspace files:', e);
            }
        }

        return files;
    }, [tabState.tabs, isWorkspaceOpen]);

    const handleFuzzyFinderSelect = useCallback(async (file: FileItem) => {
        // If already open tab, switch to it
        if (file.isOpen) {
            tabState.switchTab(file.id);
            return;
        }

        // Otherwise, need to open file via backend
        const actualPath = file.path;
        const jpath = file.fileOptions?.jpath;

        // Show loading spinner while opening file
        setIsOpeningFile(true);

        try {
            // Check if file is already open
            const existingFileOptions: FileOptions = file.fileOptions || { jpath: '' };
            const existingTabId = tabState.findTabByFilePath(actualPath, existingFileOptions);
            if (existingTabId) {
                tabState.switchTab(existingTabId);
                setIsOpeningFile(false);
                return;
            }

            // Check if JSON file needs expression BEFORE calling backend
            const isJsonFile = actualPath.toLowerCase().endsWith('.json');
            if (isJsonFile && (!jpath || jpath === '')) {
                // Show file options dialog for JSON expression first
                setFileOptionsFilePath(actualPath);
                setFileOptionsMode('open');
                setFileOptionsFileType('json');
                setFileOptionsShowTimezone(false); // Simple JSON open doesn't need timezone option
                setFileOptionsExistingTabId(''); // No tab created yet
                setShowFileOptionsDialog(true);
                setIsOpeningFile(false);
                return;
            }

            // Open file via backend with full options (jpath, noHeaderRow, ingestTimezoneOverride, directory options)
            const AppAPI = await import("../wailsjs/go/app/App");
            const response = await AppAPI.OpenFileTabWithOptions(actualPath, {
                jpath: jpath || '',
                noHeaderRow: file.fileOptions?.noHeaderRow || false,
                ingestTimezoneOverride: file.fileOptions?.ingestTimezoneOverride || '',
                isDirectory: file.fileOptions?.isDirectory || false,
                filePattern: file.fileOptions?.filePattern || '',
                includeSourceColumn: file.fileOptions?.includeSourceColumn || false,
            });

            if (!response || !response.id) {
                addLog('error', 'Failed to open file from workspace');
                return;
            }

            // Create tab with headers and file options
            tabState.createTab(response.id, actualPath, response.fileHash || '', file.fileOptions);

            const hdr = response.headers || [];
            if (hdr && hdr.length > 0) {
                const tabId = response.id;

                tabState.setHeaderForTab(tabId, hdr);
                tabState.setOriginalHeaderForTab(tabId, hdr);
                // Note: fileOptions already passed to createTab above

                const detectedTimeField = fileOps.detectTimestampField(hdr);
                const defs = fileOps.buildColumnDefs(hdr, detectedTimeField);

                tabState.setColumnDefsForTab(tabId, defs);
                tabState.setTimeFieldForTab(tabId, detectedTimeField);
                tabState.incrementFileTokenForTab(tabId);
                tabState.incrementGenerationForTab(tabId);

                addLog('info', `Opened file: ${actualPath}`);
            }
        } catch (e) {
            console.error('Failed to open file from fuzzy finder:', e);
            const errorMsg = e instanceof Error ? e.message : String(e);
            addLog('error', 'Failed to open file: ' + errorMsg);
            setErrorDialogMessage(errorMsg);
            setShowErrorDialog(true);
        } finally {
            setIsOpeningFile(false);
        }
    }, [tabState, fileOps, addLog]);

    // Event handlers
    const handleOpenFile = async () => {
        setError("");
        setIsOpeningFile(true);
        try {
            // First, get the file path via dialog
            const filePath = await AppAPI.OpenFileDialog();
            if (!filePath) {
                setIsOpeningFile(false);
                return;
            }

            // Check if file requires a plugin that's not available
            const pluginReq = await AppAPI.CheckPluginRequirement(filePath, "", "");
            if (pluginReq.requiresPlugin && !pluginReq.pluginAvailable) {
                setIsOpeningFile(false);

                // Build error message based on the situation
                if (!pluginReq.pluginsEnabled) {
                    // Plugin support is disabled globally
                    showMessageDialogAction(
                        'File Type Not Supported',
                        `Files with extension "${pluginReq.fileExtension}" require plugin support to open.\n\nPlugin support is currently disabled. Please enable plugin support in Settings → Plugins to open this file type.`,
                        true
                    );
                } else if (pluginReq.requiredPluginId) {
                    // Plugin exists but is disabled
                    showMessageDialogAction(
                        'Plugin Required',
                        `This file type requires a plugin that is currently disabled.\n\nPlease enable the plugin "${pluginReq.requiredPluginName}" (ID: ${pluginReq.requiredPluginId}) in Settings → Plugins to open this file.`,
                        true
                    );
                } else {
                    // No plugin configured for this extension
                    showMessageDialogAction(
                        'File Type Not Supported',
                        `Files with extension "${pluginReq.fileExtension}" are not supported by the built-in loaders and no plugin is available to handle this file type.\n\nPlease add a plugin that supports "${pluginReq.fileExtension}" files in Settings → Plugins.`,
                        true
                    );
                }
                return;
            }

            // Check if plugins support this file type (for showing warning/selection dialogs)
            const plugins = await AppAPI.GetPluginsForFile(filePath);
            if (plugins && plugins.length > 1) {
                // Show plugin selection dialog
                pluginHandlers.showPluginSelectionFor(filePath, plugins, 'open');
                setIsOpeningFile(false);
                return;
            }

            // If only one plugin, show warning dialog before proceeding
            if (plugins && plugins.length === 1) {
                pluginHandlers.showPluginWarningFor(filePath, plugins[0], 'open');
                setIsOpeningFile(false);
                return;
            }

            // No plugins - proceed with opening directly
            await openFileWithOptions(filePath, {});
        } catch (e: any) {
            const errorMsg = e?.message || String(e);
            if (errorMsg.includes('requires a valid license')) {
                showMessageDialogAction('Premium Feature', 'JSON file ingestion requires a valid license. Please import a license file to use this feature.', true);
            } else {
                addLog('error', 'Failed to open file: ' + errorMsg);
            }
        } finally {
            setIsOpeningFile(false);
        }
    };

    // Open file with specific options (used after plugin selection or directly)
    const openFileWithOptions = async (filePath: string, opts: { pluginId?: string; pluginName?: string } = {}) => {
        try {
            // Check if this is a JSON file that needs the file options dialog
            if (isJsonFile(filePath)) {
                setFileOptionsFilePath(filePath);
                setFileOptionsMode('open');
                setFileOptionsFileType('json');
                setFileOptionsShowTimezone(false);
                setFileOptionsExistingTabId('');
                setShowFileOptionsDialog(true);
                return;
            }

            // For non-JSON files (CSV/XLSX/Plugin), open with provided options
            const fileOptions: FileOptions = {
                pluginId: opts.pluginId || '',
                pluginName: opts.pluginName || '',
            };

            const response = await AppAPI.OpenFileTabWithOptions(filePath, fileOptions);

            if (!response || !response.id) {
                return null;
            }

            // Check if file is already open in another tab
            const existingTabId = tabState.findTabByFilePath(response.filePath || '', fileOptions);
            if (existingTabId) {
                tabState.switchTab(existingTabId);
                addLog('info', `File already open, switched to existing tab`);
                return;
            }

            // Create new tab
            tabState.createTab(response.id, response.filePath || '', response.fileHash || '', fileOptions);

            // Headers are already included in response
            const hdr = response.headers || [];

            if (hdr && hdr.length > 0) {
                const tabId = response.id;

                tabState.setHeaderForTab(tabId, hdr);
                tabState.setOriginalHeaderForTab(tabId, hdr);

                const detectedTimeField = fileOps.detectTimestampField(hdr);
                const defs = fileOps.buildColumnDefs(hdr, detectedTimeField);

                tabState.setColumnDefsForTab(tabId, defs);
                tabState.setTimeFieldForTab(tabId, detectedTimeField);
                tabState.setHistBucketsForTab(tabId, []);
                tabState.setHistogramVersionForTab(tabId, `${tabId}:1`);
                tabState.incrementFileTokenForTab(tabId);
                tabState.incrementGenerationForTab(tabId);

                addLog('info', `Opened file: ${response.filePath || 'Unknown'}. Columns: ${hdr.length}`);

                // Check for decompression warning
                if (response.decompressionWarning) {
                    addLog('warn', `Decompression warning: ${response.decompressionWarning}`);
                    alert(`Warning: ${response.decompressionWarning}\n\nSome data may be missing from this file.`);
                }

                // Fetch total row count
                try {
                    const total = await AppAPI.GetCSVRowCountForTab(tabId);
                    tabState.setTotalRowsForTab(tabId, total || 0);
                } catch (e) {
                    console.warn('Failed to get total row count:', e);
                }

                // Give React time to update state
                await new Promise(resolve => setTimeout(resolve, 100));

                // Access tab by ID
                const tab = tabState.getTabState(tabId);
                if (tab && tab.gridApi && !tab.datasource) {
                    const ds = gridOps.createDataSource(tabId, '', hdr, detectedTimeField);
                    tabState.setDatasourceForTab(tabId, ds);
                }

                // Refresh histogram
                if (detectedTimeField) {
                    try {
                        await histogram.refreshHistogram(tabId, detectedTimeField, '', true);
                    } catch (e) {
                        console.warn('Failed to refresh histogram:', e);
                    }
                }
            }
        } catch (e: any) {
            const errorMsg = e?.message || String(e);
            if (errorMsg.includes('requires a valid license')) {
                showMessageDialogAction('Premium Feature', 'This feature requires a valid license.', true);
            } else {
                addLog('error', 'Failed to open file: ' + errorMsg);
                setErrorDialogMessage(errorMsg);
                setShowErrorDialog(true);
            }
        }
    };

    const handleTabChange = async (tabId: string) => {
        tabState.switchTab(tabId);

        // Set active tab in backend (skip for dashboard)
        if (tabId !== '__dashboard__') {
            try {
                await AppAPI.SetActiveTab(tabId);
            } catch (e) {
                console.warn('Failed to set active tab:', e);
            }
        }

        // Only do expensive operations if really needed
        const tab = tabState.getTabState(tabId);
        if (tab && tabId !== '__dashboard__') {
            // Only refresh histogram if the tab has a timeField but no histogram data yet
            if (tab.timeField && (!tab.histBuckets || tab.histBuckets.length === 0)) {
                histogram.refreshHistogram(tabId, tab.timeField, tab.appliedQuery || '', true).catch(e => {
                    console.warn('Failed to refresh histogram on tab switch:', e);
                });
            }
        }
    };

    const handleTabClose = async (tabId: string) => {
        try {
            await AppAPI.CloseTab(tabId);
            tabState.closeTab(tabId);
        } catch (e) {
            addLog('error', 'Failed to close tab: ' + (e instanceof Error ? e.message : String(e)));
        }
    };

    const handleNewTab = async () => {
        await handleOpenFile();
    };

    const handleDashboardFileOpen = async (filePath: string, fileOptions?: FileOptions, storedFileHash?: string) => {

        // Check if file+fileOptions combination is already open in a tab
        const normalizedOptions = fileOptions || createDefaultFileOptions();
        const normalizedJPath = normalizedOptions.jpath || '';
        const normalizedNoHeaderRow = normalizedOptions.noHeaderRow || false;
        const normalizedIngestTzOverride = normalizedOptions.ingestTimezoneOverride || '';
        const existingTabId = tabState.findTabByFilePath(filePath, normalizedOptions);
        if (existingTabId) {
            // Switch to existing tab
            tabState.switchTab(existingTabId);
            if (existingTabId !== '__dashboard__') {
                try {
                    await AppAPI.SetActiveTab(existingTabId);
                } catch (e) {
                    console.warn('Failed to set active tab:', e);
                }
            }
            return;
        }

        // For directories from workspace, check if the hash has changed
        if (normalizedOptions.isDirectory && storedFileHash) {
            try {
                const hashCheck = await AppAPI.CheckDirectoryHashMismatch(
                    filePath,
                    normalizedOptions.filePattern || '',
                    storedFileHash
                );
                if (hashCheck.hasMismatch) {
                    // Show warning dialog instead of proceeding directly
                    setDirHashWarningPath(filePath);
                    setDirHashWarningOptions(normalizedOptions);
                    setShowDirHashWarning(true);
                    return;
                }
            } catch (e) {
                console.warn('Failed to check directory hash mismatch:', e);
                // Continue anyway if check fails
            }
        }

        // Check if file requires a plugin that's not available
        // For workspace files, pass the stored pluginId and pluginName (if any) to check for that specific plugin
        const pluginIdToCheck = normalizedOptions.pluginId || "";
        const pluginNameToCheck = normalizedOptions.pluginName || "";
        try {
            const pluginReq = await AppAPI.CheckPluginRequirement(filePath, pluginIdToCheck, pluginNameToCheck);
            if (pluginReq.requiresPlugin && !pluginReq.pluginAvailable) {
                // Build error message based on the situation
                if (!pluginReq.pluginsEnabled) {
                    // Plugin support is disabled globally
                    if (pluginReq.requiredPluginId) {
                        // Workspace file has a stored plugin reference
                        showMessageDialogAction(
                            'Plugin Required',
                            `This workspace file requires the plugin "${pluginReq.requiredPluginName}" (ID: ${pluginReq.requiredPluginId}) to open.\n\nPlugin support is currently disabled. Please enable plugin support in Settings → Plugins, and ensure the required plugin is enabled.`,
                            true
                        );
                    } else {
                        showMessageDialogAction(
                            'File Type Not Supported',
                            `Files with extension "${pluginReq.fileExtension}" require plugin support to open.\n\nPlugin support is currently disabled. Please enable plugin support in Settings → Plugins to open this file type.`,
                            true
                        );
                    }
                } else if (pluginReq.requiredPluginId) {
                    // Plugin exists but is disabled
                    showMessageDialogAction(
                        'Plugin Required',
                        `This workspace file requires the plugin "${pluginReq.requiredPluginName}" (ID: ${pluginReq.requiredPluginId}) to open.\n\nPlease enable this plugin in Settings → Plugins.`,
                        true
                    );
                } else {
                    // No plugin configured for this extension
                    showMessageDialogAction(
                        'File Type Not Supported',
                        `Files with extension "${pluginReq.fileExtension}" are not supported by the built-in loaders and no plugin is available to handle this file type.\n\nPlease add a plugin that supports "${pluginReq.fileExtension}" files in Settings → Plugins.`,
                        true
                    );
                }
                return;
            }
        } catch (e) {
            // Continue anyway if check fails
        }

        // Check for plugins - either use stored pluginId or detect from file (for warning dialogs)
        try {
            const plugins = await AppAPI.GetPluginsForFile(filePath);

            if (normalizedOptions.pluginId) {
                // File has stored pluginId - find the plugin info and show warning
                const storedPlugin = plugins?.find((p: any) => p.id === normalizedOptions.pluginId);
                if (storedPlugin) {
                    pluginHandlers.showPluginWarningFor(filePath, storedPlugin, 'open', normalizedOptions);
                    return;
                }
                // Plugin not found - continue without warning (plugin may have been removed)
            } else if (plugins && plugins.length > 1) {
                // Multiple plugins available - show selection dialog
                pluginHandlers.showPluginSelectionFor(filePath, plugins, 'open');
                return;
            } else if (plugins && plugins.length === 1) {
                // Single plugin available - show warning dialog
                pluginHandlers.showPluginWarningFor(filePath, plugins[0], 'open');
                return;
            }
        } catch (e) {
            // Continue anyway if check fails
        }

        // Show loading spinner while opening file
        setIsOpeningFile(true);

        // Check if this is a JSON file with a jpath expression from workspace
        const isJsonFile = filePath.toLowerCase().endsWith('.json');
        if (isJsonFile && normalizedJPath) {
            // Handle JSON file with workspace jpath expression
            try {
                // Load the JSON file with full file options (including jpath and ingestTimezoneOverride)
                // This ensures the file is opened with all the same options as it was saved with in the workspace
                const result = await fileOps.loadJsonFile(filePath, normalizedOptions);

                if (result && result.header && result.header.length > 0) {
                    // Switch to the tab so the Grid component mounts
                    tabState.switchTab(result.tabId);

                    // Set active tab in backend (skip for dashboard)
                    if (result.tabId !== '__dashboard__') {
                        try {
                            await AppAPI.SetActiveTab(result.tabId);
                        } catch (e) {
                            console.warn('Failed to set active tab:', e);
                        }
                    }

                    // Give React multiple ticks to update state and mount the Grid
                    await new Promise(resolve => setTimeout(resolve, 100));

                    // Access tab by ID
                    const tab = tabState.getTabState(result.tabId);

                    if (tab) {

                        // For JSON files, grid may be ready but no datasource was created yet
                        // (because headers weren't available when grid first mounted)
                        // Create datasource now if needed
                        if (tab.gridApi && !tab.datasource) {
                            const ds = gridOps.createDataSource(
                                result.tabId,
                                tab.appliedQuery || '',
                                result.header,
                                result.timeField
                            );
                            tabState.setDatasourceForTab(result.tabId, ds);
                        }

                        // Refresh histogram (skip if unified fetch already loaded it)
                        if (result.timeField) {
                            try {
                                await histogram.refreshHistogram(result.tabId, result.timeField, '', true);
                            } catch (e) {
                                console.warn('Failed to refresh histogram:', e);
                            }
                        }

                        addLog('info', `Opened JSON file from workspace: ${filePath}`);
                    }
                }
            } catch (e: any) {
                addLog('error', 'Failed to open JSON file: ' + (e?.message || String(e)));
            } finally {
                setIsOpeningFile(false);
            }
            return;
        }

        // Open the file in a new tab using backend API (CSV, XLSX, or JSON without jpath)
        // Use OpenFileTabWithOptions to pass the options from workspace metadata
        // Include all options including directory-specific ones (isDirectory, filePattern, includeSourceColumn)
        try {
            const response = await AppAPI.OpenFileTabWithOptions(filePath, {
                jpath: normalizedJPath,
                noHeaderRow: normalizedNoHeaderRow,
                ingestTimezoneOverride: normalizedIngestTzOverride,
                pluginId: normalizedOptions.pluginId || '',
                isDirectory: normalizedOptions.isDirectory || false,
                filePattern: normalizedOptions.filePattern || '',
                includeSourceColumn: normalizedOptions.includeSourceColumn || false,
            });

            if (!response || !response.id) {
                addLog('error', 'Failed to open file');
                return;
            }

            const tabId = response.id;

            // Create new tab with noHeaderRow and ingestTimezoneOverride options
            tabState.createTab(tabId, filePath, response.fileHash || '', normalizedOptions);

            // Get headers from response
            const hdr = response.headers || [];

            if (hdr && hdr.length > 0) {
                tabState.setHeaderForTab(tabId, hdr);
                tabState.setOriginalHeaderForTab(tabId, hdr);

                const detectedTimeField = fileOps.detectTimestampField(hdr);
                const defs = fileOps.buildColumnDefs(hdr, detectedTimeField, appliedDisplayTZ);

                tabState.setColumnDefsForTab(tabId, defs);
                tabState.setTimeFieldForTab(tabId, detectedTimeField);
                tabState.setHistBucketsForTab(tabId, []);
                // Initialize histogram version so the first async histogram event will be accepted
                tabState.setHistogramVersionForTab(tabId, `${tabId}:1`);
                tabState.incrementFileTokenForTab(tabId);
                tabState.incrementGenerationForTab(tabId);

                // Fetch total row count
                try {
                    const total = await AppAPI.GetCSVRowCount();
                    tabState.setTotalRowsForTab(tabId, total || 0);
                } catch (e) {
                    console.warn('Failed to get total row count:', e);
                }

                // Set active tab (skip for dashboard)
                if (tabId !== '__dashboard__') {
                    try {
                        await AppAPI.SetActiveTab(tabId);
                    } catch (e) {
                        console.warn('Failed to set active tab:', e);
                    }
                }

                // Switch to the new tab
                tabState.switchTab(tabId);

                // Wait for tab to be active
                await new Promise(resolve => setTimeout(resolve, 100));

                // Start loading histogram asynchronously in the background (don't block tab switching)
                // Skip if unified fetch already loaded histogram data
                if (detectedTimeField) {
                    histogram.refreshHistogram(tabId, detectedTimeField, '', true).catch(e => {
                        console.warn('Failed to refresh histogram:', e);
                    });
                }

                addLog('info', `Opened file: ${filePath}`);
            }
        } catch (e: any) {
            const errorMsg = e?.message || String(e);
            addLog('error', 'Failed to open file: ' + errorMsg);
            setErrorDialogMessage(errorMsg);
            setShowErrorDialog(true);
        } finally {
            setIsOpeningFile(false);
        }
    };

    // Directory hash warning dialog handlers
    const handleDirHashWarningCancel = () => {
        setShowDirHashWarning(false);
        setDirHashWarningPath('');
        setDirHashWarningOptions(null);
    };

    const handleDirHashWarningContinue = () => {
        // User chose to proceed despite hash mismatch
        // Re-call handleDashboardFileOpen without the storedFileHash so it won't check again
        const path = dirHashWarningPath;
        const options = dirHashWarningOptions;

        setShowDirHashWarning(false);
        setDirHashWarningPath('');
        setDirHashWarningOptions(null);

        if (path && options) {
            // Call without the hash to skip the check
            handleDashboardFileOpen(path, options);
        }
    };

    const handleIngestExpressionSave = async (expression: string) => {
        hideIngestExpressionDialog();

        // Show loading spinner while loading JSON file
        setIsOpeningFile(true);

        try {
            // Normal flow - load JSON file with expression
            const result = await fileOps.loadJsonFile(ingestFilePath, { jpath: expression });

            if (result && result.header && result.header.length > 0) {
                // Switch to the tab so the Grid component mounts
                tabState.switchTab(result.tabId);

                // Set active tab in backend (skip for dashboard)
                if (result.tabId !== '__dashboard__') {
                    try {
                        await AppAPI.SetActiveTab(result.tabId);
                    } catch (e) {
                        console.warn('Failed to set active tab:', e);
                    }
                }

                // Give React multiple ticks to update state and mount the Grid
                await new Promise(resolve => setTimeout(resolve, 100));

                // Access tab by ID
                const tab = tabState.getTabState(result.tabId);

                if (tab) {

                    // For JSON files, grid may be ready but no datasource was created yet
                    // (because headers weren't available when grid first mounted)
                    // Create datasource now if needed
                    if (tab.gridApi && !tab.datasource) {
                        const ds = gridOps.createDataSource(
                            result.tabId,
                            tab.appliedQuery || '',
                            result.header,
                            result.timeField
                        );
                        tabState.setDatasourceForTab(result.tabId, ds);
                    }

                    // Refresh histogram (skip if unified fetch already loaded it)
                    if (result.timeField) {
                        try {
                            await histogram.refreshHistogram(result.tabId, result.timeField, '', true);
                        } catch (e) {
                            console.warn('Failed to refresh histogram:', e);
                        }
                    }
                } else {
                    console.warn('Could not find tab by ID:', result.tabId);
                }
            }
        } finally {
            setIsOpeningFile(false);
        }
    };

    const handleIngestExpressionClose = () => {
        hideIngestExpressionDialog();
        // Close the tab that was created for the JSON file
        if (ingestTabId) {
            handleTabClose(ingestTabId);
        }
    };

    const handleAddFileToWorkspace = async () => {
        // Check if workspace is open
        if (!isWorkspaceOpen) {
            showMessageDialogAction('No Workspace Open', 'Please open a workspace file first (File → Open workspace) to add files.', true);
            return;
        }

        // Check license
        if (!isLicensed) {
            showMessageDialogAction('Premium Feature', 'Adding files to workspace requires a valid license. Please import a license file to use this feature.', true);
            return;
        }

        try {
            const AppAPI = await import('../wailsjs/go/app/App');
            // Use OpenFileDialog instead of OpenFileWithDialogTab to avoid trying to read headers before we have the JSONPath
            const filePath = await AppAPI.OpenFileDialog();

            if (!filePath) {
                return; // User cancelled dialog
            }

            if (isJsonFile(filePath)) {
                // Show file options dialog for JSON expression
                setFileOptionsFilePath(filePath);
                setFileOptionsMode('addToWorkspace');
                setFileOptionsFileType('json');
                setFileOptionsShowTimezone(false); // Simple add doesn't need timezone option
                setFileOptionsExistingTabId('');
                setShowFileOptionsDialog(true);
            } else {
                // Check if multiple plugins support this file type
                const plugins = await AppAPI.GetPluginsForFile(filePath);
                if (plugins && plugins.length > 1) {
                    // Show plugin selection dialog
                    pluginHandlers.showPluginSelectionFor(filePath, plugins, 'addToWorkspace');
                    return;
                }

                // If only one plugin, show warning dialog before proceeding
                if (plugins && plugins.length === 1) {
                    pluginHandlers.showPluginWarningFor(filePath, plugins[0], 'addToWorkspace');
                    return;
                }

                // No plugins - add directly to workspace
                await AppAPI.AddFileToWorkspace(filePath, createDefaultFileOptions());
                addLog('info', `Added file to workspace: ${filePath}`);
            }
        } catch (e: any) {
            const errorMsg = e?.message || String(e);
            if (errorMsg.includes('requires a valid license')) {
                showMessageDialogAction('Premium Feature', 'Adding files to workspace requires a valid license. Please import a license file to use this feature.', true);
            } else {
                addLog('error', 'Failed to add file to workspace: ' + errorMsg);
            }
        }
    };

    const handleAddToWorkspaceSaveExpression = async (expression: string) => {
        hideAddToWorkspaceDialog();

        try {
            const WorkspaceAPI = await import('../wailsjs/go/app/App');
            await WorkspaceAPI.AddFileToWorkspace(addToWorkspaceFilePath, { jpath: expression });
            addLog('info', `Added JSON file to workspace: ${addToWorkspaceFilePath}`);
        } catch (e: any) {
            const errorMsg = e?.message || String(e);
            addLog('error', 'Failed to add file to workspace: ' + errorMsg);
        }
    };

    const handleAddToWorkspaceClose = () => {
        hideAddToWorkspaceDialog();
    };

    // Handler for "Open File with Options..." menu - shows choice dialog
    const handleOpenFileWithOptions = async () => {
        setError("");
        setFileOrDirChoiceMode('open');
        setShowFileOrDirChoice(true);
    };

    // Handler when user chooses "File" from the choice dialog
    const handleChooseFile = async (mode: 'open' | 'addToWorkspace') => {
        setShowFileOrDirChoice(false);
        try {
            const filePath = await AppAPI.OpenFileDialog();
            if (!filePath) return; // User cancelled

            // Determine file type
            const lowerPath = filePath.toLowerCase();
            let fileType: 'json' | 'csv' | 'xlsx' = 'csv';
            if (isJsonFile(filePath)) {
                fileType = 'json';
            } else if (lowerPath.endsWith('.xlsx') || lowerPath.endsWith('.xls')) {
                fileType = 'xlsx';
            }

            // Show file options dialog
            setFileOptionsFilePath(filePath);
            setFileOptionsMode(mode);
            setFileOptionsFileType(fileType);
            setFileOptionsShowTimezone(true);
            setFileOptionsExistingTabId('');
            setShowFileOptionsDialog(true);
        } catch (e: any) {
            addLog('error', 'Failed to open file dialog: ' + (e?.message || String(e)));
        }
    };

    // Handler when user chooses "Directory" from the choice dialog
    const handleChooseDirectory = async (mode: 'open' | 'addToWorkspace') => {
        setShowFileOrDirChoice(false);
        try {
            const dirPath = await AppAPI.OpenDirectoryDialog();
            if (!dirPath) return; // User cancelled

            // Show file options dialog for directory
            setFileOptionsFilePath(dirPath);
            setFileOptionsMode(mode);
            setFileOptionsFileType('directory');
            setFileOptionsShowTimezone(true);
            setFileOptionsExistingTabId('');
            setShowFileOptionsDialog(true);
        } catch (e: any) {
            addLog('error', 'Failed to open directory dialog: ' + (e?.message || String(e)));
        }
    };

    // Handler for "Add File to Workspace with Options..." menu - shows choice dialog
    const handleAddFileToWorkspaceWithOptions = async () => {
        // Check if workspace is open
        if (!isWorkspaceOpen) {
            showMessageDialogAction('No Workspace Open', 'Please open a workspace file first (File → Open workspace) to add files.', true);
            return;
        }

        // Check license
        if (!isLicensed) {
            showMessageDialogAction('Premium Feature', 'Adding files to workspace requires a valid license. Please import a license file to use this feature.', true);
            return;
        }

        // Show choice dialog
        setFileOrDirChoiceMode('addToWorkspace');
        setShowFileOrDirChoice(true);
    };

    // Handler for opening a directory (can be called from file dialogs or directly)
    const handleOpenDirectory = async () => {
        try {
            const dirPath = await AppAPI.OpenDirectoryDialog();
            if (!dirPath) return; // User cancelled

            // Use FileOptionsDialog for directories
            setFileOptionsFilePath(dirPath);
            setFileOptionsFileType('directory');
            setFileOptionsMode('open');
            setFileOptionsExistingTabId('');
            setShowFileOptionsDialog(true);
        } catch (e: any) {
            addLog('error', 'Failed to open directory dialog: ' + (e?.message || String(e)));
        }
    };

    // Handler for FileOptionsDialog confirmation
    const handleFileOptionsConfirm = async (options: FileOpenOptions) => {
        setShowFileOptionsDialog(false);
        const existingTabIdForJson = fileOptionsExistingTabId;
        setFileOptionsExistingTabId(''); // Clear for next use

        // Handle directory opening
        if (options.isDirectory) {
            setIsOpeningFile(true);
            try {
                // Build FileOptions for directory
                const fileOpts: FileOptions = {
                    jpath: options.jpathExpression,
                    noHeaderRow: options.noHeaderRow,
                    ingestTimezoneOverride: options.ingestTimezoneOverride,
                    isDirectory: true,
                    filePattern: options.filePattern,
                    includeSourceColumn: options.includeSourceColumn,
                };

                // Check if any files in the directory require plugins
                // We do this check here (after file options are confirmed) because we need the file pattern
                // to know which files will actually be included.
                // We use the same pattern as fileloader to ensure consistency.
                const pluginReq = await AppAPI.CheckDirectoryPluginRequirements(
                    fileOptionsFilePath,
                    fileOpts.filePattern || ''
                );

                if (pluginReq.requiresPlugin) {
                    if (!pluginReq.pluginAvailable) {
                        setIsOpeningFile(false);

                        // Build error message based on the situation
                        if (!pluginReq.pluginsEnabled) {
                            // Plugin support is disabled globally
                            if (pluginReq.requiredPluginId) {
                                // Workspace file has a stored plugin reference (unlikely for new directory open, but possible conceptually)
                                showMessageDialogAction(
                                    'Plugin Required',
                                    `Some files in this directory require the plugin "${pluginReq.requiredPluginName}" (ID: ${pluginReq.requiredPluginId}) to open.\n\nPlugin support is currently disabled. Please enable plugin support in Settings → Plugins, and ensure the required plugin is enabled.`,
                                    true
                                );
                            } else {
                                showMessageDialogAction(
                                    'File Type Not Supported',
                                    `Some files in this directory (extension "${pluginReq.fileExtension}") require plugin support to open.\n\nPlugin support is currently disabled. Please enable plugin support in Settings → Plugins to open these files.`,
                                    true
                                );
                            }
                        } else if (pluginReq.requiredPluginId) {
                            // Plugin exists but is disabled
                            showMessageDialogAction(
                                'Plugin Required',
                                `Some files in this directory require a plugin that is currently disabled.\n\nPlease enable the plugin "${pluginReq.requiredPluginName}" (ID: ${pluginReq.requiredPluginId}) in Settings → Plugins to open these files.`,
                                true
                            );
                        } else {
                            // No plugin configured for this extension
                            showMessageDialogAction(
                                'File Type Not Supported',
                                `Some files in this directory (extension "${pluginReq.fileExtension}") are not supported by the built-in loaders and no plugin is available to handle them.\n\nPlease add a plugin that supports "${pluginReq.fileExtension}" files in Settings → Plugins.`,
                                true
                            );
                        }
                        return;
                    }

                    // Plugin IS available - show selection or warning dialog
                    if (pluginReq.plugins && pluginReq.plugins.length > 0) {
                        // Store pending options for after plugin selection
                        pluginHandlers.setPendingFileOptionsDialogOptions(options);

                        if (pluginReq.plugins.length > 1) {
                            // Multiple plugins - show selection dialog
                            pluginHandlers.showPluginSelectionFor(fileOptionsFilePath, pluginReq.plugins as any, fileOptionsMode);
                            return;
                        } else {
                            // Single plugin - show warning dialog
                            pluginHandlers.showPluginWarningFor(fileOptionsFilePath, pluginReq.plugins[0] as any, fileOptionsMode, fileOpts);
                            return;
                        }
                    }
                }

                // Check if we are adding to workspace or opening
                if (fileOptionsMode === 'addToWorkspace') {
                    await AppAPI.AddFileToWorkspace(fileOptionsFilePath, fileOpts);
                    addLog('info', `Added directory to workspace: ${fileOptionsFilePath}`);
                    return;
                }

                // @ts-ignore - OpenDirectoryTabWithOptions available after Wails bindings regeneration
                const response = await AppAPI.OpenDirectoryTabWithOptions(fileOptionsFilePath, fileOpts);

                if (!response || !response.id) {
                    addLog('error', 'Failed to open directory');
                    setErrorDialogMessage('The directory could not be opened. Please check the directory path and options.');
                    setShowErrorDialog(true);
                    return;
                }

                // Add detected file type from backend response
                if (response.detectedFileType) {
                    fileOpts.detectedFileType = response.detectedFileType;
                }

                // Create tab with headers and file options
                tabState.createTab(response.id, response.filePath || '', response.fileHash || '', fileOpts);

                const hdr = response.headers || [];
                if (hdr && hdr.length > 0) {
                    const tabId = response.id;
                    tabState.setHeaderForTab(tabId, hdr);
                    tabState.setOriginalHeaderForTab(tabId, hdr);

                    const detectedTimeField = fileOps.detectTimestampField(hdr);
                    const defs = fileOps.buildColumnDefs(hdr, detectedTimeField, appliedDisplayTZ);

                    tabState.setColumnDefsForTab(tabId, defs);
                    tabState.setTimeFieldForTab(tabId, detectedTimeField);
                    tabState.setHistBucketsForTab(tabId, []);
                    tabState.setHistogramVersionForTab(tabId, `${tabId}:1`);
                    tabState.incrementFileTokenForTab(tabId);
                    tabState.incrementGenerationForTab(tabId);

                    const dirName = fileOptionsFilePath.split('/').pop() || fileOptionsFilePath.split('\\').pop() || fileOptionsFilePath;
                    addLog('info', `Opened directory: ${dirName}/. Columns: ${hdr.length}`);

                    // Refresh histogram if time field detected
                    if (detectedTimeField) {
                        try {
                            await histogram.refreshHistogram(tabId, detectedTimeField, '', true);
                        } catch (e) {
                            console.warn('Failed to refresh histogram:', e);
                        }
                    }
                }
            } catch (e: any) {
                const errorMsg = e?.message || String(e);
                addLog('error', 'Failed to open directory: ' + errorMsg);
                setErrorDialogMessage(errorMsg);
                setShowErrorDialog(true);
            } finally {
                setIsOpeningFile(false);
            }
            return;
        }

        if (fileOptionsMode === 'open') {
            // Check for plugins before opening
            try {
                const plugins = await AppAPI.GetPluginsForFile(fileOptionsFilePath);
                if (plugins && plugins.length > 0) {
                    // Store pending options for after plugin selection
                    pluginHandlers.setPendingFileOptionsDialogOptions(options);

                    if (plugins.length > 1) {
                        // Show plugin selection dialog
                        pluginHandlers.showPluginSelectionFor(fileOptionsFilePath, plugins, 'open');
                        return;
                    } else {
                        // Single plugin - show warning dialog
                        const fileOpts: FileOptions = {
                            jpath: options.jpathExpression,
                            noHeaderRow: options.noHeaderRow,
                            ingestTimezoneOverride: options.ingestTimezoneOverride,
                        };
                        pluginHandlers.showPluginWarningFor(fileOptionsFilePath, plugins[0], 'open', fileOpts);
                        return;
                    }
                }
            } catch (e) {
                console.warn('Failed to check plugins for file:', e);
            }

            // No plugins - open file with options
            setIsOpeningFile(true);
            try {
                // If we have an existing tab (from workspace JSON open), load JSON with expression into it
                if (existingTabIdForJson && options.jpathExpression) {
                    // Pass full file options including ingestTimezoneOverride
                    const fileOpts: FileOptions = {
                        jpath: options.jpathExpression,
                        noHeaderRow: options.noHeaderRow,
                        ingestTimezoneOverride: options.ingestTimezoneOverride,
                    };
                    const result = await fileOps.loadJsonFile(fileOptionsFilePath, fileOpts);

                    if (result && result.header && result.header.length > 0) {
                        // Switch to the tab
                        tabState.switchTab(result.tabId);

                        // Set active tab in backend
                        if (result.tabId !== '__dashboard__') {
                            try {
                                await AppAPI.SetActiveTab(result.tabId);
                            } catch (e) {
                                console.warn('Failed to set active tab:', e);
                            }
                        }

                        addLog('info', `Opened JSON file: ${fileOptionsFilePath}. Columns: ${result.header.length}`);
                    }
                    setIsOpeningFile(false);
                    return;
                }

                // Pass all options including jpath for JSON files
                const fileOpts: FileOptions = {
                    jpath: options.jpathExpression,
                    noHeaderRow: options.noHeaderRow,
                    ingestTimezoneOverride: options.ingestTimezoneOverride,
                };
                const response = await AppAPI.OpenFileTabWithOptions(fileOptionsFilePath, fileOpts);

                if (!response || !response.id) {
                    setIsOpeningFile(false);
                    return;
                }

                // Check if file is already open in another tab with same options
                const existingTabId = tabState.findTabByFilePath(response.filePath || '', fileOpts);
                if (existingTabId && existingTabId !== response.id) {
                    tabState.switchTab(existingTabId);
                    addLog('info', `File already open, switched to existing tab`);
                    setIsOpeningFile(false);
                    return;
                }

                // Create new tab with FileOptions
                tabState.createTab(response.id, response.filePath || '', response.fileHash || '', fileOpts);

                const hdr = response.headers || [];
                if (hdr && hdr.length > 0) {
                    const tabId = response.id;
                    tabState.setHeaderForTab(tabId, hdr);
                    tabState.setOriginalHeaderForTab(tabId, hdr);

                    // Store jpath expression for JSON files - already set via createTab's fileOpts
                    // No need to call setFileOptionsForTab separately since options were passed to createTab

                    const detectedTimeField = fileOps.detectTimestampField(hdr);
                    const defs = fileOps.buildColumnDefs(hdr, detectedTimeField, appliedDisplayTZ);

                    tabState.setColumnDefsForTab(tabId, defs);
                    tabState.setTimeFieldForTab(tabId, detectedTimeField);
                    tabState.setHistBucketsForTab(tabId, []);
                    tabState.setHistogramVersionForTab(tabId, `${tabId}:1`);
                    tabState.incrementFileTokenForTab(tabId);
                    tabState.incrementGenerationForTab(tabId);

                    addLog('info', `Opened file with options: ${response.filePath || 'Unknown'}. Columns: ${hdr.length}`);

                    // Fetch total row count
                    try {
                        const total = await AppAPI.GetCSVRowCount();
                        tabState.setTotalRowsForTab(tabId, total || 0);
                    } catch (e) {
                        console.warn('Failed to get total row count:', e);
                    }

                    // Refresh histogram if time field detected
                    if (detectedTimeField) {
                        try {
                            await histogram.refreshHistogram(tabId, detectedTimeField, '', true);
                        } catch (e) {
                            console.warn('Failed to refresh histogram:', e);
                        }
                    }
                }
            } catch (e: any) {
                const errorMsg = e?.message || String(e);
                addLog('error', 'Failed to open file: ' + errorMsg);
                setErrorDialogMessage(errorMsg);
                setShowErrorDialog(true);
            } finally {
                setIsOpeningFile(false);
            }
        } else {
            // Add file to workspace with options - check for plugins first
            try {
                const plugins = await AppAPI.GetPluginsForFile(fileOptionsFilePath);
                if (plugins && plugins.length > 0) {
                    // Store pending options for after plugin selection
                    pluginHandlers.setPendingFileOptionsDialogOptions(options);

                    if (plugins.length > 1) {
                        // Show plugin selection dialog
                        pluginHandlers.showPluginSelectionFor(fileOptionsFilePath, plugins, 'addToWorkspace');
                        return;
                    } else {
                        // Single plugin - show warning dialog
                        const fileOpts: FileOptions = {
                            jpath: options.jpathExpression,
                            noHeaderRow: options.noHeaderRow,
                            ingestTimezoneOverride: options.ingestTimezoneOverride,
                        };
                        pluginHandlers.showPluginWarningFor(fileOptionsFilePath, plugins[0], 'addToWorkspace', fileOpts);
                        return;
                    }
                }
            } catch (e) {
                console.warn('Failed to check plugins for file:', e);
            }

            // No plugins - add file to workspace with options
            try {
                // Build FileOptions from the dialog options
                const fileOpts: FileOptions = {
                    jpath: options.jpathExpression,
                    noHeaderRow: options.noHeaderRow,
                    ingestTimezoneOverride: options.ingestTimezoneOverride
                };
                await AppAPI.AddFileToWorkspace(fileOptionsFilePath, fileOpts);
                addLog('info', `Added file to workspace: ${fileOptionsFilePath}`);
            } catch (e: any) {
                addLog('error', 'Failed to add file to workspace: ' + (e?.message || String(e)));
            }
        }
    };

    const handleFileOptionsCancel = () => {
        setShowFileOptionsDialog(false);
    };

    const handleApplySearch = async (queryText?: string) => {
        const currentTab = tabState.currentTab;
        if (!currentTab) return;

        const q = queryText ?? currentTab.query;
        addToHistory(q);
        await searchOps.applySearch(q);

        // Refresh histogram (skip if unified fetch already loaded it)
        await histogram.refreshHistogram(
            currentTab.tabId,
            currentTab.timeField || '',
            tabState.appliedQueryRef.current,
            true  // skipIfRecent - don't reload if unified fetch just updated it
        );
    };

    const handleHistogramBrushEnd = async (start: number, end: number) => {
        const currentTab = tabState.currentTab;
        if (!currentTab) return;

        const afterMs = start;
        const beforeMs = end;

        // Format timestamp in display timezone without timezone suffix, accurate to seconds only
        const formatTimestamp = (ms: number, displayTZ: string): string => {
            const d = new Date(ms);

            // Use UTC methods if displayTZ is UTC, otherwise use local timezone methods
            const isUTC = displayTZ?.toUpperCase() === 'UTC';

            const year = isUTC ? d.getUTCFullYear() : d.getFullYear();
            const month = String(isUTC ? d.getUTCMonth() + 1 : d.getMonth() + 1).padStart(2, '0');
            const day = String(isUTC ? d.getUTCDate() : d.getDate()).padStart(2, '0');
            const hours = String(isUTC ? d.getUTCHours() : d.getHours()).padStart(2, '0');
            const minutes = String(isUTC ? d.getUTCMinutes() : d.getMinutes()).padStart(2, '0');
            const seconds = String(isUTC ? d.getUTCSeconds() : d.getSeconds()).padStart(2, '0');

            return `${year}-${month}-${day}T${hours}:${minutes}:${seconds}`;
        };

        const afterStr = `'${formatTimestamp(afterMs, appliedDisplayTZ)}'`;
        const beforeStr = `'${formatTimestamp(beforeMs, appliedDisplayTZ)}'`;

        const currentQuery = currentTab.query || '';
        const replaced = searchOps.replaceExistingTimeRange(currentQuery, afterStr, beforeStr);

        if (replaced) {
            tabState.setQuery(replaced);
            await searchOps.applySearch(replaced);
        } else {
            const newQ = currentQuery
                ? `${currentQuery} | after ${afterStr} | before ${beforeStr}`
                : `after ${afterStr} | before ${beforeStr}`;
            tabState.setQuery(newQ);
            await searchOps.applySearch(newQ);
        }

        addToHistory(tabState.currentTab?.query || '');
        await histogram.refreshHistogram(
            currentTab.tabId,
            currentTab.timeField || '',
            tabState.appliedQueryRef.current,
            true  // skipIfRecent - don't reload if unified fetch just updated it
        );
    };

    const handleSetTimestampColumn = async (columnName: string) => {
        const currentTab = tabState.currentTab;
        if (!currentTab) return;

        // Show loading dialog immediately for better UX
        setIsChangingTimestamp(true);

        try {
            // First validate the column
            const validation = await AppAPI.ValidateTimestampColumn(columnName);

            if (!validation.valid) {
                addLog('error', validation.errorMessage || 'Failed to validate timestamp column');
                setErrorDialogMessage(validation.errorMessage || 'Failed to validate timestamp column');
                setShowErrorDialog(true);
                setIsChangingTimestamp(false);
                return;
            }

            // Set the timestamp column (this also expires cache)
            const result = await AppAPI.SetTimestampColumn(columnName);

            if (!result.success) {
                addLog('error', result.message || 'Failed to set timestamp column');
                setErrorDialogMessage(result.message || 'Failed to set timestamp column');
                setShowErrorDialog(true);
                setIsChangingTimestamp(false);
                return;
            }

            // Update the tab state with new timestamp field FIRST
            // This is the ONLY place (besides file load) where timeField should be set
            tabState.setTimeFieldForTab(currentTab.tabId, columnName);

            // Re-apply the current search to refresh data and headers with new timestamp column
            // Pass columnName as timeFieldOverride to ensure applySearch uses it immediately
            // even if the state update hasn't propagated yet due to React batching
            await searchOps.applySearch(currentTab.appliedQuery || '', undefined, columnName);

            // Refresh histogram with new timestamp field
            await histogram.refreshHistogram(
                currentTab.tabId,
                columnName,
                currentTab.appliedQuery || '',
                true  // skipIfRecent - don't reload if unified fetch just updated it
            );

            addLog('info', `Timestamp column set to '${columnName}'`);
            setIsChangingTimestamp(false);
        } catch (err: any) {
            const errorMessage = 'Failed to set timestamp column: ' + (err?.message || String(err));
            addLog('error', errorMessage);
            setErrorDialogMessage(errorMessage);
            setShowErrorDialog(true);
            setIsChangingTimestamp(false);
        }
    };

    // JPath expression handlers
    const handleOpenJPathDialog = async (columnName: string) => {
        const currentTab = tabState.currentTab;
        if (!currentTab) return;

        try {
            // Get current JPath expression for this column (if any)
            const currentExpressions = tabState.getColumnJPathExpressionsForTab(currentTab.tabId);
            const currentExpression = currentExpressions[columnName] || '$';

            // Get first 5 rows of data from this column for preview
            // We need to get the column index from the header
            const header = currentTab.header || [];
            const columnIndex = header.findIndex(h => h === columnName || h.trim() === columnName);

            let previewData: string[] = [];
            if (columnIndex >= 0) {
                try {
                    // Fetch a few rows to get preview data using the unified data fetch endpoint
                    const result = await AppAPI.GetDataAndHistogram(
                        currentTab.tabId,
                        0,  // startRow
                        5,  // endRow (first 5 rows)
                        currentTab.appliedQuery || '',
                        currentTab.timeField || '',
                        300  // bucketSeconds (not used for this purpose)
                    );
                    const rows = result?.rows || [];
                    previewData = rows.map((row: string[]) => row[columnIndex] || '');
                } catch (e) {
                    console.warn('Failed to fetch preview data:', e);
                }
            }

            showColumnJPathDialogWithData(columnName, currentExpression, previewData);
        } catch (err: any) {
            addLog('error', 'Failed to open JPath dialog: ' + (err?.message || String(err)));
        }
    };

    const handleApplyJPath = (expression: string) => {
        const currentTab = tabState.currentTab;
        if (!currentTab || !columnJPathTarget.columnName) return;

        // Update the JPath expression for this column
        tabState.setColumnJPathExpressionForTab(currentTab.tabId, columnJPathTarget.columnName, expression);

        // Rebuild column definitions with the new JPath expression
        const jpathExprs = {
            ...tabState.getColumnJPathExpressionsForTab(currentTab.tabId),
            [columnJPathTarget.columnName]: expression,
        };
        const newDefs = fileOps.buildColumnDefs(
            currentTab.header,
            currentTab.timeField,
            appliedDisplayTZ,
            jpathExprs,
            currentTab.highlightTerms || []
        );
        tabState.setColumnDefsForTab(currentTab.tabId, newDefs);

        // Refresh the grid to show transformed data
        if (currentTab.gridApi) {
            currentTab.gridApi.refreshCells({ force: true });
        }

        addLog('info', `Applied JPath '${expression}' to column '${columnJPathTarget.columnName}'`);
        hideColumnJPathDialog();
    };

    const handleClearJPath = () => {
        const currentTab = tabState.currentTab;
        if (!currentTab || !columnJPathTarget.columnName) return;

        // Clear the JPath expression for this column
        tabState.clearColumnJPathExpressionForTab(currentTab.tabId, columnJPathTarget.columnName);

        // Rebuild column definitions without the JPath expression
        const jpathExprs = tabState.getColumnJPathExpressionsForTab(currentTab.tabId);
        const newDefs = fileOps.buildColumnDefs(
            currentTab.header,
            currentTab.timeField,
            appliedDisplayTZ,
            jpathExprs,
            currentTab.highlightTerms || []
        );
        tabState.setColumnDefsForTab(currentTab.tabId, newDefs);

        // Refresh the grid to show original data
        if (currentTab.gridApi) {
            currentTab.gridApi.refreshCells({ force: true });
        }

        addLog('info', `Cleared JPath from column '${columnJPathTarget.columnName}'`);
        hideColumnJPathDialog();
    };

    // Check if a column has an active JPath expression
    const hasColumnJPath = (columnName: string): boolean => {
        const currentTab = tabState.currentTab;
        if (!currentTab) return false;
        const exprs = tabState.getColumnJPathExpressionsForTab(currentTab.tabId);
        const expr = exprs[columnName];
        return !!expr && expr !== '$';
    };

    // Menu events hook - handles all native menu events
    const { shouldTriggerLoginAfterDialog, setShouldTriggerLoginAfterDialog } = useMenuEvents({
        handleOpenFile,
        handleOpenFileWithOptions,
        handleOpenDirectory,
        handleAddFileToWorkspace,
        handleAddFileToWorkspaceWithOptions,
        handleOpenWorkspace,
        handleCloseWorkspace,
        handleCreateLocalWorkspace,
        handleOpenCreateRemoteWorkspaceDialog,
        clipboardOps,
        gridContainerRef,
        dialogState,
        dialogActions,
        openSettings,
        getFuzzyFinderFiles,
        setFuzzyFinderFiles,
        setLicenseEmail,
        handleSyncLogin,
        showCacheIndicator,
        setShowCacheIndicator,
        isWorkspaceOpen,
        isLicensed,
        tabState,
        addLog,
    });

    // Fetch license details when About modal opens
    useEffect(() => {
        if (showAbout) {
            (async () => {
                try {
                    const details = await LicenseAPI.GetLicenseDetails();
                    setLicenseEmail(details.email || null);
                    // Parse the endDate string to a Date object
                    if (details.endDate) {
                        setLicenseEndDate(new Date(details.endDate));
                    } else {
                        setLicenseEndDate(null);
                    }
                } catch (e) {
                    // Not licensed or error fetching
                    setLicenseEmail(null);
                    setLicenseEndDate(null);
                }
            })();
        }
    }, [showAbout]);

    // Monitor license dialog state and trigger login when user closes the re-auth dialog
    useEffect(() => {
        if (!showMessageDialog && shouldTriggerLoginAfterDialog) {
            setShouldTriggerLoginAfterDialog(false);
            // Trigger login flow after user has closed the re-auth dialog
            setTimeout(async () => {
                await handleSyncLogin();
            }, 100); // Small delay to ensure dialog is fully closed
        }
    }, [showMessageDialog, shouldTriggerLoginAfterDialog, handleSyncLogin, setShouldTriggerLoginAfterDialog]);

    // Set up global keyboard shortcuts
    useEffect(() => {
        const handleGlobalKey = (e: KeyboardEvent) => {
            const isMac = /Mac|iPod|iPhone|iPad/.test(navigator.userAgent);
            const cmdOrCtrl = isMac ? e.metaKey : e.ctrlKey;

            // Ctrl/Cmd+L: focus query search bar
            if (cmdOrCtrl && e.key === 'l') {
                e.preventDefault();
                searchInputRef.current?.focus();
                searchInputRef.current?.select();
            }

            // Tab navigation
            if (e.ctrlKey && (e.key === 'Tab' || e.code === 'Tab')) {
                e.preventDefault();
                e.stopPropagation();

                const tabs = tabState.tabs;
                const currentIndex = tabs.findIndex(t => t.id === tabState.activeTabId);

                if (currentIndex !== -1 && tabs.length > 1) {
                    let nextIndex;
                    if (e.shiftKey) {
                        // Previous tab
                        nextIndex = currentIndex === 0 ? tabs.length - 1 : currentIndex - 1;
                    } else {
                        // Next tab
                        nextIndex = currentIndex === tabs.length - 1 ? 0 : currentIndex + 1;
                    }
                    tabState.switchTab(tabs[nextIndex].id);
                }
            }
        };

        // Listen on both window and document for better capture
        window.addEventListener('keydown', handleGlobalKey, true);
        document.addEventListener('keydown', handleGlobalKey, true);

        return () => {
            window.removeEventListener('keydown', handleGlobalKey, true);
            document.removeEventListener('keydown', handleGlobalKey, true);
        };
    }, [tabState]);

    // Handle Escape key to close settings modal
    useEffect(() => {
        if (!showSettings) return;

        const handleEscape = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.preventDefault();
                cancelSettings();
            }
        };

        document.addEventListener('keydown', handleEscape);
        return () => document.removeEventListener('keydown', handleEscape);
    }, [showSettings]);

    // Handle Escape key to close file/directory choice dialog
    useEffect(() => {
        if (!showFileOrDirChoice) return;

        const handleEscape = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.preventDefault();
                setShowFileOrDirChoice(false);
            }
        };

        document.addEventListener('keydown', handleEscape);
        return () => document.removeEventListener('keydown', handleEscape);
    }, [showFileOrDirChoice]);

    // Re-execute search when appliedQuery changes (grid data has changed)
    const prevAppliedQueryRef = useRef<string | undefined>(undefined);
    useEffect(() => {
        const currentTab = tabState.currentTab;
        if (!currentTab || currentTab.tabId === '__dashboard__') return;

        const currentAppliedQuery = currentTab.appliedQuery;
        const prevAppliedQuery = prevAppliedQueryRef.current;

        // Update ref for next comparison
        prevAppliedQueryRef.current = currentAppliedQuery;

        // Skip if this is the first render or if query hasn't changed
        if (prevAppliedQuery === undefined || prevAppliedQuery === currentAppliedQuery) return;

        // Re-execute search if there's an active search term
        if (currentTab.searchTerm && currentTab.searchTerm.trim()) {
            handleSearchForTab(currentTab.tabId, currentTab.searchTerm, currentTab.searchIsRegex);
        }
    }, [tabState.currentTab?.appliedQuery, tabState.currentTab?.tabId, handleSearchForTab]);

    // Handle Escape key and outside clicks to close header context menu
    useEffect(() => {
        if (!headerContextMenu.visible) return;

        const handleEscape = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.preventDefault();
                hideHeaderContextMenu();
            }
        };

        const handleClickOutside = () => {
            hideHeaderContextMenu();
        };

        document.addEventListener('keydown', handleEscape);
        document.addEventListener('click', handleClickOutside);
        return () => {
            document.removeEventListener('keydown', handleEscape);
            document.removeEventListener('click', handleClickOutside);
        };
    }, [headerContextMenu.visible, hideHeaderContextMenu]);

    // Window resize listener with debounced saving
    useEffect(() => {
        let resizeTimeout: number;

        const handleResize = () => {
            // Clear existing timeout
            if (resizeTimeout) {
                clearTimeout(resizeTimeout);
            }

            // Set new timeout to save window size after 500ms of no resize events
            resizeTimeout = setTimeout(async () => {
                try {
                    const width = window.outerWidth;
                    const height = window.outerHeight;
                    await AppAPI.SaveWindowSize(width, height);
                } catch (error) {
                    console.warn('Failed to save window size:', error);
                }
            }, 500);
        };

        // Add resize listener
        window.addEventListener('resize', handleResize);

        // Cleanup
        return () => {
            window.removeEventListener('resize', handleResize);
            if (resizeTimeout) {
                clearTimeout(resizeTimeout);
            }
        };
    }, []);

    // Note: Datasource setup is handled in handleOpenFile and GridManager
    // Do NOT create a useEffect that watches stateVersion and calls setDatasource
    // as it creates an infinite loop (setDatasource increments stateVersion)

    const currentTab = tabState.currentTab;
    const isDashboardActive = tabState.activeTabId === '__dashboard__';

    // Mark dashboard tab as sticky in tabs array and add logo icon
    const tabsWithSticky = tabState.tabs.map(tab => ({
        ...tab,
        isSticky: tab.id === '__dashboard__',
        icon: tab.id === '__dashboard__' ? LogoUniversal : undefined
    }));

    return (
        <div id="App" className="app-container">
            {/* Tab Bar */}
            <TabBar
                tabs={tabsWithSticky}
                activeTabId={tabState.activeTabId || ''}
                onTabChange={handleTabChange}
                onTabClose={handleTabClose}
                onNewTab={handleNewTab}
                showCacheIndicator={showCacheIndicator}
            />

            {error && <div style={{ color: 'red', padding: '8px 12px' }}>{error}</div>}

            {/* Search bar - only show when a regular tab is open (not dashboard) */}
            {currentTab && !isDashboardActive && (
                <SearchBar
                    appliedQuery={currentTab.query || ''}
                    onApply={(text) => {
                        tabState.setQuery(text);
                        handleApplySearch(text);
                    }}
                    inputRef={searchInputRef}
                    history={queryHistory}
                    queryError={currentTab.queryError}
                />
            )}

            <div className="content" style={{ position: 'relative' }}>
                {/* Histogram Manager - always mounted but hidden when dashboard is active or no tab */}
                <div className="histogram-wrap" style={{
                    display: (isDashboardActive || !currentTab) ? 'none' : 'block'
                }}>
                    <HistogramManager
                        tabState={tabState}
                        histogram={histogram}
                        showHistogram={showHistogram}
                        displayTimeZone={appliedDisplayTZ}
                        onRangeSelected={handleHistogramBrushEnd}
                    />
                </div>

                <div className="main-split" style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }}>
                    <div className="grid-wrap" ref={gridContainerRef}>
                        {/* Dashboard */}
                        {isDashboardActive && (
                            <Dashboard key={workspaceKey} onFileOpen={handleDashboardFileOpen} />
                        )}

                        {/* Logo screen when no tabs */}
                        {!isDashboardActive && !currentTab && (
                            <div style={{ width: '100%', height: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', opacity: 0.9, overflow: 'hidden' }}>
                                <img src={LogoUniversal} alt="App logo" style={{ width: 320, maxWidth: '60%', height: 'auto' }} />
                                <div style={{ fontSize: 14, color: '#bbb', marginTop: 12 }}>Open a file to get started</div>
                            </div>
                        )}

                        {/* Grid Manager - always mounted but hidden when dashboard is active or no tab */}
                        <div style={{
                            width: '100%',
                            height: '100%',
                            display: (isDashboardActive || !currentTab) ? 'none' : 'block'
                        }}>
                            <GridManager
                                tabState={tabState}
                                gridOps={gridOps}
                                pageSize={pageSize}
                                theme={gridTheme}
                                onCopySelected={clipboardOps.copySelectedToClipboard}
                                onAnnotateRow={handleAnnotateRow}
                                isLicensed={isLicensed}
                                isWorkspaceOpen={isWorkspaceOpen}
                                addLog={addLog}
                                copyPending={clipboardOps.copyPending}
                                onJumpToOriginal={handleJumpToOriginal}
                                onClearSearch={handleClearSearchForTab}
                                onSearch={handleSearchForTab}
                                onCancelSearch={handleCancelSearchForTab}
                            />
                        </div>
                    </div>

                    {/* Console panel - inside main-split so it's constrained by the container */}
                    <ConsolePanel
                        show={showConsole}
                        logs={logs}
                        height={consoleHeight}
                        onHeightChange={setConsoleHeight}
                        onClear={clearLogs}
                    />
                </div>

                {/* Cell Viewer Dialog - rendered at content level to cover histogram + grid + results */}
                {currentTab && !isDashboardActive && (
                    <CellViewerDialog
                        show={currentTab.cellViewer.visible}
                        columnName={currentTab.cellViewer.columnName}
                        cellValue={currentTab.cellViewer.cellValue}
                        searchTerms={currentTab.highlightTerms}
                        onClose={() => tabState.hideCellViewerForTab(currentTab.tabId)}
                        contained
                    />
                )}
            </div>

            {/* Settings modal */}
            <Settings
                show={showSettings}
                settings={settings}
                onCancel={cancelSettings}
                onSave={saveSettings}
                onSettingsChange={setSettings}
                onLog={addLog}
            />

            {/* About modal */}
            <AboutDialog
                show={showAbout}
                onClose={() => setShowAbout(false)}
                licenseEmail={licenseEmail}
                licenseEndDate={licenseEndDate}
            />

            {/* Syntax help modal */}
            <SyntaxHelpDialog show={showSyntax} onClose={() => setShowSyntax(false)} />

            {/* Header context menu */}
            <HeaderContextMenu
                visible={headerContextMenu.visible}
                x={headerContextMenu.x}
                y={headerContextMenu.y}
                columnName={headerContextMenu.columnName}
                onClose={() => hideHeaderContextMenu()}
                onSetTimestamp={handleSetTimestampColumn}
                onApplyJPath={handleOpenJPathDialog}
                onClearJPath={(colName) => {
                    const currentTab = tabState.currentTab;
                    if (currentTab) {
                        tabState.clearColumnJPathExpressionForTab(currentTab.tabId, colName);
                        const jpathExprs = tabState.getColumnJPathExpressionsForTab(currentTab.tabId);
                        const newDefs = fileOps.buildColumnDefs(
                            currentTab.header,
                            currentTab.timeField,
                            appliedDisplayTZ,
                            jpathExprs,
                            currentTab.highlightTerms || []
                        );
                        tabState.setColumnDefsForTab(currentTab.tabId, newDefs);
                        if (currentTab.gridApi) {
                            currentTab.gridApi.refreshCells({ force: true });
                        }
                        addLog('info', `Cleared JPath from column '${colName}'`);
                    }
                }}
                hasJPath={hasColumnJPath}
            />

            {/* Message dialog */}
            <MessageDialog
                show={showMessageDialog}
                onClose={() => hideMessageDialog()}
                title={messageDialogTitle}
                message={messageDialogMessage}
                isError={messageDialogIsError}
            />

            {/* Error dialog */}
            <ErrorDialog
                isOpen={showErrorDialog}
                message={errorDialogMessage}
                onClose={() => setShowErrorDialog(false)}
            />

            {/* Directory hash warning dialog */}
            <DirectoryHashWarningDialog
                isOpen={showDirHashWarning}
                directoryPath={dirHashWarningPath}
                onCancel={handleDirHashWarningCancel}
                onContinue={handleDirHashWarningContinue}
            />

            {/* Shortcuts help modal */}
            <ShortcutsDialog show={showShortcuts} onClose={() => setShowShortcuts(false)} />

            {/* Annotation dialog */}
            {showAnnotationDialog && (() => {
                console.log('Rendering AnnotationDialog with props:', {
                    show: showAnnotationDialog,
                    initialNote: annotationNote,
                    initialColor: annotationColor,
                    rowCount: annotationRowIndices.length,
                    isLoading: isAnnotationLoading,
                    annotationRowIndices: annotationRowIndices
                });
                return null;
            })()}
            <AnnotationDialog
                show={showAnnotationDialog}
                initialNote={annotationNote}
                initialColor={annotationColor}
                rowCount={annotationRowIndices.length}
                isLoading={isAnnotationLoading}
                onClose={() => hideAnnotationDialog()}
                onSave={handleSubmitAnnotation}
                onDelete={handleDeleteAnnotation}
            />


            {/* Column JPath dialog for column-level JPath transformations */}
            <ColumnJPathDialog
                show={showColumnJPathDialog}
                columnName={columnJPathTarget.columnName}
                currentExpression={columnJPathTarget.currentExpression}
                previewData={columnJPathTarget.previewData}
                onClose={() => hideColumnJPathDialog()}
                onApply={handleApplyJPath}
                onClear={handleClearJPath}
            />

            {/* Auth PIN Entry Dialog */}
            <AuthPinEntry
                show={showAuthPinDialog}
                onClose={() => hideAuthPinDialog()}
                onSubmit={handlePinSubmit}
                message={authPinMessage}
                isLoading={isAuthPinLoading}
            />

            {/* Sync Status Dialog */}
            <SyncStatus
                show={showSyncStatus}
                onClose={() => setShowSyncStatus(false)}
                onLogin={handleSyncLogin}
                onLogout={handleSyncLogout}
                onSelectWorkspace={handleSelectRemoteWorkspace}
                onWorkspaceDeleted={handleWorkspaceDeleted}
            />

            {/* Fuzzy Finder Dialog */}
            {showFuzzyFinder && (
                <FuzzyFinderDialog
                    show={showFuzzyFinder}
                    onClose={() => setShowFuzzyFinder(false)}
                    files={fuzzyFinderFiles}
                    onSelect={handleFuzzyFinderSelect}
                />
            )}

            {/* File Options Dialog (unified for all file types) */}
            <FileOptionsDialog
                show={showFileOptionsDialog}
                filePath={fileOptionsFilePath}
                fileType={fileOptionsFileType}
                showTimezoneOverride={fileOptionsShowTimezone}
                onConfirm={handleFileOptionsConfirm}
                onCancel={handleFileOptionsCancel}
            />

            {/* Workspace Name Dialog (for creating remote workspaces) */}
            <WorkspaceNameDialog
                show={showWorkspaceNameDialog}
                onClose={() => setShowWorkspaceNameDialog(false)}
                onSubmit={handleCreateRemoteWorkspace}
            />

            {/* Plugin Selection Dialog */}
            <PluginSelectionDialog
                show={pluginHandlers.showPluginSelection}
                plugins={pluginHandlers.pluginSelectionPlugins}
                fileName={pluginHandlers.pluginSelectionFileName}
                onSelect={pluginHandlers.handlePluginSelected}
                onCancel={pluginHandlers.handlePluginSelectionCancel}
            />

            {/* Plugin Warning Dialog */}
            <PluginWarningDialog
                show={pluginHandlers.showPluginWarning}
                pluginName={pluginHandlers.pluginWarningPlugin?.name || ''}
                executablePath={pluginHandlers.pluginWarningPlugin?.executablePath || ''}
                executableHash={pluginHandlers.pluginWarningPlugin?.executableHash || ''}
                onContinue={pluginHandlers.handlePluginWarningContinue}
                onCancel={pluginHandlers.handlePluginWarningCancel}
            />

            {/* Loading overlays */}
            <LoadingOverlay
                show={showExportLoading}
                title="Exporting Timeline"
                message="Processing annotated rows from workspace files..."
            />
            {/* File or Directory Choice Dialog */}
            {showFileOrDirChoice && (
                <div
                    style={{
                        position: 'fixed',
                        top: 0,
                        left: 0,
                        right: 0,
                        bottom: 0,
                        backgroundColor: 'rgba(0,0,0,0.6)',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        zIndex: 10000,
                    }}
                    onClick={() => setShowFileOrDirChoice(false)}
                >
                    <div
                        style={{
                            background: '#1e1e1e',
                            border: '1px solid #444',
                            borderRadius: 8,
                            padding: 24,
                            minWidth: 300,
                            boxShadow: '0 4px 20px rgba(0,0,0,0.5)',
                        }}
                        onClick={(e) => e.stopPropagation()}
                    >
                        <div style={{ fontSize: 16, fontWeight: 500, marginBottom: 16, color: '#fff' }}>
                            {fileOrDirChoiceMode === 'open' ? 'Open File or Directory' : 'Add to Workspace'}
                        </div>
                        <div style={{ fontSize: 13, color: '#999', marginBottom: 20 }}>
                            What would you like to open?
                        </div>
                        <div style={{ display: 'flex', gap: 12, justifyContent: 'center' }}>
                            <button
                                onClick={() => handleChooseFile(fileOrDirChoiceMode)}
                                style={{
                                    padding: '10px 24px',
                                    borderRadius: 6,
                                    border: '1px solid #555',
                                    background: '#333',
                                    color: '#fff',
                                    fontSize: 13,
                                    cursor: 'pointer',
                                    display: 'flex',
                                    alignItems: 'center',
                                    gap: 8,
                                }}
                            >
                                <i className="fa fa-file" /> File
                            </button>
                            <button
                                onClick={() => handleChooseDirectory(fileOrDirChoiceMode)}
                                style={{
                                    padding: '10px 24px',
                                    borderRadius: 6,
                                    border: '1px solid #555',
                                    background: '#333',
                                    color: '#fff',
                                    fontSize: 13,
                                    cursor: 'pointer',
                                    display: 'flex',
                                    alignItems: 'center',
                                    gap: 8,
                                }}
                            >
                                <i className="fa fa-folder" /> Directory
                            </button>
                        </div>
                        <div style={{ textAlign: 'right', marginTop: 16 }}>
                            <button
                                onClick={() => setShowFileOrDirChoice(false)}
                                style={{
                                    padding: '6px 16px',
                                    borderRadius: 4,
                                    border: '1px solid #a33',
                                    background: 'transparent',
                                    color: '#e55',
                                    fontSize: 12,
                                    cursor: 'pointer',
                                }}
                            >
                                Cancel
                            </button>
                        </div>
                    </div>
                </div>
            )}

            <LoadingOverlay
                show={isOpeningFile}
                title="Opening File"
                message="Loading and parsing file contents..."
            />
            <LoadingOverlay
                show={isOpeningWorkspace}
                title="Opening Workspace"
                message="Loading workspace and files..."
            />
            <LoadingOverlay
                show={isChangingTimestamp}
                title="Changing Timestamp Column"
                message="Refreshing data with new timestamp column..."
            />
        </div>
    );
}

export default App;
