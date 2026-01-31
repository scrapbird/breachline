import React, { useState, useEffect } from 'react';
// @ts-ignore - Wails generated bindings
import { GetPlugins, AddPlugin, RemovePlugin, TogglePlugin, ValidatePluginPath, OpenPluginDialog } from '../../wailsjs/go/app/App';

interface PluginConfig {
    id: string;         // UUID from plugin.yml
    name: string;
    enabled: boolean;
    path: string;
    extensions: string[];
    description: string;
}

interface PluginSettingsTabProps {
    enablePlugins: boolean;
    onEnablePluginsChange: (enabled: boolean) => void;
    onLog: (level: 'info' | 'warn' | 'error', message: string) => void;
}

const PluginSettingsTab: React.FC<PluginSettingsTabProps> = ({
    enablePlugins,
    onEnablePluginsChange,
    onLog,
}) => {
    const [plugins, setPlugins] = useState<PluginConfig[]>([]);
    const [loading, setLoading] = useState(false);
    const [showConfirmDelete, setShowConfirmDelete] = useState<string | null>(null);
    const [pendingPlugin, setPendingPlugin] = useState<{
        path: string;
        manifest: {
            Name: string;
            Version: string;
            Description: string;
            Extensions: string[];
        };
    } | null>(null);

    // Load plugins on mount and when enablePlugins changes
    useEffect(() => {
        if (enablePlugins) {
            loadPlugins();
        }
    }, [enablePlugins]);

    const loadPlugins = async () => {
        try {
            setLoading(true);
            const pluginList = await GetPlugins();
            setPlugins(pluginList || []);
        } catch (e) {
            onLog('error', 'Failed to load plugins: ' + (e instanceof Error ? e.message : String(e)));
        } finally {
            setLoading(false);
        }
    };

    const handleAddPlugin = async () => {
        try {
            const selected = await OpenPluginDialog();

            if (!selected) return; // User cancelled

            // Validate plugin before adding
            try {
                const manifest = await ValidatePluginPath(selected);

                // Show confirmation dialog with plugin details
                setPendingPlugin({
                    path: selected,
                    manifest: manifest,
                });
            } catch (validationError) {
                onLog('error', 'Invalid plugin: ' + (validationError instanceof Error ? validationError.message : String(validationError)));
            }
        } catch (e) {
            onLog('error', 'Failed to add plugin: ' + (e instanceof Error ? e.message : String(e)));
        }
    };

    const confirmAddPlugin = async () => {
        if (!pendingPlugin) return;

        const { path, manifest } = pendingPlugin;
        setPendingPlugin(null);

        try {
            await AddPlugin(path);
            onLog('info', `Plugin "${manifest.Name}" added successfully`);

            // Reload plugin list
            await loadPlugins();
        } catch (e) {
            onLog('error', 'Failed to add plugin: ' + (e instanceof Error ? e.message : String(e)));
        }
    };

    const handleTogglePlugin = async (path: string, enabled: boolean) => {
        try {
            await TogglePlugin(path, enabled);
            onLog('info', `Plugin ${enabled ? 'enabled' : 'disabled'}`);

            // Update local state
            setPlugins(prev => prev.map(p =>
                p.path === path ? { ...p, enabled } : p
            ));
        } catch (e) {
            onLog('error', 'Failed to toggle plugin: ' + (e instanceof Error ? e.message : String(e)));
        }
    };

    const handleRemovePlugin = async (path: string) => {
        setShowConfirmDelete(null);

        try {
            await RemovePlugin(path);
            onLog('info', 'Plugin removed successfully');

            // Update local state
            setPlugins(prev => prev.filter(p => p.path !== path));
        } catch (e) {
            onLog('error', 'Failed to remove plugin: ' + (e instanceof Error ? e.message : String(e)));
        }
    };

    return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16, textAlign: 'left' }}>
            {/* Enable Plugins Toggle */}
            <label style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                <input
                    type="checkbox"
                    checked={enablePlugins}
                    onChange={(e) => onEnablePluginsChange(e.target.checked)}
                    style={{ marginTop: '2px' }}
                />
                <div style={{ display: 'flex', flexDirection: 'column' }}>
                    <span>Enable plugin support</span>
                    <div style={{ fontSize: 11, opacity: 0.65, marginTop: 4 }}>
                        Allows custom file loaders to handle additional file formats
                    </div>
                </div>
            </label>

            {/* Plugin List */}
            {enablePlugins && (
                <>
                    <div style={{ height: '1px', backgroundColor: '#444', margin: '8px 0' }}></div>

                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                        <div style={{ fontSize: 14, fontWeight: 500 }}>Installed Plugins</div>
                        <button
                            onClick={handleAddPlugin}
                            disabled={loading}
                            style={{
                                padding: '6px 12px',
                                borderRadius: 6,
                                border: '1px solid #3a6',
                                background: '#2d4733',
                                color: '#cfe',
                                cursor: loading ? 'wait' : 'pointer',
                                fontSize: 13,
                            }}
                        >
                            <i className="fa-solid fa-plus" style={{ marginRight: 6 }}></i>
                            Add Plugin
                        </button>
                    </div>

                    {loading ? (
                        <div style={{ textAlign: 'left', padding: 20, opacity: 0.6 }}>
                            Loading plugins...
                        </div>
                    ) : plugins.length === 0 ? (
                        <div style={{
                            textAlign: 'left',
                            padding: 30,
                            opacity: 0.5,
                            border: '1px dashed #444',
                            borderRadius: 8,
                        }}>
                            No plugins installed. Click "Add Plugin" to get started.
                        </div>
                    ) : (
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                            {plugins.map((plugin) => (
                                <div
                                    key={plugin.path}
                                    style={{
                                        border: '1px solid #444',
                                        borderRadius: 8,
                                        padding: 12,
                                        backgroundColor: '#2a2a2a',
                                    }}
                                >
                                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                                        <div
                                            style={{ flex: 1, cursor: 'pointer' }}
                                            onClick={() => handleTogglePlugin(plugin.path, !plugin.enabled)}
                                        >
                                            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
                                                <input
                                                    type="checkbox"
                                                    checked={plugin.enabled}
                                                    onChange={(e) => handleTogglePlugin(plugin.path, e.target.checked)}
                                                    onClick={(e) => e.stopPropagation()}
                                                    title={plugin.enabled ? 'Disable plugin' : 'Enable plugin'}
                                                />
                                                <div style={{ fontWeight: 500, fontSize: 14 }}>
                                                    {plugin.name}
                                                </div>
                                            </div>

                                            {plugin.description && (
                                                <div style={{ fontSize: 12, opacity: 0.7, marginBottom: 6 }}>
                                                    {plugin.description}
                                                </div>
                                            )}

                                            <div style={{ fontSize: 11, opacity: 0.6 }}>
                                                <span style={{ fontWeight: 500 }}>Extensions:</span> {plugin.extensions.join(', ')}
                                            </div>

                                            <div style={{
                                                fontSize: 10,
                                                opacity: 0.5,
                                                marginTop: 6,
                                                fontFamily: 'monospace',
                                                wordBreak: 'break-all',
                                            }}>
                                                {plugin.path}
                                            </div>
                                        </div>

                                        <button
                                            onClick={() => setShowConfirmDelete(plugin.path)}
                                            style={{
                                                padding: '6px 10px',
                                                borderRadius: 6,
                                                border: '1px solid #644',
                                                background: '#3a2a2a',
                                                color: '#faa',
                                                cursor: 'pointer',
                                                fontSize: 12,
                                                marginLeft: 12,
                                            }}
                                            title="Remove plugin"
                                        >
                                            <i className="fa-solid fa-trash"></i>
                                        </button>
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </>
            )}

            {/* Confirmation Dialog for Delete */}
            {showConfirmDelete && (
                <div
                    style={{
                        position: 'fixed',
                        top: 0,
                        left: 0,
                        right: 0,
                        bottom: 0,
                        backgroundColor: 'rgba(0, 0, 0, 0.7)',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        zIndex: 10000,
                    }}
                    onClick={() => setShowConfirmDelete(null)}
                >
                    <div
                        style={{
                            backgroundColor: '#2a2a2a',
                            border: '1px solid #444',
                            borderRadius: 8,
                            padding: 20,
                            maxWidth: 400,
                            width: '90%',
                        }}
                        onClick={(e) => e.stopPropagation()}
                    >
                        <h3 style={{ margin: '0 0 12px 0', fontSize: 16 }}>Remove Plugin?</h3>
                        <p style={{ margin: '0 0 16px 0', opacity: 0.8, fontSize: 13 }}>
                            Are you sure you want to remove this plugin?
                        </p>
                        <div style={{
                            fontSize: 11,
                            opacity: 0.6,
                            marginBottom: 16,
                            fontFamily: 'monospace',
                            wordBreak: 'break-all',
                        }}>
                            {showConfirmDelete}
                        </div>
                        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
                            <button
                                onClick={() => setShowConfirmDelete(null)}
                                style={{
                                    padding: '8px 12px',
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
                                onClick={() => handleRemovePlugin(showConfirmDelete)}
                                style={{
                                    padding: '8px 12px',
                                    borderRadius: 6,
                                    border: '1px solid #a44',
                                    background: '#4a2a2a',
                                    color: '#faa',
                                    cursor: 'pointer',
                                }}
                            >
                                Remove
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* Confirmation Dialog for Add Plugin */}
            {pendingPlugin && (
                <div
                    style={{
                        position: 'fixed',
                        top: 0,
                        left: 0,
                        right: 0,
                        bottom: 0,
                        backgroundColor: 'rgba(0, 0, 0, 0.7)',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        zIndex: 10000,
                    }}
                    onClick={() => setPendingPlugin(null)}
                >
                    <div
                        style={{
                            backgroundColor: '#2a2a2a',
                            border: '1px solid #444',
                            borderRadius: 8,
                            padding: 20,
                            maxWidth: 450,
                            width: '90%',
                        }}
                        onClick={(e) => e.stopPropagation()}
                    >
                        <h3 style={{ margin: '0 0 16px 0', fontSize: 16 }}>Add Plugin?</h3>

                        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginBottom: 16 }}>
                            <div style={{ display: 'flex', fontSize: 13 }}>
                                <span style={{ opacity: 0.6, minWidth: 90 }}>Name:</span>
                                <span style={{ fontWeight: 500 }}>{pendingPlugin.manifest.Name}</span>
                            </div>
                            <div style={{ display: 'flex', fontSize: 13 }}>
                                <span style={{ opacity: 0.6, minWidth: 90 }}>Version:</span>
                                <span>{pendingPlugin.manifest.Version}</span>
                            </div>
                            <div style={{ display: 'flex', fontSize: 13 }}>
                                <span style={{ opacity: 0.6, minWidth: 90 }}>Description:</span>
                                <span>{pendingPlugin.manifest.Description || 'N/A'}</span>
                            </div>
                            <div style={{ display: 'flex', fontSize: 13 }}>
                                <span style={{ opacity: 0.6, minWidth: 90 }}>Extensions:</span>
                                <span>{pendingPlugin.manifest.Extensions.join(', ')}</span>
                            </div>
                        </div>

                        <div style={{
                            fontSize: 11,
                            opacity: 0.5,
                            marginBottom: 16,
                            fontFamily: 'monospace',
                            wordBreak: 'break-all',
                        }}>
                            {pendingPlugin.path}
                        </div>

                        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
                            <button
                                onClick={() => setPendingPlugin(null)}
                                style={{
                                    padding: '8px 12px',
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
                                onClick={confirmAddPlugin}
                                style={{
                                    padding: '8px 12px',
                                    borderRadius: 6,
                                    border: '1px solid #3a6',
                                    background: '#2d4733',
                                    color: '#cfe',
                                    cursor: 'pointer',
                                }}
                            >
                                Add Plugin
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default PluginSettingsTab;
