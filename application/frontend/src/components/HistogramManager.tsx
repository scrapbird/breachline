import React from 'react';
import Histogram from './Histogram';
import { UseTabStateReturn } from '../hooks/useTabState';

interface HistogramManagerProps {
  tabState: UseTabStateReturn;
  histogram: {
    refreshHistogram: (tabId: string, timeField: string, query: string) => Promise<void>;
  };
  showHistogram: boolean;
  displayTimeZone: string;
  onRangeSelected: (startMs: number, endMs: number) => void;
}

const HistogramManager: React.FC<HistogramManagerProps> = ({
  tabState,
  histogram,
  showHistogram,
  displayTimeZone,
  onRangeSelected,
}) => {
  return (
    <div 
      style={{ 
        position: 'relative', 
        width: '100%', 
        height: showHistogram ? '140px' : '0px',
        overflow: 'hidden',
        visibility: showHistogram ? 'visible' : 'hidden',
      }}
    >
      {tabState.tabs.map(tab => {
        const tabData = tabState.getTabState(tab.id);
        
        // Skip dashboard and tabs without data
        if (!tabData || tab.id === '__dashboard__' || tabData.columnDefs.length === 0) {
          return null;
        }

        const isActive = tab.id === tabState.activeTabId;
        
        return (
          <div
            key={tab.id}
            data-tab-id={tab.id}
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              width: '100%',
              height: '100%',
              visibility: isActive ? 'visible' : 'hidden',
              pointerEvents: isActive ? 'auto' : 'none', // Prevent interaction with hidden histograms
              zIndex: isActive ? 1 : 0,
            }}
          >
            <Histogram
              buckets={(() => {
                const buckets = tabData.histBuckets || [];
                console.log('[HISTOGRAM_MANAGER_DEBUG] Tab', tabData.tabId, 'buckets:', buckets.length, 'loading:', tabData.histPending > 0);
                return buckets;
              })()}
              height={140}
              loading={tabData.histPending > 0}
              displayTimeZone={displayTimeZone}
              onRangeSelected={onRangeSelected}
            />
          </div>
        );
      })}
    </div>
  );
};

export default HistogramManager;
