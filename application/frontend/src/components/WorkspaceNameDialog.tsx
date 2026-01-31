import { useState, useEffect, useRef } from 'react';
import Dialog from './Dialog';

interface WorkspaceNameDialogProps {
    show: boolean;
    onClose: () => void;
    onSubmit: (name: string) => void;
}

export default function WorkspaceNameDialog({
    show,
    onClose,
    onSubmit,
}: WorkspaceNameDialogProps) {
    const [name, setName] = useState<string>('');
    const inputRef = useRef<HTMLInputElement>(null);

    // Reset name when dialog opens
    useEffect(() => {
        if (show) {
            setName('');
            // Focus the input after a short delay to ensure the dialog is rendered
            setTimeout(() => {
                inputRef.current?.focus();
            }, 50);
        }
    }, [show]);

    const handleSubmit = () => {
        const trimmedName = name.trim();
        if (trimmedName) {
            onSubmit(trimmedName);
        }
    };

    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (e.key === 'Enter' && name.trim()) {
            e.preventDefault();
            handleSubmit();
        }
        if (e.key === 'Escape') {
            e.preventDefault();
            onClose();
        }
    };

    return (
        <Dialog show={show} onClose={onClose} title="Create Remote Workspace">
            <div
                style={{
                    padding: '20px 24px',
                    minWidth: 400,
                }}
            >
                <div style={{ marginBottom: 16 }}>
                    <label style={{ display: 'block', fontSize: 13, fontWeight: 500, marginBottom: 8, color: '#cfe' }}>
                        Workspace Name
                    </label>
                    <input
                        ref={inputRef}
                        type="text"
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                        onKeyDown={handleKeyDown}
                        placeholder="Enter workspace name..."
                        style={{
                            width: '100%',
                            padding: '10px 12px',
                            fontSize: 13,
                            background: '#1a2332',
                            color: '#eee',
                            border: '1px solid #444',
                            borderRadius: 6,
                            fontFamily: 'inherit',
                            boxSizing: 'border-box',
                        }}
                    />
                </div>
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
                    <button
                        onClick={handleSubmit}
                        disabled={!name.trim()}
                        style={{
                            padding: '8px 16px',
                            fontSize: 13,
                            background: name.trim() ? '#0066cc' : '#003366',
                            color: '#fff',
                            border: 'none',
                            borderRadius: 6,
                            cursor: name.trim() ? 'pointer' : 'not-allowed',
                            fontWeight: 500,
                            opacity: name.trim() ? 1 : 0.6,
                        }}
                    >
                        Create
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
