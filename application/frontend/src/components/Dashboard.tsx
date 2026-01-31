import React, { useState, useEffect, useRef } from 'react';
// @ts-ignore - Wails generated bindings
import * as WorkspaceAPI from "../../wailsjs/go/app/App";
import { EventsOn } from "../../wailsjs/runtime";
import WorkspaceFileEditDialog from './WorkspaceFileEditDialog';
import LocateFilesDialog from './LocateFilesDialog';
import ErrorDialog from './ErrorDialog';
import { FileOptions, createDefaultFileOptions } from '../types/FileOptions';
import './Dashboard.css';
import './LocateFilesDialog.css';

interface WorkspaceFile {
  filePath: string;
  relativePath?: string;
  fileHash: string;
  annotations: number;
  options?: FileOptions;
  description?: string;
}

interface DashboardProps {
  onFileOpen?: (filePath: string, fileOptions?: FileOptions, fileHash?: string) => void;
}

// Helper function to strip compression extension and get the inner file extension
const getInnerExtension = (filePath: string): string => {
  const lowerPath = filePath.toLowerCase();
  const compressionExtensions = ['.gz', '.bz2', '.xz'];

  // Strip compression extension if present
  let pathWithoutCompression = lowerPath;
  for (const compExt of compressionExtensions) {
    if (lowerPath.endsWith(compExt)) {
      pathWithoutCompression = lowerPath.slice(0, -compExt.length);
      break;
    }
  }

  // Get the extension from the remaining path
  return pathWithoutCompression.split('.').pop() || '';
};

// Helper function to get inner file type from pattern (e.g., "*.json.gz" -> "json")
const getFileTypeFromPattern = (pattern: string): string => {
  if (!pattern) return '';
  // Remove glob characters and get extension
  const cleaned = pattern.replace(/^\*\.?/, '').replace(/\*/g, '');
  // Handle compressed files like "json.gz"
  const parts = cleaned.split('.');
  for (const part of parts) {
    if (['csv', 'json', 'xlsx', 'xls'].includes(part.toLowerCase())) {
      return part.toLowerCase();
    }
  }
  return parts[0]?.toLowerCase() || '';
};

// Get icon color based on file type
const getIconColor = (fileType: string): string => {
  switch (fileType) {
    case 'csv': return '#4caf50';
    case 'xlsx': case 'xls': return '#217346';
    case 'json': return '#f0ad4e';
    default: return '#888';
  }
};

// Get the inner icon element for a file type
const getInnerFileIcon = (fileType: string, color: string): React.ReactNode => {
  switch (fileType) {
    case 'csv':
      return <i className="fa-solid fa-file-csv" style={{ color }} />;
    case 'xlsx':
    case 'xls':
      return <i className="fa-solid fa-file-excel" style={{ color }} />;
    case 'json':
      return <i className="fa-solid fa-file-code" style={{ color }} />;
    default:
      // Always show a generic file icon as fallback
      return <i className="fa-solid fa-file" style={{ color: '#888' }} />;
  }
};

// Directory icon component - plain folder icon
const DirectoryIcon: React.FC<{ size?: string }> = ({ size = '1.2em' }) => {
  return <i className="fa-solid fa-folder" style={{ color: '#e8a838', fontSize: size }} />;
};

// Helper function to get file icon based on extension and options
const getFileIcon = (filePath: string, fileOptions?: FileOptions): React.ReactNode => {
  // Check if it's a directory
  if (fileOptions?.isDirectory) {
    return <DirectoryIcon />;
  }

  const extension = getInnerExtension(filePath);

  switch (extension) {
    case 'csv':
      return <i className="fa-solid fa-file-csv" style={{ color: '#4caf50' }} />;
    case 'xlsx':
    case 'xls':
      return <i className="fa-solid fa-file-excel" style={{ color: '#217346' }} />;
    case 'json':
      return <i className="fa-solid fa-file-code" style={{ color: '#f0ad4e' }} />;
    default:
      return <i className="fa-solid fa-file" style={{ color: '#888' }} />;
  }
};

