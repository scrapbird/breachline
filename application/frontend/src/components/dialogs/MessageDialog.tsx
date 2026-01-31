import React from 'react';
import Dialog from '../Dialog';

interface MessageDialogProps {
    show: boolean;
    onClose: () => void;
    title: string;
    message: string;
    isError: boolean;
}

const MessageDialog: React.FC<MessageDialogProps> = ({ show, onClose, title, message, isError }) => {
    return (
        <Dialog show={show} onClose={onClose} title={title} maxWidth={500}>
            <div style={{ padding: '24px 20px', textAlign: 'center' }}>
                <div style={{
                    fontSize: 48,
                    marginBottom: 16,
                    color: isError ? '#ff6b6b' : '#51cf66'
                }}>
                    <i className={isError ? 'fa-solid fa-circle-xmark' : 'fa-solid fa-circle-check'} />
                </div>
                <div style={{ fontSize: 15, lineHeight: 1.6 }}>
                    {message}
                </div>
            </div>
        </Dialog>
    );
};

export default MessageDialog;
