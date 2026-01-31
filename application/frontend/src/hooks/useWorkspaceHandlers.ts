import { useCallback } from 'react';
// @ts-ignore - Wails generated bindings
import * as AppAPI from '../../wailsjs/go/app/App';
import { DialogActions } from './useDialogState';

interface UseWorkspaceHandlersParams {
    tabState: {
        tabs: { id: string; filePath?: string }[];
        activeTabId: string | null;
        createTab: (id: string, name: string, fileHash?: string) => void;
        switchTab: (id: string) => void;
        closeTab: (id: string) => void;
        getTabState: (id: string) => any;
    };
    setIsWorkspaceOpen: (open: boolean) => void;
    setWorkspaceKey: (updater: (prev: number) => number) => void;
    dialogActions: DialogActions;
    addLog: (level: 'info' | 'warn' | 'error', message: string) => void;
}

interface UseWorkspaceHandlersReturn {
    handleOpenWorkspace: () => Promise<void>;
    handleCloseWorkspace: () => Promise<void>;
    handleCreateLocalWorkspace: () => Promise<void>;
    handleOpenCreateRemoteWorkspaceDialog: () => void;
    handleCreateRemoteWorkspace: (name: string) => Promise<void>;
}

export function useWorkspaceHandlers({
    tabState,
    setIsWorkspaceOpen,
    setWorkspaceKey,
    dialogActions,
    addLog,
}: UseWorkspaceHandlersParams): UseWorkspaceHandlersReturn {
    const {
        setOpeningWorkspace,
        setCreatingWorkspace,
        setShowWorkspaceNameDialog,
        showMessageDialog,
    } = dialogActions;

    const handleOpenWorkspace = useCallback(async () => {
        setOpeningWorkspace(true);
        try {
            await AppAPI.OpenWorkspace();
            
            // Check if a workspace is actually open (user may have cancelled)
            const isOpen = await AppAPI.IsWorkspaceOpen();
            if (!isOpen) {
                // User cancelled the dialog, don't create a tab
                setOpeningWorkspace(false);
                return;
            }
            
            setIsWorkspaceOpen(true);
            setWorkspaceKey((prev) => prev + 1); // Force Dashboard remount
            addLog('info', 'Workspace file opened successfully');

            // Create dashboard tab as sticky tab at first position
            const dashboardTabId = '__dashboard__';
            const existingDashboard = tabState.tabs.find(t => t.id === dashboardTabId);
            if (!existingDashboard) {
                tabState.createTab(dashboardTabId, 'Dashboard');
            }
            
            // Switch to dashboard tab
            tabState.switchTab(dashboardTabId);

            // Refresh all tab grids to show annotations immediately
            tabState.tabs.forEach((tabInfo) => {
                if (tabInfo.id !== '__dashboard__') {
                    const tab = tabState.getTabState(tabInfo.id);
                    if (tab?.gridApi) {
                        tab.gridApi.refreshInfiniteCache();
                    }
                }
            });
        } catch (e: any) {
            const errorMsg = e?.message || String(e);
            if (errorMsg.includes('requires a valid license')) {
                showMessageDialog('Premium Feature', 'This action requires a valid license. Please import a license file to use this feature.', true);
            } else {
                addLog('error', 'Failed to open workspace: ' + errorMsg);
            }
        } finally {
            setOpeningWorkspace(false);
        }
    }, [tabState, setIsWorkspaceOpen, setWorkspaceKey, setOpeningWorkspace, showMessageDialog, addLog]);

    const handleCloseWorkspace = useCallback(async () => {
        try {
            await AppAPI.CloseWorkspace();
            setIsWorkspaceOpen(false);
            addLog('info', 'Workspace closed');
            
            // Close dashboard tab
            const dashboardTabId = '__dashboard__';
            const existingDashboard = tabState.tabs.find(t => t.id === dashboardTabId);
            if (existingDashboard) {
                tabState.closeTab(dashboardTabId);
            }
            
            // Refresh all tab grids to clear annotations
            tabState.tabs.forEach((tabInfo) => {
                const tab = tabState.getTabState(tabInfo.id);
                if (tab?.gridApi) {
                    tab.gridApi.refreshInfiniteCache();
                }
            });
        } catch (e: any) {
            const errorMsg = e?.message || String(e);
            addLog('error', 'Failed to close workspace: ' + errorMsg);
        }
    }, [tabState, setIsWorkspaceOpen, addLog]);

    const handleCreateLocalWorkspace = useCallback(async () => {
        setOpeningWorkspace(true);
        try {
            await AppAPI.CreateLocalWorkspace();
            
            // Check if a workspace is actually open (user may have cancelled)
            const isOpen = await AppAPI.IsWorkspaceOpen();
            if (!isOpen) {
                // User cancelled the dialog, don't update state
                setOpeningWorkspace(false);
                return;
            }
            
            setIsWorkspaceOpen(true);
            setWorkspaceKey((prev) => prev + 1); // Force Dashboard remount
            addLog('info', 'Local workspace created and opened successfully');

            // Create dashboard tab as sticky tab at first position
            const dashboardTabId = '__dashboard__';
            const existingDashboard = tabState.tabs.find(t => t.id === dashboardTabId);
            if (!existingDashboard) {
                tabState.createTab(dashboardTabId, 'Dashboard');
            }
            
            // Switch to dashboard tab
            tabState.switchTab(dashboardTabId);

            // Refresh all tab grids to show annotations immediately
            tabState.tabs.forEach((tabInfo) => {
                if (tabInfo.id !== '__dashboard__') {
                    const tab = tabState.getTabState(tabInfo.id);
                    if (tab?.gridApi) {
                        tab.gridApi.refreshInfiniteCache();
                    }
                }
            });

            // Show success dialog
            showMessageDialog('Workspace Created', 'Local workspace has been created and opened successfully.', false);
        } catch (e: any) {
            const errorMsg = e?.message || String(e);
            // User cancelled the dialog - do nothing
            if (errorMsg.includes('cancelled') || errorMsg.includes('User cancelled')) {
                setOpeningWorkspace(false);
                return;
            }
            showMessageDialog('Create Workspace Failed', 'Failed to create local workspace: ' + errorMsg, true);
        } finally {
            setOpeningWorkspace(false);
        }
    }, [tabState, setIsWorkspaceOpen, setWorkspaceKey, setOpeningWorkspace, showMessageDialog, addLog]);

    const handleOpenCreateRemoteWorkspaceDialog = useCallback(() => {
        setShowWorkspaceNameDialog(true);
    }, [setShowWorkspaceNameDialog]);

    const handleCreateRemoteWorkspace = useCallback(async (name: string) => {
        // Close the name dialog
        setShowWorkspaceNameDialog(false);
        
        // Set loading state and show loading dialog
        setCreatingWorkspace(true);
        showMessageDialog('Creating Remote Workspace', `Creating and opening remote workspace "${name}"...\n\nThis may take up to 10 seconds as we wait for the workspace to become available.`, false);
        
        try {
            await AppAPI.CreateRemoteWorkspace(name);
            
            // Check if workspace is now open
            const isOpen = await AppAPI.IsWorkspaceOpen();
            if (isOpen) {
                setIsWorkspaceOpen(true);
                setWorkspaceKey((prev) => prev + 1); // Force Dashboard remount
                addLog('info', `Remote workspace "${name}" created and opened successfully`);

                // Create dashboard tab as sticky tab at first position
                const dashboardTabId = '__dashboard__';
                const existingDashboard = tabState.tabs.find(t => t.id === dashboardTabId);
                if (!existingDashboard) {
                    tabState.createTab(dashboardTabId, 'Dashboard');
                }
                
                // Switch to dashboard tab
                tabState.switchTab(dashboardTabId);

                // Refresh all tab grids to show annotations immediately
                tabState.tabs.forEach((tabInfo) => {
                    if (tabInfo.id !== '__dashboard__') {
                        const tab = tabState.getTabState(tabInfo.id);
                        if (tab?.gridApi) {
                            tab.gridApi.refreshInfiniteCache();
                        }
                    }
                });
            }

            // Clear loading state and show success dialog
            setCreatingWorkspace(false);
            showMessageDialog('Remote Workspace Created', `Remote workspace "${name}" has been created and opened successfully.`, false);
        } catch (e: any) {
            // Clear loading state
            setCreatingWorkspace(false);
            
            const errorMsg = e?.message || String(e);
            if (errorMsg.includes('not logged in')) {
                showMessageDialog('Not Signed In', 'You must be signed in to create remote workspaces. Please sign in first.', true);
            } else if (errorMsg.includes('workspace_limit_exceeded')) {
                showMessageDialog('Workspace Limit Exceeded', 'You have reached your workspace limit. Please upgrade your subscription or delete existing workspaces.', true);
            } else {
                showMessageDialog('Create Remote Workspace Failed', 'Failed to create remote workspace: ' + errorMsg, true);
            }
        }
    }, [tabState, setIsWorkspaceOpen, setWorkspaceKey, setCreatingWorkspace, setShowWorkspaceNameDialog, showMessageDialog, addLog]);

    return {
        handleOpenWorkspace,
        handleCloseWorkspace,
        handleCreateLocalWorkspace,
        handleOpenCreateRemoteWorkspaceDialog,
        handleCreateRemoteWorkspace,
    };
}