const Dashboard: React.FC<DashboardProps> = ({ onFileOpen }) => {
  const [files, setFiles] = useState<WorkspaceFile[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string>('');
  const [workspacePath, setWorkspacePath] = useState<string>('');
  const [workspaceName, setWorkspaceName] = useState<string>('');
  const [isRemoteWorkspace, setIsRemoteWorkspace] = useState<boolean>(false);

  // Track previous data to detect actual changes
  const previousDataRef = useRef<{
    files: WorkspaceFile[];
    path: string;
    name: string;
    isRemote: boolean;
  } | null>(null);

  // Edit dialog state
  const [showEditDialog, setShowEditDialog] = useState<boolean>(false);
  const [editingFile, setEditingFile] = useState<WorkspaceFile | null>(null);

  // Delete confirmation state
  const [showDeleteConfirm, setShowDeleteConfirm] = useState<boolean>(false);
  const [deletingFile, setDeletingFile] = useState<WorkspaceFile | null>(null);

  // Locate files dialog state
  const [showLocateFilesDialog, setShowLocateFilesDialog] = useState<boolean>(false);

  // Action error dialog state (for errors that shouldn't replace the dashboard)
  const [actionError, setActionError] = useState<string>('');

  // Sort files consistently to prevent random reordering on sync events
  const sortFiles = (fileList: WorkspaceFile[]): WorkspaceFile[] => {
    return [...fileList].sort((a, b) => {
      // Sort by filePath first
      const pathCompare = a.filePath.localeCompare(b.filePath);
      if (pathCompare !== 0) return pathCompare;

      // Then by jpath
      const jpathA = a.options?.jpath || '';
      const jpathB = b.options?.jpath || '';
      const jpathCompare = jpathA.localeCompare(jpathB);
      if (jpathCompare !== 0) return jpathCompare;

      // Then by noHeaderRow
      const noHeaderA = a.options?.noHeaderRow ? 1 : 0;
      const noHeaderB = b.options?.noHeaderRow ? 1 : 0;
      const noHeaderCompare = noHeaderA - noHeaderB;
      if (noHeaderCompare !== 0) return noHeaderCompare;

      // Finally by ingestTimezoneOverride
      const tzA = a.options?.ingestTimezoneOverride || '';
      const tzB = b.options?.ingestTimezoneOverride || '';
      return tzA.localeCompare(tzB);
    });
  };
  const [showActionErrorDialog, setShowActionErrorDialog] = useState<boolean>(false);

  // Helper to compare workspace data for changes
  const hasWorkspaceDataChanged = (
    newFiles: WorkspaceFile[],
    newPath: string,
    newName: string,
    newIsRemote: boolean
  ): boolean => {
    const prev = previousDataRef.current;
    if (!prev) return true; // First load always considered changed

    // Check simple fields
    if (prev.path !== newPath || prev.name !== newName || prev.isRemote !== newIsRemote) {
      return true;
    }

    // Check files array
    if (prev.files.length !== newFiles.length) {
      return true;
    }

    // Sort new files for consistent comparison (prev.files is already sorted)
    const sortedNewFiles = sortFiles(newFiles);

    // Deep compare files
    for (let i = 0; i < sortedNewFiles.length; i++) {
      const prevFile = prev.files[i];
      const newFile = sortedNewFiles[i];

      if (
        prevFile.filePath !== newFile.filePath ||
        prevFile.fileHash !== newFile.fileHash ||
        prevFile.annotations !== newFile.annotations ||
        prevFile.options?.jpath !== newFile.options?.jpath ||
        prevFile.description !== newFile.description ||
        prevFile.options?.noHeaderRow !== newFile.options?.noHeaderRow ||
        prevFile.options?.ingestTimezoneOverride !== newFile.options?.ingestTimezoneOverride ||
        prevFile.options?.isDirectory !== newFile.options?.isDirectory ||
        prevFile.options?.filePattern !== newFile.options?.filePattern ||
        prevFile.options?.includeSourceColumn !== newFile.options?.includeSourceColumn ||
        prevFile.relativePath !== newFile.relativePath
      ) {
        return true;
      }
    }

    return false; // No changes detected
  };

  const loadWorkspaceFiles = async (silentRefresh = false) => {
    // Only show loading state on initial load, not during background refreshes
    if (!silentRefresh) {
      setLoading(true);
    }
    setError('');
    try {
      const workspaceFiles = await WorkspaceAPI.GetWorkspaceFiles();
      const path = await WorkspaceAPI.GetWorkspacePath();
      const name = await WorkspaceAPI.GetWorkspaceName();
      const isRemote = await WorkspaceAPI.IsRemoteWorkspace();

      // Check if data actually changed
      const hasChanged = hasWorkspaceDataChanged(
        workspaceFiles || [],
        path || '',
        name || '',
        isRemote || false
      );

      // Only update state if something changed
      if (hasChanged) {
        setFiles(sortFiles(workspaceFiles || []));
        setWorkspacePath(path || '');
        setWorkspaceName(name || '');
        setIsRemoteWorkspace(isRemote || false);

        // Update previous data reference (use sorted files for consistent comparison)
        previousDataRef.current = {
          files: sortFiles(workspaceFiles || []),
          path: path || '',
          name: name || '',
          isRemote: isRemote || false,
        };
      }
    } catch (err: any) {
      // Wails returns errors as strings, not objects with message property
      const errorMessage = typeof err === 'string' ? err : (err?.message || 'Failed to load workspace files');
      setError(errorMessage);
      setFiles([]);
      previousDataRef.current = null; // Reset on error
    } finally {
      if (!silentRefresh) {
        setLoading(false);
      }
    }
  };

  useEffect(() => {
    // Initial load
    loadWorkspaceFiles(false);

    // Listen for workspace updates
    const unsubscribe = EventsOn("workspace:updated", () => {
      // Don't reload workspace files while the locate files dialog is open
      // to prevent resetting the dialog's internal state
      if (!showLocateFilesDialog) {
        // Use silent refresh to avoid loading flicker
        loadWorkspaceFiles(true);
      }
    });

    return () => {
      if (unsubscribe) unsubscribe();
    };
  }, [showLocateFilesDialog]);

  const handleFileClick = (file: WorkspaceFile) => {
    // Check if this is a remote workspace and the file hasn't been located yet
    if (isRemoteWorkspace && (!file.relativePath || file.relativePath.trim() === '')) {
      setActionError(
        'This file has not been located on your local machine yet. ' +
        'Please use the "Locate Files" button to find local copies of your workspace files before opening them.'
      );
      setShowActionErrorDialog(true);
      return;
    }

    if (onFileOpen) {
      onFileOpen(file.filePath, file.options || createDefaultFileOptions(), file.fileHash);
    }
  };

  const getWorkspaceDisplayName = () => {
    const displayName = workspaceName || 'Workspace';
    return isRemoteWorkspace ? (
      <>
        <i className="fa-solid fa-cloud" style={{ marginRight: '18px' }} />
        {displayName}
      </>
    ) : displayName;
  };

  const handleEditDescription = (file: WorkspaceFile, e: React.MouseEvent) => {
    e.stopPropagation(); // Prevent file from opening
    setEditingFile(file);
    setShowEditDialog(true);
  };

  const handleSaveDescription = async (description: string) => {
    if (!editingFile) return;

    try {
      await WorkspaceAPI.UpdateFileDescription(
        editingFile.fileHash,
        editingFile.options || {},
        description
      );
      setShowEditDialog(false);
      setEditingFile(null);
      // Files will reload automatically via workspace:updated event
    } catch (err: any) {
      console.error('Failed to update description:', err);
      // Wails returns errors as strings, not objects with message property
      const errorMessage = typeof err === 'string' ? err : (err?.message || 'Failed to update description');
      setActionError(errorMessage);
      setShowActionErrorDialog(true);
    }
  };

  const handleDeleteClick = (file: WorkspaceFile, e: React.MouseEvent) => {
    e.stopPropagation(); // Prevent file from opening
    setDeletingFile(file);
    setShowDeleteConfirm(true);
  };

  const handleConfirmDelete = async () => {
    if (!deletingFile) return;

    try {
      await WorkspaceAPI.RemoveFileFromWorkspaceByHash(
        deletingFile.fileHash,
        deletingFile.options || {}
      );
      setShowDeleteConfirm(false);
      setDeletingFile(null);
      // Files will reload automatically via workspace:updated event
    } catch (err: any) {
      console.error('Failed to delete file:', err);
      // Wails returns errors as strings, not objects with message property
      const errorMessage = typeof err === 'string' ? err : (err?.message || 'Failed to delete file from workspace');
      setActionError(errorMessage);
      setShowActionErrorDialog(true);
      setShowDeleteConfirm(false);
      setDeletingFile(null);
    }
  };

  const handleCancelDelete = () => {
    setShowDeleteConfirm(false);
    setDeletingFile(null);
  };

  const handleLocateFilesSuccess = async (matchedCount: number) => {
    // Show success message or notification
    if (matchedCount > 0) {
      console.log(`Successfully located and stored locations for ${matchedCount} files`);

      // Refresh file locations from sync API to pick up newly stored paths
      try {
        await WorkspaceAPI.RefreshFileLocations();
      } catch (err: any) {
        console.warn('Failed to refresh file locations:', err);
        // Continue anyway - the loadWorkspaceFiles() call below will still help
      }

      // You could show a toast notification here in the future
    } else {
      console.log('No matching files found in the selected location(s)');
    }
    // Reload workspace files to show any updates (not silent since user action)
    loadWorkspaceFiles(false);
  };

  if (loading) {
    return (
      <div className="dashboard">
        <div className="dashboard-header">
          <h2>Workspace Dashboard</h2>
        </div>
        <div className="dashboard-content">
          <div className="dashboard-loading">Loading workspace files...</div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="dashboard">
        <div className="dashboard-header">
          <h2>Workspace Dashboard</h2>
        </div>
        <div className="dashboard-content">
          <div className="dashboard-error">{error}</div>
        </div>
      </div>
    );
  }

  return (
    <div className="dashboard">
      <div className="dashboard-header">
        <h2>{getWorkspaceDisplayName()}</h2>
        {isRemoteWorkspace && (
          <button
            className="locate-files-btn"
            onClick={() => setShowLocateFilesDialog(true)}
            title="Locate local copies of workspace files"
          >
            <i className="fa-solid fa-folder-open" style={{ marginRight: '6px' }} />
            Locate Files
          </button>
        )}
      </div>

      <div className="dashboard-content">
        {files.length === 0 ? (
          <div className="dashboard-empty">
            <p>No files in workspace yet.</p>
            <p className="dashboard-hint">
              Add files to track them in this workspace.
            </p>
          </div>
        ) : (
          <>
            <div className="dashboard-files">
              <div className="file-list">
                {files.map((file, index) => (
                  <div
                    key={index}
                    className="file-item"
                    onClick={() => handleFileClick(file)}
                    title={file.filePath}
                  >
                    <div className="file-left-column">
                      <div className="file-icon">{getFileIcon(file.filePath, file.options)}</div>
                      <div className="file-info">
                        {!isRemoteWorkspace || (file.relativePath && file.relativePath.trim() !== '') ? (
                          <div className="file-name">{file.relativePath || file.filePath}</div>
                        ) : (
                          <div className="file-name-not-located">Locate file on this machine to open</div>
                        )}
                        {(file.options?.jpath || file.options?.noHeaderRow || file.options?.ingestTimezoneOverride || file.options?.filePattern || file.options?.includeSourceColumn || file.options?.pluginName || file.annotations > 0) && (
                          <div className="file-metadata">
                            {file.annotations > 0 && (
                              <span className="file-annotations">
                                {file.annotations} Annotation{file.annotations !== 1 ? 's' : ''}
                              </span>
                            )}
                            {file.options?.pluginName && (
                              <span className="file-plugin" title={`Loaded with plugin: ${file.options.pluginName}`}>
                                {file.options.pluginName}
                              </span>
                            )}
                            {file.options?.noHeaderRow && (
                              <span className="file-no-header" title="First row is treated as data, not headers">
                                No Header
                              </span>
                            )}
                            {file.options?.ingestTimezoneOverride && (
                              <span className="file-timezone-override" title={`Ingest timezone override: ${file.options.ingestTimezoneOverride}`}>
                                {file.options.ingestTimezoneOverride}
                              </span>
                            )}
                            {file.options?.jpath && (
                              <span className="file-jpath" title="JSONPath expression">
                                {file.options.jpath}
                              </span>
                            )}
                            {file.options?.isDirectory && file.options?.filePattern && (
                              <span className="file-pattern" title={`File pattern: ${file.options.filePattern}`}>
                                {file.options.filePattern}
                              </span>
                            )}
                            {file.options?.isDirectory && file.options?.includeSourceColumn && (
                              <span className="file-source-column" title="Source file path included as a column">
                                Source Path
                              </span>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                    <div className="file-right-column">
                      {file.description && (
                        <div className="file-description" title={file.description}>{file.description}</div>
                      )}
                    </div>
                    <div className="file-actions">
                      <button
                        className="file-action-btn edit-btn"
                        onClick={(e) => handleEditDescription(file, e)}
                        title="Edit description"
                      >
                        <i className="fa-solid fa-pen" />
                      </button>
                      <button
                        className="file-action-btn delete-btn"
                        onClick={(e) => handleDeleteClick(file, e)}
                        title="Remove from workspace"
                      >
                        <i className="fa-solid fa-trash" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </>
        )}
      </div>

      {/* Edit Description Dialog */}
      {editingFile && (
        <WorkspaceFileEditDialog
          isOpen={showEditDialog}
          filePath={editingFile.filePath}
          jpath={editingFile.options?.jpath}
          currentDescription={editingFile.description || ''}
          onClose={() => {
            setShowEditDialog(false);
            setEditingFile(null);
          }}
          onSave={handleSaveDescription}
        />
      )}

      {/* Delete Confirmation Dialog */}
      {showDeleteConfirm && deletingFile && (
        <div className="modal-overlay" onClick={handleCancelDelete}>
          <div
            className="modal-content"
            onClick={(e) => e.stopPropagation()}
            style={{ maxWidth: '500px' }}
          >
            <div className="modal-header">
              <h2>Confirm Deletion</h2>
              <button onClick={handleCancelDelete} className="close-button">
                <i className="fa-solid fa-xmark" />
              </button>
            </div>
            <div className="modal-body" style={{ textAlign: 'left' }}>
              <p style={{ marginBottom: '16px', fontSize: '14px', lineHeight: '1.5' }}>
                Are you sure you want to remove this file from the workspace?
              </p>
              <div style={{
                padding: '12px',
                background: '#2a2a2a',
                borderRadius: '4px',
                marginBottom: '20px',
                wordBreak: 'break-all',
              }}>
                <div style={{ fontSize: '13px', color: '#bbb', marginBottom: '4px' }}>File:</div>
                <div style={{ fontSize: '14px', fontWeight: '500' }}>{deletingFile.filePath}</div>
                {deletingFile.options?.jpath && (
                  <>
                    <div style={{ fontSize: '13px', color: '#bbb', marginTop: '8px', marginBottom: '4px' }}>JSONPath:</div>
                    <div style={{ fontSize: '12px', fontFamily: 'monospace', color: '#9b9b9b' }}>{deletingFile.options.jpath}</div>
                  </>
                )}
              </div>
              <p style={{ fontSize: '13px', color: '#ff6b6b', marginBottom: '20px' }}>
                <i className="fa-solid fa-triangle-exclamation" style={{ marginRight: '6px' }} />
                This will remove the file and all its annotations from the workspace.
              </p>
              <div style={{ display: 'flex', gap: '12px', justifyContent: 'flex-end' }}>
                <button
                  onClick={handleCancelDelete}
                  style={{
                    padding: '8px 20px',
                    fontSize: '14px',
                    background: '#3a3a3a',
                    border: '1px solid #555',
                    borderRadius: '4px',
                    color: '#ddd',
                    cursor: 'pointer',
                    fontWeight: '500',
                  }}
                >
                  Cancel
                </button>
                <button
                  onClick={handleConfirmDelete}
                  style={{
                    padding: '8px 20px',
                    fontSize: '14px',
                    background: '#ff4444',
                    border: 'none',
                    borderRadius: '4px',
                    color: '#fff',
                    cursor: 'pointer',
                    fontWeight: '500',
                  }}
                >
                  Delete
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Locate Files Dialog */}
      <LocateFilesDialog
        isOpen={showLocateFilesDialog}
        onClose={() => setShowLocateFilesDialog(false)}
        onSuccess={handleLocateFilesSuccess}
      />

      {/* Action Error Dialog */}
      <ErrorDialog
        isOpen={showActionErrorDialog}
        message={actionError}
        onClose={() => setShowActionErrorDialog(false)}
      />
    </div>
  );
};

export default Dashboard;
