import React from 'react';

interface HeaderContextMenuProps {
    visible: boolean;
    x: number;
    y: number;
    columnName: string;
    onClose: () => void;
    onSetTimestamp: (columnName: string) => void;
    onApplyJPath: (columnName: string) => void;
    onClearJPath: (columnName: string) => void;
    hasJPath: (columnName: string) => boolean;
}

const HeaderContextMenu: React.FC<HeaderContextMenuProps> = ({
    visible,
    x,
    y,
    columnName,
    onClose,
    onSetTimestamp,
    onApplyJPath,
    onClearJPath,
    hasJPath
}) => {
    if (!visible) return null;

    const handleSetTimestamp = async () => {
        // Close menu immediately for better UX
        onClose();
        // Then perform the async operation
        await onSetTimestamp(columnName);
    };

    const handleApplyJPath = async () => {
        onClose();
        await onApplyJPath(columnName);
    };

    const handleClearJPath = () => {
        onClose();
        onClearJPath(columnName);
    };

    return (
        <div
            style={{
                position: 'fixed',
                left: x,
                top: y,
                zIndex: 10000,
            }}
            onClick={(e) => e.stopPropagation()}
        >
            <div
                style={{
                    minWidth: 180,
                    background: '#222',
                    color: '#eee',
                    border: '1px solid #444',
                    borderRadius: 6,
                    boxShadow: '0 8px 20px rgba(0,0,0,0.45)',
                    padding: 6,
                    textAlign: 'left',
                }}
            >
                <div
                    role="menuitem"
                    tabIndex={0}
                    onClick={handleSetTimestamp}
                    onKeyDown={async (ev) => {
                        if (ev.key === 'Enter' || ev.key === ' ') {
                            ev.preventDefault();
                            await handleSetTimestamp();
                        }
                    }}
                    style={{ padding: '8px 10px', borderRadius: 4, cursor: 'pointer' }}
                    onMouseEnter={(e) => { (e.currentTarget as HTMLElement).style.background = '#2a2a2a'; }}
                    onMouseLeave={(e) => { (e.currentTarget as HTMLElement).style.background = 'transparent'; }}
                >
                    Set as timestamp
                </div>

                {/* Separator */}
                <div style={{ borderTop: '1px solid #444', margin: '4px 0' }} />

                {/* Apply JPath */}
                <div
                    role="menuitem"
                    tabIndex={0}
                    onClick={handleApplyJPath}
                    onKeyDown={async (ev) => {
                        if (ev.key === 'Enter' || ev.key === ' ') {
                            ev.preventDefault();
                            await handleApplyJPath();
                        }
                    }}
                    style={{ padding: '8px 10px', borderRadius: 4, cursor: 'pointer' }}
                    onMouseEnter={(e) => { (e.currentTarget as HTMLElement).style.background = '#2a2a2a'; }}
                    onMouseLeave={(e) => { (e.currentTarget as HTMLElement).style.background = 'transparent'; }}
                >
                    Apply JPath
                </div>

                {/* Clear JPath - only show if column has JPath */}
                {hasJPath(columnName) && (
                    <div
                        role="menuitem"
                        tabIndex={0}
                        onClick={handleClearJPath}
                        onKeyDown={(ev) => {
                            if (ev.key === 'Enter' || ev.key === ' ') {
                                ev.preventDefault();
                                handleClearJPath();
                            }
                        }}
                        style={{ padding: '8px 10px', borderRadius: 4, cursor: 'pointer', color: '#f85149' }}
                        onMouseEnter={(e) => { (e.currentTarget as HTMLElement).style.background = '#2a2a2a'; }}
                        onMouseLeave={(e) => { (e.currentTarget as HTMLElement).style.background = 'transparent'; }}
                    >
                        Clear JPath
                    </div>
                )}
            </div>
        </div>
    );
};

export default HeaderContextMenu;
