import { useState, useEffect } from 'react';
import Dialog from './Dialog';

interface AnnotationDialogProps {
    show: boolean;
    initialNote: string;
    initialColor: string;
    rowCount: number; // Number of rows being annotated
    isLoading?: boolean; // Loading state for save/delete operations
    onClose: () => void;
    onSave: (note: string, color: string) => void;
    onDelete: () => void;
}

export default function AnnotationDialog({
    show,
    initialNote,
    initialColor,
    rowCount,
    isLoading = false,
    onClose,
    onSave,
    onDelete,
}: AnnotationDialogProps) {
    // Local state for the dialog - only updates parent on save
    const [note, setNote] = useState<string>(initialNote);
    const [color, setColor] = useState<string>(initialColor);

    // Update local state when dialog opens with new values
    useEffect(() => {
        if (show) {
            setNote(initialNote);
            setColor(initialColor);
        }
    }, [show, initialNote, initialColor]);

    const handleSave = () => {
        onSave(note, color);
    };

    const title = rowCount === 1 ? "Add Annotation" : `Add Annotation to ${rowCount} Rows`;

    return (
        <Dialog show={show} onClose={onClose} title={title}>
            <div
                style={{
                    padding: '20px 24px',
                    minWidth: 400,
                    maxWidth: 600,
                }}
            >
                <div style={{ marginBottom: 16 }}>
                    <label style={{ display: 'block', fontSize: 13, fontWeight: 500, marginBottom: 8, color: '#cfe' }}>
                        Note
                    </label>
                    <textarea
                        value={note}
                        onChange={(e) => setNote(e.target.value)}
                        placeholder="Enter your annotation note..."
                        disabled={isLoading}
                        style={{
                            width: '100%',
                            minHeight: 120,
                            padding: 10,
                            fontSize: 13,
                            background: isLoading ? '#0f1419' : '#1a2332',
                            color: isLoading ? '#888' : '#eee',
                            border: '1px solid #444',
                            borderRadius: 6,
                            resize: 'vertical',
                            fontFamily: 'inherit',
                            cursor: isLoading ? 'not-allowed' : 'text',
                            opacity: isLoading ? 0.6 : 1,
                        }}
                        autoFocus
                    />
                </div>
                <div style={{ marginBottom: 16 }}>
                    <label style={{ display: 'block', fontSize: 13, fontWeight: 500, marginBottom: 8, color: '#cfe' }}>
                        Color
                    </label>
                    <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', justifyContent: 'center' }}>
                        {[
                            { name: 'white', bg: '#6b675d' },
                            { name: 'grey', bg: '#3a3a3a' },
                            { name: 'blue', bg: '#2a4a7c' },
                            { name: 'green', bg: '#2a5a3a' },
                            { name: 'yellow', bg: '#6b5d2a' },
                            { name: 'orange', bg: '#6b4a2a' },
                            { name: 'red', bg: '#6b2a2a' },
                        ].map((colorOption) => (
                            <div
                                key={colorOption.name}
                                onClick={isLoading ? undefined : () => setColor(colorOption.name)}
                                style={{
                                    width: 48,
                                    height: 32,
                                    background: colorOption.bg,
                                    borderRadius: 6,
                                    cursor: isLoading ? 'not-allowed' : 'pointer',
                                    border: color === colorOption.name ? '2px solid #0cf' : '2px solid transparent',
                                    transition: 'border-color 0.15s',
                                    opacity: isLoading ? 0.6 : 1,
                                }}
                                title={colorOption.name}
                            />
                        ))}
                    </div>
                </div>
                <div style={{ display: 'flex', gap: 10, justifyContent: 'space-between' }}>
                    <button
                        onClick={() => {
                            console.log('Delete button clicked in AnnotationDialog', { isLoading, rowCount });
                            onDelete();
                        }}
                        disabled={isLoading}
                        style={{
                            padding: '8px 16px',
                            fontSize: 13,
                            background: 'transparent',
                            color: isLoading ? '#888' : '#d44',
                            border: `1px solid ${isLoading ? '#888' : '#d44'}`,
                            borderRadius: 6,
                            cursor: isLoading ? 'not-allowed' : 'pointer',
                            opacity: isLoading ? 0.6 : 1,
                        }}
                    >
                        {isLoading ? (
                            <>
                                <i className="fa-solid fa-spinner fa-spin" style={{ marginRight: '8px' }} />
                                Deleting...
                            </>
                        ) : (
                            'Delete'
                        )}
                    </button>
                    <div style={{ display: 'flex', gap: 10 }}>
                        <button
                            onClick={onClose}
                            disabled={isLoading}
                            style={{
                                padding: '8px 16px',
                                fontSize: 13,
                                background: 'transparent',
                                color: isLoading ? '#888' : '#ccc',
                                border: `1px solid ${isLoading ? '#888' : '#555'}`,
                                borderRadius: 6,
                                cursor: isLoading ? 'not-allowed' : 'pointer',
                                opacity: isLoading ? 0.6 : 1,
                            }}
                        >
                            Cancel
                        </button>
                        <button
                            onClick={handleSave}
                            disabled={isLoading}
                            style={{
                                padding: '8px 16px',
                                fontSize: 13,
                                background: isLoading ? '#004499' : '#0066cc',
                                color: '#fff',
                                border: 'none',
                                borderRadius: 6,
                                cursor: isLoading ? 'not-allowed' : 'pointer',
                                fontWeight: 500,
                                opacity: isLoading ? 0.8 : 1,
                            }}
                        >
                            {isLoading ? (
                                <>
                                    <i className="fa-solid fa-spinner fa-spin" style={{ marginRight: '8px' }} />
                                    Saving...
                                </>
                            ) : (
                                'Save'
                            )}
                        </button>
                    </div>
                </div>
            </div>
        </Dialog>
    );
}
