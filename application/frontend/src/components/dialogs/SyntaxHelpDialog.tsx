import React, { useState } from 'react';
import Dialog from '../Dialog';

interface SyntaxHelpDialogProps {
    show: boolean;
    onClose: () => void;
}

const codeStyle: React.CSSProperties = { background: '#222', padding: '2px 6px', borderRadius: 4, fontSize: 12 };
const codeBlockStyle: React.CSSProperties = { ...codeStyle, display: 'block', margin: '4px 0' };
const inlineCodeStyle: React.CSSProperties = { background: '#222', padding: '2px 4px', borderRadius: 3, fontSize: 12 };

type TabId = 'filtering' | 'pipeline' | 'time';

const SyntaxHelpDialog: React.FC<SyntaxHelpDialogProps> = ({ show, onClose }) => {
    const [activeTab, setActiveTab] = useState<TabId>('filtering');

    const tabStyle = (isActive: boolean): React.CSSProperties => ({
        padding: '8px 16px',
        background: isActive ? '#2d4733' : 'transparent',
        border: isActive ? '1px solid #3a6' : '1px solid #444',
        borderRadius: 6,
        color: isActive ? '#cfe' : '#aaa',
        cursor: 'pointer',
        fontSize: 13,
        fontWeight: isActive ? 600 : 400,
    });

    return (
        <Dialog show={show} onClose={onClose} title="Query Syntax" maxWidth={750}>
            <div style={{ padding: '16px 24px', textAlign: 'left' }}>
                {/* Tab Navigation */}
                <div style={{ display: 'flex', gap: 8, marginBottom: 20, borderBottom: '1px solid #333', paddingBottom: 12 }}>
                    <button style={tabStyle(activeTab === 'filtering')} onClick={() => setActiveTab('filtering')}>Filtering</button>
                    <button style={tabStyle(activeTab === 'pipeline')} onClick={() => setActiveTab('pipeline')}>Pipeline Operations</button>
                    <button style={tabStyle(activeTab === 'time')} onClick={() => setActiveTab('time')}>Time Filters</button>
                </div>

                {/* Filtering Tab */}
                {activeTab === 'filtering' && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
                        <section>
                            <p style={{ fontSize: 13, opacity: 0.8, margin: '0 0 12px' }}>
                                These expressions are used with the <code style={inlineCodeStyle}>filter</code> operation:
                                <code style={{ ...codeStyle, marginLeft: 8 }}>filter level=error</code>
                            </p>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>Basic Search</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <p style={{ margin: '4px 0' }}>Search across all fields:</p>
                                <code style={codeStyle}>error</code>
                                <p style={{ margin: '8px 0 4px' }}>Use quotes for phrases:</p>
                                <code style={codeStyle}>"connection timeout"</code>
                            </div>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>Field-Specific Search</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <code style={codeBlockStyle}>level=error</code>
                                <code style={codeBlockStyle}>level!=info</code>
                                <code style={codeBlockStyle}>"user name"=john</code>
                                <p style={{ fontSize: 12, opacity: 0.7, margin: '4px 0' }}>Field names with spaces need quotes</p>
                            </div>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>Wildcards</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <code style={codeStyle}>warn*</code>
                                <span style={{ opacity: 0.7, marginLeft: 8 }}>(prefix match: "warning", "warn")</span>
                                <br />
                                <code style={{ ...codeStyle, marginTop: 4, display: 'inline-block' }}>error=*</code>
                                <span style={{ opacity: 0.7, marginLeft: 8 }}>(field exists / has value)</span>
                            </div>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>Logical Operators</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <p style={{ margin: '4px 0' }}><strong>AND</strong> (implicit with space, or explicit):</p>
                                <code style={codeStyle}>error database</code>
                                <span style={{ opacity: 0.6, margin: '0 8px' }}>or</span>
                                <code style={codeStyle}>error AND database</code>
                                <p style={{ margin: '10px 0 4px' }}><strong>OR</strong>:</p>
                                <code style={codeStyle}>error OR warning</code>
                                <p style={{ margin: '10px 0 4px' }}><strong>NOT</strong>:</p>
                                <code style={codeStyle}>NOT error</code>
                                <span style={{ opacity: 0.6, margin: '0 8px' }}>or</span>
                                <code style={codeStyle}>level!=debug</code>
                                <p style={{ margin: '10px 0 4px' }}><strong>Parentheses</strong> for grouping:</p>
                                <code style={codeStyle}>(error OR warning) AND database</code>
                            </div>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>JSON Path Expressions</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <p style={{ margin: '4px 0' }}>Query nested JSON fields using <code style={inlineCodeStyle}>{'column{$.path}'}</code>:</p>
                                <code style={codeBlockStyle}>{'requestParameters{$.bucketName}=my-bucket'}</code>
                                <code style={codeBlockStyle}>{'userIdentity{$.userName}=admin'}</code>
                                <p style={{ fontSize: 12, opacity: 0.7, margin: '4px 0' }}>Uses JSONPath syntax ($.field, $.array[0], etc.)</p>
                            </div>
                        </section>

                    </div>
                )}

                {/* Pipeline Operations Tab */}
                {activeTab === 'pipeline' && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
                        <section>
                            <p style={{ fontSize: 13, opacity: 0.8, margin: '0 0 12px' }}>
                                Chain operations with <code style={inlineCodeStyle}>|</code> (pipe):
                                <code style={{ ...codeStyle, marginLeft: 8 }}>filter level=error | sort timestamp desc | limit 100</code>
                            </p>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>columns</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <p style={{ margin: '4px 0' }}>Select specific columns to display:</p>
                                <code style={codeBlockStyle}>columns timestamp, level, message</code>
                                <code style={codeBlockStyle}>columns "user name", "event type"</code>
                            </div>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>sort</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <p style={{ margin: '4px 0' }}>Sort results by column(s):</p>
                                <code style={codeBlockStyle}>sort timestamp</code>
                                <code style={codeBlockStyle}>sort timestamp desc</code>
                                <code style={codeBlockStyle}>sort level asc, timestamp desc</code>
                                <p style={{ margin: '8px 0 4px' }}>Sort by JSON path values:</p>
                                <code style={codeBlockStyle}>{'sort requestParameters{$.durationSeconds} desc'}</code>
                            </div>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>dedup</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <p style={{ margin: '4px 0' }}>Remove duplicate rows:</p>
                                <code style={codeBlockStyle}>dedup</code>
                                <span style={{ fontSize: 12, opacity: 0.7 }}>(deduplicate on all columns)</span>
                                <code style={codeBlockStyle}>dedup username</code>
                                <span style={{ fontSize: 12, opacity: 0.7 }}>(deduplicate on specific column)</span>
                                <code style={codeBlockStyle}>dedup "source ip", "event type"</code>
                            </div>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>limit</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <p style={{ margin: '4px 0' }}>Limit number of results:</p>
                                <code style={codeBlockStyle}>limit 100</code>
                                <code style={codeBlockStyle}>filter level=error | sort timestamp desc | limit 50</code>
                            </div>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>strip</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <p style={{ margin: '4px 0' }}>Remove columns that are empty in all rows:</p>
                                <code style={codeBlockStyle}>strip</code>
                            </div>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>filter</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <p style={{ margin: '4px 0' }}>Apply a filter expression to the data:</p>
                                <code style={codeBlockStyle}>filter level=error</code>
                                <code style={codeBlockStyle}>filter status=200 OR status=201</code>
                                <code style={codeBlockStyle}>filter message=timeout*</code>
                                <p style={{ margin: '8px 0 4px' }}>Chain filters with other operations:</p>
                                <code style={codeBlockStyle}>sort timestamp | filter level=error | limit 100</code>
                                <p style={{ fontSize: 12, opacity: 0.7, margin: '6px 0' }}>See the Filtering tab for all supported filter expressions</p>
                            </div>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>annotated / NOT annotated</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <p style={{ margin: '4px 0' }}>Filter rows by annotation status:</p>
                                <code style={codeBlockStyle}>annotated</code>
                                <span style={{ fontSize: 12, opacity: 0.7 }}>(show only annotated rows)</span>
                                <code style={codeBlockStyle}>NOT annotated</code>
                                <span style={{ fontSize: 12, opacity: 0.7 }}>(show rows without annotations)</span>
                            </div>
                        </section>
                    </div>
                )}

                {/* Time Filters Tab */}
                {activeTab === 'time' && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>Time Range Filters</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <p style={{ margin: '4px 0' }}><strong>after</strong> — events after a time:</p>
                                <code style={codeBlockStyle}>after 2024-01-15</code>
                                <code style={codeBlockStyle}>after '2024-01-15 10:30:00'</code>
                                <p style={{ margin: '10px 0 4px' }}><strong>before</strong> — events before a time:</p>
                                <code style={codeBlockStyle}>before 2024-01-16</code>
                                <p style={{ margin: '10px 0 4px' }}>Combine for a range:</p>
                                <code style={codeBlockStyle}>after 2024-01-15 before 2024-01-16</code>
                            </div>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>Absolute Time Formats</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <code style={codeBlockStyle}>2024-01-15</code>
                                <code style={codeBlockStyle}>2024-01-15 14:30:00</code>
                                <code style={codeBlockStyle}>2024-01-15T14:30:00</code>
                                <code style={codeBlockStyle}>2024-01-15T14:30:00Z</code>
                            </div>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>Relative Times</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <code style={codeStyle}>now</code>
                                <span style={{ opacity: 0.7, marginLeft: 8 }}>(current time)</span>
                                <br />
                                <code style={{ ...codeStyle, marginTop: 4, display: 'inline-block' }}>5m ago</code>
                                <span style={{ opacity: 0.7, marginLeft: 8 }}>(5 minutes ago)</span>
                                <br />
                                <code style={{ ...codeStyle, marginTop: 4, display: 'inline-block' }}>2h ago</code>
                                <span style={{ opacity: 0.7, marginLeft: 8 }}>(2 hours ago)</span>
                                <br />
                                <code style={{ ...codeStyle, marginTop: 4, display: 'inline-block' }}>1d ago</code>
                                <span style={{ opacity: 0.7, marginLeft: 8 }}>(1 day ago)</span>
                                <br />
                                <code style={{ ...codeStyle, marginTop: 4, display: 'inline-block' }}>1w ago</code>
                                <span style={{ opacity: 0.7, marginLeft: 8 }}>(1 week ago)</span>
                                <p style={{ margin: '10px 0 4px', fontSize: 12, opacity: 0.7 }}>
                                    <strong>Units:</strong> s (seconds), m (minutes), h (hours), d (days), w (weeks), mo (months), y (years)
                                </p>
                            </div>
                        </section>

                        <section>
                            <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 6, color: '#cfe' }}>Examples</h3>
                            <div style={{ fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>
                                <code style={codeBlockStyle}>after 1h ago</code>
                                <p style={{ fontSize: 12, opacity: 0.7, margin: '2px 0 8px', paddingLeft: 8 }}>Events from the last hour</p>

                                <code style={codeBlockStyle}>after 1d ago | filter level=error</code>
                                <p style={{ fontSize: 12, opacity: 0.7, margin: '2px 0 8px', paddingLeft: 8 }}>Errors from the last 24 hours</p>

                                <code style={codeBlockStyle}>after 2024-01-15 before 2024-01-16 | sort timestamp</code>
                                <p style={{ fontSize: 12, opacity: 0.7, margin: '2px 0', paddingLeft: 8 }}>All events on Jan 15, sorted by time</p>
                            </div>
                        </section>
                    </div>
                )}
            </div>
        </Dialog>
    );
};

export default SyntaxHelpDialog;
