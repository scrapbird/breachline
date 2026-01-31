import { useState, useCallback } from 'react';
// @ts-ignore - Wails generated bindings
import * as AppAPI from "../../wailsjs/go/app/App";
import { FileOptions, createDefaultFileOptions } from '../types/FileOptions';
import { FileOpenOptions } from '../components/FileOptionsDialog';

/**
 * Plugin information returned from backend
 */
export interface PluginInfo {
    id: string;
    name: string;
    description: string;
    path: string;
    extensions: string[];
    executablePath: string;
    executableHash: string;
}

/**
 * Plugin warning info (subset needed for warning dialog)
 */
export interface PluginWarningInfo {
    id: string;
    name: string;
    executablePath: string;
    executableHash: string;
}

/**
 * Result of checking plugins for a file
 */
export type PluginCheckResult =
    | { action: 'none' }                                      // No plugins, proceed normally
    | { action: 'showSelection'; plugins: PluginInfo[] }      // Multiple plugins, show selection
    | { action: 'showWarning'; plugin: PluginInfo }           // Single plugin, show warning
    | { action: 'showStoredWarning'; plugin: PluginInfo };    // Stored plugin found, show warning

type LogLevel = 'info' | 'warn' | 'error';

interface UsePluginHandlersProps {
    tabState: any;
    fileOps: any;
    appliedDisplayTZ: string;
    addLog: (level: LogLevel, msg: string) => void;
    setIsOpeningFile: (loading: boolean) => void;
    showMessageDialogAction: (title: string, message: string, isError: boolean) => void;
    gridOps: any;
    histogram: any;
}

/**
 * Hook for managing plugin selection and warning dialogs
 * 
 * Extracts plugin-related state and handlers from App.tsx to reduce file size
 * and improve code organization.
 */
