import { useCallback, useState } from 'react';
import { UseTabStateReturn } from './useTabState';

export const useHistogram = (tabState: UseTabStateReturn) => {
  
  const refreshHistogram = useCallback(async (tabId: string, timeField: string, query: string, skipIfRecent = false) => {
    // Histogram now comes via events from async generation triggered by query execution
    // This function is kept for backward compatibility but is now a no-op
    console.log('[HISTOGRAM] Histogram refresh requested, but histograms now come via events');
    
    // The histogram will be automatically generated when a query is executed via GetCSVRowsFilteredForTab
    // and will arrive via the histogram:ready event listener in App.tsx
    return;
  }, [tabState]);
  
  return {
    refreshHistogram,
  };
};
