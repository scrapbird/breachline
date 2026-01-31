import React, { useState, useEffect } from 'react';
import Dialog from './Dialog';
import TimezoneSelector from './TimezoneSelector';

export interface FileOpenOptions {
    jpathExpression: string;
    noHeaderRow: boolean;
    ingestTimezoneOverride: string;
    // Directory options
    isDirectory: boolean;
    filePattern: string;      // Required for directories
    includeSourceColumn: boolean;
}

interface PreviewData {
    headers: string[];
    rows: string[][];
    error: string;
    availableKeys?: string[];
}

interface FileOptionsDialogProps {
    show: boolean;
    filePath: string;
    fileType: 'json' | 'csv' | 'xlsx' | 'directory';
    showTimezoneOverride?: boolean; // Whether to show timezone override option (default: true)
    onConfirm: (options: FileOpenOptions) => void;
    onCancel: () => void;
}

const FileOptionsDialog: React.FC<FileOptionsDialogProps> = ({
    show,
    filePath,
    fileType,
    showTimezoneOverride = true,
    onConfirm,
    onCancel,
}) => {
    const [jpathExpression, setJpathExpression] = useState('');
    const [noHeaderRow, setNoHeaderRow] = useState(false);
    const [ingestTimezoneOverride, setIngestTimezoneOverride] = useState('');
    const [preview, setPreview] = useState<PreviewData | null>(null);
    const [isLoading, setIsLoading] = useState(false);
    // Directory-specific state
    const [filePattern, setFilePattern] = useState('');
    const [includeSourceColumn, setIncludeSourceColumn] = useState(false);

    const isJsonFile = fileType === 'json';
    const isDirectory = fileType === 'directory';

    // Reset options when dialog opens with a new file
    useEffect(() => {
        if (show) {
            setJpathExpression('');
            setNoHeaderRow(false);
            setIngestTimezoneOverride('');
            setPreview(null);
            setFilePattern('');
            setIncludeSourceColumn(false);
        }
    }, [show, filePath]);

    // Auto-preview when expression changes for JSON files (with debounce)
    useEffect(() => {
        if (!show || !isJsonFile || !jpathExpression) return;

        const timer = setTimeout(() => {
            fetchPreview();
        }, 500);

        return () => clearTimeout(timer);
    }, [jpathExpression, show, isJsonFile]);

    const fetchPreview = async () => {
        if (!jpathExpression.trim()) {
            setPreview(null);
            return;
        }

        setIsLoading(true);
        try {
            const AppAPI = await import('../../wailsjs/go/app/App');
            // Strip trailing dots from the expression before evaluation
            const cleanExpression = jpathExpression.trim().replace(/\.+$/, '');
            const response = await AppAPI.PreviewJSONWithExpression({
                filePath,
                expression: cleanExpression,
                maxRows: 5,
            });

            setPreview(response);
        } catch (err: any) {
            // Wails returns errors as strings, not objects with message property
            const errorMessage = typeof err === 'string' ? err : (err?.message || 'Failed to preview JSON');
            setPreview({
                headers: [],
                rows: [],
                error: errorMessage,
            });
        } finally {
            setIsLoading(false);
        }
    };

    const handleConfirm = () => {
        // For directories, validate pattern is provided
        if (isDirectory) {
            if (!filePattern.trim()) return;
        }
        // For JSON files, validate the expression
        if (isJsonFile) {
            if (!jpathExpression.trim()) return;
            if (preview && preview.error) return;
            if (!preview || !preview.headers || preview.headers.length === 0) return;
        }

        // Strip trailing dots from the expression before saving
        const cleanExpression = jpathExpression.trim().replace(/\.+$/, '');

        onConfirm({
            jpathExpression: (isJsonFile || isDirectory) ? cleanExpression : '',
            noHeaderRow: isJsonFile ? false : noHeaderRow, // JSON always has headers
            ingestTimezoneOverride,
            isDirectory,
            filePattern: isDirectory ? filePattern.trim() : '',
            includeSourceColumn: isDirectory ? includeSourceColumn : false,
        });
    };

    // Extract just the filename from the path for display
    const fileName = filePath.split('/').pop() || filePath.split('\\').pop() || filePath;

    // Determine if confirm button should be disabled
    const isConfirmDisabled =
        (isDirectory && !filePattern.trim()) ||
        (isJsonFile && (
            !jpathExpression.trim() ||
            !preview ||
            !!preview.error ||
            !preview.headers ||
            preview.headers.length === 0
        ));

    return (
        <Dialog
            show={show}
            onClose={onCancel}
            title={`File Options: ${fileName}`}
            maxWidth={isJsonFile ? 800 : 450}
        >
            <div style={{
                display: 'flex',
                flexDirection: 'column',
                gap: 16,
                textAlign: 'left',
            }}>
                {/* JSONPath Expression Section - JSON files only */}
                {isJsonFile && (
                    <>
                        <div style={{ marginBottom: 4 }}>
                            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, marginBottom: 8, color: '#cfe' }}>
                                JSONPath Expression
                            </label>
                            <input
                                type="text"
                                value={jpathExpression}
                                onChange={(e) => setJpathExpression(e.target.value)}
                                placeholder="e.g., $.rows or $.data.items"
                                style={{
                                    width: '100%',
                                    padding: 10,
                                    fontSize: 13,
                                    background: '#1a2332',
                                    color: '#eee',
                                    border: '1px solid #444',
                                    borderRadius: 6,
                                    fontFamily: 'monospace',
                                    boxSizing: 'border-box',
                                }}
                                autoFocus
                            />
                            <div style={{ fontSize: 12, color: '#999', marginTop: 6 }}>
                                Enter a JSONPath expression that returns an array of objects or an array of arrays.
                                The first object's keys (or first array) will become column headers.
                            </div>
                        </div>

                        {/* Preview Section */}
                        <div style={{ marginBottom: 4 }}>
                            <div style={{
                                fontSize: 13,
                                fontWeight: 500,
                                marginBottom: 8,
                                color: '#cfe',
                                display: 'flex',
                                alignItems: 'center',
                                gap: 8,
                            }}>
                                Preview (first 5 rows)
                                {isLoading && (
                                    <span style={{ fontSize: 11, color: '#888' }}>Loading...</span>
                                )}
                            </div>

                            <div
                                style={{
                                    background: '#0d1117',
                                    border: '1px solid #444',
                                    borderRadius: 6,
                                    minHeight: 200,
                                    maxHeight: 300,
                                    overflow: 'auto',
                                    padding: 12,
                                }}
                            >
                                {preview && preview.error && (
                                    <div>
                                        <div style={{ color: '#f85149', fontSize: 13, fontFamily: 'monospace', marginBottom: 12 }}>
                                            {preview.error}
                                        </div>
                                        {preview.availableKeys && preview.availableKeys.length > 0 && (
                                            <div style={{ marginTop: 16 }}>
                                                <div style={{ fontSize: 12, color: '#8b949e', marginBottom: 8, fontWeight: 500 }}>
                                                    Available keys at this path:
                                                </div>
                                                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                                                    {preview.availableKeys.map((key, idx) => (
                                                        <button
                                                            key={idx}
                                                            onClick={() => {
                                                                const currentExpr = jpathExpression.trim();
                                                                const newExpr = currentExpr + (currentExpr.endsWith('.') ? '' : '.') + key;
                                                                setJpathExpression(newExpr);
                                                            }}
                                                            style={{
                                                                padding: '4px 10px',
                                                                fontSize: 12,
                                                                background: '#21262d',
                                                                color: '#58a6ff',
                                                                border: '1px solid #30363d',
                                                                borderRadius: 4,
                                                                cursor: 'pointer',
                                                                fontFamily: 'monospace',
                                                            }}
                                                            onMouseEnter={(e) => {
                                                                e.currentTarget.style.background = '#30363d';
                                                            }}
                                                            onMouseLeave={(e) => {
                                                                e.currentTarget.style.background = '#21262d';
                                                            }}
                                                        >
                                                            {key}
                                                        </button>
                                                    ))}
                                                </div>
                                            </div>
                                        )}
                                    </div>
                                )}

                                {preview && !preview.error && preview.headers.length > 0 && (
                                    <table style={{
                                        width: '100%',
                                        borderCollapse: 'collapse',
                                        fontSize: 12,
                                        fontFamily: 'monospace',
                                    }}>
                                        <thead>
                                            <tr>
                                                {preview.headers.map((header, idx) => (
                                                    <th
                                                        key={idx}
                                                        style={{
                                                            textAlign: 'left',
                                                            padding: '8px 12px',
                                                            background: '#161b22',
                                                            borderBottom: '1px solid #30363d',
                                                            color: '#58a6ff',
                                                            fontWeight: 600,
                                                            position: 'sticky',
                                                            top: 0,
                                                        }}
                                                    >
                                                        {header}
                                                    </th>
                                                ))}
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {preview.rows.map((row, rowIdx) => (
                                                <tr key={rowIdx}>
                                                    {row.map((cell, cellIdx) => (
                                                        <td
                                                            key={cellIdx}
                                                            style={{
                                                                padding: '6px 12px',
                                                                borderBottom: '1px solid #21262d',
                                                                color: '#c9d1d9',
                                                                textAlign: 'left',
                                                            }}
                                                        >
                                                            {cell || <span style={{ color: '#6e7681' }}>(empty)</span>}
                                                        </td>
                                                    ))}
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                )}

                                {!preview && !isLoading && (
                                    <div style={{ color: '#6e7681', fontSize: 13, textAlign: 'center', paddingTop: 80 }}>
                                        Enter a JSONPath expression to see preview
                                    </div>
                                )}

                                {preview && !preview.error && preview.headers.length === 0 && (
                                    <div style={{ color: '#6e7681', fontSize: 13, textAlign: 'center', paddingTop: 80 }}>
                                        No data to preview
                                    </div>
                                )}
                            </div>
                        </div>
                    </>
                )}

                {/* Directory Options Section */}
                {isDirectory && (
                    <>
                        <div style={{ marginBottom: 4 }}>
                            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, marginBottom: 8, color: '#cfe' }}>
                                File Pattern <span style={{ color: '#f85149' }}>*</span>
                            </label>
                            <input
                                type="text"
                                value={filePattern}
                                onChange={(e) => setFilePattern(e.target.value)}
                                placeholder="e.g., *.json.gz, *.csv, cloudtrail*.json"
                                style={{
                                    width: '100%',
                                    padding: 10,
                                    fontSize: 13,
                                    background: '#1a2332',
                                    color: '#eee',
                                    border: filePattern.trim() ? '1px solid #444' : '1px solid #664',
                                    borderRadius: 6,
                                    fontFamily: 'monospace',
                                    boxSizing: 'border-box',
                                }}
                                autoFocus
                            />
                            <div style={{ fontSize: 12, color: '#999', marginTop: 6 }}>
                                Required. Glob pattern to match files. Use */ for directory wildcards, ** for recursive matching. Examples: *.json.gz (filenames), */2025/20/*.log (specific paths), logs/**/*.json (recursive). All matched files should be of the same log type.
                            </div>
                        </div>

                        <div style={{ marginBottom: 4 }}>
                            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, marginBottom: 8, color: '#cfe' }}>
                                JSONPath Expression (optional)
                            </label>
                            <input
                                type="text"
                                value={jpathExpression}
                                onChange={(e) => setJpathExpression(e.target.value)}
                                placeholder="e.g., $.Records[*] - leave empty for CSV files"
                                style={{
                                    width: '100%',
                                    padding: 10,
                                    fontSize: 13,
                                    background: '#1a2332',
                                    color: '#eee',
                                    border: '1px solid #444',
                                    borderRadius: 6,
                                    fontFamily: 'monospace',
                                    boxSizing: 'border-box',
                                }}
                            />
                            <div style={{ fontSize: 12, color: '#999', marginTop: 6 }}>
                                If files are JSON, enter a JSONPath expression to extract records. Leave empty for CSV/XLSX files.
                            </div>
                        </div>

                        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                            <label style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                                <input
                                    type="checkbox"
                                    checked={includeSourceColumn}
                                    onChange={(e) => setIncludeSourceColumn(e.target.checked)}
                                    style={{ marginTop: '2px' }}
                                />
                                <span>Include source file column (__source_file__)</span>
                            </label>
                            <div style={{ fontSize: 11, opacity: 0.65, textAlign: 'left', marginLeft: 24, marginTop: -4 }}>
                                Adds a column showing which file each row came from.
                            </div>
                        </div>
                    </>
                )}

                {/* No Header Row Section - CSV/XLSX/Directory files only */}
                {!isJsonFile && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                        <label style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                            <input
                                type="checkbox"
                                checked={noHeaderRow}
                                onChange={(e) => setNoHeaderRow(e.target.checked)}
                                style={{ marginTop: '2px' }}
                            />
                            <span>File has no header row</span>
                        </label>
                        <div style={{ fontSize: 11, opacity: 0.65, textAlign: 'left', marginLeft: 24, marginTop: -4 }}>
                            When enabled, the first row is treated as data and column headers will be automatically generated (unnamed_a, unnamed_b, etc.).
                        </div>
                    </div>
                )}

                {/* Timezone Override Section - Only shown when showTimezoneOverride is true */}
                {showTimezoneOverride && (
                    <div style={{
                        display: 'flex',
                        flexDirection: 'column',
                        gap: 6,
                        paddingTop: isJsonFile ? 12 : 0,
                        borderTop: isJsonFile ? '1px solid #30363d' : 'none',
                    }}>
                        <div style={{ fontSize: 13, fontWeight: isJsonFile ? 500 : undefined, opacity: isJsonFile ? 1 : 0.85, color: isJsonFile ? '#cfe' : undefined, textAlign: 'left' }}>
                            Ingest Timezone Override
                        </div>
                        <TimezoneSelector
                            value={ingestTimezoneOverride}
                            onChange={setIngestTimezoneOverride}
                            placeholder="Search timezones..."
                            showEmptyOption={true}
                            emptyOptionLabel="Use default setting"
                        />
                        <div style={{ fontSize: 11, opacity: 0.65, textAlign: 'left' }}>
                            Override the default ingest timezone for timestamps without timezone info in this file only.
                        </div>
                    </div>
                )}

                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 8 }}>
                    <button
                        onClick={onCancel}
                        style={{
                            padding: '8px 16px',
                            borderRadius: 6,
                            border: '1px solid #444',
                            background: '#333',
                            color: '#eee',
                            cursor: 'pointer',
                        }}
                    >
                        Cancel
                    </button>
                    <button
                        onClick={handleConfirm}
                        disabled={isConfirmDisabled}
                        style={{
                            padding: '8px 16px',
                            borderRadius: 6,
                            border: '1px solid #3a6',
                            background: isConfirmDisabled ? '#444' : '#2d4733',
                            color: isConfirmDisabled ? '#888' : '#cfe',
                            cursor: isConfirmDisabled ? 'not-allowed' : 'pointer',
                        }}
                    >
                        Continue
                    </button>
                </div>
            </div>
        </Dialog>
    );
};

export default FileOptionsDialog;
