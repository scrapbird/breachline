import React from 'react';

interface TimeHeaderProps {
    displayName: string;
    column?: any;
}

const TimeHeader: React.FC<TimeHeaderProps> = (props) => {
    const paramTZ = props?.column?.getColDef?.()?.headerComponentParams?.displayTZ as string | undefined;
    const onHeaderContextMenu = props?.column?.getColDef?.()?.headerComponentParams?.onHeaderContextMenu;
    const appliedDisplayTZ = props?.column?.getColDef?.()?.headerComponentParams?.appliedDisplayTZ;

    // Prioritize the parameter from column definition over global state
    const shownTZ = (paramTZ && String(paramTZ)) || appliedDisplayTZ || 'Local';

    const handleContextMenu = (e: React.MouseEvent) => {
        if (onHeaderContextMenu) {
            e.preventDefault();
            e.stopPropagation();
            const columnName = props?.column?.getColDef?.()?.headerName || props?.column?.getColDef?.()?.field || '';
            onHeaderContextMenu(e, columnName);
        }
    };

    return (
        <span
            style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%' }}
            onContextMenu={handleContextMenu}
        >
            <span>{props.displayName}</span>
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                <span title="Display timezone" style={{ fontSize: 11, color: '#bbb' }}>{shownTZ}</span>
                <i className="fa-regular fa-clock" aria-hidden="true" title="Timestamp column" style={{ opacity: 0.9 }} />
            </span>
        </span>
    );
};

export default TimeHeader;
