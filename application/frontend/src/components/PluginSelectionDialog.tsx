import React, { useState, useEffect } from 'react';

interface PluginOption {
    id: string;  // UUID from plugin.yml
    name: string;
    description: string;
    path: string;
    extensions: string[];
}

interface PluginSelectionDialogProps {
    show: boolean;
    plugins: PluginOption[];
    fileName: string;
    onSelect: (pluginId: string, pluginName: string) => void;  // Receives the plugin UUID and name
    onCancel: () => void;
}

const PluginSelectionDialog: React.FC<PluginSelectionDialogProps> = ({
    show,
    plugins,
    fileName,
    onSelect,
    onCancel,
}) => {
    const [selectedPlugin, setSelectedPlugin] = useState<string | null>(null);

    // Reset selection when dialog opens - use plugin ID for selection
    useEffect(() => {
        if (show && plugins.length > 0) {
            setSelectedPlugin(plugins[0].id);
        } else {
            setSelectedPlugin(null);
        }
    }, [show, plugins]);

    // Handle Escape key to close dialog
    useEffect(() => {
        if (!show) return;
        
        const handleEscape = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.preventDefault();
                onCancel();
            }
        };
        
        document.addEventListener('keydown', handleEscape);
        return () => document.removeEventListener('keydown', handleEscape);
    }, [show, onCancel]);

    // Handle Enter key to confirm selection
    useEffect(() => {
        if (!show) return;
        
        const handleEnter = (e: KeyboardEvent) => {
            if (e.key === 'Enter' && selectedPlugin) {
                e.preventDefault();
                const plugin = plugins.find(p => p.id === selectedPlugin);
                onSelect(selectedPlugin, plugin?.name || '');
            }
        };
        
        document.addEventListener('keydown', handleEnter);
        return () => document.removeEventListener('keydown', handleEnter);
    }, [show, selectedPlugin, onSelect]);

    if (!show) return null;

    const handleContinue = () => {
        if (selectedPlugin) {
            const plugin = plugins.find(p => p.id === selectedPlugin);
            onSelect(selectedPlugin, plugin?.name || '');
        }
    };

    return (
        <div 
            className="modal-overlay" 
            onClick={onCancel}
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
        >
            <div 
                className="modal-content"
                onClick={(e) => e.stopPropagation()}
                style={{
                    backgroundColor: '#2E3436',
                    border: '1px solid #444',
                    borderRadius: 8,
                    padding: 0,
                    maxWidth: 500,
                    width: '90%',
                    display: 'flex',
                    flexDirection: 'column',
                }}
            >
                {/* Header */}
                <div style={{ 
                    padding: '16px 20px', 
                    borderBottom: '1px solid #444',
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                }}>
                    <h3 style={{ margin: 0, fontSize: 16, fontWeight: 500 }}>
                        Select Plugin
                    </h3>
                    <button 
                        onClick={onCancel} 
                        className="close-button"
                        style={{
                            background: 'none',
                            border: 'none',
                            color: '#999',
                            cursor: 'pointer',
                            padding: '4px 8px',
                            fontSize: 16,
                        }}
                    >
                        <i className="fa-solid fa-xmark" />
                    </button>
                </div>

                {/* Body */}
                <div style={{ padding: '16px 20px' }}>
                    <p style={{ 
                        margin: '0 0 16px 0', 
                        fontSize: 13, 
                        opacity: 0.8,
                        textAlign: 'left',
                    }}>
                        Multiple plugins can open <strong style={{ color: '#9cf' }}>{fileName}</strong>. 
                        Select which plugin to use:
                    </p>

                    {/* Plugin List */}
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                        {plugins.map((plugin) => (
                            <div
                                key={plugin.id}
                                onClick={() => setSelectedPlugin(plugin.id)}
                                style={{
                                    border: selectedPlugin === plugin.id 
                                        ? '2px solid #3a6' 
                                        : '1px solid #444',
                                    borderRadius: 8,
                                    padding: selectedPlugin === plugin.id ? 11 : 12,
                                    backgroundColor: selectedPlugin === plugin.id 
                                        ? '#2d4733' 
                                        : '#2a2a2a',
                                    cursor: 'pointer',
                                    transition: 'all 0.15s ease',
                                    textAlign: 'left',
                                }}
                            >
                                <div style={{ 
                                    display: 'flex', 
                                    alignItems: 'center', 
                                    gap: 8, 
                                    marginBottom: plugin.description ? 6 : 0,
                                }}>
                                    <div style={{ 
                                        fontWeight: 500, 
                                        fontSize: 14,
                                        color: selectedPlugin === plugin.id ? '#cfe' : '#fff',
                                    }}>
                                        {plugin.name}
                                    </div>
                                    {selectedPlugin === plugin.id && (
                                        <i 
                                            className="fa-solid fa-check" 
                                            style={{ color: '#3a6', fontSize: 12 }}
                                        />
                                    )}
                                </div>
                                
                                {plugin.description && (
                                    <div style={{ 
                                        fontSize: 12, 
                                        opacity: 0.7, 
                                        marginBottom: 6,
                                    }}>
                                        {plugin.description}
                                    </div>
                                )}
                                
                                <div style={{ fontSize: 11, opacity: 0.5 }}>
                                    <span style={{ fontWeight: 500 }}>Extensions:</span> {plugin.extensions.join(', ')}
                                </div>
                            </div>
                        ))}
                    </div>
                </div>

                {/* Footer */}
                <div style={{ 
                    padding: '16px 20px', 
                    borderTop: '1px solid #444',
                    display: 'flex', 
                    justifyContent: 'flex-end', 
                    gap: 8,
                }}>
                    <button
                        onClick={onCancel}
                        style={{ 
                            padding: '8px 16px', 
                            borderRadius: 8, 
                            border: '1px solid #444', 
                            background: '#333', 
                            color: '#eee', 
                            cursor: 'pointer',
                            fontSize: 13,
                        }}
                    >
                        Cancel
                    </button>
                    <button
                        onClick={handleContinue}
                        disabled={!selectedPlugin}
                        style={{ 
                            padding: '8px 16px', 
                            borderRadius: 8, 
                            border: '1px solid #3a6', 
                            background: selectedPlugin ? '#2d4733' : '#333', 
                            color: selectedPlugin ? '#cfe' : '#888', 
                            cursor: selectedPlugin ? 'pointer' : 'not-allowed',
                            fontSize: 13,
                        }}
                    >
                        Continue
                    </button>
                </div>
            </div>
        </div>
    );
};

export default PluginSelectionDialog;
