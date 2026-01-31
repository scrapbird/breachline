import React, { useEffect } from 'react';

interface DialogProps {
    show: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
    maxWidth?: number | string;
    /** If true, dialog is positioned absolute within its parent container instead of fixed to viewport */
    contained?: boolean;
}

const Dialog: React.FC<DialogProps> = ({ show, onClose, title, children, maxWidth = 700, contained = false }) => {
    // Handle Escape key to close dialog
    useEffect(() => {
        if (!show) return;
        
        const handleEscape = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.preventDefault();
                onClose();
            }
        };
        
        document.addEventListener('keydown', handleEscape);
        return () => document.removeEventListener('keydown', handleEscape);
    }, [show, onClose]);
    
    if (!show) return null;
    
    const overlayStyle: React.CSSProperties = contained ? {
        position: 'absolute',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        background: 'rgba(0, 0, 0, 0.7)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 10000,
    } : {};
    
    const contentStyle: React.CSSProperties = {
        maxWidth: typeof maxWidth === 'number' ? `${maxWidth}px` : maxWidth,
        maxHeight: contained ? '90%' : '85vh',
        display: 'flex',
        flexDirection: 'column',
    };
    
    return (
        <div 
            className={contained ? undefined : "modal-overlay"} 
            style={overlayStyle}
            onClick={onClose}
        >
            <div 
                className="modal-content" 
                onClick={(e) => e.stopPropagation()} 
                style={contentStyle}
            >
                <div className="modal-header">
                    <h2>{title}</h2>
                    <button onClick={onClose} className="close-button">
                        <i className="fa-solid fa-xmark" />
                    </button>
                </div>
                
                <div className="modal-body" style={{ 
                    overflowY: contained ? 'hidden' : 'auto', 
                    flex: 1, 
                    minHeight: 0,
                    display: contained ? 'flex' : undefined,
                    flexDirection: contained ? 'column' : undefined,
                }}>
                    {children}
                </div>
            </div>
        </div>
    );
};

export default Dialog;
