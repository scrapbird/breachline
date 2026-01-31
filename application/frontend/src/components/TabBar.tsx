import React from 'react';
import './TabBar.css';
import { FileOptions } from '../types/FileOptions';
import { CacheIndicator } from './CacheIndicator';

// Helper function to strip compression extension and get the inner file extension
const getInnerExtension = (filePath: string): string => {
  const lowerPath = filePath.toLowerCase();
  const compressionExtensions = ['.gz', '.bz2', '.xz'];
  
  // Strip compression extension if present
  let pathWithoutCompression = lowerPath;
  for (const compExt of compressionExtensions) {
    if (lowerPath.endsWith(compExt)) {
      pathWithoutCompression = lowerPath.slice(0, -compExt.length);
      break;
    }
  }
  
  // Get the extension from the remaining path
  return pathWithoutCompression.split('.').pop() || '';
};

// Helper function to get inner file type from pattern (e.g., "*.json.gz" -> "json")
const getFileTypeFromPattern = (pattern: string): string => {
  if (!pattern) return '';
  // Remove glob characters and get extension
  const cleaned = pattern.replace(/^\*\.?/, '').replace(/\*/g, '');
  // Handle compressed files like "json.gz"
  const parts = cleaned.split('.');
  for (const part of parts) {
    if (['csv', 'json', 'xlsx', 'xls'].includes(part.toLowerCase())) {
      return part.toLowerCase();
    }
  }
  return parts[0]?.toLowerCase() || '';
};

// Get icon color based on file type
const getIconColor = (fileType: string): string => {
  switch (fileType) {
    case 'csv': return '#4caf50';
    case 'xlsx': case 'xls': return '#217346';
    case 'json': return '#f0ad4e';
    default: return '#888';
  }
};

// Get the inner icon element for a file type
const getInnerFileIcon = (fileType: string, color: string): React.ReactNode => {
  switch (fileType) {
    case 'csv':
      return <i className="fa-solid fa-file-csv" style={{ color }} />;
    case 'xlsx':
    case 'xls':
      return <i className="fa-solid fa-file-excel" style={{ color }} />;
    case 'json':
      return <i className="fa-solid fa-file-code" style={{ color }} />;
    default:
      // Always show a generic file icon as fallback
      return <i className="fa-solid fa-file" style={{ color: '#888' }} />;
  }
};

// Directory icon component - plain folder icon
const DirectoryIcon: React.FC = () => {
  return <i className="fa-solid fa-folder" style={{ color: '#e8a838' }} />;
};

// Helper function to get file icon based on extension and options
const getFileIcon = (filePath: string, fileOptions?: FileOptions): React.ReactNode => {
  // Check if it's a directory
  if (fileOptions?.isDirectory) {
    return <DirectoryIcon />;
  }
  
  const extension = getInnerExtension(filePath);
  
  switch (extension) {
    case 'csv':
      return <i className="fa-solid fa-file-csv" style={{ color: '#4caf50' }} />;
    case 'xlsx':
    case 'xls':
      return <i className="fa-solid fa-file-excel" style={{ color: '#217346' }} />;
    case 'json':
      return <i className="fa-solid fa-file-code" style={{ color: '#f0ad4e' }} />;
    default:
      return <i className="fa-solid fa-file" style={{ color: '#888' }} />;
  }
};

export interface TabInfo {
  id: string;
  filePath: string;
  isSticky?: boolean; // Sticky tabs cannot be closed and stay at the front
  icon?: string; // Optional icon path for the tab
  fileOptions?: FileOptions;
}

// Helper to extract city name from timezone string (e.g., "Australia/Sydney" -> "Sydney")
const getCityFromTimezone = (tz: string): string => {
  if (!tz) return '';
  const parts = tz.split('/');
  return parts.length > 1 ? parts[parts.length - 1].replace(/_/g, ' ') : tz;
};

interface TabBarProps {
  tabs: TabInfo[];
  activeTabId: string;
  onTabChange: (tabId: string) => void;
  onTabClose: (tabId: string) => void;
  onNewTab: () => void;
  showCacheIndicator?: boolean;
}

