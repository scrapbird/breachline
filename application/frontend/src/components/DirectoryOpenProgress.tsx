import React, { useRef } from 'react';

// OpenProgress is one phase of a load, as reported by the backend's load:progress
// event. Total is -1 for work whose size is not knowable in advance: the file count
// before a scan finishes, or a parse that is one opaque call into a library.
export interface OpenProgress {
    kind: string;
    phase: string;
    current: number;
    total: number;
    message: string;
}

// Phase names are part of the event contract with the loader.
export const PHASE_LABELS: Record<string, string> = {
    // Directory phases
    discovering: 'Scanning directory',
    hashing: 'Hashing files',
    schema: 'Reading columns',
    loading: 'Loading rows',
    // Single-file phases
    reading: 'Reading file',
    decompressing: 'Decompressing',
    parsing: 'Parsing',
    mapping: 'Building rows',
    preparing: 'Preparing rows',
};

// A load runs through several phases, each of which counts something different:
// bytes off disk, then rows built, then rows prepared. Reporting each phase's own
// percentage made the bar restart from zero at every handover, and an unmeasurable
// phase drawn as a full-width bar read as "nearly done" right before it did so.
// Instead every phase owns a slice of one bar, sized by how much of the wall clock
// that phase actually takes. Two measured JSON loads split read/parse/build as
// 5/55/40 and 10/29/61, and a gzipped one 19/42/38, so the weights below are a
// compromise across them rather than a fit to any one file.
interface PhaseSegment {
    start: number;
    end: number;
}

// reading and decompressing share a slice because a file is either compressed or
// not, so only one of them ever runs.
const FILE_SEGMENTS: Record<string, PhaseSegment> = {
    reading: { start: 0, end: 12 },
    decompressing: { start: 0, end: 20 },
    parsing: { start: 20, end: 50 },
    mapping: { start: 50, end: 88 },
    preparing: { start: 88, end: 100 },
};

// Reading the rows dominates a directory load and the scan barely registers next to
// it: opening a 145,780 file archive took 0.9s to scan and resolve columns and 37.5s
// to read, so the scan phases get a narrow slice at the front. hashing only appears
// when content hashing is turned on.
const DIRECTORY_SEGMENTS: Record<string, PhaseSegment> = {
    discovering: { start: 0, end: 5 },
    hashing: { start: 5, end: 10 },
    schema: { start: 10, end: 15 },
    loading: { start: 15, end: 100 },
};

const segmentFor = (kind: string, phase: string): PhaseSegment | null => {
    const segments = kind === 'directory' ? DIRECTORY_SEGMENTS : FILE_SEGMENTS;
    return segments[phase] || null;
};

// The band that sweeps across the unfinished part of the current segment while a
// phase runs that cannot say how far through it is.
const SHIMMER_KEYFRAMES = `@keyframes blOpenShimmer {
    0% { transform: translateX(-110%); }
    100% { transform: translateX(340%); }
}`;

interface DirectoryOpenProgressProps {
    // show is owned by the open flow rather than derived from progress events, so a
    // phase event that never gets a matching completion cannot strand the dialog.
    show: boolean;
    progress: OpenProgress | null;
    onCancel: () => void;
    // Only directory loads are cancellable: a single file is parsed by one
    // uninterruptible library call, so a Cancel button there would do nothing.
    cancellable?: boolean;
}

