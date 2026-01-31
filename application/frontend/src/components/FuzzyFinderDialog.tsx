import React, { useState, useEffect, useRef } from 'react';
import Fuse from 'fuse.js';
import './FuzzyFinderDialog.css';
import { FileOptions } from '../types/FileOptions';

export interface FileItem {
  id: string;
  path: string;
  isOpen: boolean; // true if already open tab
  fileOptions?: FileOptions;
}

// Helper to extract city name from timezone string (e.g., "Australia/Sydney" -> "Sydney")
const getCityFromTimezone = (tz: string): string => {
  if (!tz) return '';
  const parts = tz.split('/');
  return parts.length > 1 ? parts[parts.length - 1].replace(/_/g, ' ') : tz;
};

interface FuzzyFinderDialogProps {
  show: boolean;
  onClose: () => void;
  files: FileItem[];
  onSelect: (file: FileItem) => void;
}

const FuzzyFinderDialog: React.FC<FuzzyFinderDialogProps> = ({ show, onClose, files, onSelect }) => {
  const [query, setQuery] = useState<string>('');
  const [selectedIndex, setSelectedIndex] = useState<number>(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const resultsRef = useRef<HTMLDivElement>(null);

  // Configure Fuse.js for fuzzy matching on full path
  const fuse = useRef<Fuse<FileItem>>(
    new Fuse(files, {
      keys: ['path'],
      threshold: 0.4,
      ignoreLocation: true,
      includeScore: true,
    })
  );

  // Update fuse index when files change
  useEffect(() => {
    fuse.current = new Fuse(files, {
      keys: ['path'],
      threshold: 0.4,
      ignoreLocation: true,
      includeScore: true,
    });
  }, [files]);

  // Get filtered results
  const results = query.trim() === ''
    ? files
    : fuse.current.search(query).map(result => result.item);

  // Reset state when dialog opens
  useEffect(() => {
    if (show) {
      setQuery('');
      setSelectedIndex(0);
      inputRef.current?.focus();
    }
  }, [show]);

  // Reset selected index when query changes
  useEffect(() => {
    setSelectedIndex(0);
  }, [query]);

  // Handle keyboard navigation
  useEffect(() => {
    if (!show) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        onClose();
      } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        setSelectedIndex(prev => Math.min(prev + 1, results.length - 1));
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        setSelectedIndex(prev => Math.max(prev - 1, 0));
      } else if (e.key === 'Enter') {
        e.preventDefault();
        if (results.length > 0 && selectedIndex >= 0 && selectedIndex < results.length) {
          onSelect(results[selectedIndex]);
          onClose();
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [show, onClose, results, selectedIndex, onSelect]);

  // Scroll selected item into view
  useEffect(() => {
    if (!show || !resultsRef.current) return;

    const selectedElement = resultsRef.current.querySelector(`[data-index="${selectedIndex}"]`);
    if (selectedElement) {
      selectedElement.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
    }
  }, [selectedIndex, show]);

  if (!show) return null;

  return (
    <div className="fuzzy-finder-overlay" onClick={onClose}>
      <div className="fuzzy-finder-dialog" onClick={(e) => e.stopPropagation()}>
        <input
          ref={inputRef}
          type="text"
          className="fuzzy-finder-input"
          placeholder="Type to search files..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <div className="fuzzy-finder-results" ref={resultsRef}>
          {results.length === 0 ? (
            <div className="fuzzy-finder-no-results">No files found</div>
          ) : (
            results.map((file, index) => (
              <div
                key={file.id}
                data-index={index}
                className={`fuzzy-finder-item ${index === selectedIndex ? 'selected' : ''} ${file.isOpen ? 'open' : ''}`}
                onClick={() => {
                  onSelect(file);
                  onClose();
                }}
                onMouseEnter={() => setSelectedIndex(index)}
              >
                <span className="fuzzy-finder-path"><span className="fuzzy-finder-path-inner">{file.path}</span></span>
                <span className="fuzzy-finder-badges">
                  {file.fileOptions?.jpath && (
                    <span className="fuzzy-finder-option-badge fuzzy-finder-badge-jpath" title={`JPath: ${file.fileOptions.jpath}`}>JP</span>
                  )}
                  {file.fileOptions?.noHeaderRow && (
                    <span className="fuzzy-finder-option-badge fuzzy-finder-badge-noheader" title="No Header Row">NH</span>
                  )}
                  {file.fileOptions?.ingestTimezoneOverride && (
                    <span className="fuzzy-finder-option-badge fuzzy-finder-badge-tz" title={`Timezone: ${file.fileOptions.ingestTimezoneOverride}`}>
                      {getCityFromTimezone(file.fileOptions.ingestTimezoneOverride)}
                    </span>
                  )}
                  {file.isOpen && <span className="fuzzy-finder-badge">Open</span>}
                </span>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
};

export default FuzzyFinderDialog;