export const TabBar: React.FC<TabBarProps> = ({
  tabs,
  activeTabId,
  onTabChange,
  onTabClose,
  onNewTab,
  showCacheIndicator = false,
}) => {
  const getFileName = (filePath: string): string => {
    if (!filePath) return 'Untitled';
    const parts = filePath.split(/[/\\]/);
    return parts[parts.length - 1] || 'Untitled';
  };

  // Count badges for a tab to adjust width
  const getBadgeCount = (tab: TabInfo): number => {
    if (!tab.fileOptions) return 0;
    let count = 0;
    if (tab.fileOptions.jpath) count++;
    if (tab.fileOptions.noHeaderRow) count++;
    if (tab.fileOptions.ingestTimezoneOverride) count++;
    if (tab.fileOptions.pluginName) count++;
    // Directory-specific badges
    if (tab.fileOptions.isDirectory && tab.fileOptions.filePattern) count++;
    if (tab.fileOptions.isDirectory && tab.fileOptions.includeSourceColumn) count++;
    return count;
  };

  return (
    <div className="tab-bar">
      <div className="tab-list">
        {tabs.map((tab) => {
          const badgeCount = getBadgeCount(tab);
          return (
          <div
            key={tab.id}
            className={`tab ${tab.id === activeTabId ? 'active' : ''} ${tab.isSticky ? 'sticky' : ''} ${tab.icon ? 'tab-icon-only' : ''} ${badgeCount > 0 ? `tab-badges-${badgeCount}` : ''}`}
            onClick={() => onTabChange(tab.id)}
          >
            {tab.icon ? (
              <img src={tab.icon} alt="" className="tab-icon-image" title={tab.filePath} />
            ) : (
              <>
                <span className="tab-file-icon">{getFileIcon(tab.filePath, tab.fileOptions)}</span>
                <span className="tab-title" title={tab.filePath}>
                  {getFileName(tab.filePath)}
                </span>
                {tab.fileOptions && (
                  <span className="tab-badges">
                    {tab.fileOptions.jpath && (
                      <span className="tab-badge tab-badge-jpath" title={`JPath: ${tab.fileOptions.jpath}`}>JP</span>
                    )}
                    {tab.fileOptions.noHeaderRow && (
                      <span className="tab-badge tab-badge-noheader" title="No Header Row">NH</span>
                    )}
                    {tab.fileOptions.ingestTimezoneOverride && (
                      <span className="tab-badge tab-badge-tz" title={`Timezone: ${tab.fileOptions.ingestTimezoneOverride}`}>
                        {getCityFromTimezone(tab.fileOptions.ingestTimezoneOverride)}
                      </span>
                    )}
                    {tab.fileOptions.isDirectory && tab.fileOptions.filePattern && (
                      <span className="tab-badge tab-badge-fp" title={`File Pattern: ${tab.fileOptions.filePattern}`}>FP</span>
                    )}
                    {tab.fileOptions.isDirectory && tab.fileOptions.includeSourceColumn && (
                      <span className="tab-badge tab-badge-sf" title="Include Source File Path Column">SF</span>
                    )}
                    {tab.fileOptions.pluginName && (
                      <span className="tab-badge tab-badge-plugin" title={`Plugin: ${tab.fileOptions.pluginName}`}>
                        <i className="fa-solid fa-puzzle-piece" style={{ fontSize: '8px' }} />
                      </span>
                    )}
                  </span>
                )}
              </>
            )}
            {!tab.isSticky && (
              <button
                className="tab-close"
                onClick={(e) => {
                  e.stopPropagation();
                  onTabClose(tab.id);
                }}
                aria-label="Close tab"
              >
                ×
              </button>
            )}
          </div>
        );})}
      </div>
      <button className="tab-new" onClick={onNewTab} aria-label="Open new file">
        +
      </button>
      <CacheIndicator visible={showCacheIndicator} />
    </div>
  );
};
