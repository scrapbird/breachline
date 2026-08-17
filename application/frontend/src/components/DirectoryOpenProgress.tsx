import React from 'react';

// OpenProgress is one phase of a directory open, as reported by the backend's
// directory:open:progress event. Total is -1 while the size of the phase is not yet
// known (the file count is only known once the scan finishes).
export interface OpenProgress {
    phase: string;
    current: number;
    total: number;
    message: string;
}

// Phase names are part of the event contract with the loader.
export const PHASE_LABELS: Record<string, string> = {
    discovering: 'Scanning directory',
    hashing: 'Hashing files',
    schema: 'Reading columns',
    loading: 'Loading rows',
};

interface DirectoryOpenProgressProps {
    // show is owned by the open flow rather than derived from progress events, so a
    // phase event that never gets a matching completion cannot strand the dialog.
    show: boolean;
    progress: OpenProgress | null;
    onCancel: () => void;
}

// DirectoryOpenProgress reports what a directory open is doing and offers a way out
// of it. Opening a large archive scans every file, resolves the column schema from a
// sample of them and then loads every row, which can run for minutes; without this
// the window simply stops responding to the user with no indication of progress and
// no way to abandon the load short of killing the app.
const DirectoryOpenProgress: React.FC<DirectoryOpenProgressProps> = ({ show, progress, onCancel }) => {
    if (!show) {
        return null;
    }

    const label = progress ? (PHASE_LABELS[progress.phase] || progress.phase) : 'Starting';
    const hasTotal = !!progress && progress.total > 0;
    const percent = hasTotal && progress
        ? Math.min(100, Math.round((progress.current / progress.total) * 100))
        : 0;

    // The rows are read after the scan finishes, so the heading follows the phase
    // rather than claiming the directory is still being opened throughout.
    const heading = progress?.phase === 'loading' ? 'Loading directory' : 'Opening directory';

    return (
        <div
            className="modal-overlay"
            style={{ zIndex: 3000 }}
            // No click-to-dismiss: the work continues in the background, so the only
            // way to stop it is the explicit Cancel below.
            onClick={(e) => e.stopPropagation()}
        >
            <div
                className="modal-content"
                onClick={(e) => e.stopPropagation()}
                style={{ maxWidth: '460px', padding: '20px' }}
            >
                <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '10px' }}>
                    {heading}
                </div>

                <div style={{ fontSize: '13px', marginBottom: '6px' }}>
                    {label}
                    {hasTotal && <span style={{ opacity: 0.65 }}> ({percent}%)</span>}
                </div>

                {progress?.message && (
                    <div style={{ fontSize: '12px', opacity: 0.7, marginBottom: '12px' }}>
                        {progress.message}
                    </div>
                )}

                <div
                    style={{
                        height: '6px',
                        borderRadius: '3px',
                        backgroundColor: '#2a2a2a',
                        overflow: 'hidden',
                        marginBottom: '16px',
                    }}
                >
                    <div
                        style={{
                            height: '100%',
                            // An indeterminate phase (total unknown) shows a full bar of
                            // muted colour rather than pretending to know how far along it is.
                            width: hasTotal ? `${percent}%` : '100%',
                            backgroundColor: hasTotal ? '#4a90d9' : '#3a3a3a',
                            transition: 'width 120ms linear',
                        }}
                    />
                </div>

                <button
                    onClick={onCancel}
                    style={{
                        padding: '8px 14px',
                        border: '1px solid #555',
                        borderRadius: '6px',
                        backgroundColor: '#333',
                        color: '#fff',
                        fontSize: '13px',
                        cursor: 'pointer',
                    }}
                >
                    Cancel
                </button>
            </div>
        </div>
    );
};

export default DirectoryOpenProgress;
