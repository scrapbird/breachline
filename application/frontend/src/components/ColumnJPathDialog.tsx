import { useState, useEffect, useMemo } from 'react';
import Dialog from './Dialog';
import { previewJPath } from '../utils/jpathUtils';

interface ColumnJPathDialogProps {
    show: boolean;
    columnName: string;
    currentExpression: string;
    previewData: string[];  // First 5 cell values from this column
    onClose: () => void;
    onApply: (expression: string) => void;
    onClear: () => void;
}

export default function ColumnJPathDialog({
    show,
    columnName,
    currentExpression,
    previewData,
    onClose,
    onApply,
    onClear,
}: ColumnJPathDialogProps) {
    const [expression, setExpression] = useState<string>(currentExpression || '$');
    const [debouncedExpression, setDebouncedExpression] = useState<string>(expression);

    // Reset expression when dialog opens or currentExpression changes
    useEffect(() => {
        if (show) {
            setExpression(currentExpression || '$');
            setDebouncedExpression(currentExpression || '$');
        }
    }, [show, currentExpression]);

    // Debounce expression changes for preview
    useEffect(() => {
        const timer = setTimeout(() => {
            setDebouncedExpression(expression);
        }, 300);
        return () => clearTimeout(timer);
    }, [expression]);

    // Calculate preview results
    const previewResults = useMemo(() => {
        if (!debouncedExpression || previewData.length === 0) {
            return [];
        }
        return previewJPath(previewData, debouncedExpression);
    }, [previewData, debouncedExpression]);

    // Check if any preview has errors
    const hasErrors = previewResults.some((r) => r.error);

    // Check if expression is different from current
    const hasChanges = expression !== (currentExpression || '$');

    // Check if any transformations actually occurred
    const hasTransformations = previewResults.some(
        (r) => r.original !== r.transformed && !r.error
    );

    const handleApply = () => {
        if (!expression.trim() || hasErrors) {
            return;
        }
        onApply(expression.trim());
    };

    const handleClear = () => {
        setExpression('$');
        onClear();
    };

    return (
        <Dialog show={show} onClose={onClose} title="Apply JPath Expression" maxWidth="calc(100vw - 80px)">
            <div
                style={{
                    padding: '20px 24px',
                    boxSizing: 'border-box',
                    textAlign: 'left',
                }}
            >
                {/* Column info */}
                <div style={{ marginBottom: 16 }}>
                    <span style={{ fontSize: 13, color: '#888' }}>Column: </span>
                    <span style={{ fontSize: 13, color: '#cfe', fontWeight: 500 }}>{columnName}</span>
                </div>

                {/* Expression input */}
                <div style={{ marginBottom: 20 }}>
                    <label style={{ display: 'block', fontSize: 13, fontWeight: 500, marginBottom: 8, color: '#cfe' }}>
                        JPath Expression
                    </label>
                    <input
                        type="text"
                        value={expression}
                        onChange={(e) => setExpression(e.target.value)}
                        placeholder="e.g., $.data.name or $[0].value"
                        style={{
                            width: '100%',
                            padding: 10,
                            fontSize: 13,
                            background: '#1a2332',
                            color: '#eee',
                            border: '1px solid #444',
                            borderRadius: 6,
                            fontFamily: 'monospace',
                        }}
                        autoFocus
                        onKeyDown={(e) => {
                            if (e.key === 'Enter' && !hasErrors && expression.trim()) {
                                e.preventDefault();
                                handleApply();
                            }
                        }}
                    />
                    <div style={{ fontSize: 12, color: '#999', marginTop: 6 }}>
                        Enter a JSONPath expression to extract values from JSON cell contents.
                        Use <code style={{ background: '#333', padding: '1px 4px', borderRadius: 3 }}>$</code> to show original data.
                    </div>
                </div>

                {/* Preview section */}
                <div style={{ marginBottom: 20 }}>
                    <div style={{
                        fontSize: 13,
                        fontWeight: 500,
                        marginBottom: 8,
                        color: '#cfe',
                    }}>
                        Preview (first {previewData.length} rows)
                    </div>

                    <div
                        style={{
                            background: '#0d1117',
                            border: '1px solid #444',
                            borderRadius: 6,
                        }}
                    >
                        {previewData.length === 0 ? (
                            <div style={{ color: '#6e7681', fontSize: 13, textAlign: 'left', padding: '20px 12px' }}>
                                No data available for preview
                            </div>
                        ) : (
                            <table style={{
                                width: '100%',
                                borderCollapse: 'collapse',
                                fontSize: 12,
                                fontFamily: 'monospace',
                            }}>
                                <thead>
                                    <tr>
                                        <th style={{
                                            textAlign: 'left',
                                            padding: '10px 12px',
                                            background: '#161b22',
                                            borderBottom: '1px solid #30363d',
                                            color: '#8b949e',
                                            fontWeight: 600,
                                            width: '50%',
                                        }}>
                                            Original
                                        </th>
                                        <th style={{
                                            textAlign: 'left',
                                            padding: '10px 12px',
                                            background: '#161b22',
                                            borderBottom: '1px solid #30363d',
                                            color: '#58a6ff',
                                            fontWeight: 600,
                                            width: '50%',
                                        }}>
                                            Transformed
                                        </th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {previewResults.map((result, idx) => (
                                        <tr key={idx}>
                                            <td style={{
                                                padding: '10px 12px',
                                                borderBottom: '1px solid #21262d',
                                                color: '#8b949e',
                                                verticalAlign: 'top',
                                                wordBreak: 'break-word',
                                                whiteSpace: 'pre-wrap',
                                            }}>
                                                {result.original ? (
                                                    <pre style={{
                                                        margin: 0,
                                                        fontFamily: 'monospace',
                                                        fontSize: 12,
                                                        whiteSpace: 'pre-wrap',
                                                        wordBreak: 'break-word',
                                                    }}>
                                                        {result.original}
                                                    </pre>
                                                ) : (
                                                    <span style={{ color: '#6e7681', fontStyle: 'italic' }}>(empty)</span>
                                                )}
                                            </td>
                                            <td style={{
                                                padding: '10px 12px',
                                                borderBottom: '1px solid #21262d',
                                                color: result.error ? '#f85149' : (
                                                    result.original !== result.transformed ? '#7ee787' : '#c9d1d9'
                                                ),
                                                verticalAlign: 'top',
                                                wordBreak: 'break-word',
                                                whiteSpace: 'pre-wrap',
                                            }}>
                                                {result.error ? (
                                                    <span style={{ color: '#f85149' }}>
                                                        <i className="fa-solid fa-circle-exclamation" style={{ marginRight: 6 }} />
                                                        {result.error}
                                                    </span>
                                                ) : result.transformed ? (
                                                    <pre style={{
                                                        margin: 0,
                                                        fontFamily: 'monospace',
                                                        fontSize: 12,
                                                        whiteSpace: 'pre-wrap',
                                                        wordBreak: 'break-word',
                                                        color: 'inherit',
                                                    }}>
                                                        {result.transformed}
                                                    </pre>
                                                ) : (
                                                    <span style={{ color: '#6e7681', fontStyle: 'italic' }}>(empty)</span>
                                                )}
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        )}
                    </div>

                    {/* Info message about transformation */}
                    {!hasErrors && hasTransformations && (
                        <div style={{ fontSize: 12, color: '#7ee787', marginTop: 8 }}>
                            <i className="fa-solid fa-check" style={{ marginRight: 6 }} />
                            Expression produces transformed values
                        </div>
                    )}
                    {!hasErrors && !hasTransformations && expression !== '$' && previewData.length > 0 && (
                        <div style={{ fontSize: 12, color: '#d29922', marginTop: 8 }}>
                            <i className="fa-solid fa-info-circle" style={{ marginRight: 6 }} />
                            Expression returns same values as original (cells may not contain matching JSON)
                        </div>
                    )}
                </div>

                {/* Info box */}
                <div style={{
                    background: '#1a2332',
                    border: '1px solid #30363d',
                    borderRadius: 6,
                    padding: 12,
                    marginBottom: 20,
                }}>
                    <div style={{ fontSize: 12, color: '#8b949e', lineHeight: 1.5 }}>
                        <strong style={{ color: '#cfe' }}>Note:</strong> This transformation is display-only. 
                        Queries, filters, and annotations will continue to operate on the original data.
                    </div>
                </div>

                {/* Buttons */}
                <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
                    <button
                        onClick={onClose}
                        style={{
                            padding: '8px 16px',
                            fontSize: 13,
                            background: 'transparent',
                            color: '#ccc',
                            border: '1px solid #555',
                            borderRadius: 6,
                            cursor: 'pointer',
                        }}
                    >
                        Cancel
                    </button>
                    {currentExpression && currentExpression !== '$' && (
                        <button
                            onClick={handleClear}
                            style={{
                                padding: '8px 16px',
                                fontSize: 13,
                                background: '#8b2d2d',
                                color: '#fff',
                                border: 'none',
                                borderRadius: 6,
                                cursor: 'pointer',
                            }}
                        >
                            Clear
                        </button>
                    )}
                    <button
                        onClick={handleApply}
                        disabled={!expression.trim() || hasErrors}
                        style={{
                            padding: '8px 16px',
                            fontSize: 13,
                            background: (!expression.trim() || hasErrors) ? '#444' : '#0066cc',
                            color: (!expression.trim() || hasErrors) ? '#888' : '#fff',
                            border: 'none',
                            borderRadius: 6,
                            cursor: (!expression.trim() || hasErrors) ? 'not-allowed' : 'pointer',
                            fontWeight: 500,
                        }}
                    >
                        Apply
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
