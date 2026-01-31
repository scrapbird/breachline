import React, { useState, useEffect } from 'react';
import Dialog from './Dialog';
// @ts-ignore - Wails generated bindings
import * as SyncAPI from "../../wailsjs/go/sync/SyncService";

interface RemoteWorkspace {
    WorkspaceID: string;
    Name: string;
    Description?: string;
    CreatedAt: string;
    UpdatedAt: string;
    FileCount?: number;
    OwnerEmail?: string;
    IsShared?: boolean;
    MemberCount?: number;
    Version?: number;
}

interface SyncStatusProps {
    show: boolean;
    onClose: () => void;
    onLogin: () => void;
    onLogout: () => void;
    onSelectWorkspace: (workspaceId: string, workspaceName: string) => void;
    onWorkspaceDeleted?: (workspaceId: string) => void;
}

const SyncStatus: React.FC<SyncStatusProps> = ({ show, onClose, onLogin, onLogout, onSelectWorkspace, onWorkspaceDeleted }) => {
    const [isLoggedIn, setIsLoggedIn] = useState<boolean>(false);
    const [userEmail, setUserEmail] = useState<string>('');
    const [isLoading, setIsLoading] = useState<boolean>(true);
    const [workspaces, setWorkspaces] = useState<RemoteWorkspace[]>([]);
    const [workspacesLoading, setWorkspacesLoading] = useState<boolean>(false);
    const [workspacesError, setWorkspacesError] = useState<string | null>(null);
    const [selectedWorkspaceId, setSelectedWorkspaceId] = useState<string | null>(null);
    const [showErrorDialog, setShowErrorDialog] = useState<boolean>(false);
    const [errorDialogMessage, setErrorDialogMessage] = useState<string>('');
    const [showDeleteConfirmDialog, setShowDeleteConfirmDialog] = useState<boolean>(false);
    const [workspaceToDelete, setWorkspaceToDelete] = useState<RemoteWorkspace | null>(null);
    const [isDeleting, setIsDeleting] = useState<boolean>(false);

    useEffect(() => {
        if (show) {
            checkLoginStatus();
            setSelectedWorkspaceId(null);
            // Clear any previous error when reopening
            setShowErrorDialog(false);
            setErrorDialogMessage('');
        }
    }, [show]);

    const checkLoginStatus = async () => {
        setIsLoading(true);
        try {
            const loggedIn = await SyncAPI.IsLoggedIn();
            setIsLoggedIn(loggedIn);
            
            if (loggedIn) {
                const email = await SyncAPI.GetCurrentUserEmail();
                setUserEmail(email);
                // Fetch workspaces if logged in
                fetchWorkspaces();
            }
        } catch (e) {
            console.error('Failed to check login status:', e);
        } finally {
            setIsLoading(false);
        }
    };

    const fetchWorkspaces = async () => {
        setWorkspacesLoading(true);
        setWorkspacesError(null);
        try {
            const remoteWorkspaces = await SyncAPI.GetRemoteWorkspaces();
            setWorkspaces(remoteWorkspaces || []);
        } catch (e: any) {
            // The error object from Wails may have the actual error in the object itself
            // Convert the entire error to a string to check for session expiration
            const errorMessage = e?.message || 'Failed to fetch remote workspaces';
            const errorString = String(e);
            
            console.log('GetRemoteWorkspaces error message:', errorMessage);
            console.log('GetRemoteWorkspaces error string:', errorString);
            
            // Check if session expired - this happens when token refresh fails
            // Check both the message and the stringified error object
            const isSessionExpired = errorMessage.toLowerCase().includes('session expired') || 
                                    errorMessage.toLowerCase().includes('please log in again') ||
                                    errorMessage.toLowerCase().includes('failed to refresh auth token') ||
                                    errorString.toLowerCase().includes('session expired') ||
                                    errorString.toLowerCase().includes('please log in again') ||
                                    errorString.toLowerCase().includes('failed to refresh auth token');
            
            console.log('Is session expired?', isSessionExpired);
            
            if (isSessionExpired) {
                console.log('Session expired detected - switching to login UI');
                // Update login state to show login UI
                setIsLoggedIn(false);
                setUserEmail('');
                setWorkspaces([]);
                // Show error dialog explaining what happened
                setErrorDialogMessage('Your session has expired. Please log in again to continue.');
                setShowErrorDialog(true);
            } else {
                console.log('Other error - showing error dialog but keeping logged in state');
                // For other errors, show dialog but keep the UI state
                setErrorDialogMessage(errorMessage);
                setShowErrorDialog(true);
            }
        } finally {
            setWorkspacesLoading(false);
        }
    };

    const handleLogin = () => {
        onClose();
        onLogin();
    };

    const handleLogout = () => {
        onClose();
        onLogout();
    };

    const handleOpenWorkspace = () => {
        if (selectedWorkspaceId) {
            // Find the workspace by ID to get its name
            const workspace = workspaces.find((w, idx) => {
                const identifier = w.WorkspaceID || idx.toString();
                return identifier === selectedWorkspaceId;
            });
            
            console.log('Opening workspace:', {
                selectedWorkspaceId,
                workspace,
                allWorkspaces: workspaces
            });
            
            if (workspace) {
                onClose();
                onSelectWorkspace(selectedWorkspaceId, workspace.Name);
            }
        }
    };

    const handleDeleteClick = (e: React.MouseEvent, workspace: RemoteWorkspace) => {
        e.stopPropagation(); // Prevent workspace selection
        setWorkspaceToDelete(workspace);
        setShowDeleteConfirmDialog(true);
    };

    const handleConfirmDelete = async () => {
        if (!workspaceToDelete) return;
        
        setIsDeleting(true);
        try {
            const deletedWorkspaceId = workspaceToDelete.WorkspaceID;
            await SyncAPI.DeleteWorkspace(deletedWorkspaceId);
            setShowDeleteConfirmDialog(false);
            setWorkspaceToDelete(null);
            // Clear selection if deleted workspace was selected
            if (selectedWorkspaceId === deletedWorkspaceId) {
                setSelectedWorkspaceId(null);
            }
            // Notify parent that workspace was deleted (to close if it's the current one)
            if (onWorkspaceDeleted) {
                onWorkspaceDeleted(deletedWorkspaceId);
            }
            // Refresh workspace list
            fetchWorkspaces();
        } catch (e: any) {
            const errorMessage = e?.message || 'Failed to delete workspace';
            setShowDeleteConfirmDialog(false);
            setWorkspaceToDelete(null);
            setErrorDialogMessage(errorMessage);
            setShowErrorDialog(true);
        } finally {
            setIsDeleting(false);
        }
    };

    const handleCancelDelete = () => {
        setShowDeleteConfirmDialog(false);
        setWorkspaceToDelete(null);
    };

    const formatDate = (dateStr: string) => {
        try {
            return new Date(dateStr).toLocaleDateString();
        } catch {
            return dateStr;
        }
    };

    return (
        <>
            <Dialog show={show} onClose={onClose} title="Sync" maxWidth={600}>
                <div style={{ padding: '24px 20px' }}>
                    {isLoading ? (
                        <div style={{ textAlign: 'center', padding: '20px 0' }}>
                            <div style={{ fontSize: 14, opacity: 0.7 }}>Loading...</div>
                        </div>
                    ) : isLoggedIn ? (
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
                            <div style={{ 
                                display: 'flex', 
                                justifyContent: 'space-between', 
                                alignItems: 'center',
                                paddingBottom: 16,
                                borderBottom: '1px solid #444'
                            }}>
                                <div>
                                    <div style={{ fontSize: 12, opacity: 0.7, marginBottom: 4 }}>Signed in as</div>
                                    <div style={{ fontSize: 14, fontWeight: 500 }}>{userEmail}</div>
                                </div>
                                <button
                                    onClick={handleLogout}
                                    style={{
                                        padding: '6px 12px',
                                        fontSize: 13,
                                        fontWeight: 500,
                                        backgroundColor: '#e74c3c',
                                        color: 'white',
                                        border: 'none',
                                        borderRadius: 4,
                                        cursor: 'pointer',
                                        transition: 'background-color 0.2s'
                                    }}
                                    onMouseOver={(e) => e.currentTarget.style.backgroundColor = '#c0392b'}
                                    onMouseOut={(e) => e.currentTarget.style.backgroundColor = '#e74c3c'}
                                >
                                    Logout
                                </button>
                            </div>

                            <div>
                                <h3 style={{ 
                                    margin: '0 0 12px 0', 
                                    fontSize: 14, 
                                    fontWeight: 600,
                                    color: '#fff'
                                }}>
                                    Remote Workspaces
                                </h3>

                                {workspacesLoading && (
                                    <div style={{ textAlign: 'center', padding: '20px 0', color: '#888' }}>
                                        Loading workspaces...
                                    </div>
                                )}

                                {!workspacesLoading && workspaces.length === 0 && (
                                    <div style={{
                                        textAlign: 'center',
                                        padding: '20px 0',
                                        color: '#888',
                                        fontSize: 13
                                    }}>
                                        No remote workspaces found.
                                    </div>
                                )}

                                {!workspacesLoading && workspaces.length > 0 && (
                                <>
                                    <div style={{
                                        maxHeight: '250px',
                                        overflowY: 'auto',
                                        marginBottom: '16px'
                                    }}>
                                        {workspaces.map((workspace, index) => {
                                            const workspaceKey = workspace.WorkspaceID || `workspace-${index}`;
                                            const workspaceIdentifier = workspace.WorkspaceID || index.toString();
                                            
                                            return (
                                                <div
                                                    key={workspaceKey}
                                                    onClick={() => setSelectedWorkspaceId(workspaceIdentifier)}
                                                    style={{
                                                        padding: '10px',
                                                        border: `1px solid ${selectedWorkspaceId === workspaceIdentifier ? '#4a9eff' : '#444'}`,
                                                        borderRadius: 4,
                                                        marginBottom: '8px',
                                                        cursor: 'pointer',
                                                        backgroundColor: selectedWorkspaceId === workspaceIdentifier ? '#1a3a5a' : '#333',
                                                        transition: 'all 0.2s ease'
                                                    }}
                                                >
                                                    <div style={{
                                                        display: 'flex',
                                                        justifyContent: 'space-between',
                                                        alignItems: 'flex-start',
                                                        marginBottom: '6px'
                                                    }}>
                                                        <div style={{
                                                            fontSize: 13,
                                                            fontWeight: 600,
                                                            color: '#fff',
                                                            display: 'flex',
                                                            alignItems: 'center',
                                                            gap: '6px'
                                                        }}>
                                                            {selectedWorkspaceId === workspaceIdentifier && (
                                                                <span style={{ color: '#4a9eff', fontSize: 11 }}>✓</span>
                                                            )}
                                                            {workspace.Name}
                                                        </div>
                                                        <span style={{
                                                            fontSize: 11,
                                                            color: '#888',
                                                            backgroundColor: '#444',
                                                            padding: '2px 6px',
                                                            borderRadius: 3
                                                        }}>
                                                            {workspace.FileCount || 0} files
                                                        </span>
                                                    </div>
                                                    {workspace.Description && (
                                                        <p style={{
                                                            margin: '0 0 6px 0',
                                                            color: '#ccc',
                                                            fontSize: 12,
                                                            lineHeight: '1.4'
                                                        }}>
                                                            {workspace.Description}
                                                        </p>
                                                    )}
                                                    <div style={{
                                                        display: 'flex',
                                                        justifyContent: 'space-between',
                                                        alignItems: 'center'
                                                    }}>
                                                        <div style={{
                                                            fontSize: 11,
                                                            color: '#888'
                                                        }}>
                                                            Created: {formatDate(workspace.CreatedAt)}
                                                            {workspace.UpdatedAt !== workspace.CreatedAt && (
                                                                <span> • Updated: {formatDate(workspace.UpdatedAt)}</span>
                                                            )}
                                                        </div>
                                                        <button
                                                            onClick={(e) => handleDeleteClick(e, workspace)}
                                                            style={{
                                                                padding: '4px 8px',
                                                                fontSize: 11,
                                                                backgroundColor: 'transparent',
                                                                color: '#888',
                                                                border: '1px solid #555',
                                                                borderRadius: 4,
                                                                cursor: 'pointer',
                                                                transition: 'all 0.2s'
                                                            }}
                                                            onMouseOver={(e) => {
                                                                e.currentTarget.style.backgroundColor = '#e74c3c';
                                                                e.currentTarget.style.color = 'white';
                                                                e.currentTarget.style.borderColor = '#e74c3c';
                                                            }}
                                                            onMouseOut={(e) => {
                                                                e.currentTarget.style.backgroundColor = 'transparent';
                                                                e.currentTarget.style.color = '#888';
                                                                e.currentTarget.style.borderColor = '#555';
                                                            }}
                                                        >
                                                            Delete
                                                        </button>
                                                    </div>
                                                </div>
                                            );
                                        })}
                                    </div>

                                    <button
                                        onClick={handleOpenWorkspace}
                                        disabled={!selectedWorkspaceId}
                                        style={{
                                            width: '100%',
                                            padding: '10px 20px',
                                            fontSize: 14,
                                            fontWeight: 500,
                                            backgroundColor: selectedWorkspaceId ? '#3498db' : '#555',
                                            color: selectedWorkspaceId ? 'white' : '#888',
                                            border: 'none',
                                            borderRadius: 6,
                                            cursor: selectedWorkspaceId ? 'pointer' : 'not-allowed',
                                            transition: 'background-color 0.2s'
                                        }}
                                        onMouseOver={(e) => {
                                            if (selectedWorkspaceId) {
                                                e.currentTarget.style.backgroundColor = '#2980b9';
                                            }
                                        }}
                                        onMouseOut={(e) => {
                                            if (selectedWorkspaceId) {
                                                e.currentTarget.style.backgroundColor = '#3498db';
                                            }
                                        }}
                                    >
                                        Open Workspace
                                    </button>
                                </>
                            )}
                        </div>
                    </div>
                ) : (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
                        <div style={{ textAlign: 'center' }}>
                            <div style={{ fontSize: 14, opacity: 0.7 }}>Not signed in</div>
                        </div>
                        <button
                            onClick={handleLogin}
                            style={{
                                padding: '10px 20px',
                                fontSize: 14,
                                fontWeight: 500,
                                backgroundColor: '#3498db',
                                color: 'white',
                                border: 'none',
                                borderRadius: 6,
                                cursor: 'pointer',
                                transition: 'background-color 0.2s'
                            }}
                            onMouseOver={(e) => e.currentTarget.style.backgroundColor = '#2980b9'}
                            onMouseOut={(e) => e.currentTarget.style.backgroundColor = '#3498db'}
                        >
                            Login
                        </button>
                    </div>
                )}
            </div>
        </Dialog>

        {/* Error Dialog */}
        <Dialog 
            show={showErrorDialog} 
            onClose={() => setShowErrorDialog(false)} 
            title="Error" 
            maxWidth={500}
        >
            <div style={{ padding: '24px 20px' }}>
                <div style={{
                    padding: '16px',
                    backgroundColor: '#4a1a1a',
                    border: '1px solid #8a2a2a',
                    borderRadius: 6,
                    color: '#ff6b6b',
                    fontSize: 14,
                    lineHeight: '1.5',
                    marginBottom: '20px'
                }}>
                    {errorDialogMessage}
                </div>
                <button
                    onClick={() => setShowErrorDialog(false)}
                    style={{
                        width: '100%',
                        padding: '10px 20px',
                        fontSize: 14,
                        fontWeight: 500,
                        backgroundColor: '#3498db',
                        color: 'white',
                        border: 'none',
                        borderRadius: 6,
                        cursor: 'pointer',
                        transition: 'background-color 0.2s'
                    }}
                    onMouseOver={(e) => e.currentTarget.style.backgroundColor = '#2980b9'}
                    onMouseOut={(e) => e.currentTarget.style.backgroundColor = '#3498db'}
                >
                    OK
                </button>
            </div>
        </Dialog>

        {/* Delete Confirmation Dialog */}
        <Dialog 
            show={showDeleteConfirmDialog} 
            onClose={handleCancelDelete} 
            title="Delete Workspace" 
            maxWidth={500}
        >
            <div style={{ padding: '24px 20px' }}>
                <div style={{
                    padding: '16px',
                    backgroundColor: '#4a1a1a',
                    border: '1px solid #8a2a2a',
                    borderRadius: 6,
                    color: '#ff6b6b',
                    fontSize: 14,
                    lineHeight: '1.6',
                    marginBottom: '20px'
                }}>
                    <div style={{ fontWeight: 600, marginBottom: 8 }}>⚠️ Warning: This action cannot be undone!</div>
                    <p style={{ margin: '0 0 12px 0' }}>
                        Are you sure you want to delete the workspace <strong>"{workspaceToDelete?.Name}"</strong>?
                    </p>
                    <p style={{ margin: 0, color: '#ffaaaa' }}>
                        All files and annotations in this workspace will be permanently deleted.
                    </p>
                </div>
                <div style={{ display: 'flex', gap: 12 }}>
                    <button
                        onClick={handleCancelDelete}
                        disabled={isDeleting}
                        style={{
                            flex: 1,
                            padding: '10px 20px',
                            fontSize: 14,
                            fontWeight: 500,
                            backgroundColor: '#333',
                            color: '#eee',
                            border: '1px solid #444',
                            borderRadius: 6,
                            cursor: isDeleting ? 'not-allowed' : 'pointer',
                            opacity: isDeleting ? 0.6 : 1,
                            transition: 'background-color 0.2s'
                        }}
                        onMouseOver={(e) => !isDeleting && (e.currentTarget.style.backgroundColor = '#444')}
                        onMouseOut={(e) => !isDeleting && (e.currentTarget.style.backgroundColor = '#333')}
                    >
                        Cancel
                    </button>
                    <button
                        onClick={handleConfirmDelete}
                        disabled={isDeleting}
                        style={{
                            flex: 1,
                            padding: '10px 20px',
                            fontSize: 14,
                            fontWeight: 500,
                            backgroundColor: isDeleting ? '#a33' : '#e74c3c',
                            color: 'white',
                            border: 'none',
                            borderRadius: 6,
                            cursor: isDeleting ? 'not-allowed' : 'pointer',
                            transition: 'background-color 0.2s'
                        }}
                        onMouseOver={(e) => !isDeleting && (e.currentTarget.style.backgroundColor = '#c0392b')}
                        onMouseOut={(e) => !isDeleting && (e.currentTarget.style.backgroundColor = '#e74c3c')}
                    >
                        {isDeleting ? 'Deleting...' : 'Delete Workspace'}
                    </button>
                </div>
            </div>
        </Dialog>
    </>
    );
};

export default SyncStatus;
