import React, { useState, useEffect, useRef } from 'react';
import Dialog from './Dialog';

export interface FindDialogProps {
  show: boolean;
  onClose: () => void;
  onSearch: (searchTerm: string, isRegex: boolean) => void;
  initialSearchTerm?: string;
  initialIsRegex?: boolean;
  isSearching?: boolean;
  onCancel?: () => void;
}

const FindDialog: React.FC<FindDialogProps> = ({
  show,
  onClose,
  onSearch,
  initialSearchTerm = '',
  initialIsRegex = false,
  isSearching = false,
  onCancel,
}) => {
  const [searchTerm, setSearchTerm] = useState(initialSearchTerm);
  const [isRegex, setIsRegex] = useState(initialIsRegex);
  const [regexError, setRegexError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Reset state when dialog opens
  useEffect(() => {
    if (show) {
      setSearchTerm(initialSearchTerm);
      setIsRegex(initialIsRegex);
      setRegexError(null);
      // Focus the input after a short delay to ensure dialog is rendered
      setTimeout(() => {
        inputRef.current?.focus();
        inputRef.current?.select();
      }, 50);
    }
  }, [show, initialSearchTerm, initialIsRegex]);

  // Validate regex when it changes
  useEffect(() => {
    if (isRegex && searchTerm) {
      try {
        new RegExp(searchTerm);
        setRegexError(null);
      } catch (e) {
        setRegexError((e as Error).message);
      }
    } else {
      setRegexError(null);
    }
  }, [searchTerm, isRegex]);

  const handleSearch = () => {
    if (!searchTerm.trim()) return;
    if (isRegex && regexError) return;
    onSearch(searchTerm, isRegex);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !isSearching) {
      e.preventDefault();
      handleSearch();
    }
  };

  const handleCancel = () => {
    if (onCancel) {
      onCancel();
    }
  };

  return (
    <Dialog show={show} onClose={onClose} title="Find in File" maxWidth={500}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
        {/* Search input */}
        <div>
          <label 
            htmlFor="search-term" 
            style={{ 
              display: 'block', 
              marginBottom: '6px', 
              color: '#ccc',
              fontSize: '14px'
            }}
          >
            Search term:
          </label>
          <input
            ref={inputRef}
            id="search-term"
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={isRegex ? "Enter regex pattern..." : "Enter search text..."}
            disabled={isSearching}
            style={{
              width: '100%',
              padding: '10px 12px',
              fontSize: '14px',
              backgroundColor: '#2a2a2a',
              border: regexError ? '1px solid #d32f2f' : '1px solid #444',
              borderRadius: '4px',
              color: '#eee',
              outline: 'none',
              boxSizing: 'border-box',
            }}
          />
          {regexError && (
            <div style={{ color: '#f44336', fontSize: '12px', marginTop: '4px' }}>
              Invalid regex: {regexError}
            </div>
          )}
        </div>

        {/* Regex checkbox */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <input
            type="checkbox"
            id="use-regex"
            checked={isRegex}
            onChange={(e) => setIsRegex(e.target.checked)}
            disabled={isSearching}
            style={{ 
              width: '16px', 
              height: '16px',
              accentColor: '#4a9eff',
            }}
          />
          <label 
            htmlFor="use-regex" 
            style={{ 
              color: '#ccc', 
              fontSize: '14px',
              cursor: isSearching ? 'default' : 'pointer',
            }}
          >
            Use regular expression (RE2 syntax)
          </label>
        </div>

        {/* Buttons */}
        <div style={{ 
          display: 'flex', 
          justifyContent: 'flex-end', 
          gap: '10px',
          marginTop: '8px'
        }}>
          {isSearching ? (
            <button
              onClick={handleCancel}
              style={{
                padding: '10px 20px',
                fontSize: '14px',
                backgroundColor: '#d32f2f',
                color: 'white',
                border: 'none',
                borderRadius: '4px',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '8px',
              }}
            >
              <i className="fa-solid fa-stop" />
              Cancel Search
            </button>
          ) : (
            <>
              <button
                onClick={onClose}
                style={{
                  padding: '10px 20px',
                  fontSize: '14px',
                  backgroundColor: '#333',
                  color: '#ccc',
                  border: '1px solid #444',
                  borderRadius: '4px',
                  cursor: 'pointer',
                }}
              >
                Close
              </button>
              <button
                onClick={handleSearch}
                disabled={!searchTerm.trim() || !!regexError}
                style={{
                  padding: '10px 20px',
                  fontSize: '14px',
                  backgroundColor: (!searchTerm.trim() || !!regexError) ? '#333' : '#4a9eff',
                  color: (!searchTerm.trim() || !!regexError) ? '#666' : 'white',
                  border: 'none',
                  borderRadius: '4px',
                  cursor: (!searchTerm.trim() || !!regexError) ? 'not-allowed' : 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px',
                }}
              >
                <i className="fa-solid fa-magnifying-glass" />
                Search
              </button>
            </>
          )}
        </div>

        {/* Search in progress indicator */}
        {isSearching && (
          <div style={{ 
            display: 'flex', 
            alignItems: 'center', 
            justifyContent: 'center',
            gap: '10px',
            padding: '12px',
            backgroundColor: '#2a2a2a',
            borderRadius: '4px',
          }}>
            <i className="fa-solid fa-spinner fa-spin" style={{ color: '#4a9eff' }} />
            <span style={{ color: '#ccc', fontSize: '14px' }}>
              Searching...
            </span>
          </div>
        )}
      </div>
    </Dialog>
  );
};

export default FindDialog;
