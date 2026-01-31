import { useState, useMemo } from 'react';
import Dialog from './Dialog';
import { JSONTree } from 'react-json-tree';
import { findMatches, MatchRange } from '../utils/searchHighlight';

interface CellViewerDialogProps {
    show: boolean;
    columnName: string;
    cellValue: string;
    searchTerms?: string[];
    onClose: () => void;
    contained?: boolean;
}

// Dark theme for react-json-tree matching app colors
const jsonTreeTheme = {
    scheme: 'breachline',
    base00: '#252B2D', // background - slightly darker than dialog for contrast
    base01: '#353B3D',
    base02: '#454B4D',
    base03: '#666',    // comments
    base04: '#888',
    base05: '#eee',    // text
    base06: '#fff',
    base07: '#fff',
    base08: '#ff6b6b', // red (null, undefined)
    base09: '#f0a060', // orange (numbers)
    base0A: '#ffd93d', // yellow
    base0B: '#6bcb77', // green (strings)
    base0C: '#4dd4ac', // cyan
    base0D: '#74c0fc', // blue (keys)
    base0E: '#b197fc', // purple (booleans)
    base0F: '#ff8787',
};

// Try to parse string as JSON, returns parsed object or null if not valid JSON
function tryParseJson(value: string): any | null {
    if (!value || typeof value !== 'string') return null;
    const trimmed = value.trim();
    // Quick check: must start with { or [ to be JSON object/array
    if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return null;
    try {
        return JSON.parse(trimmed);
    } catch {
        return null;
    }
}

/**
 * Render text with highlighted segments using the same style as HighlightCellRenderer.
 */
function renderHighlightedText(text: string, matches: MatchRange[]): React.ReactNode[] {
    const segments: React.ReactNode[] = [];
    let lastEnd = 0;
    
    for (let i = 0; i < matches.length; i++) {
        const match = matches[i];
        
        // Add non-highlighted text before this match
        if (match.start > lastEnd) {
            segments.push(
                <span key={`text-${i}`}>
                    {text.slice(lastEnd, match.start)}
                </span>
            );
        }
        
        // Add highlighted match - using same class as grid highlighting
        segments.push(
            <span 
                key={`match-${i}`} 
                className="search-highlight"
            >
                {text.slice(match.start, match.end)}
            </span>
        );
        
        lastEnd = match.end;
    }
    
    // Add remaining text after last match
    if (lastEnd < text.length) {
        segments.push(
            <span key="text-final">
                {text.slice(lastEnd)}
            </span>
        );
    }
    
    return segments;
}

export default function CellViewerDialog({
    show,
    columnName,
    cellValue,
    searchTerms,
    onClose,
    contained,
}: CellViewerDialogProps) {
    const [copied, setCopied] = useState(false);

    // Memoize JSON parsing to avoid re-parsing on every render
    const parsedJson = useMemo(() => tryParseJson(cellValue), [cellValue]);
    const isJson = parsedJson !== null;
    
    // Custom value renderer for JSONTree to highlight matching search terms
    const valueRenderer = useMemo(() => {
        if (!searchTerms || searchTerms.length === 0) return undefined;
        
        return (valueAsString: unknown, value: unknown, ...keyPath: (string | number)[]) => {
            const strValue = String(valueAsString);
            const matches = findMatches(strValue, searchTerms);
            
            if (matches.length === 0) {
                return <span>{strValue}</span>;
            }
            return <span>{renderHighlightedText(strValue, matches)}</span>;
        };
    }, [searchTerms]);

    const handleCopy = async () => {
        try {
            await navigator.clipboard.writeText(cellValue);
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
        } catch {
            // Fallback for older browsers
            const ta = document.createElement('textarea');
            ta.value = cellValue;
            ta.style.position = 'fixed';
            ta.style.opacity = '0';
            document.body.appendChild(ta);
            ta.select();
            document.execCommand('copy');
            document.body.removeChild(ta);
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
        }
    };

    return (
        <Dialog show={show} onClose={onClose} title={columnName || 'Cell Content'} maxWidth="90vw" contained={contained}>
            <div style={{ padding: '16px 20px', display: 'flex', flexDirection: 'column', gap: 12, minWidth: '80vw', flex: 1, minHeight: 0 }}>
                <div
                    style={{
                        background: '#252B2D',
                        border: '1px solid #444',
                        borderRadius: 6,
                        padding: 12,
                        flex: '1 1 auto',
                        minHeight: 0,
                        overflowY: 'auto',
                        overflowX: 'auto',
                        textAlign: 'left',
                    }}
                >
                    {isJson ? (
                        <JSONTree
                            data={parsedJson}
                            theme={jsonTreeTheme}
                            invertTheme={false}
                            hideRoot={false}
                            shouldExpandNodeInitially={() => true}
                            valueRenderer={valueRenderer}
                        />
                    ) : (
                        <pre
                            className="cell-viewer-plaintext"
                            style={{
                                margin: 0,
                                fontFamily: 'monospace',
                                fontSize: 13,
                                color: '#eee',
                                whiteSpace: 'pre-wrap',
                                wordBreak: 'break-word',
                                textAlign: 'left',
                            }}
                        >
                            {cellValue ? (
                                searchTerms && searchTerms.length > 0 ? (
                                    renderHighlightedText(cellValue, findMatches(cellValue, searchTerms))
                                ) : (
                                    cellValue
                                )
                            ) : (
                                <span style={{ color: '#666', fontStyle: 'italic' }}>(empty)</span>
                            )}
                        </pre>
                    )}
                </div>
                <div style={{ display: 'flex', justifyContent: 'flex-end', flex: '0 0 auto' }}>
                    <button
                        onClick={handleCopy}
                        style={{
                            padding: '8px 16px',
                            fontSize: 13,
                            background: copied ? '#2a5a3a' : '#0066cc',
                            color: '#fff',
                            border: 'none',
                            borderRadius: 6,
                            cursor: 'pointer',
                            fontWeight: 500,
                            display: 'flex',
                            alignItems: 'center',
                            gap: 8,
                            transition: 'background 0.15s',
                        }}
                    >
                        <i className={copied ? 'fa-solid fa-check' : 'fa-solid fa-copy'} />
                        {copied ? 'Copied!' : 'Copy'}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
