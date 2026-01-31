import React, { useEffect } from 'react';

interface PluginWarningDialogProps {
    show: boolean;
    pluginName: string;
    executablePath: string;
    executableHash: string;
    onContinue: () => void;
    onCancel: () => void;
}

const PluginWarningDialog: React.FC<PluginWarningDialogProps> = ({
    show,
    pluginName,
    executablePath,
    executableHash,
    onContinue,
    onCancel,
}) => {
    // Handle Escape key to cancel
    useEffect(() => {
        if (!show) return;
        
        const handleKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.preventDefault();
                onCancel();
            } else if (e.key === 'Enter') {
                e.preventDefault();
                onContinue();
            }
        };
        
        document.addEventListener('keydown', handleKeyDown);
        return () => document.removeEventListener('keydown', handleKeyDown);
    }, [show, onCancel, onContinue]);

    if (!show) return null;

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
                textAlign: 'left',
                justifyContent: 'center',
                zIndex: 10000,
            }}
        >
            <div 
                className="modal-content"
                onClick={(e) => e.stopPropagation()}
                style={{
                    backgroundColor: '#2d2d2d',
                    borderRadius: '8px',
                    padding: '24px',
                    maxWidth: '550px',
                    width: '90%',
                    boxShadow: '0 4px 20px rgba(0, 0, 0, 0.5)',
                    border: '1px solid #404040',
                }}
            >
                <div style={{ display: 'flex', alignItems: 'center', marginBottom: '16px', gap: '12px' }}>
                    <i 
                        className="fa-solid fa-triangle-exclamation" 
                        style={{ fontSize: '24px', color: '#ffc107' }}
                    />
                    <h3 style={{ margin: 0, color: '#ffffff', fontSize: '18px' }}>
                        Plugin Execution Warning
                    </h3>
                </div>
                
                <p style={{ color: '#cccccc', marginBottom: '16px', lineHeight: '1.5' }}>
                    Opening this file will execute an external plugin to load its contents. 
                    Only continue if you trust this plugin.
                </p>
                
                <div style={{
                    backgroundColor: '#1e1e1e',
                    borderRadius: '6px',
                    padding: '12px',
                    marginBottom: '20px',
                    border: '1px solid #3a3a3a',
                }}>
                    <div style={{ marginBottom: '12px' }}>
                        <div style={{ color: '#888888', fontSize: '11px', textTransform: 'uppercase', marginBottom: '4px' }}>
                            Plugin Name
                        </div>
                        <div style={{ color: '#5ecfcf', fontWeight: 500 }}>
                            {pluginName}
                        </div>
                    </div>
                    
                    <div style={{ marginBottom: '12px' }}>
                        <div style={{ color: '#888888', fontSize: '11px', textTransform: 'uppercase', marginBottom: '4px' }}>
                            Executable Path
                        </div>
                        <div style={{ 
                            color: '#ffffff', 
                            fontFamily: 'monospace', 
                            fontSize: '12px',
                            wordBreak: 'break-all',
                            backgroundColor: '#252525',
                            padding: '6px 8px',
                            borderRadius: '4px',
                            WebkitUserSelect: 'text',
                            MozUserSelect: 'text',
                            msUserSelect: 'text',
                            userSelect: 'text',
                            cursor: 'text',
                        }}>
                            {executablePath}
                        </div>
                    </div>
                    
                    <div>
                        <div style={{ color: '#888888', fontSize: '11px', textTransform: 'uppercase', marginBottom: '4px' }}>
                            SHA256 Hash
                        </div>
                        <div style={{ 
                            color: '#a0a0a0', 
                            fontFamily: 'monospace', 
                            fontSize: '11px',
                            wordBreak: 'break-all',
                            backgroundColor: '#252525',
                            padding: '6px 8px',
                            borderRadius: '4px',
                            WebkitUserSelect: 'text',
                            MozUserSelect: 'text',
                            msUserSelect: 'text',
                            userSelect: 'text',
                            cursor: 'text',
                        }}>
                            {executableHash || 'Unable to calculate hash'}
                        </div>
                    </div>
                </div>
                
                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px' }}>
                    <button
                        onClick={onCancel}
                        style={{
                            padding: '8px 20px',
                            borderRadius: '4px',
                            border: '1px solid #555555',
                            backgroundColor: 'transparent',
                            color: '#cccccc',
                            cursor: 'pointer',
                            fontSize: '14px',
                        }}
                    >
                        Cancel
                    </button>
                    <button
                        onClick={onContinue}
                        style={{
                            padding: '8px 20px',
                            borderRadius: '4px',
                            border: 'none',
                            backgroundColor: '#4a9eff',
                            color: '#ffffff',
                            cursor: 'pointer',
                            fontSize: '14px',
                            fontWeight: 500,
                        }}
                    >
                        Continue
                    </button>
                </div>
            </div>
        </div>
    );
};

export default PluginWarningDialog;
