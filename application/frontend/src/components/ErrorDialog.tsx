import React from 'react';

interface ErrorDialogProps {
  isOpen: boolean;
  message: string;
  onClose: () => void;
}

const ErrorDialog: React.FC<ErrorDialogProps> = ({ isOpen, message, onClose }) => {
  if (!isOpen) return null;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div 
        className="modal-content" 
        onClick={(e) => e.stopPropagation()}
        style={{ maxWidth: '450px' }}
      >
        <div className="modal-header">
          <h2>Error</h2>
          <button onClick={onClose} className="close-button">
            <i className="fa-solid fa-xmark" />
          </button>
        </div>
        <div className="modal-body">
          <div style={{ 
            display: 'flex', 
            alignItems: 'flex-start', 
            gap: '12px',
            marginBottom: '20px' 
          }}>
            <i 
              className="fa-solid fa-circle-exclamation" 
              style={{ color: '#ff6b6b', fontSize: '24px', marginTop: '2px' }} 
            />
            <p style={{ 
              fontSize: '14px', 
              lineHeight: '1.5', 
              color: '#ddd',
              margin: 0,
              wordBreak: 'break-word'
            }}>
              {message}
            </p>
          </div>
          <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <button
              onClick={onClose}
              style={{
                padding: '8px 20px',
                fontSize: '14px',
                background: '#3a3a3a',
                border: '1px solid #555',
                borderRadius: '4px',
                color: '#ddd',
                cursor: 'pointer',
                fontWeight: '500',
              }}
            >
              OK
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default ErrorDialog;
