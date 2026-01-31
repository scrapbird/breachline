import React from 'react';
import Dialog from '../Dialog';

interface ShortcutsDialogProps {
    show: boolean;
    onClose: () => void;
}

const ShortcutsDialog: React.FC<ShortcutsDialogProps> = ({ show, onClose }) => {
    return (
        <Dialog show={show} onClose={onClose} title="Keyboard Shortcuts" maxWidth={600}>
            <div style={{ padding: '20px 24px', textAlign: 'left' }}>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
                    {/* File Operations */}
                    <section>
                        <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12, color: '#cfe' }}>File Operations</h3>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontSize: 13, opacity: 0.9 }}>Open file</span>
                                <kbd style={{ background: '#333', padding: '4px 8px', borderRadius: 4, fontSize: 12, border: '1px solid #555' }}>Ctrl+O</kbd>
                            </div>
                        </div>
                    </section>

                    {/* Search & Navigation */}
                    <section>
                        <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12, color: '#cfe' }}>Search & Navigation</h3>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontSize: 13, opacity: 0.9 }}>Focus search bar</span>
                                <kbd style={{ background: '#333', padding: '4px 8px', borderRadius: 4, fontSize: 12, border: '1px solid #555' }}>Ctrl+F</kbd>
                            </div>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontSize: 13, opacity: 0.9 }}>Focus search bar (alternate)</span>
                                <kbd style={{ background: '#333', padding: '4px 8px', borderRadius: 4, fontSize: 12, border: '1px solid #555' }}>Ctrl+L</kbd>
                            </div>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontSize: 13, opacity: 0.9 }}>Jump to top</span>
                                <kbd style={{ background: '#333', padding: '4px 8px', borderRadius: 4, fontSize: 12, border: '1px solid #555' }}>g</kbd>
                            </div>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontSize: 13, opacity: 0.9 }}>Jump to bottom</span>
                                <kbd style={{ background: '#333', padding: '4px 8px', borderRadius: 4, fontSize: 12, border: '1px solid #555' }}>Shift+G</kbd>
                            </div>
                        </div>
                    </section>

                    {/* Selection & Clipboard */}
                    <section>
                        <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12, color: '#cfe' }}>Selection & Clipboard</h3>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontSize: 13, opacity: 0.9 }}>Select all rows</span>
                                <kbd style={{ background: '#333', padding: '4px 8px', borderRadius: 4, fontSize: 12, border: '1px solid #555' }}>Ctrl+A</kbd>
                            </div>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontSize: 13, opacity: 0.9 }}>Copy selected rows</span>
                                <kbd style={{ background: '#333', padding: '4px 8px', borderRadius: 4, fontSize: 12, border: '1px solid #555' }}>Ctrl+C</kbd>
                            </div>
                        </div>
                    </section>

                    {/* Annotations */}
                    <section>
                        <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12, color: '#cfe' }}>Annotations</h3>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontSize: 13, opacity: 0.9 }}>Annotate selected row</span>
                                <kbd style={{ background: '#333', padding: '4px 8px', borderRadius: 4, fontSize: 12, border: '1px solid #555' }}>a</kbd>
                            </div>
                        </div>
                    </section>

                    {/* View Controls */}
                    <section>
                        <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12, color: '#cfe' }}>View Controls</h3>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontSize: 13, opacity: 0.9 }}>Toggle histogram</span>
                                <kbd style={{ background: '#333', padding: '4px 8px', borderRadius: 4, fontSize: 12, border: '1px solid #555' }}>Ctrl+H</kbd>
                            </div>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontSize: 13, opacity: 0.9 }}>Toggle console</span>
                                <kbd style={{ background: '#333', padding: '4px 8px', borderRadius: 4, fontSize: 12, border: '1px solid #555' }}>Ctrl+`</kbd>
                            </div>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontSize: 13, opacity: 0.9 }}>Toggle annotations panel</span>
                                <kbd style={{ background: '#333', padding: '4px 8px', borderRadius: 4, fontSize: 12, border: '1px solid #555' }}>Ctrl+B</kbd>
                            </div>
                        </div>
                    </section>

                    {/* Settings */}
                    <section>
                        <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12, color: '#cfe' }}>Settings</h3>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontSize: 13, opacity: 0.9 }}>Open settings</span>
                                <kbd style={{ background: '#333', padding: '4px 8px', borderRadius: 4, fontSize: 12, border: '1px solid #555' }}>Ctrl+,</kbd>
                            </div>
                        </div>
                    </section>

                    {/* Modal Controls */}
                    <section>
                        <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12, color: '#cfe' }}>Modal Controls</h3>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontSize: 13, opacity: 0.9 }}>Close modal / dialog</span>
                                <kbd style={{ background: '#333', padding: '4px 8px', borderRadius: 4, fontSize: 12, border: '1px solid #555' }}>Esc</kbd>
                            </div>
                        </div>
                    </section>

                    <div style={{ marginTop: 8, padding: '12px', background: '#1a2332', borderRadius: 6, fontSize: 12, opacity: 0.8 }}>
                        <strong>Note:</strong> On macOS, use <kbd style={{ background: '#333', padding: '2px 6px', borderRadius: 3, fontSize: 11, border: '1px solid #555', margin: '0 2px' }}>Cmd</kbd> instead of <kbd style={{ background: '#333', padding: '2px 6px', borderRadius: 3, fontSize: 11, border: '1px solid #555', margin: '0 2px' }}>Ctrl</kbd> for most shortcuts.
                    </div>
                </div>
            </div>
        </Dialog>
    );
};

export default ShortcutsDialog;
