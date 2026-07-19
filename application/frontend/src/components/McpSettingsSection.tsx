import React, { useCallback, useEffect, useState } from 'react';
import * as BridgeAPI from '../../wailsjs/go/mcpserver/BridgeService';

interface McpStatus {
    enabled: boolean;
    running: boolean;
    address: string;
    token: string;
    error?: string;
}

interface McpSettingsSectionProps {
    settings: {
        mcp_server_enabled: boolean;
        mcp_server_address: string;
    };
    onSettingsChange: (updater: (prev: any) => any) => void;
}

const label: React.CSSProperties = { fontSize: 13, fontWeight: 500, color: '#cfe' };
const mono: React.CSSProperties = { fontFamily: 'monospace', fontSize: 12 };

// McpSettingsSection configures the built-in MCP server that lets an AI client
// drive this window. It is self-contained: it owns its own status polling and
// client-config helper, and reports setting changes up via onSettingsChange.
const McpSettingsSection: React.FC<McpSettingsSectionProps> = ({ settings, onSettingsChange }) => {
    const [status, setStatus] = useState<McpStatus | null>(null);
    const [copied, setCopied] = useState<string>('');

    const refresh = useCallback(async () => {
        try {
            const s = await BridgeAPI.MCPGetStatus();
            setStatus(s as McpStatus);
        } catch {
            setStatus(null);
        }
    }, []);

    // Poll status while the panel is mounted so the indicator reflects reality
    // after the user saves (which is what actually starts/stops the server).
    useEffect(() => {
        refresh();
        const id = setInterval(refresh, 1500);
        return () => clearInterval(id);
    }, [refresh]);

    const copy = (text: string, key: string) => {
        try {
            navigator.clipboard.writeText(text);
            setCopied(key);
            setTimeout(() => setCopied(''), 1200);
        } catch { /* clipboard may be unavailable */ }
    };

    const running = !!status?.running;
    const token = status?.token || '';
    const address = settings.mcp_server_address || '127.0.0.1:8765';
    const url = `http://${address}/`;
    const cliConfig = `claude mcp add --transport http breachline ${url}` +
        (token ? ` --header "Authorization: Bearer ${token}"` : '');

    return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16, textAlign: 'left' }}>
            <label style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                <input
                    type="checkbox"
                    checked={settings.mcp_server_enabled}
                    onChange={(e) => onSettingsChange(s => ({ ...s, mcp_server_enabled: e.target.checked }))}
                    style={{ marginTop: 2 }}
                />
                <span>
                    <span style={label}>Enable MCP server</span>
                    <div style={{ fontSize: 12, opacity: 0.8 }}>
                        Lets a local AI client open files, run queries, and annotate in this window.
                        Changes take effect when you press Save.
                    </div>
                </span>
            </label>

            <div>
                <div style={label}>Listen address (host:port)</div>
                <input
                    type="text"
                    value={settings.mcp_server_address}
                    onChange={(e) => onSettingsChange(s => ({ ...s, mcp_server_address: e.target.value }))}
                    placeholder="127.0.0.1:8765"
                    spellCheck={false}
                    style={{ ...mono, width: 240, marginTop: 6, padding: '6px 8px', borderRadius: 6, border: '1px solid #444', background: '#1f2426', color: '#eee' }}
                />
                <div style={{ fontSize: 12, opacity: 0.7, marginTop: 4 }}>
                    Keep this on a loopback address (127.0.0.1) so it is never reachable off this machine.
                </div>
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={label}>Status:</span>
                <span style={{
                    ...mono,
                    padding: '2px 8px', borderRadius: 10,
                    background: running ? '#1c3b2a' : '#3a2a2a',
                    color: running ? '#7fdca4' : '#e0a0a0',
                }}>
                    {running ? `running on ${status?.address}` : (settings.mcp_server_enabled ? 'starting / not running' : 'stopped')}
                </span>
                {status?.error ? <span style={{ ...mono, color: '#e0a0a0' }}>{status.error}</span> : null}
            </div>

            {token ? (
                <div>
                    <div style={label}>Access token</div>
                    <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginTop: 6 }}>
                        <code style={{ ...mono, padding: '6px 8px', borderRadius: 6, background: '#1f2426', border: '1px solid #444', color: '#cfe', maxWidth: 320, overflow: 'hidden', textOverflow: 'ellipsis' }}>{token}</code>
                        <button onClick={() => copy(token, 'token')} style={{ ...mono, padding: '6px 8px', borderRadius: 6, border: '1px solid #444', background: '#333', color: '#eee', cursor: 'pointer' }}>
                            {copied === 'token' ? 'Copied' : 'Copy'}
                        </button>
                    </div>
                </div>
            ) : null}

            <div>
                <div style={label}>Connect an MCP client</div>
                <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start', marginTop: 6 }}>
                    <code style={{ ...mono, flex: 1, padding: '8px 10px', borderRadius: 6, background: '#1f2426', border: '1px solid #444', color: '#cfe', whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>{cliConfig}</code>
                    <button onClick={() => copy(cliConfig, 'cli')} style={{ ...mono, padding: '8px 10px', borderRadius: 6, border: '1px solid #444', background: '#333', color: '#eee', cursor: 'pointer' }}>
                        {copied === 'cli' ? 'Copied' : 'Copy'}
                    </button>
                </div>
            </div>
        </div>
    );
};

export default McpSettingsSection;
