import React, { useRef, useEffect } from 'react';
import AnnotationPanel, { AnnotationInfo } from './AnnotationPanel';
import SearchPanel, { SearchResultInfo } from './SearchPanel';

export type ResultsPanelTab = 'annotations' | 'search';

export interface ResultsPanelProps {
  // Visibility
  show: boolean;
  showAnnotationTab: boolean;  // Whether annotations tab is enabled
  showSearchTab: boolean;      // Whether search tab is enabled
  activeTab: ResultsPanelTab;
  onTabChange: (tab: ResultsPanelTab) => void;
  
  // Sizing
  height: number;
  onHeightChange: (h: number) => void;
  
  // Annotation panel props
  annotations: AnnotationInfo[];
  onAnnotationClick: (displayIndex: number) => void;
  onEditAnnotation: (displayIndex: number) => void;
  
  // Search panel props
  searchResults: SearchResultInfo[];
  searchTotalCount: number;
  searchCurrentPage: number;
  searchPageSize: number;
  searchTerm: string;
  searchIsRegex: boolean;
  isSearching: boolean;
  onSearchResultClick: (rowIndex: number, columnIndex: number, columnName: string) => void;
  onSearchPageChange: (page: number) => void;
  onClearSearch: () => void;
  onSearch: (term: string, isRegex: boolean) => void;
  onCancelSearch: () => void;
}

const ResultsPanel: React.FC<ResultsPanelProps> = ({
  show,
  showAnnotationTab,
  showSearchTab,
  activeTab,
  onTabChange,
  height,
  onHeightChange,
  annotations,
  onAnnotationClick,
  onEditAnnotation,
  searchResults,
  searchTotalCount,
  searchCurrentPage,
  searchPageSize,
  searchTerm,
  searchIsRegex,
  isSearching,
  onSearchResultClick,
  onSearchPageChange,
  onClearSearch,
  onSearch,
  onCancelSearch,
}) => {
  const panelBodyRef = useRef<HTMLDivElement | null>(null);
  const isResizingRef = useRef<boolean>(false);
  const startYRef = useRef<number>(0);
  const startHRef = useRef<number>(0);

  // Filter to only show annotations that are visible in current view (displayRowIndex >= 0)
  const visibleAnnotations = annotations
    .filter(a => a.displayRowIndex >= 0)
    .sort((a, b) => a.displayRowIndex - b.displayRowIndex);

  // A tab is displayed if the user has enabled it (via keybind or action).
  // Don't hide tabs based on content - the content area handles empty states gracefully.
  // This prevents tabs from disappearing when switching between them.
  const displayAnnotationsTab = showAnnotationTab;
  const displaySearchTab = showSearchTab;

  const onResizeStart = (e: React.MouseEvent<HTMLDivElement>) => {
    isResizingRef.current = true;
    startYRef.current = e.clientY;
    startHRef.current = height;
    window.addEventListener('mousemove', onResizeMove, true);
    window.addEventListener('mouseup', onResizeEnd, true);
  };

  const onResizeMove = (e: MouseEvent) => {
    if (!isResizingRef.current) return;
    const dy = startYRef.current - e.clientY; // dragging up increases panel height
    const next = Math.max(80, startHRef.current + dy);
    onHeightChange(next);
  };

  const onResizeEnd = () => {
    isResizingRef.current = false;
    window.removeEventListener('mousemove', onResizeMove, true);
    window.removeEventListener('mouseup', onResizeEnd, true);
  };

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      window.removeEventListener('mousemove', onResizeMove, true);
      window.removeEventListener('mouseup', onResizeEnd, true);
    };
  }, []);

  if (!show) return null;

  return (
    <>
      <div 
        className="resizer-h annotation-panel-resizer" 
        onMouseDown={onResizeStart} 
        title="Drag to resize panel" 
      />
      <div className="results-panel" style={{ height }}>
        {/* Tab bar - only show if both tabs are displayed */}
        {(displayAnnotationsTab && displaySearchTab) && (
          <div className="results-panel-tabs">
            <button
              className={`results-panel-tab ${activeTab === 'annotations' ? 'active' : ''}`}
              onClick={() => onTabChange('annotations')}
            >
              <i className="fa-solid fa-bookmark" style={{ marginRight: '6px' }} />
              Annotations
              <span className="results-panel-tab-count">({visibleAnnotations.length})</span>
            </button>
            <button
              className={`results-panel-tab ${activeTab === 'search' ? 'active' : ''}`}
              onClick={() => onTabChange('search')}
            >
              <i className="fa-solid fa-magnifying-glass" style={{ marginRight: '6px' }} />
              Search
              <span className="results-panel-tab-count">({searchTotalCount.toLocaleString()})</span>
            </button>
          </div>
        )}

        {/* Panel content */}
        <div className="results-panel-content" ref={panelBodyRef}>
          {activeTab === 'annotations' ? (
            <div className="annotation-panel-inner">
              <div className="annotation-panel-header">
                <div className="annotation-panel-title">
                  Annotations
                  <span className="annotation-count">({visibleAnnotations.length})</span>
                </div>
              </div>
              <div className="annotation-panel-body">
                {visibleAnnotations.length === 0 ? (
                  <div className="annotation-empty">No annotations in current view</div>
                ) : (
                  <table className="annotation-table">
                    <thead>
                      <tr>
                        <th className="annotation-col-index">Index</th>
                        <th className="annotation-col-note">Note</th>
                        <th className="annotation-col-actions"></th>
                      </tr>
                    </thead>
                    <tbody>
                      {visibleAnnotations.map((annot, idx) => {
                        const colorClasses: Record<string, string> = {
                          blue: 'annotation-color-blue',
                          green: 'annotation-color-green',
                          yellow: 'annotation-color-yellow',
                          orange: 'annotation-color-orange',
                          red: 'annotation-color-red',
                          grey: 'annotation-color-grey',
                          gray: 'annotation-color-grey',
                          white: 'annotation-color-white',
                        };
                        const colorClass = colorClasses[annot.color.toLowerCase()] || 'annotation-color-grey';
                        return (
                          <tr
                            key={`${annot.originalRowIndex}-${idx}`}
                            className={`annotation-row ${colorClass}`}
                            onClick={() => onAnnotationClick(annot.displayRowIndex)}
                            title={annot.note || '(no note)'}
                          >
                            <td className="annotation-col-index">{annot.displayRowIndex + 1}</td>
                            <td className="annotation-col-note">
                              <span className="annotation-note-text">
                                {annot.note || <em className="annotation-no-note">(no note)</em>}
                              </span>
                            </td>
                            <td className="annotation-col-actions">
                              <button
                                className="annotation-edit-btn"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  onEditAnnotation(annot.displayRowIndex);
                                }}
                                title="Edit annotation"
                              >
                                <i className="fa-solid fa-pen-to-square" />
                              </button>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
          ) : (
            <SearchPanel
              results={searchResults}
              totalCount={searchTotalCount}
              currentPage={searchCurrentPage}
              pageSize={searchPageSize}
              searchTerm={searchTerm}
              isRegex={searchIsRegex}
              isSearching={isSearching}
              onResultClick={onSearchResultClick}
              onPageChange={onSearchPageChange}
              onClearSearch={onClearSearch}
              onSearch={onSearch}
              onCancelSearch={onCancelSearch}
            />
          )}
        </div>
      </div>
    </>
  );
};

export default ResultsPanel;
