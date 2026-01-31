import { useCallback, useRef } from 'react';
// @ts-ignore - Wails generated bindings
import * as SettingsAPI from "../../wailsjs/go/settings/SettingsService";

export interface AppSettings {
    sort_by_time: boolean;
    sort_ascending: boolean;
    enable_query_cache: boolean;
    cache_size_limit_mb: number;
    default_ingest_timezone: string;
    display_timezone: string;
    timestamp_display_format: string;
    pin_timestamp_column: boolean;
    max_directory_files: number;
    enable_plugins: boolean;
}

export const defaultSettings: AppSettings = {
    sort_by_time: true,
    sort_ascending: false,
    enable_query_cache: true,
    cache_size_limit_mb: 100,
    default_ingest_timezone: 'Local',
    display_timezone: 'Local',
    timestamp_display_format: 'yyyy-MM-dd HH:mm:ss',
    pin_timestamp_column: false,
    max_directory_files: 500,
    enable_plugins: false,
};

interface UseSettingsHandlersParams {
    settings: AppSettings;
    setSettings: React.Dispatch<React.SetStateAction<AppSettings>>;
    setShowSettings: (show: boolean) => void;
    appliedDisplayTZ: string;
    setAppliedDisplayTZ: (tz: string) => void;
    tabState: {
        currentTab: any;
    };
    searchOps: {
        applySearch: (query: string, tz?: string) => Promise<void>;
        applySearchToAllTabs: (tz?: string) => Promise<void>;
    };
    histogram: {
        refreshHistogram: (tabId: string, timeField: string, query: string, skipIfRecent?: boolean) => Promise<void>;
    };
    addLog: (level: 'info' | 'warn' | 'error', message: string) => void;
}

interface UseSettingsHandlersReturn {
    originalSettingsRef: React.MutableRefObject<AppSettings | null>;
    openSettings: () => void;
    cancelSettings: () => void;
    saveSettings: () => Promise<void>;
}

export function useSettingsHandlers({
    settings,
    setSettings,
    setShowSettings,
    appliedDisplayTZ,
    setAppliedDisplayTZ,
    tabState,
    searchOps,
    histogram,
    addLog,
}: UseSettingsHandlersParams): UseSettingsHandlersReturn {
    const originalSettingsRef = useRef<AppSettings | null>(null);

    const openSettings = useCallback(() => {
        originalSettingsRef.current = { ...settings };
        setShowSettings(true);
    }, [settings, setShowSettings]);

    const cancelSettings = useCallback(() => {
        // Restore original settings without saving
        if (originalSettingsRef.current) {
            setSettings(originalSettingsRef.current);
        }
        setShowSettings(false);
    }, [setSettings, setShowSettings]);

    const saveSettings = useCallback(async () => {
        try {
            const payload = {
                sort_by_time: settings.sort_by_time,
                sort_descending: !settings.sort_ascending,
                enable_query_cache: settings.enable_query_cache,
                cache_size_limit_mb: settings.cache_size_limit_mb,
                default_ingest_timezone: settings.default_ingest_timezone,
                display_timezone: settings.display_timezone,
                timestamp_display_format: settings.timestamp_display_format,
                pin_timestamp_column: settings.pin_timestamp_column,
                max_directory_files: settings.max_directory_files,
                enable_plugins: settings.enable_plugins,
            };
            // Cast to any - Wails will handle the type conversion automatically
            await SettingsAPI.SaveSettings(payload as any);
            
            // Check if sort settings changed (compare with original values from when modal was opened)
            const sortChanged = originalSettingsRef.current 
                ? (originalSettingsRef.current.sort_by_time !== settings.sort_by_time || 
                   originalSettingsRef.current.sort_ascending !== settings.sort_ascending)
                : false;
            
            console.log('Sort change detection:', {
                original: originalSettingsRef.current,
                current: settings,
                sortChanged
            });
            
            // Apply display timezone immediately
            const oldTZ = appliedDisplayTZ;
            const newTZ = settings.display_timezone;
            const tzChanged = oldTZ !== newTZ;

            // Check if timestamp display format changed
            const timestampFormatChanged = originalSettingsRef.current
                ? originalSettingsRef.current.timestamp_display_format !== settings.timestamp_display_format
                : false;
            
            // Check if default ingest timezone changed
            const ingestTzChanged = originalSettingsRef.current
                ? originalSettingsRef.current.default_ingest_timezone !== settings.default_ingest_timezone
                : false;
            
            // Check if pin timestamp column changed
            const pinTimestampChanged = originalSettingsRef.current
                ? originalSettingsRef.current.pin_timestamp_column !== settings.pin_timestamp_column
                : false;
            
            if (tzChanged) {
                setAppliedDisplayTZ(newTZ);
            }
            
            // Update originalSettingsRef to the newly saved values so subsequent saves work correctly
            originalSettingsRef.current = { ...settings };
            
            // Close settings modal immediately
            setShowSettings(false);
            addLog('info', 'Settings saved successfully');
            
            // Re-apply search if sort settings, timezone, timestamp format, ingest timezone, or pin timestamp column changed (runs in background)
            if (sortChanged || tzChanged || timestampFormatChanged || ingestTzChanged || pinTimestampChanged) {
                // For settings that affect all tabs (ingest timezone, display timezone, timestamp format),
                // refresh ALL open tabs, not just the current one
                const shouldRefreshAllTabs = ingestTzChanged || tzChanged || timestampFormatChanged;
                
                if (shouldRefreshAllTabs) {
                    // Refresh all tabs with new timezone/format settings
                    searchOps.applySearchToAllTabs(tzChanged ? newTZ : undefined).then(() => {
                        // Refresh histogram for current tab if sort changed
                        const currentTab = tabState.currentTab;
                        if (sortChanged && currentTab && currentTab.timeField) {
                            histogram.refreshHistogram(
                                currentTab.tabId,
                                currentTab.timeField,
                                currentTab.appliedQuery || '',
                                true  // skipIfRecent
                            );
                        }
                    }).catch((e) => {
                        addLog('error', 'Failed to refresh tabs: ' + (e instanceof Error ? e.message : String(e)));
                    });
                } else {
                    // Only sort or pin timestamp changed - just refresh current tab
                    const currentTab = tabState.currentTab;
                    if (currentTab) {
                        searchOps.applySearch(currentTab.appliedQuery, undefined).then(() => {
                            if (sortChanged && currentTab.timeField) {
                                histogram.refreshHistogram(
                                    currentTab.tabId,
                                    currentTab.timeField,
                                    currentTab.appliedQuery || '',
                                    true  // skipIfRecent
                                );
                            }
                        }).catch((e) => {
                            addLog('error', 'Failed to apply sort: ' + (e instanceof Error ? e.message : String(e)));
                        });
                    }
                }
            }
        } catch (e) {
            addLog('error', 'Failed to save settings: ' + (e instanceof Error ? e.message : String(e)));
        }
    }, [settings, appliedDisplayTZ, setAppliedDisplayTZ, setShowSettings, tabState, searchOps, histogram, addLog]);

    return {
        originalSettingsRef,
        openSettings,
        cancelSettings,
        saveSettings,
    };
}
