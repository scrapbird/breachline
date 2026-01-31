import React, { useState, useEffect, useRef } from 'react';
// @ts-ignore - Wails generated bindings
import * as AppAPI from "../../wailsjs/go/app/App";
// @ts-ignore - Wails generated bindings
import * as SyncAPI from "../../wailsjs/go/sync/SyncService";
import { EventsOn } from "../../wailsjs/runtime";
import './LocateFilesDialog.css';

interface LocateFilesDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (matchedCount: number) => void;
}

interface FileSelection {
  path: string;
  name: string;
  isDirectory: boolean;
}

const LocateFilesDialog: React.FC<LocateFilesDialogProps> = ({ isOpen, onClose, onSuccess }) => {
  const [selectedFiles, setSelectedFiles] = useState<FileSelection[]>([]);
  const [isProcessing, setIsProcessing] = useState<boolean>(false);
  const [isCancelling, setIsCancelling] = useState<boolean>(false);
  const [scanCompleted, setScanCompleted] = useState<boolean>(false);
  const [successCount, setSuccessCount] = useState<number>(0);
  const [processingStatus, setProcessingStatus] = useState<string>('');
  const [error, setError] = useState<string>('');
  const [progressLog, setProgressLog] = useState<string[]>([]);
  const [filesScanned, setFilesScanned] = useState<number>(0);
  const [matchCount, setMatchCount] = useState<number>(0);
  const logEndRef = useRef<HTMLDivElement>(null);
  const maxLogEntries = 100; // Limit displayed log entries to prevent memory issues

  const handleSelectFiles = async () => {
    try {
      setError('');
      const result = await AppAPI.OpenMultipleFilesDialog();
      
      if (result && Array.isArray(result) && result.length > 0) {
        const fileSelections: FileSelection[] = result.map((path: string) => ({
          path,
          name: path.split(/[/\\]/).pop() || path,
          isDirectory: false
        }));
        setSelectedFiles(fileSelections);
      }
    } catch (err: any) {
      // Wails returns errors as strings, not objects with message property
      const errorMessage = typeof err === 'string' ? err : (err?.message || 'Failed to select files');
      setError(errorMessage);
    }
  };

  const handleSelectDirectory = async () => {
    try {
      setError('');
      const result = await AppAPI.OpenDirectoryDialog();
      
      if (result && typeof result === 'string' && result.length > 0) {
        const dirSelection: FileSelection = {
          path: result,
          name: result.split(/[/\\]/).pop() || result,
          isDirectory: true
        };
        setSelectedFiles([dirSelection]);
      }
    } catch (err: any) {
      // Wails returns errors as strings, not objects with message property
      const errorMessage = typeof err === 'string' ? err : (err?.message || 'Failed to select directory');
      setError(errorMessage);
    }
  };

  const handleLocateFiles = async () => {
    if (selectedFiles.length === 0) {
      setError('Please select files or a directory first');
      return;
    }

    setIsProcessing(true);
    setIsCancelling(false);
    setScanCompleted(false);
    setSuccessCount(0);
    setProcessingStatus('Scanning files...');
    setError('');
    setProgressLog([]);
    setFilesScanned(0);
    setMatchCount(0);

    try {
      // First, find matching files
      const result = await AppAPI.LocateWorkspaceFiles(selectedFiles);
      
      if (!result || typeof result.matchedCount !== 'number') {
        setError('Unexpected response from server');
        return;
      }

      if (result.matchedCount === 0) {
        setProcessingStatus('Scan complete - No matching files found');
        setScanCompleted(true);
        setSuccessCount(0);
        return;
      }

      // Store file locations using sync API
      setProcessingStatus(`Storing locations for ${result.matchedCount} matched files...`);
      
      let storedCount = 0;
      for (const matchedFile of result.matchedFiles || []) {
        try {
          await SyncAPI.StoreFileLocation(
            matchedFile.instanceId,
            matchedFile.workspaceId,
            matchedFile.fileHash,
            matchedFile.filePath
          );
          storedCount++;
        } catch (syncErr: any) {
          console.warn(`Failed to store location for ${matchedFile.filePath}:`, syncErr);
          // Continue with other files even if one fails
        }
      }

      if (storedCount > 0) {
        setProcessingStatus(`Scan complete - Successfully located and stored ${storedCount} file(s)`);
        setScanCompleted(true);
        setSuccessCount(storedCount);
      } else {
        setError('Failed to store any file locations. Please check your sync connection.');
      }
    } catch (err: any) {
      // Check if it was a cancellation
      if (err?.message && err.message.includes('cancel')) {
        setProcessingStatus('Scan cancelled - Review files scanned before cancellation');
        setScanCompleted(true);
      } else {
        // Wails returns errors as strings, not objects with message property
        const errorMessage = typeof err === 'string' ? err : (err?.message || 'Failed to locate files');
        setError(errorMessage);
      }
    } finally {
      setIsProcessing(false);
      setIsCancelling(false);
    }
  };

  const handleCancelLocate = async () => {
    setIsCancelling(true);
    setProcessingStatus('Cancelling...');
    try {
      await AppAPI.CancelLocateFiles();
    } catch (err: any) {
      console.error('Failed to cancel:', err);
    }
  };

  const handleRemoveFile = (index: number) => {
    setSelectedFiles(prev => prev.filter((_, i) => i !== index));
  };

  const handleClose = () => {
    if (!isProcessing) {
      // Call onSuccess with the stored count before closing
      if (scanCompleted && successCount >= 0) {
        onSuccess(successCount);
      }
      
      setSelectedFiles([]);
      setError('');
      setProcessingStatus('');
      setProgressLog([]);
      setFilesScanned(0);
      setMatchCount(0);
      setScanCompleted(false);
      setSuccessCount(0);
      onClose();
    }
  };

  // Auto-scroll log to bottom
  useEffect(() => {
    if (logEndRef.current) {
      logEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [progressLog]);

  // Listen for progress events
  useEffect(() => {
    if (!isOpen) return;

    const unsubProgress = EventsOn('locate:progress', (data: any) => {
      const filePath = data?.filePath || '';
      const scanned = data?.filesScanned || 0;
      const matches = data?.matchCount || 0;

      setFilesScanned(scanned);
      setMatchCount(matches);
      
      if (filePath) {
        setProgressLog(prev => {
          const newLog = [...prev, filePath];
          // Keep only last maxLogEntries
          if (newLog.length > maxLogEntries) {
            return newLog.slice(-maxLogEntries);
          }
          return newLog;
        });
      }
    });

    const unsubComplete = EventsOn('locate:complete', () => {
      setProcessingStatus('Scan complete');
    });

    const unsubCancelled = EventsOn('locate:cancelled', () => {
      setProcessingStatus('Scan cancelled - Review files scanned before cancellation');
      setScanCompleted(true);
      setIsProcessing(false);
      setIsCancelling(false);
    });

    return () => {
      if (typeof unsubProgress === 'function') unsubProgress();
      if (typeof unsubComplete === 'function') unsubComplete();
      if (typeof unsubCancelled === 'function') unsubCancelled();
    };
  }, [isOpen]);

  if (!isOpen) return null;

  return (
    <div className="modal-overlay" onClick={handleClose}>
      <div 
        className="modal-content locate-files-modal" 
        onClick={(e) => e.stopPropagation()}
      >
        <div className="modal-header">
          <h2>Locate Local Files</h2>
          {!isProcessing && (
            <button onClick={handleClose} className="close-button">
              <i className="fa-solid fa-xmark" />
            </button>
          )}
        </div>
        
        <div className="modal-body">
          <p style={{ marginBottom: '20px', fontSize: '14px', lineHeight: '1.5', color: '#ccc' }}>
            Select local files or a directory to match with workspace files. Files will be hashed and matched 
            with remote workspace files, then their locations will be stored for this workstation.
          </p>

          <div className="file-selection-section">
            <div className="selection-buttons">
              <button
                onClick={handleSelectFiles}
                disabled={isProcessing}
                className="select-btn files-btn"
              >
                <i className="fa-solid fa-file" style={{ marginRight: '8px' }} />
                Select Files
              </button>
              <button
                onClick={handleSelectDirectory}
                disabled={isProcessing}
                className="select-btn directory-btn"
              >
                <i className="fa-solid fa-folder" style={{ marginRight: '8px' }} />
                Select Directory
              </button>
            </div>

            {selectedFiles.length > 0 && (
              <div className="selected-files">
                <h4>Selected {selectedFiles.length === 1 && selectedFiles[0].isDirectory ? 'Directory' : 'Files'}:</h4>
                <div className="file-list">
                  {selectedFiles.map((file, index) => (
                    <div key={index} className="selected-file-item">
                      <div className="file-info" style={{ textAlign: 'left' }}>
                        <i className={`fa-solid ${file.isDirectory ? 'fa-folder' : 'fa-file'}`} 
                           style={{ marginRight: '8px', color: file.isDirectory ? '#4a9eff' : '#999' }} />
                        <span className="file-name" title={file.path} style={{ textAlign: 'left' }}>{file.name}</span>
                        <span className="file-path" style={{ textAlign: 'left' }}>{file.path}</span>
                      </div>
                      {!isProcessing && (
                        <button
                          onClick={() => handleRemoveFile(index)}
                          className="remove-file-btn"
                          title="Remove"
                        >
                          <i className="fa-solid fa-xmark" />
                        </button>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {error && (
            <div className="error-message" style={{ 
              marginTop: '16px', 
              padding: '12px', 
              background: '#3a1f1f', 
              border: '1px solid #ff6b6b', 
              borderRadius: '4px',
              color: '#ff6b6b',
              fontSize: '14px'
            }}>
              <i className="fa-solid fa-triangle-exclamation" style={{ marginRight: '8px' }} />
              {error}
            </div>
          )}

          {(isProcessing || scanCompleted) && (
            <div className="progress-section" style={{ marginTop: '16px' }}>
              <div className="progress-stats" style={{
                padding: '12px',
                background: '#2a2a2a',
                border: scanCompleted ? '1px solid #4ade80' : '1px solid #4a9eff',
                borderRadius: '4px',
                marginBottom: '12px',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center'
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  {isProcessing ? (
                    <i className="fa-solid fa-spinner fa-spin" style={{ color: '#4a9eff' }} />
                  ) : scanCompleted ? (
                    <i className="fa-solid fa-circle-check" style={{ color: '#4ade80' }} />
                  ) : null}
                  <span style={{ 
                    color: scanCompleted ? '#4ade80' : '#4a9eff', 
                    fontSize: '14px', 
                    fontWeight: '500' 
                  }}>
                    {processingStatus}
                  </span>
                </div>
                <div style={{ fontSize: '13px', color: '#999' }}>
                  Scanned: <span style={{ color: '#fff', fontWeight: '500' }}>{filesScanned}</span> | 
                  Matched: <span style={{ color: scanCompleted ? '#4ade80' : '#4a9eff', fontWeight: '500' }}>{matchCount}</span>
                </div>
              </div>
              
              <div className="progress-log" style={{
                maxHeight: '200px',
                overflowY: 'auto',
                background: '#1a1a1a',
                border: '1px solid #333',
                borderRadius: '4px',
                padding: '8px',
                fontFamily: 'monospace',
                fontSize: '11px',
                textAlign: 'left'
              }}>
                {progressLog.map((path, index) => (
                  <div key={index} style={{ 
                    color: '#888',
                    padding: '2px 0',
                    wordBreak: 'break-all',
                    textAlign: 'left'
                  }}>
                    {path}
                  </div>
                ))}
                <div ref={logEndRef} />
              </div>
            </div>
          )}

          <div className="modal-actions" style={{ 
            marginTop: '24px', 
            display: 'flex', 
            gap: '12px', 
            justifyContent: 'flex-end' 
          }}>
            <button
              onClick={isProcessing ? handleCancelLocate : handleClose}
              disabled={isCancelling}
              style={{
                padding: '10px 20px',
                fontSize: '14px',
                background: isProcessing ? '#ff4444' : '#3a3a3a',
                border: isProcessing ? 'none' : '1px solid #555',
                borderRadius: '6px',
                color: '#fff',
                cursor: isCancelling ? 'not-allowed' : 'pointer',
                fontWeight: '500',
                opacity: isCancelling ? 0.5 : 1,
              }}
            >
              {isCancelling ? 'Cancelling...' : (isProcessing ? 'Cancel Scan' : 'Close')}
            </button>
            <button
              onClick={handleLocateFiles}
              disabled={isProcessing || scanCompleted || selectedFiles.length === 0}
              style={{
                padding: '10px 20px',
                fontSize: '14px',
                background: selectedFiles.length > 0 && !isProcessing && !scanCompleted ? '#4a9eff' : '#666',
                border: 'none',
                borderRadius: '6px',
                color: '#fff',
                cursor: selectedFiles.length > 0 && !isProcessing && !scanCompleted ? 'pointer' : 'not-allowed',
                fontWeight: '500',
                opacity: selectedFiles.length > 0 && !isProcessing && !scanCompleted ? 1 : 0.5,
              }}
            >
              {isProcessing ? 'Processing...' : 'Locate Files'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default LocateFilesDialog;
