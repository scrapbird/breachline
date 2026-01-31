import React, { useState, useEffect } from 'react';
import Dialog from './Dialog';

interface WorkspaceFileEditDialogProps {
  isOpen: boolean;
  filePath: string;
  jpath?: string;
  currentDescription: string;
  onClose: () => void;
  onSave: (description: string) => void;
}

const WorkspaceFileEditDialog: React.FC<WorkspaceFileEditDialogProps> = ({
  isOpen,
  filePath,
  jpath,
  currentDescription,
  onClose,
  onSave,
}) => {
  const [description, setDescription] = useState(currentDescription);

  // Update description when prop changes
  useEffect(() => {
    setDescription(currentDescription);
  }, [currentDescription, isOpen]);

  const handleSave = () => {
    onSave(description);
  };

  const handleCancel = () => {
    setDescription(currentDescription); // Reset to original
    onClose();
  };

  const getFileName = () => {
    const parts = filePath.split(/[/\\]/);
    return parts[parts.length - 1] || filePath;
  };

  return (
    <Dialog
      show={isOpen}
      title="Edit File Description"
      onClose={handleCancel}
      maxWidth="600px"
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: '16px', textAlign: 'left' }}>
        <div>
          <div style={{ fontSize: '13px', color: '#bbb', marginBottom: '4px' }}>
            File:
          </div>
          <div style={{ 
            fontSize: '14px', 
            fontWeight: '500', 
            color: '#fff',
            wordBreak: 'break-all',
            padding: '8px 12px',
            background: '#2a2a2a',
            borderRadius: '4px',
          }}>
            {getFileName()}
          </div>
          {jpath && (
            <>
              <div style={{ fontSize: '13px', color: '#bbb', marginTop: '12px', marginBottom: '4px' }}>
                JSONPath:
              </div>
              <div style={{ 
                fontSize: '12px', 
                fontFamily: 'monospace',
                color: '#9b9b9b',
                padding: '8px 12px',
                background: '#2a2a2a',
                borderRadius: '4px',
                overflowX: 'auto',
              }}>
                {jpath}
              </div>
            </>
          )}
        </div>

        <div>
          <label 
            htmlFor="description-input" 
            style={{ 
              display: 'block', 
              fontSize: '13px', 
              color: '#bbb', 
              marginBottom: '8px',
              fontWeight: '500',
            }}
          >
            Description:
          </label>
          <textarea
            id="description-input"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Enter a description for this file..."
            rows={4}
            style={{
              width: '100%',
              padding: '10px 12px',
              fontSize: '14px',
              background: '#2a2a2a',
              border: '1px solid #444',
              borderRadius: '4px',
              color: '#fff',
              resize: 'vertical',
              fontFamily: 'inherit',
              boxSizing: 'border-box',
            }}
            autoFocus
          />
          <div style={{ fontSize: '12px', color: '#888', marginTop: '4px' }}>
            Add notes about what this file contains or how it's used in your analysis.
          </div>
        </div>

        <div style={{ 
          display: 'flex', 
          gap: '12px', 
          justifyContent: 'flex-end',
          marginTop: '8px',
        }}>
          <button
            onClick={handleCancel}
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
            onMouseOver={(e) => {
              e.currentTarget.style.background = '#444';
            }}
            onMouseOut={(e) => {
              e.currentTarget.style.background = '#3a3a3a';
            }}
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            style={{
              padding: '8px 20px',
              fontSize: '14px',
              background: '#4a9eff',
              border: 'none',
              borderRadius: '4px',
              color: '#fff',
              cursor: 'pointer',
              fontWeight: '500',
            }}
            onMouseOver={(e) => {
              e.currentTarget.style.background = '#3a8eef';
            }}
            onMouseOut={(e) => {
              e.currentTarget.style.background = '#4a9eff';
            }}
          >
            Save
          </button>
        </div>
      </div>
    </Dialog>
  );
};

export default WorkspaceFileEditDialog;
