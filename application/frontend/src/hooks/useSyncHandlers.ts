import { useCallback } from 'react';
// @ts-ignore - Wails generated bindings
import * as SyncAPI from '../../wailsjs/go/sync/SyncService';
import { DialogActions } from './useDialogState';

interface UseSyncHandlersParams {
    tabState: {
        tabs: { id: string }[];
        createTab: (id: string, name: string) => void;
        switchTab: (id: string) => void;
        closeTab: (id: string) => void;
        getTabState: (id: string) => any;
    };
    setIsWorkspaceOpen: (open: boolean) => void;
    setWorkspaceKey: (updater: (prev: number) => number) => void;
    dialogActions: DialogActions;
    addLog: (level: 'info' | 'warn' | 'error', message: string) => void;
}

interface UseSyncHandlersReturn {
    handlePinSubmit: (pin: string) => Promise<void>;
    handleSyncLogin: () => Promise<void>;
    handleSyncLogout: () => Promise<void>;
    handleSelectRemoteWorkspace: (workspaceId: string, workspaceName: string) => Promise<void>;
}

export function useSyncHandlers({
    tabState,
    setIsWorkspaceOpen,
    setWorkspaceKey,
    dialogActions,
    addLog,
}: UseSyncHandlersParams): UseSyncHandlersReturn {
    const {
        setOpeningWorkspace,
        showAuthPinDialogWithMessage,
        hideAuthPinDialog,
        setAuthPinLoading,
        setAuthPinMessage,
        showMessageDialog,
    } = dialogActions;

    const handlePinSubmit = useCallback(async (pin: string) => {
        setAuthPinLoading(true);
        try {
            const result = await SyncAPI.CompleteLogin(pin);
            
            if (result.success) {
                hideAuthPinDialog();
                
                // Get the user email from license after successful login
                const userEmail = await SyncAPI.GetCurrentUserEmail();
                
                showMessageDialog('Login Successful', `Successfully logged in as ${userEmail}`, false);
                addLog('info', 'Successfully authenticated with sync service');
            } else {
                showMessageDialog('Verification Failed', result.message, true);
                hideAuthPinDialog();
            }
        } catch (e: any) {
            showMessageDialog('Verification Error', 'Failed to verify PIN: ' + (e?.message || String(e)), true);
            hideAuthPinDialog();
            addLog('error', 'PIN verification error: ' + (e?.message || String(e)));
        } finally {
            setAuthPinLoading(false);
        }
    }, [setAuthPinLoading, hideAuthPinDialog, showMessageDialog, addLog]);

    const handleSyncLogin = useCallback(async () => {
        try {
            // Initiate login flow - request PIN
            const result = await SyncAPI.InitiateLogin();
            
            if (!result.success) {
                // Check if this is a rate limit error
                if (result.rate_limited) {
                    // Still show PIN dialog but with rate limit message
                    showAuthPinDialogWithMessage(`${result.message}\n\nYou can still enter a previously received PIN if you have one.`);
                    addLog('warn', 'PIN request rate limited - you can still use a previous PIN');
                    return;
                }
                
                // Other errors - show error dialog
                showMessageDialog('Login Failed', result.message, true);
                return;
            }
            
            // Show PIN entry dialog
            showAuthPinDialogWithMessage(result.message);
            addLog('info', 'PIN sent to your email');
        } catch (e: any) {
            showMessageDialog('Login Error', 'Failed to initiate login: ' + (e?.message || String(e)), true);
            addLog('error', 'Login error: ' + (e?.message || String(e)));
        }
    }, [showAuthPinDialogWithMessage, showMessageDialog, addLog]);

    const handleSyncLogout = useCallback(async () => {
        try {
            // Check if a remote workspace is open before logout
            const WorkspaceAPI = await import('../../wailsjs/go/app/WorkspaceManager');
            const wasRemoteWorkspace = await WorkspaceAPI.IsRemoteWorkspace();
            
            await SyncAPI.Logout();
            addLog('info', 'Logged out from sync service');
            
            // If a remote workspace was open, it was closed by the backend - update UI state
            if (wasRemoteWorkspace) {
                setIsWorkspaceOpen(false);
                setWorkspaceKey((prev) => prev + 1); // Force Dashboard remount
                // Close the dashboard tab since workspace is closed
                if (tabState.tabs.some(t => t.id === '__dashboard__')) {
                    tabState.closeTab('__dashboard__');
                }
                addLog('info', 'Remote workspace closed due to logout');
            }
            
            showMessageDialog('Logged Out', 'You have been logged out from the sync service.', false);
        } catch (e: any) {
            showMessageDialog('Logout Error', 'Failed to logout: ' + (e?.message || String(e)), true);
            addLog('error', 'Logout error: ' + (e?.message || String(e)));
        }
    }, [showMessageDialog, addLog, setIsWorkspaceOpen, setWorkspaceKey, tabState]);

    const handleSelectRemoteWorkspace = useCallback(async (workspaceId: string, workspaceName: string) => {
        setOpeningWorkspace(true);
        try {
            addLog('info', `Opening remote workspace: ${workspaceName}`);
            
            // Call backend to open the remote workspace
            const result = await SyncAPI.OpenRemoteWorkspace(workspaceId);
            
            if (!result.success) {
                showMessageDialog('Error', `Failed to open remote workspace "${workspaceName}": ${result.message}`, true);
                addLog('error', `Failed to open remote workspace: ${result.message}`);
                return;
            }
            
            // Success - show confirmation and update workspace status
            setIsWorkspaceOpen(true);
            setWorkspaceKey((prev) => prev + 1); // Force Dashboard remount
            showMessageDialog('Remote Workspace Opened', `Successfully opened remote workspace: ${workspaceName}`, false);
            addLog('info', `Successfully opened remote workspace: ${workspaceName}`);
            
            // Create dashboard tab if it doesn't exist
            if (!tabState.tabs.some(t => t.id === '__dashboard__')) {
                tabState.createTab('__dashboard__', 'Dashboard');
            }
            
            // Switch to dashboard (frontend only - don't notify backend)
            tabState.switchTab('__dashboard__');

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
            showMessageDialog('Error', `Failed to open remote workspace "${workspaceName}": ${e?.message || String(e)}`, true);
            addLog('error', `Remote workspace error: ${e?.message || String(e)}`);
        } finally {
            setOpeningWorkspace(false);
        }
    }, [tabState, setIsWorkspaceOpen, setWorkspaceKey, setOpeningWorkspace, showMessageDialog, addLog]);

    return {
        handlePinSubmit,
        handleSyncLogin,
        handleSyncLogout,
        handleSelectRemoteWorkspace,
    };
}
