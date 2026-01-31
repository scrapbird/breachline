import React from 'react';

interface LoadingOverlayProps {
    show: boolean;
    title: string;
    message: string;
}

const LoadingOverlay: React.FC<LoadingOverlayProps> = ({ show, title, message }) => {
    if (!show) return null;

    return (
        <div className="modal-overlay">
            <div
                className="modal-content"
                style={{ maxWidth: '400px', textAlign: 'center', padding: '40px' }}
            >
                <div style={{ marginBottom: '20px' }}>
                    <div
                        style={{
                            width: '48px',
                            height: '48px',
                            border: '4px solid #333',
                            borderTop: '4px solid #4a9eff',
                            borderRadius: '50%',
                            margin: '0 auto',
                            animation: 'spinner-rotate 1s linear infinite'
                        }}
                    />
                </div>
                <h2 style={{ marginBottom: '12px', fontSize: '18px' }}>{title}</h2>
                <p style={{ color: '#aaa', fontSize: '14px', margin: 0 }}>
                    {message}
                </p>
            </div>
        </div>
    );
};

export default LoadingOverlay;
