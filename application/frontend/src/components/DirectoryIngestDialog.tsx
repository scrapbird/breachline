import React, { useState, useEffect } from 'react';
// @ts-ignore - Wails generated bindings
import * as AppAPI from "../../wailsjs/go/app/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import TimezoneSelector from './TimezoneSelector';

interface DirectoryIngestDialogProps {
    dirPath: string;
    onSave: (options: DirectoryOptions) => void;
    onClose: () => void;
}

export interface DirectoryOptions {
    pattern: string;
    jpath: string;
    includeSourceColumn: boolean;
    noHeaderRow: boolean;
    ingestTimezoneOverride: string;
}

interface DiscoveryProgress {
    filesFound: number;
    dirsScanned: number;
    currentPath: string;
    totalSize: number;
}

interface PreviewResult {
    files: string[];
    headers: string[];
    totalFiles: number;
    totalSize: number;
}

const DirectoryIngestDialog: React.FC<DirectoryIngestDialogProps> = ({
    dirPath,
    onSave,
    onClose,
}) => {
    const [pattern, setPattern] = useState('');
    const [jpath, setJpath] = useState('');
    const [includeSourceColumn, setIncludeSourceColumn] = useState(false);
    const [noHeaderRow, setNoHeaderRow] = useState(false);
    const [ingestTimezone, setIngestTimezone] = useState('');
    const [progress, setProgress] = useState<DiscoveryProgress | null>(null);
    const [isScanning, setIsScanning] = useState(false);
    const [previewResult, setPreviewResult] = useState<PreviewResult | null>(null);
    const [error, setError] = useState<string | null>(null);

    // Get just the directory name for display
    const dirName = dirPath.split('/').pop() || dirPath.split('\\').pop() || dirPath;

    useEffect(() => {
        // Listen for discovery progress events
        const progressUnsub = EventsOn('directory:discovery:progress', (data: DiscoveryProgress) => {
            setProgress(data);
        });
        
        const completeUnsub = EventsOn('directory:discovery:complete', () => {
            setIsScanning(false);
        });
        
        return () => {
            EventsOff('directory:discovery:progress');
            EventsOff('directory:discovery:complete');
        };
    }, []);

    // Handle Escape key to close dialog
    useEffect(() => {
        const handleEscape = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.preventDefault();
                onClose();
            }
        };
        
        document.addEventListener('keydown', handleEscape);
        return () => document.removeEventListener('keydown', handleEscape);
    }, [onClose]);

    const handlePreview = async () => {
        setIsScanning(true);
        setError(null);
        setPreviewResult(null);
        
        try {
            const result = await AppAPI.PreviewDirectory(dirPath, pattern, jpath);
            setPreviewResult({
                files: result.files || [],
                headers: result.headers || [],
                totalFiles: result.totalFiles || 0,
                totalSize: result.totalSize || 0,
            });
        } catch (err: any) {
            setError(err.message || 'Failed to preview directory');
        } finally {
            setIsScanning(false);
        }
    };

    const handleSave = () => {
        onSave({
            pattern,
            jpath,
            includeSourceColumn,
            noHeaderRow,
            ingestTimezoneOverride: ingestTimezone,
        });
    };

    const formatSize = (bytes: number): string => {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    };

    return (
        <div className="modal-overlay" onClick={onClose}>
            <div 
                className="modal-content" 
                onClick={(e) => e.stopPropagation()}
                style={{ maxWidth: '600px', maxHeight: '85vh', display: 'flex', flexDirection: 'column' }}
            >
                <div className="modal-header">
                    <h2>Open Directory: {dirName}</h2>
                    <button onClick={onClose} className="close-button">
                        <i className="fa-solid fa-xmark" />
                    </button>
                </div>
                
                <div className="modal-body" style={{ overflowY: 'auto', flex: 1, padding: '16px' }}>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                        {/* File Pattern */}
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                            <label style={{ fontSize: '13px', opacity: 0.85 }}>File Pattern <span style={{ color: '#ff6b6b' }}>*</span></label>
                            <input
                                type="text"
                                value={pattern}
                                onChange={(e) => setPattern(e.target.value)}
                                placeholder="e.g., *.json.gz, */2025/20/*.log, data/**/audit*.csv"
                                style={{
                                    padding: '8px 12px',
                                    border: pattern.trim() ? '1px solid #444' : '1px solid #664',
                                    borderRadius: '6px',
                                    backgroundColor: '#2a2a2a',
                                    color: '#fff',
                                    fontSize: '13px'
                                }}
                            />
                            <div style={{ fontSize: '11px', opacity: 0.65 }}>
                                Required. Glob pattern to match files. Use */ for directory wildcards, ** for recursive matching. Examples: *.json.gz (filenames), */2025/20/*.log (specific paths), logs/**/*.json (recursive). All matched files should be of the same log type.
                            </div>
                        </div>
                        
                        {/* JSONPath Expression */}
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                            <label style={{ fontSize: '13px', opacity: 0.85 }}>JSONPath Expression (for JSON files)</label>
                            <input
                                type="text"
                                value={jpath}
                                onChange={(e) => setJpath(e.target.value)}
                                placeholder="e.g., $.Records[*]"
                                style={{
                                    padding: '8px 12px',
                                    border: '1px solid #444',
                                    borderRadius: '6px',
                                    backgroundColor: '#2a2a2a',
                                    color: '#fff',
                                    fontSize: '13px'
                                }}
                            />
                            <div style={{ fontSize: '11px', opacity: 0.65 }}>
                                Required if loading JSON files. Applied to all JSON files in directory.
                            </div>
                        </div>
                        
                        {/* Checkboxes */}
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                                <input
                                    type="checkbox"
                                    checked={includeSourceColumn}
                                    onChange={(e) => setIncludeSourceColumn(e.target.checked)}
                                />
                                <span>Include source file column (__source_file__)</span>
                            </label>
                            <div style={{ fontSize: '11px', opacity: 0.65, marginLeft: '24px' }}>
                                Adds a column showing which file each row came from.
                            </div>
                            
                            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', marginTop: '8px' }}>
                                <input
                                    type="checkbox"
                                    checked={noHeaderRow}
                                    onChange={(e) => setNoHeaderRow(e.target.checked)}
                                />
                                <span>No header row (first row is data)</span>
                            </label>
                        </div>
                        
                        {/* Ingest Timezone */}
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                            <label style={{ fontSize: '13px', opacity: 0.85 }}>Ingest Timezone Override (optional)</label>
                            <TimezoneSelector
                                value={ingestTimezone}
                                onChange={setIngestTimezone}
                                placeholder="Use default"
                            />
                            <div style={{ fontSize: '11px', opacity: 0.65 }}>
                                Override the default timezone for parsing timestamps without timezone info.
                            </div>
                        </div>
                        
                        {/* Separator */}
                        <div style={{ height: '1px', backgroundColor: '#444', margin: '8px 0' }}></div>
                        
                        {/* Preview Button */}
                        <button 
                            onClick={handlePreview} 
                            disabled={isScanning || !pattern.trim()}
                            style={{
                                padding: '10px 16px',
                                border: '1px solid #555',
                                borderRadius: '6px',
                                backgroundColor: '#333',
                                color: '#fff',
                                fontSize: '13px',
                                cursor: (isScanning || !pattern.trim()) ? 'not-allowed' : 'pointer',
                                opacity: (isScanning || !pattern.trim()) ? 0.7 : 1,
                            }}
                        >
                            {isScanning ? 'Scanning...' : 'Preview Directory'}
                        </button>
                        
                        {/* Progress Info */}
                        {progress && isScanning && (
                            <div style={{ 
                                padding: '12px', 
                                backgroundColor: '#1a1a1a', 
                                borderRadius: '6px',
                                fontSize: '12px'
                            }}>
                                <div>Found {progress.filesFound} files in {progress.dirsScanned} directories</div>
                                <div style={{ opacity: 0.65, marginTop: '4px', wordBreak: 'break-all' }}>
                                    {progress.currentPath}
                                </div>
                            </div>
                        )}
                        
                        {/* Preview Results */}
                        {previewResult && (
                            <div style={{ 
                                padding: '12px', 
                                backgroundColor: '#1a1a1a', 
                                borderRadius: '6px',
                                fontSize: '12px'
                            }}>
                                <div style={{ marginBottom: '12px' }}>
                                    <strong>Summary:</strong> {previewResult.totalFiles} files, {formatSize(previewResult.totalSize)}
                                </div>
                                
                                {previewResult.files.length > 0 && (
                                    <div style={{ marginBottom: '12px' }}>
                                        <div style={{ fontWeight: 'bold', marginBottom: '6px' }}>
                                            Files to load (showing first {Math.min(previewResult.files.length, 10)}):
                                        </div>
                                        <div style={{ 
                                            maxHeight: '120px', 
                                            overflowY: 'auto',
                                            backgroundColor: '#222',
                                            padding: '8px',
                                            borderRadius: '4px',
                                            fontFamily: 'monospace',
                                            fontSize: '11px'
                                        }}>
                                            {previewResult.files.slice(0, 10).map((file, i) => (
                                                <div key={i} style={{ padding: '2px 0' }}>{file}</div>
                                            ))}
                                            {previewResult.files.length > 10 && (
                                                <div style={{ opacity: 0.65, marginTop: '4px' }}>
                                                    ... and {previewResult.files.length - 10} more files
                                                </div>
                                            )}
                                        </div>
                                    </div>
                                )}
                                
                                {previewResult.headers.length > 0 && (
                                    <div>
                                        <div style={{ fontWeight: 'bold', marginBottom: '6px' }}>
                                            Unified columns ({previewResult.headers.length}):
                                        </div>
                                        <div style={{ 
                                            maxHeight: '100px', 
                                            overflowY: 'auto',
                                            backgroundColor: '#222',
                                            padding: '8px',
                                            borderRadius: '4px',
                                            fontFamily: 'monospace',
                                            fontSize: '11px'
                                        }}>
                                            {previewResult.headers.map((col, i) => (
                                                <span key={i} style={{ 
                                                    display: 'inline-block',
                                                    padding: '2px 6px',
                                                    margin: '2px',
                                                    backgroundColor: '#333',
                                                    borderRadius: '3px'
                                                }}>
                                                    {col}
                                                </span>
                                            ))}
                                        </div>
                                    </div>
                                )}
                            </div>
                        )}
                        
                        {/* Error Display */}
                        {error && (
                            <div style={{ 
                                padding: '12px', 
                                backgroundColor: '#3a1a1a', 
                                borderRadius: '6px',
                                color: '#ff6b6b',
                                fontSize: '12px'
                            }}>
                                {error}
                            </div>
                        )}
                    </div>
                </div>
                
                {/* Action Buttons */}
                <div style={{ 
                    display: 'flex', 
                    justifyContent: 'flex-end', 
                    gap: '8px', 
                    padding: '16px',
                    borderTop: '1px solid #333'
                }}>
                    <button
                        onClick={onClose}
                        style={{ 
                            padding: '8px 16px', 
                            borderRadius: '6px', 
                            border: '1px solid #444', 
                            background: '#333', 
                            color: '#eee',
                            cursor: 'pointer'
                        }}
                    >
                        Cancel
                    </button>
                    <button
                        onClick={handleSave}
                        disabled={isScanning || !pattern.trim()}
                        style={{ 
                            padding: '8px 16px', 
                            borderRadius: '6px', 
                            border: '1px solid #3a6', 
                            background: '#2d4733', 
                            color: '#cfe',
                            cursor: (isScanning || !pattern.trim()) ? 'not-allowed' : 'pointer',
                            opacity: (isScanning || !pattern.trim()) ? 0.7 : 1,
                        }}
                    >
                        Open Directory
                    </button>
                </div>
            </div>
        </div>
    );
};

export default DirectoryIngestDialog;
