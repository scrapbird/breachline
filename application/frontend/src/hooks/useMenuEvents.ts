import { useEffect, useCallback, useState } from 'react';
import { EventsOn } from "../../wailsjs/runtime";
// @ts-ignore - Wails generated bindings
import * as LicenseAPI from "../../wailsjs/go/app/LicenseService";
// @ts-ignore - Wails generated bindings
import * as AppAPI from '../../wailsjs/go/app/App';
import { FileItem } from '../components/FuzzyFinderDialog';
import { DialogActions, DialogState } from './useDialogState';

interface UseMenuEventsParams {
    // File operations
    handleOpenFile: () => Promise<void>;
    handleOpenFileWithOptions: () => Promise<void>;
    handleOpenDirectory: () => Promise<void>;
    handleAddFileToWorkspace: () => Promise<void>;
    handleAddFileToWorkspaceWithOptions: () => Promise<void>;
    
    // Workspace handlers
    handleOpenWorkspace: () => Promise<void>;
    handleCloseWorkspace: () => Promise<void>;
    handleCreateLocalWorkspace: () => Promise<void>;
    handleOpenCreateRemoteWorkspaceDialog: () => void;
    
    // Clipboard operations
    clipboardOps: {
        copySelectedToClipboard: () => Promise<void>;
    };
    gridContainerRef: React.RefObject<HTMLDivElement | null>;
    
    // Dialog actions
    dialogState: DialogState;
    dialogActions: DialogActions;
    openSettings: () => void;
    
    // Fuzzy finder
    getFuzzyFinderFiles: () => Promise<FileItem[]>;
    setFuzzyFinderFiles: (files: FileItem[]) => void;
    
    // License email setter (for About dialog)
    setLicenseEmail: (email: string | null) => void;
    
    // Sync handlers
    handleSyncLogin: () => Promise<void>;
    
    // Cache indicator
    showCacheIndicator: boolean;
    setShowCacheIndicator: (show: boolean) => void;
    
    // Dependencies for effects
    isWorkspaceOpen: boolean;
    isLicensed: boolean;
    tabState: any;
    
    // Logger
    addLog: (level: 'info' | 'warn' | 'error', message: string) => void;
}

interface UseMenuEventsReturn {
    shouldTriggerLoginAfterDialog: boolean;
    setShouldTriggerLoginAfterDialog: (value: boolean) => void;
}