export function usePluginHandlers({
    tabState,
    fileOps,
    appliedDisplayTZ,
    addLog,
    setIsOpeningFile,
    showMessageDialogAction,
    gridOps,
    histogram,
}: UsePluginHandlersProps) {
    // Plugin selection dialog state
    const [showPluginSelection, setShowPluginSelection] = useState<boolean>(false);
    const [pluginSelectionPlugins, setPluginSelectionPlugins] = useState<PluginInfo[]>([]);
    const [pluginSelectionFilePath, setPluginSelectionFilePath] = useState<string>('');
    const [pluginSelectionFileName, setPluginSelectionFileName] = useState<string>('');
    const [pluginSelectionMode, setPluginSelectionMode] = useState<'open' | 'addToWorkspace'>('open');

    // Plugin warning dialog state
    const [showPluginWarning, setShowPluginWarning] = useState<boolean>(false);
    const [pluginWarningPlugin, setPluginWarningPlugin] = useState<PluginWarningInfo | null>(null);
    const [pluginWarningFilePath, setPluginWarningFilePath] = useState<string>('');
    const [pluginWarningMode, setPluginWarningMode] = useState<'open' | 'addToWorkspace'>('open');
    const [pluginWarningFileOptions, setPluginWarningFileOptions] = useState<FileOptions | null>(null);

    // Pending options from FileOptionsDialog (for plugin selection flow)
    const [pendingFileOptionsDialogOptions, setPendingFileOptionsDialogOptions] = useState<FileOpenOptions | null>(null);

    /**
     * Check if multiple plugins support a file and show appropriate dialog
     * Returns result indicating what action was taken
     */
    const checkPluginsForFile = useCallback(async (
        filePath: string,
        mode: 'open' | 'addToWorkspace',
        existingOptions?: FileOptions
    ): Promise<PluginCheckResult> => {
        try {
            const plugins = await AppAPI.GetPluginsForFile(filePath);

            // Check if file has a stored plugin ID
            if (existingOptions?.pluginId) {
                const storedPlugin = plugins?.find(p => p.id === existingOptions.pluginId);
                if (storedPlugin) {
                    // Show warning for stored plugin
                    setPluginWarningPlugin({
                        id: storedPlugin.id,
                        name: storedPlugin.name,
                        executablePath: storedPlugin.executablePath,
                        executableHash: storedPlugin.executableHash,
                    });
                    setPluginWarningFilePath(filePath);
                    setPluginWarningMode(mode);
                    setPluginWarningFileOptions(existingOptions);
                    setShowPluginWarning(true);
                    return { action: 'showStoredWarning', plugin: storedPlugin };
                }
                // Plugin not found - continue without warning (plugin may have been removed)
            }

            if (plugins && plugins.length > 1) {
                // Multiple plugins - show selection dialog
                const fileName = filePath.split('/').pop() || filePath.split('\\').pop() || filePath;
                setPluginSelectionFilePath(filePath);
                setPluginSelectionFileName(fileName);
                setPluginSelectionPlugins(plugins);
                setPluginSelectionMode(mode);
                setShowPluginSelection(true);
                return { action: 'showSelection', plugins };
            }

            if (plugins && plugins.length === 1) {
                // Single plugin - show warning dialog
                const plugin = plugins[0];
                setPluginWarningPlugin({
                    id: plugin.id,
                    name: plugin.name,
                    executablePath: plugin.executablePath,
                    executableHash: plugin.executableHash,
                });
                setPluginWarningFilePath(filePath);
                setPluginWarningMode(mode);
                setShowPluginWarning(true);
                return { action: 'showWarning', plugin };
            }

            // No plugins
            return { action: 'none' };
        } catch (e) {
            console.warn('Failed to check plugins for file:', e);
            return { action: 'none' };
        }
    }, []);

    /**
     * Open a file with plugin options (internal helper)
     */
    const openFileWithPlugin = useCallback(async (
        filePath: string,
        plugin: { id: string; name: string },
        storedFileOptions?: FileOptions | null
    ) => {
        setIsOpeningFile(true);
        try {
            // Build file options with plugin info
            const fileOptions: FileOptions = storedFileOptions ? {
                ...storedFileOptions,
                pluginId: plugin.id,
                pluginName: plugin.name,
            } : {
                pluginId: plugin.id,
                pluginName: plugin.name,
            };

            // Open the file or directory using the backend API
            let response;
            if (fileOptions.isDirectory) {
                // @ts-ignore - OpenDirectoryTabWithOptions available after Wails bindings regeneration
                response = await AppAPI.OpenDirectoryTabWithOptions(filePath, fileOptions);
            } else {
                response = await AppAPI.OpenFileTabWithOptions(filePath, fileOptions);
            }

            if (!response || !response.id) {
                addLog('error', `Failed to open ${fileOptions.isDirectory ? 'directory' : 'file'}`);
                return;
            }

            const tabId = response.id;

            // Create new tab with all options
            tabState.createTab(tabId, filePath, response.fileHash || '', fileOptions);

            // Get headers from response
            const hdr = response.headers || [];

            if (hdr && hdr.length > 0) {
                tabState.setHeaderForTab(tabId, hdr);
                tabState.setOriginalHeaderForTab(tabId, hdr);

                const detectedTimeField = fileOps.detectTimestampField(hdr);
                const defs = fileOps.buildColumnDefs(hdr, detectedTimeField, appliedDisplayTZ);

                tabState.setColumnDefsForTab(tabId, defs);
                tabState.setTimeFieldForTab(tabId, detectedTimeField);
                tabState.setHistBucketsForTab(tabId, []);
                tabState.setHistogramVersionForTab(tabId, `${tabId}:1`);
                tabState.incrementFileTokenForTab(tabId);
                tabState.incrementGenerationForTab(tabId);

                // Switch to the new tab
                tabState.switchTab(tabId);
                try {
                    await AppAPI.SetActiveTab(tabId);
                } catch (e) {
                    console.warn('Failed to set active tab:', e);
                }

                addLog('info', `Opened ${fileOptions.isDirectory ? 'directory' : 'file'} with plugin: ${filePath}`);

                // Give React time to update state
                await new Promise(resolve => setTimeout(resolve, 100));

                // Access tab by ID and create datasource if needed
                const tab = tabState.getTabState(tabId);
                if (tab && tab.gridApi && !tab.datasource) {
                    const ds = gridOps.createDataSource(tabId, '', hdr, detectedTimeField);
                    tabState.setDatasourceForTab(tabId, ds);
                }

                // Refresh histogram
                if (detectedTimeField) {
                    try {
                        await histogram.refreshHistogram(tabId, detectedTimeField, '', true);
                    } catch (e) {
                        console.warn('Failed to refresh histogram:', e);
                    }
                }
            }
        } catch (e: any) {
            const errorMsg = e?.message || String(e);
            addLog('error', 'Failed to open file: ' + errorMsg);
            showMessageDialogAction('Failed to Open File', errorMsg, true);
        } finally {
            setIsOpeningFile(false);
        }
    }, [tabState, fileOps, appliedDisplayTZ, addLog, setIsOpeningFile, gridOps, histogram]);

    /**
     * Handle plugin selection from the selection dialog
     */
    const handlePluginSelected = useCallback(async (pluginId: string, pluginName: string) => {
        setShowPluginSelection(false);
        const filePath = pluginSelectionFilePath;
        const mode = pluginSelectionMode;
        const dialogOptions = pendingFileOptionsDialogOptions;

        // Find the selected plugin to get executable info
        const selectedPlugin = pluginSelectionPlugins.find(p => p.id === pluginId);
        if (selectedPlugin) {
            // Build file options from dialog options if available
            const fileOpts: FileOptions = dialogOptions ? {
                jpath: dialogOptions.jpathExpression,
                noHeaderRow: dialogOptions.noHeaderRow,
                ingestTimezoneOverride: dialogOptions.ingestTimezoneOverride,
                // Directory options
                isDirectory: dialogOptions.isDirectory,
                filePattern: dialogOptions.filePattern,
                includeSourceColumn: dialogOptions.includeSourceColumn,
            } : {};

            // Show warning dialog before proceeding
            setPluginWarningPlugin({
                id: pluginId,
                name: pluginName,
                executablePath: selectedPlugin.executablePath,
                executableHash: selectedPlugin.executableHash,
            });
            setPluginWarningFilePath(filePath);
            setPluginWarningMode(mode);
            setPluginWarningFileOptions(fileOpts);
            setShowPluginWarning(true);
            setPendingFileOptionsDialogOptions(null); // Clear pending options
        } else {
            // Fallback: proceed without warning (shouldn't happen)
            if (mode === 'addToWorkspace') {
                try {
                    const fileOpts = createDefaultFileOptions();
                    fileOpts.pluginId = pluginId;
                    fileOpts.pluginName = pluginName;
                    await AppAPI.AddFileToWorkspace(filePath, fileOpts);
                    addLog('info', `Added file to workspace: ${filePath}`);
                } catch (e: any) {
                    addLog('error', 'Failed to add file to workspace: ' + (e?.message || String(e)));
                }
            } else {
                await openFileWithPlugin(filePath, { id: pluginId, name: pluginName });
            }
        }
    }, [pluginSelectionFilePath, pluginSelectionMode, pluginSelectionPlugins, pendingFileOptionsDialogOptions, addLog, openFileWithPlugin]);

    /**
     * Handle plugin selection cancel
     */
    const handlePluginSelectionCancel = useCallback(() => {
        setShowPluginSelection(false);
        setPluginSelectionFilePath('');
        setPluginSelectionPlugins([]);
        setPendingFileOptionsDialogOptions(null);
    }, []);

    /**
     * Handle plugin warning continue - actually execute the plugin
     */
    const handlePluginWarningContinue = useCallback(async () => {
        setShowPluginWarning(false);
        const plugin = pluginWarningPlugin;
        const filePath = pluginWarningFilePath;
        const mode = pluginWarningMode;
        const storedFileOptions = pluginWarningFileOptions;

        if (!plugin) return;

        if (mode === 'addToWorkspace') {
            // Add file to workspace with selected plugin, then open it
            try {
                const fileOpts = createDefaultFileOptions();
                fileOpts.pluginId = plugin.id;
                fileOpts.pluginName = plugin.name;
                await AppAPI.AddFileToWorkspace(filePath, fileOpts);
                addLog('info', `Added file to workspace: ${filePath}`);

                // Also open the file after adding to workspace
                await openFileWithPlugin(filePath, plugin, storedFileOptions);
            } catch (e: any) {
                addLog('error', 'Failed to add file to workspace: ' + (e?.message || String(e)));
            }
        } else {
            // Open file with selected plugin
            await openFileWithPlugin(filePath, plugin, storedFileOptions);
        }

        // Reset warning state
        setPluginWarningPlugin(null);
        setPluginWarningFilePath('');
        setPluginWarningFileOptions(null);
    }, [pluginWarningPlugin, pluginWarningFilePath, pluginWarningMode, pluginWarningFileOptions, addLog, openFileWithPlugin]);

    /**
     * Handle plugin warning cancel
     */
    const handlePluginWarningCancel = useCallback(() => {
        setShowPluginWarning(false);
        setPluginWarningPlugin(null);
        setPluginWarningFilePath('');
        setPluginWarningFileOptions(null);
    }, []);

    /**
     * Show plugin selection dialog for a file (used by handleOpenFile)
     */
    const showPluginSelectionFor = useCallback((
        filePath: string,
        plugins: PluginInfo[],
        mode: 'open' | 'addToWorkspace'
    ) => {
        const fileName = filePath.split('/').pop() || filePath.split('\\').pop() || filePath;
        setPluginSelectionFilePath(filePath);
        setPluginSelectionFileName(fileName);
        setPluginSelectionPlugins(plugins);
        setPluginSelectionMode(mode);
        setShowPluginSelection(true);
    }, []);

    /**
     * Show plugin warning dialog for a file (used by handleOpenFile)
     */
    const showPluginWarningFor = useCallback((
        filePath: string,
        plugin: PluginInfo,
        mode: 'open' | 'addToWorkspace',
        fileOptions?: FileOptions | null
    ) => {
        setPluginWarningPlugin({
            id: plugin.id,
            name: plugin.name,
            executablePath: plugin.executablePath,
            executableHash: plugin.executableHash,
        });
        setPluginWarningFilePath(filePath);
        setPluginWarningMode(mode);
        setPluginWarningFileOptions(fileOptions || null);
        setShowPluginWarning(true);
    }, []);

    return {
        // Dialog state for PluginSelectionDialog
        showPluginSelection,
        pluginSelectionPlugins,
        pluginSelectionFileName,

        // Dialog state for PluginWarningDialog
        showPluginWarning,
        pluginWarningPlugin,

        // Handlers for dialogs
        handlePluginSelected,
        handlePluginSelectionCancel,
        handlePluginWarningContinue,
        handlePluginWarningCancel,

        // Utility functions
        checkPluginsForFile,
        showPluginSelectionFor,
        showPluginWarningFor,

        // For file options dialog flow
        setPendingFileOptionsDialogOptions,
    };
}
