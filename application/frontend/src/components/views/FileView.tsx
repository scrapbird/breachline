import React, { useRef } from 'react';
import SearchBar from '../SearchBar';
import HistogramManager from '../HistogramManager';
import GridManager from '../GridManager';
import LogoUniversal from '../../assets/images/logo-universal.png';

export interface FileViewProps {
    // Tab state
    tabState: any;
    currentTab: any;
    
    // Query and search
    queryHistory: string[];
    onQueryChange: (text: string) => void;
    onApplySearch: (text: string) => void;
    searchInputRef: React.MutableRefObject<HTMLInputElement | null>;
    
    // Histogram
    histogram: any;
    showHistogram: boolean;
    displayTimeZone: string;
    onHistogramRangeSelected: (start: number, end: number) => void;
    
    // Grid operations
    gridOps: any;
    gridContainerRef: React.MutableRefObject<HTMLDivElement | null>;
    pageSize: number;
    theme: any;
    
    // Actions
    onCopySelected: () => Promise<void>;
    onAnnotateRow: (rowIndices?: number[]) => void;
    onJumpToOriginal?: (tabId: string, rowIndex: number) => void;
    copyPending: number;
    
    // License/workspace state
    isLicensed: boolean;
    isWorkspaceOpen: boolean;
    
    // Logger
    addLog: (level: 'info' | 'warn' | 'error', message: string) => void;
}

/**
 * FileView component - Renders the file viewing area with search, histogram, and grid.
 * Extracted from App.tsx to reduce complexity.
 */
const FileView: React.FC<FileViewProps> = ({
    tabState,
    currentTab,
    queryHistory,
    onQueryChange,
    onApplySearch,
    searchInputRef,
    histogram,
    showHistogram,
    displayTimeZone,
    onHistogramRangeSelected,
    gridOps,
    gridContainerRef,
    pageSize,
    theme,
    onCopySelected,
    onAnnotateRow,
    onJumpToOriginal,
    copyPending,
    isLicensed,
    isWorkspaceOpen,
    addLog,
}) => {
    // If no current tab, show the logo screen
    if (!currentTab) {
        return (
            <div className="file-view-empty">
                <div className="content">
                    <div className="main-split" style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }}>
                        <div className="grid-wrap" ref={gridContainerRef}>
                            <div style={{ 
                                width: '100%', 
                                height: '100%', 
                                display: 'flex', 
                                flexDirection: 'column', 
                                alignItems: 'center', 
                                justifyContent: 'center', 
                                opacity: 0.9, 
                                overflow: 'hidden' 
                            }}>
                                <img src={LogoUniversal} alt="App logo" style={{ width: 320, maxWidth: '60%', height: 'auto' }} />
                                <div style={{ fontSize: 14, color: '#bbb', marginTop: 12 }}>Open a file to get started</div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        );
    }
    
    return (
        <div className="file-view">
            {/* Search bar */}
            <SearchBar
                appliedQuery={currentTab.query || ''}
                onApply={(text) => {
                    onQueryChange(text);
                    onApplySearch(text);
                }}
                inputRef={searchInputRef}
                history={queryHistory}
                queryError={currentTab.queryError}
            />
            
            <div className="content">
                {/* Histogram Manager */}
                <div className="histogram-wrap">
                    <HistogramManager
                        tabState={tabState}
                        histogram={histogram}
                        showHistogram={showHistogram}
                        displayTimeZone={displayTimeZone}
                        onRangeSelected={onHistogramRangeSelected}
                    />
                </div>
                
                <div className="main-split" style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }}>
                    <div className="grid-wrap" ref={gridContainerRef}>
                        {/* Grid Manager */}
                        <div style={{ width: '100%', height: '100%' }}>
                            <GridManager
                                tabState={tabState}
                                gridOps={gridOps}
                                pageSize={pageSize}
                                theme={theme}
                                onCopySelected={onCopySelected}
                                onAnnotateRow={onAnnotateRow}
                                onJumpToOriginal={onJumpToOriginal}
                                isLicensed={isLicensed}
                                isWorkspaceOpen={isWorkspaceOpen}
                                addLog={addLog}
                                copyPending={copyPending}
                            />
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default FileView;
