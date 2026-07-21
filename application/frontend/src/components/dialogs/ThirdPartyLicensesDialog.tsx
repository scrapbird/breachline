import React, { useMemo, useState } from 'react';
import Dialog from '../Dialog';
import manifest from '../../assets/third-party-licenses.json';

interface ThirdPartyLicensesDialogProps {
    show: boolean;
    onClose: () => void;
}

interface LicenseEntry {
    name: string;
    version: string;
    source: string;
    license: string;
    text: string;
}

const entries = manifest.entries as LicenseEntry[];

const ThirdPartyLicensesDialog: React.FC<ThirdPartyLicensesDialogProps> = ({ show, onClose }) => {
    const [query, setQuery] = useState('');

    const filtered = useMemo(() => {
        const q = query.trim().toLowerCase();
        if (!q) return entries;
        return entries.filter((e) =>
            e.name.toLowerCase().includes(q) || e.license.toLowerCase().includes(q));
    }, [query]);

    return (
        <Dialog show={show} onClose={onClose} title="Third-party licenses" maxWidth={720}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12, padding: '16px 20px', minHeight: 0 }}>
                <div style={{ fontSize: 13, opacity: 0.7 }}>
                    BreachLine is built with {entries.length} open source components. Their copyright notices and license terms are reproduced below.
                </div>

                <input
                    type="text"
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    placeholder="Filter by name or license..."
                    style={{
                        padding: '8px 10px',
                        borderRadius: 4,
                        border: '1px solid rgba(128,128,128,0.4)',
                        background: 'transparent',
                        color: 'inherit',
                        fontSize: 14,
                    }}
                />

                <div style={{ fontSize: 12, opacity: 0.6 }}>
                    {filtered.length} of {entries.length} shown
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                    {filtered.map((e) => (
                        <details
                            key={`${e.source}:${e.name}@${e.version}`}
                            style={{
                                border: '1px solid rgba(128,128,128,0.25)',
                                borderRadius: 4,
                                padding: '6px 10px',
                            }}
                        >
                            <summary style={{ cursor: 'pointer', display: 'flex', gap: 8, alignItems: 'baseline', flexWrap: 'wrap' }}>
                                <span style={{ fontWeight: 500 }}>{e.name}</span>
                                <span style={{ fontSize: 12, opacity: 0.6 }}>{e.version}</span>
                                <span style={{
                                    fontSize: 11,
                                    padding: '1px 6px',
                                    borderRadius: 10,
                                    border: '1px solid rgba(128,128,128,0.4)',
                                    opacity: 0.8,
                                }}>{e.license}</span>
                            </summary>
                            <pre style={{
                                whiteSpace: 'pre-wrap',
                                wordBreak: 'break-word',
                                fontSize: 12,
                                marginTop: 8,
                                marginBottom: 4,
                                opacity: 0.85,
                                fontFamily: 'monospace',
                            }}>{e.text}</pre>
                        </details>
                    ))}
                </div>
            </div>
        </Dialog>
    );
};

export default ThirdPartyLicensesDialog;
