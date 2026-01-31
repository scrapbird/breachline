import React from 'react';

interface JPathHeaderProps {
    displayName: string;
    column?: any;
}

const JPathHeader: React.FC<JPathHeaderProps> = (props) => {
    const onHeaderContextMenu = props?.column?.getColDef?.()?.headerComponentParams?.onHeaderContextMenu;
    const jpathExpression = props?.column?.getColDef?.()?.headerComponentParams?.jpathExpression;

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
                <span title={`JPath: ${jpathExpression || '$'}`} style={{ fontSize: 11, color: '#bbb' }}>{jpathExpression || '$'}</span>
            </span>
        </span>
    );
};

export default JPathHeader;
