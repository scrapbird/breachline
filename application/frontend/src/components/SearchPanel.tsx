import React, { useState, useCallback, useRef, useEffect } from 'react';

export interface SearchResultInfo {
  rowIndex: number;
  columnIndex: number;
  columnName: string;
  matchStart: number;
  matchEnd: number;
  snippet: string;
}

export interface SearchPanelProps {
  results: SearchResultInfo[];
  totalCount: number;
  currentPage: number;
  pageSize: number;
  searchTerm: string;
  isRegex: boolean;
  isSearching: boolean;
  onResultClick: (rowIndex: number, columnIndex: number, columnName: string) => void;
  onPageChange: (page: number) => void;
  onClearSearch: () => void;
  onSearch: (term: string, isRegex: boolean) => void;
  onCancelSearch: () => void;
}

const SearchPanel: React.FC<SearchPanelProps> = ({
  results,
  totalCount,
  currentPage,
  pageSize,
  searchTerm: appliedSearchTerm,
  isRegex: appliedIsRegex,
  isSearching,
  onResultClick,
  onPageChange,
  onClearSearch,
  onSearch,
  onCancelSearch,
}) => {
  const [inputValue, setInputValue] = useState(appliedSearchTerm);
  const [useRegex, setUseRegex] = useState(appliedIsRegex);
  const inputRef = useRef<HTMLInputElement>(null);
  
  const totalPages = Math.ceil(totalCount / pageSize);
  const startIndex = currentPage * pageSize + 1;
  const endIndex = Math.min((currentPage + 1) * pageSize, totalCount);
  
  // Focus input when panel opens
  useEffect(() => {
    inputRef.current?.focus();
  }, []);
  
  // Update input when applied search term changes (e.g., after clear)
  useEffect(() => {
    if (!appliedSearchTerm) {
      setInputValue('');
    }
  }, [appliedSearchTerm]);
  
  const handleSubmit = useCallback((e?: React.FormEvent) => {
    e?.preventDefault();
    if (inputValue.trim() && !isSearching) {
      onSearch(inputValue.trim(), useRegex);
    }
  }, [inputValue, useRegex, isSearching, onSearch]);
  
  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSubmit();
    }
  }, [handleSubmit]);

  const hasResults = totalCount > 0;

  return (
    <div className="search-panel-content">
      {/* Search input bar */}
      <div className="search-panel-input-bar">
        <div className="search-input-wrapper">
          <i className="fa-solid fa-magnifying-glass search-input-icon" />
          <input
            ref={inputRef}
            type="text"
            className="search-input"
            placeholder="Search in grid..."
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={isSearching}
          />
          <button
            className={`search-regex-toggle ${useRegex ? 'active' : ''}`}
            onClick={() => setUseRegex(!useRegex)}
            title="Use regular expression"
            disabled={isSearching}
          >
            .*
          </button>
        </div>
        
        {/* Match count indicator */}
        {hasResults && !isSearching && (
          <span className="search-count">
            {totalCount.toLocaleString()} match{totalCount !== 1 ? 'es' : ''}
          </span>
        )}
        
        {/* Search / Cancel button */}
        {isSearching ? (
          <button
            className="search-action-btn search-cancel-btn"
            onClick={onCancelSearch}
            title="Cancel search"
          >
            <i className="fa-solid fa-stop" />
          </button>
        ) : (
          <button
            className="search-action-btn"
            onClick={() => handleSubmit()}
            disabled={!inputValue.trim()}
            title="Search (Enter)"
          >
            <i className="fa-solid fa-arrow-right" />
          </button>
        )}
        
        {/* Clear button - only show when there are results */}
        {hasResults && !isSearching && (
          <button
            className="search-action-btn"
            onClick={onClearSearch}
            title="Clear search results"
          >
            <i className="fa-solid fa-xmark" />
          </button>
        )}
      </div>

      {/* Results area */}
      <div className="search-panel-body">
        {isSearching ? (
          <div className="search-loading">
            <i className="fa-solid fa-spinner fa-spin" />
            <span>Searching...</span>
          </div>
        ) : !hasResults && appliedSearchTerm ? (
          <div className="search-empty">No matches found for "{appliedSearchTerm}"</div>
        ) : !hasResults ? (
          <div className="search-empty search-placeholder">
            <span>Enter a search term above</span>
          </div>
        ) : (
          <table className="search-table">
            <thead>
              <tr>
                <th className="search-col-index">Row</th>
                <th className="search-col-column">Column</th>
                <th className="search-col-snippet">Match</th>
              </tr>
            </thead>
            <tbody>
              {results.map((result, idx) => (
                <tr
                  key={`${result.rowIndex}-${result.columnIndex}-${idx}`}
                  className="search-row"
                  onClick={() => onResultClick(result.rowIndex, result.columnIndex, result.columnName)}
                  title={`Row ${result.rowIndex + 1}, Column: ${result.columnName}`}
                >
                  <td className="search-col-index">{result.rowIndex + 1}</td>
                  <td className="search-col-column">{result.columnName}</td>
                  <td className="search-col-snippet">
                    <span className="search-snippet-text">
                      {result.snippet}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="search-pagination">
          <button
            className="search-page-btn"
            onClick={() => onPageChange(0)}
            disabled={currentPage === 0}
            title="First page"
          >
            <i className="fa-solid fa-angles-left" />
          </button>
          <button
            className="search-page-btn"
            onClick={() => onPageChange(currentPage - 1)}
            disabled={currentPage === 0}
            title="Previous page"
          >
            <i className="fa-solid fa-angle-left" />
          </button>
          <span className="search-page-info">
            {startIndex.toLocaleString()}–{endIndex.toLocaleString()} of {totalCount.toLocaleString()}
          </span>
          <button
            className="search-page-btn"
            onClick={() => onPageChange(currentPage + 1)}
            disabled={currentPage >= totalPages - 1}
            title="Next page"
          >
            <i className="fa-solid fa-angle-right" />
          </button>
          <button
            className="search-page-btn"
            onClick={() => onPageChange(totalPages - 1)}
            disabled={currentPage >= totalPages - 1}
            title="Last page"
          >
            <i className="fa-solid fa-angles-right" />
          </button>
        </div>
      )}
    </div>
  );
};

export default SearchPanel;
