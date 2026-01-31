import React, { useState, useRef, useEffect } from 'react';
import Dialog from './Dialog';

interface AuthPinEntryProps {
    show: boolean;
    onClose: () => void;
    onSubmit: (pin: string) => void;
    message?: string;
    isLoading?: boolean;
}

const AuthPinEntry: React.FC<AuthPinEntryProps> = ({ 
    show, 
    onClose, 
    onSubmit, 
    message = "Enter the 6-digit PIN sent to your email",
    isLoading = false 
}) => {
    const [pin, setPin] = useState<string[]>(['', '', '', '', '', '']);
    const inputRefs = useRef<(HTMLInputElement | null)[]>([]);

    // Focus first input when dialog opens
    useEffect(() => {
        if (show && inputRefs.current[0]) {
            setTimeout(() => inputRefs.current[0]?.focus(), 100);
        }
    }, [show]);

    // Reset PIN when dialog closes
    useEffect(() => {
        if (!show) {
            setPin(['', '', '', '', '', '']);
        }
    }, [show]);

    const handleChange = (index: number, value: string) => {
        // Only allow digits
        if (value && !/^\d$/.test(value)) {
            return;
        }

        const newPin = [...pin];
        newPin[index] = value;
        setPin(newPin);

        // Auto-focus next input
        if (value && index < 5) {
            inputRefs.current[index + 1]?.focus();
        }

        // Auto-submit when all digits are entered
        if (value && index === 5 && newPin.every(d => d !== '')) {
            const fullPin = newPin.join('');
            onSubmit(fullPin);
        }
    };

    const handleKeyDown = (index: number, e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.key === 'Backspace' && !pin[index] && index > 0) {
            // Move to previous input on backspace if current is empty
            inputRefs.current[index - 1]?.focus();
        } else if (e.key === 'Enter') {
            e.preventDefault();
            const fullPin = pin.join('');
            if (fullPin.length === 6) {
                onSubmit(fullPin);
            }
        } else if (e.key === 'ArrowLeft' && index > 0) {
            e.preventDefault();
            inputRefs.current[index - 1]?.focus();
        } else if (e.key === 'ArrowRight' && index < 5) {
            e.preventDefault();
            inputRefs.current[index + 1]?.focus();
        }
    };

    const handlePaste = (e: React.ClipboardEvent) => {
        e.preventDefault();
        const pastedData = e.clipboardData.getData('text').trim();
        
        // Only accept 6 digits
        if (/^\d{6}$/.test(pastedData)) {
            const newPin = pastedData.split('');
            setPin(newPin);
            inputRefs.current[5]?.focus();
            
            // Auto-submit after paste
            setTimeout(() => {
                onSubmit(pastedData);
            }, 100);
        }
    };

    const handleSubmit = () => {
        const fullPin = pin.join('');
        if (fullPin.length === 6) {
            onSubmit(fullPin);
        }
    };

    const isComplete = pin.every(d => d !== '');

    return (
        <Dialog show={show} onClose={onClose} title="Authentication" maxWidth={500}>
            <div style={{ padding: '20px 0' }}>
                <div style={{ 
                    marginBottom: '30px', 
                    textAlign: 'center',
                    color: '#94a3b8',
                    fontSize: '14px'
                }}>
                    {message.split('\n').map((line, index) => (
                        <p key={index} style={{ 
                            margin: index === 0 ? '0 0 10px 0' : '0',
                            color: line.includes('rate limit') || line.includes('wait') ? '#f59e0b' : '#94a3b8'
                        }}>
                            {line}
                        </p>
                    ))}
                </div>

                <div style={{ 
                    display: 'flex', 
                    justifyContent: 'center', 
                    gap: '12px',
                    marginBottom: '30px'
                }}>
                    {pin.map((digit, index) => (
                        <input
                            key={index}
                            ref={el => inputRefs.current[index] = el}
                            type="text"
                            inputMode="numeric"
                            maxLength={1}
                            value={digit}
                            onChange={(e) => handleChange(index, e.target.value)}
                            onKeyDown={(e) => handleKeyDown(index, e)}
                            onPaste={handlePaste}
                            disabled={isLoading}
                            style={{
                                width: '50px',
                                height: '60px',
                                fontSize: '24px',
                                fontWeight: 'bold',
                                textAlign: 'center',
                                border: '2px solid #334155',
                                borderRadius: '8px',
                                backgroundColor: '#1e293b',
                                color: '#f1f5f9',
                                outline: 'none',
                                transition: 'all 0.2s',
                                cursor: isLoading ? 'not-allowed' : 'text',
                                opacity: isLoading ? 0.6 : 1
                            }}
                            onFocus={(e) => {
                                e.target.style.borderColor = '#3b82f6';
                                e.target.style.boxShadow = '0 0 0 3px rgba(59, 130, 246, 0.1)';
                            }}
                            onBlur={(e) => {
                                e.target.style.borderColor = '#334155';
                                e.target.style.boxShadow = 'none';
                            }}
                        />
                    ))}
                </div>

                <div style={{ 
                    display: 'flex', 
                    gap: '10px', 
                    justifyContent: 'flex-end' 
                }}>
                    <button 
                        onClick={onClose} 
                        disabled={isLoading}
                        style={{ 
                            padding: '8px 12px', 
                            borderRadius: 8, 
                            border: '1px solid #444', 
                            background: '#333', 
                            color: '#eee',
                            opacity: isLoading ? 0.6 : 1,
                            cursor: isLoading ? 'not-allowed' : 'pointer'
                        }}
                    >
                        Cancel
                    </button>
                    <button 
                        onClick={handleSubmit} 
                        disabled={!isComplete || isLoading}
                        style={{ 
                            padding: '8px 12px', 
                            borderRadius: 8, 
                            border: '1px solid #3a6', 
                            background: '#2d4733', 
                            color: '#cfe',
                            opacity: (!isComplete || isLoading) ? 0.6 : 1,
                            cursor: (!isComplete || isLoading) ? 'not-allowed' : 'pointer'
                        }}
                    >
                        {isLoading ? (
                            <>
                                <i className="fa-solid fa-spinner fa-spin" style={{ marginRight: '8px' }} />
                                Verifying...
                            </>
                        ) : (
                            'Verify'
                        )}
                    </button>
                </div>
            </div>
        </Dialog>
    );
};

export default AuthPinEntry;
