import React from 'react';

interface DirectoryHashWarningDialogProps {
    isOpen: boolean;
    directoryPath: string;
    onCancel: () => void;
    onContinue: () => void;
}

const DirectoryHashWarningDialog: React.FC<DirectoryHashWarningDialogProps> = ({
    isOpen,
    directoryPath,
    onCancel,
    onContinue,
}) => {
    if (!isOpen) return null;

    // Get just the directory name for display
    const dirName = directoryPath.split('/').pop() || directoryPath;

    return (
        <div className="modal-overlay" onClick={onCancel}>
            <div
                className="modal-content"
                onClick={(e) => e.stopPropagation()}
                style={{
                    maxWidth: '500px',
                    textAlign: 'left'
                }}
            >
                <div className="modal-header">
                    <h2>Directory Contents Changed</h2>
                    <button onClick={onCancel} className="close-button">
                        <i className="fa-solid fa-xmark" />
                    </button>
                </div>
                <div className="modal-body">
                    <div style={{
                        display: 'flex',
                        alignItems: 'flex-start',
                        gap: '12px',
                        marginBottom: '16px'
                    }}>
                        <i
                            className="fa-solid fa-triangle-exclamation"
                            style={{ color: '#f0ad4e', fontSize: '24px', marginTop: '2px' }}
                        />
                        <div>
                            <p style={{
                                fontSize: '14px',
                                lineHeight: '1.6',
                                color: '#ddd',
                                margin: 0,
                                marginBottom: '12px'
                            }}>
                                The files in this directory have changed since it was added to the workspace.
                            </p>
                            <p style={{
                                fontSize: '14px',
                                lineHeight: '1.6',
                                color: '#bbb',
                                margin: 0,
                            }}>
                                Existing annotations may not be assigned to the correct rows if the data has been modified, reordered, or if files have been added or removed.
                            </p>
                        </div>
                    </div>
                    <div style={{
                        padding: '12px',
                        background: '#2a2a2a',
                        borderRadius: '4px',
                        marginBottom: '20px',
                        wordBreak: 'break-all',
                    }}>
                        <div style={{ fontSize: '13px', color: '#888', marginBottom: '4px' }}>Directory:</div>
                        <div style={{ fontSize: '14px', fontWeight: '500', color: '#ccc' }}>
                            <i className="fa-solid fa-folder" style={{ color: '#e8a838', marginRight: '8px' }} />
                            {dirName}
                        </div>
                    </div>
                    <div style={{ display: 'flex', gap: '12px', justifyContent: 'flex-end' }}>
                        <button
                            onClick={onCancel}
                            style={{
                                padding: '8px 20px',
                                fontSize: '14px',
                                background: '#3a3a3a',
                                border: '1px solid #555',
                                borderRadius: '4px',
                                color: '#ddd',
                                cursor: 'pointer',
                                fontWeight: '500',
                            }}
                        >
                            Cancel
                        </button>
                        <button
                            onClick={onContinue}
                            style={{
                                padding: '8px 20px',
                                fontSize: '14px',
                                background: '#4a9eff',
                                border: 'none',
                                borderRadius: '4px',
                                color: '#fff',
                                cursor: 'pointer',
                                fontWeight: '500',
                            }}
                        >
                            Continue
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default DirectoryHashWarningDialog;