// DirectoryOpenProgress reports what a directory or file open is doing and offers a
// way out of it. Opening a large archive scans every file, resolves the column
// schema from a sample of them and then loads every row, which can run for minutes;
// without this the window simply stops responding to the user with no indication of
// progress and no way to abandon the load short of killing the app.
const DirectoryOpenProgress: React.FC<DirectoryOpenProgressProps> = ({ show, progress, onCancel, cancellable = true }) => {
    // The bar only ever moves forward. Phases are reported in order but each one
    // counts from zero, and some work is done twice (the header is resolved at open,
    // then the rows are read on the first query), so the raw numbers go backwards
    // where the load as a whole does not.
    const highWater = useRef(0);
    const wasShown = useRef(false);

    // Reset during render rather than in an effect: an effect runs after the first
    // paint, which would show the previous load's finished bar for a frame.
    if (show && !wasShown.current) {
        highWater.current = 0;
    }
    wasShown.current = show;

    if (!show) {
        return null;
    }

    const label = progress ? (PHASE_LABELS[progress.phase] || progress.phase) : 'Starting';
    const segment = progress ? segmentFor(progress.kind, progress.phase) : null;
    const measured = !!progress && !!segment && progress.total > 0;

    if (progress && segment) {
        const fraction = measured
            ? Math.min(1, Math.max(0, progress.current / progress.total))
            : 0;
        const target = segment.start + (segment.end - segment.start) * fraction;
        if (target > highWater.current) {
            highWater.current = target;
        }
    }

    const percent = Math.round(highWater.current);
    // An unmeasurable phase sweeps a band across the rest of its own segment, so it
    // shows that work is happening without claiming ground it has not covered.
    const shimmerFrom = percent;
    const shimmerWidth = segment && !measured ? Math.max(6, segment.end - shimmerFrom) : 0;

    // The heading follows what is actually happening: the rows are read after the
    // scan finishes, so claiming the directory is still being "opened" throughout
    // would be wrong, and a single file is not a directory at all.
    // Before the first phase arrives the kind is not known yet, so stay neutral
    // rather than guessing wrong for a second.
    const isFile = progress?.kind === 'file';
    const loadingRows = progress?.phase === 'loading' || progress?.phase === 'mapping'
        || progress?.phase === 'preparing';
    let heading = 'Opening';
    if (progress) {
        if (isFile) {
            heading = loadingRows ? 'Loading file' : 'Opening file';
        } else {
            heading = loadingRows ? 'Loading directory' : 'Opening directory';
        }
    }

    return (
        <div
            className="modal-overlay"
            style={{ zIndex: 3000 }}
            // No click-to-dismiss: the work continues in the background, so the only
            // way to stop it is the explicit Cancel below.
            onClick={(e) => e.stopPropagation()}
        >
            <style>{SHIMMER_KEYFRAMES}</style>
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
                    {!!progress && <span style={{ opacity: 0.65 }}> ({percent}%)</span>}
                </div>

                {progress?.message && (
                    <div style={{ fontSize: '12px', opacity: 0.7, marginBottom: '12px' }}>
                        {progress.message}
                    </div>
                )}

                <div
                    style={{
                        position: 'relative',
                        height: '6px',
                        borderRadius: '3px',
                        backgroundColor: '#2a2a2a',
                        overflow: 'hidden',
                        marginBottom: '16px',
                    }}
                >
                    <div
                        style={{
                            position: 'absolute',
                            top: 0,
                            bottom: 0,
                            left: 0,
                            width: `${percent}%`,
                            backgroundColor: '#4a90d9',
                            transition: 'width 120ms linear',
                        }}
                    />
                    {shimmerWidth > 0 && (
                        <div
                            style={{
                                position: 'absolute',
                                top: 0,
                                bottom: 0,
                                left: `${shimmerFrom}%`,
                                width: `${shimmerWidth}%`,
                                overflow: 'hidden',
                            }}
                        >
                            <div
                                style={{
                                    position: 'absolute',
                                    top: 0,
                                    bottom: 0,
                                    width: '30%',
                                    background: 'linear-gradient(90deg, rgba(74,144,217,0), rgba(74,144,217,0.55), rgba(74,144,217,0))',
                                    animation: 'blOpenShimmer 1.2s linear infinite',
                                }}
                            />
                        </div>
                    )}
                </div>

                {cancellable && (
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
                )}
            </div>
        </div>
    );
};

export default DirectoryOpenProgress;
