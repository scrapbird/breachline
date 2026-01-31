import React from 'react';

interface RegularHeaderProps {
    displayName: string;
    column?: any;
}

const RegularHeader: React.FC<RegularHeaderProps> = (props) => {
    const onHeaderContextMenu = props?.column?.getColDef?.()?.headerComponentParams?.onHeaderContextMenu;

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
            style={{ display: 'flex', alignItems: 'center', width: '100%' }}
            onContextMenu={handleContextMenu}
        >
            <span>{props.displayName}</span>
        </span>
    );
};

export default RegularHeader;
