import { useCallback } from 'react';
import { UseTabStateReturn } from './useTabState';

export interface UseUnifiedDataFetchProps {
  tabState: UseTabStateReturn;
  pageSize: number;
  bucketSeconds: number;
  addLog: (level: 'info' | 'warn' | 'error', message: string) => void;
}

export interface UseUnifiedDataFetchReturn {
  fetchDataAndHistogram: (
    tabId: string,
    startRow: number,
    endRow: number,
    query: string,
    timeField: string
  ) => Promise<any>;
}

/**
 * Hook for fetching grid data and histogram data in a single unified call.
 * This reduces query executions from 2 (grid + histogram) to 1 (unified).
 */
export const useUnifiedDataFetch = ({
  tabState,
  pageSize,
  bucketSeconds,
  addLog,
}: UseUnifiedDataFetchProps): UseUnifiedDataFetchReturn => {
  
  const fetchDataAndHistogram = useCallback(async (
    tabId: string,
    startRow: number,
    endRow: number,
    query: string,
    timeField: string
  ) => {
    try {
      // Import AppAPI dynamically
      const AppAPI = await import('../../wailsjs/go/app/App');
      
      if (!AppAPI.GetDataAndHistogram) {
        throw new Error('Unified endpoint not available. Run \'wails dev\' to regenerate bindings.');
      }
      
      // Use unified endpoint
      const result = await AppAPI.GetDataAndHistogram(
        tabId,
        startRow,
        endRow,
        query || "",
        timeField || "",
        bucketSeconds
      );
      
      return result;
    } catch (e) {
      addLog('error', `Unified data fetch failed: ${e instanceof Error ? e.message : String(e)}`);
      throw e;
    }
  }, [tabState, pageSize, bucketSeconds, addLog]);
  
  return {
    fetchDataAndHistogram,
  };
};