export function useMenuEvents({
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
}: UseMenuEventsParams): UseMenuEventsReturn {
    const { showConsole, showHistogram } = dialogState;
    const {
        setShowAbout,
        setShowSyntax,
        setShowShortcuts,
        setShowConsole,
        setShowHistogram,
        setShowSyncStatus,
        setExportLoading,
        setShowFuzzyFinder,
        showMessageDialog,
    } = dialogActions;
    
    // State to track if we should trigger login after user closes re-auth dialog
    const [shouldTriggerLoginAfterDialog, setShouldTriggerLoginAfterDialog] = useState(false);

    // File menu: Open file
    useEffect(() => {
        const off = EventsOn("menu:open", () => {
            handleOpenFile();
        });
        return () => { if (typeof off === 'function') off(); };
    }, [handleOpenFile]);

    // File menu: Open file with options
    useEffect(() => {
        const off = EventsOn("menu:openWithOptions", () => {
            handleOpenFileWithOptions();
        });
        return () => { if (typeof off === 'function') off(); };
    }, [handleOpenFileWithOptions]);

    // File menu: Open directory
    useEffect(() => {
        const off = EventsOn("menu:openDirectory", () => {
            handleOpenDirectory();
        });
        return () => { if (typeof off === 'function') off(); };
    }, [handleOpenDirectory]);

    // Edit menu: Copy selected
    useEffect(() => {
        const off = EventsOn("menu:copySelected", async () => {
            const container = gridContainerRef.current;
            const active = document.activeElement as Element | null;
            
            // Only use grid copy when focus is actually inside the grid container.
            // The virtualSelectAllRef only affects WHAT gets copied (all rows vs selection),
            // not WHETHER to use grid copy. This ensures clicking away from the grid
            // allows native copy operations on other elements like console or query input.
            if (container && active && container.contains(active)) {
                await clipboardOps.copySelectedToClipboard();
                return;
            }
            // Native copy fallback
            try {
                if (active && (active instanceof HTMLInputElement || active instanceof HTMLTextAreaElement)) {
                    const el = active as HTMLInputElement | HTMLTextAreaElement;
                    const start = el.selectionStart;
                    const end = el.selectionEnd;
                    if (typeof start === 'number' && typeof end === 'number' && end > start) {
                        const selected = el.value.substring(start, end);
                        if (navigator?.clipboard?.writeText) {
                            await navigator.clipboard.writeText(selected);
                        }
                        return;
                    }
                }
                const sel = window.getSelection?.();
                const txt = sel && !sel.isCollapsed ? sel.toString() : '';
                if (txt && navigator?.clipboard?.writeText) {
                    await navigator.clipboard.writeText(txt);
                }
            } catch {}
        });
        return () => { if (typeof off === 'function') off(); };
    }, [clipboardOps, gridContainerRef]);

    // View menu: Toggle console
    useEffect(() => {
        const off = EventsOn("menu:toggleConsole", () => {
            setShowConsole(!showConsole);
        });
        return () => { if (typeof off === 'function') off(); };
    }, [setShowConsole, showConsole]);

    // View menu: Toggle histogram
    useEffect(() => {
        const off = EventsOn("menu:toggleHistogram", () => {
            setShowHistogram(!showHistogram);
        });
        return () => { if (typeof off === 'function') off(); };
    }, [setShowHistogram, showHistogram]);

    // View menu: Toggle annotations panel
    useEffect(() => {
        const off = EventsOn("menu:toggleAnnotations", () => {
            const activeTabId = tabState.activeTabId;
            if (activeTabId && activeTabId !== '__dashboard__') {
                tabState.toggleAnnotationsPanelForTab(activeTabId);
            }
        });
        return () => { if (typeof off === 'function') off(); };
    }, [tabState]);

    // View menu: Toggle search panel
    useEffect(() => {
        const off = EventsOn("menu:toggleSearch", () => {
            const activeTabId = tabState.activeTabId;
            if (activeTabId && activeTabId !== '__dashboard__') {
                tabState.toggleSearchPanelForTab(activeTabId);
            }
        });
        return () => { if (typeof off === 'function') off(); };
    }, [tabState]);

    // Edit menu: Settings
    useEffect(() => {
        const off = EventsOn("menu:settings", async () => {
            openSettings();
        });
        return () => { if (typeof off === 'function') off(); };
    }, [openSettings]);

    // Help menu: About
    useEffect(() => {
        const off = EventsOn("menu:about", () => {
            setShowAbout(true);
        });
        return () => { if (typeof off === 'function') off(); };
    }, [setShowAbout]);

    // Edit menu: Fuzzy finder
    useEffect(() => {
        const off = EventsOn("menu:fuzzyFinder", async () => {
            const files = await getFuzzyFinderFiles();
            setFuzzyFinderFiles(files);
            setShowFuzzyFinder(true);
        });
        return () => { if (typeof off === 'function') off(); };
    }, [getFuzzyFinderFiles, setFuzzyFinderFiles, setShowFuzzyFinder]);

    // Help menu: Syntax help
    useEffect(() => {
        const off = EventsOn("menu:syntax", () => {
            setShowSyntax(true);
        });
        return () => { if (typeof off === 'function') off(); };
    }, [setShowSyntax]);

    // Help menu: Shortcuts
    useEffect(() => {
        const off = EventsOn("menu:shortcuts", () => {
            setShowShortcuts(true);
        });
        return () => { if (typeof off === 'function') off(); };
    }, [setShowShortcuts]);

    // File menu: Import license
    useEffect(() => {
        const off = EventsOn("menu:importLicense", async () => {
            try {
                const result = await LicenseAPI.ImportLicenseFile();
                
                // User cancelled the file dialog
                if (!result) {
                    return;
                }
                
                if (result.success) {
                    // Show success dialog
                    showMessageDialog('License Registered', `Successfully registered to ${result.email}`, false);
                    addLog('info', 'License successfully validated and saved');
                    
                    // Update license email for About dialog
                    setLicenseEmail(result.email || null);
                } else {
                    // Show error dialog
                    if (result.isExpired) {
                        showMessageDialog('License Expired', 'Your license has expired. Please contact support to renew your license.', true);
                    } else {
                        showMessageDialog('Invalid License', 'The license file is invalid. Please ensure you have the correct license file.', true);
                    }
                    addLog('error', 'License validation failed: ' + result.message);
                }
            } catch (e: any) {
                // Handle unexpected errors (e.g., service not initialized)
                showMessageDialog('Error', 'An unexpected error occurred while opening the license file.', true);
                addLog('error', 'License error: ' + (e?.message || String(e)));
            }
        });
        return () => { if (typeof off === 'function') off(); };
    }, [showMessageDialog, setLicenseEmail, addLog]);

    // Sync menu: Show sync status
    useEffect(() => {
        const off = EventsOn("menu:sync", () => {
            setShowSyncStatus(true);
        });
        return () => { if (typeof off === 'function') off(); };
    }, [setShowSyncStatus]);

    // Listen for sync tokens expired event and automatically trigger re-authentication
    useEffect(() => {
        const off = EventsOn("sync:tokens_expired", async () => {
            addLog('warn', 'Sync tokens expired - automatically prompting for re-authentication');
            
            // Close sync status dialog if it's open
            setShowSyncStatus(false);
            
            // Show re-authentication dialog and set flag to trigger login after user closes it
            showMessageDialog('Re-authentication Required', 'Your sync session has expired. Please sign in again to continue using sync features.', false);
            setShouldTriggerLoginAfterDialog(true);
        });
        return () => { if (typeof off === 'function') off(); };
    }, [setShowSyncStatus, showMessageDialog, addLog]);

    // Workspace menu: Open workspace
    useEffect(() => {
        const off = EventsOn("menu:openWorkspace", async () => {
            await handleOpenWorkspace();
        });
        return () => { if (typeof off === 'function') off(); };
    }, [handleOpenWorkspace]);

    // Workspace menu: Close workspace
    useEffect(() => {
        const off = EventsOn("menu:closeWorkspace", async () => {
            await handleCloseWorkspace();
        });
        return () => { if (typeof off === 'function') off(); };
    }, [handleCloseWorkspace]);

    // Workspace menu: Add file to workspace
    useEffect(() => {
        const off = EventsOn("menu:addFileToWorkspace", async () => {
            await handleAddFileToWorkspace();
        });
        return () => { if (typeof off === 'function') off(); };
    }, [handleAddFileToWorkspace]);

    // Workspace menu: Add file to workspace with options
    useEffect(() => {
        const off = EventsOn("menu:addFileToWorkspaceWithOptions", async () => {
            await handleAddFileToWorkspaceWithOptions();
        });
        return () => { if (typeof off === 'function') off(); };
    }, [handleAddFileToWorkspaceWithOptions]);

    // Workspace menu: Export workspace timeline
    useEffect(() => {
        const off = EventsOn("menu:exportWorkspaceTimeline", async () => {
            try {
                setExportLoading(true);
                await AppAPI.ExportWorkspaceTimeline();
                setExportLoading(false);
                
                // Show success dialog
                showMessageDialog('Export Successful', 'Workspace timeline has been exported successfully.', false);
            } catch (e: any) {
                setExportLoading(false);
                const errorMsg = e?.message || String(e);
                // User cancelled the export - do nothing
                if (errorMsg.includes('export cancelled by user')) {
                    return;
                }
                if (errorMsg.includes('no workspace is open')) {
                    showMessageDialog('No Workspace Open', 'Please open a workspace file first (File → Open workspace) to export timeline.', true);
                } else if (errorMsg.includes('license')) {
                    showMessageDialog('License Required', 'Exporting workspace timeline requires a valid license. Please import a license file to use this feature.', true);
                } else {
                    addLog('error', 'Failed to export workspace timeline: ' + errorMsg);
                }
            }
        });
        return () => { if (typeof off === 'function') off(); };
    }, [setExportLoading, showMessageDialog, addLog]);

    // Workspace menu: Create local workspace
    useEffect(() => {
        const off = EventsOn("menu:createLocalWorkspace", async () => {
            await handleCreateLocalWorkspace();
        });
        return () => { if (typeof off === 'function') off(); };
    }, [handleCreateLocalWorkspace]);

    // Workspace menu: Create remote workspace (show dialog)
    useEffect(() => {
        const off = EventsOn("menu:createRemoteWorkspace", async () => {
            handleOpenCreateRemoteWorkspaceDialog();
        });
        return () => { if (typeof off === 'function') off(); };
    }, [handleOpenCreateRemoteWorkspaceDialog]);

    // View menu: Toggle cache indicator (UI-only, not persisted)
    useEffect(() => {
        const off = EventsOn("menu:toggleCacheIndicator", () => {
            setShowCacheIndicator(!showCacheIndicator);
        });
        return () => { if (typeof off === 'function') off(); };
    }, [showCacheIndicator, setShowCacheIndicator]);

    return {
        shouldTriggerLoginAfterDialog,
        setShouldTriggerLoginAfterDialog,
    };
}
