/**
 * Utility for tracking when grid data has finished loading.
 * 
 * This provides a promise-based approach to waiting for data load completion,
 * avoiding polling loops which are inefficient and can have timing issues with
 * React state updates.
 * 
 * Usage:
 * 1. Call createDataLoadPromise(tabId) before triggering data load
 * 2. Call resolveDataLoad(tabId) when data loading completes (in datasource getRows success)
 * 3. Await waitForDataLoad(tabId) to wait for completion
 */

interface DataLoadResolver {
  resolve: () => void;
  reject: (reason?: any) => void;
  promise: Promise<void>;
  createdAt: number;
}

// Map of tabId -> resolver for pending data loads
const pendingDataLoads = new Map<string, DataLoadResolver>();

// Timeout for data load (prevent memory leaks from abandoned promises)
const DATA_LOAD_TIMEOUT_MS = 30000; // 30 seconds

/**
 * Creates a new promise that will resolve when data loading completes for the given tab.
 * If a promise already exists for this tab, it will be replaced.
 */
export function createDataLoadPromise(tabId: string): Promise<void> {
  // Clean up any existing promise for this tab
  const existing = pendingDataLoads.get(tabId);
  if (existing) {
    // Resolve the old promise to prevent memory leaks
    existing.resolve();
  }
  
  let resolve: () => void;
  let reject: (reason?: any) => void;
  
  const promise = new Promise<void>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  
  const resolver: DataLoadResolver = {
    resolve: resolve!,
    reject: reject!,
    promise,
    createdAt: Date.now(),
  };
  
  pendingDataLoads.set(tabId, resolver);
  
  // Set up timeout to prevent memory leaks
  setTimeout(() => {
    const current = pendingDataLoads.get(tabId);
    if (current && current.createdAt === resolver.createdAt) {
      console.warn(`[DataLoadTracker] Timeout waiting for data load on tab ${tabId}`);
      current.resolve(); // Resolve anyway to unblock waiters
      pendingDataLoads.delete(tabId);
    }
  }, DATA_LOAD_TIMEOUT_MS);
  
  return promise;
}

/**
 * Resolves the pending data load promise for the given tab.
 * Call this when data loading completes successfully in the datasource.
 */
export function resolveDataLoad(tabId: string): void {
  const resolver = pendingDataLoads.get(tabId);
  if (resolver) {
    resolver.resolve();
    pendingDataLoads.delete(tabId);
  }
}

/**
 * Rejects the pending data load promise for the given tab.
 * Call this when data loading fails in the datasource.
 */
export function rejectDataLoad(tabId: string, reason?: any): void {
  const resolver = pendingDataLoads.get(tabId);
  if (resolver) {
    resolver.reject(reason);
    pendingDataLoads.delete(tabId);
  }
}

/**
 * Returns a promise that resolves when data loading completes for the given tab.
 * If no pending load exists, resolves immediately.
 */
export function waitForDataLoad(tabId: string): Promise<void> {
  const resolver = pendingDataLoads.get(tabId);
  if (resolver) {
    return resolver.promise;
  }
  // No pending load, resolve immediately
  return Promise.resolve();
}

/**
 * Checks if there's a pending data load for the given tab.
 */
export function hasPendingDataLoad(tabId: string): boolean {
  return pendingDataLoads.has(tabId);
}
