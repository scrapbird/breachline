import { useCallback } from 'react';
// @ts-ignore - Wails generated bindings
import * as AppAPI from '../../wailsjs/go/app/App';
import { DialogActions } from './useDialogState';

interface UseAnnotationHandlersProps {
    tabState: any;
    gridOps: any;
    isLicensed: boolean;
    isWorkspaceOpen: boolean;
    annotationRowIndices: number[];
    dialogActions: DialogActions;
    addLog: (level: 'info' | 'warn' | 'error', message: string) => void;
}

export function useAnnotationHandlers(props: UseAnnotationHandlersProps) {
    const {
        tabState,
        gridOps,
        isLicensed,
        isWorkspaceOpen,
        annotationRowIndices,
        dialogActions,
        addLog,
    } = props;
    
    const {
        setAnnotationRowIndices,
        setAnnotationNote,
        setAnnotationColor,
        showAnnotationDialogWithData,
        hideAnnotationDialog,
        setAnnotationLoading,
        showMessageDialog,
    } = dialogActions;

    const handleAnnotateRow = useCallback(async (rowIndices?: number[]) => {
        // Check if user has a valid license
        if (!isLicensed) {
            showMessageDialog('Premium Feature', 'Annotations require a valid license. Please purchase and import a license file to use this premium feature.', true);
            return;
        }

        // Check if a workspace is open
        if (!isWorkspaceOpen) {
            showMessageDialog('No Workspace Open', 'Please open a workspace file first (File → Open workspace) to use annotations.', true);
            return;
        }

        // Check for virtual select all first
        const isVirtualSelectAll = tabState.virtualSelectAllRef.current;

        // If no row indices provided, use selected rows from grid
        let selectedIndices: number[] = rowIndices || [];
        if (selectedIndices.length === 0) {
            selectedIndices = gridOps.getSelectedRowIndexes();
        }

        // If still no selection and not virtual select all, do nothing
        if (selectedIndices.length === 0 && !isVirtualSelectAll) {
            addLog('info', 'No rows selected to annotate');
            return;
        }

        // Determine row indices for the dialog
        const dialogRowIndices = isVirtualSelectAll ? [] : selectedIndices;
        
        // Default annotation values
        let finalNote = '';
        let finalColor = 'white';

        // Try to fetch existing annotations for selected rows (sample first 100 for performance)
        const currentTab = tabState.currentTab;
        if (currentTab?.filePath && currentTab?.fileHash) {
            try {
                const WorkspaceAPI = AppAPI;

                // Only sample first 100 rows for prefill check to avoid performance issues
                const sampleSize = 100;
                const sampleIndices = selectedIndices.slice(0, sampleSize);

                // Get annotations for sampled rows
                const annotationsMap = await WorkspaceAPI.GetRowAnnotations(
                    currentTab.fileHash,
                    currentTab.fileOptions || {},
                    sampleIndices,
                    currentTab.appliedQuery || '',
                    currentTab.timeField || ''
                );

                // Check if all sampled rows have the same annotation
                const annotations = Object.values(annotationsMap);

                if (annotations.length > 0) {
                    const firstNote = (annotations[0] as any)?.note || '';
                    const firstColor = (annotations[0] as any)?.color || 'white';

                    let allSame = true;
                    for (const annot of annotations) {
                        if ((annot as any).note !== firstNote || (annot as any).color !== firstColor) {
                            allSame = false;
                            break;
                        }
                    }

                    // Pre-fill if all rows have the same annotation
                    if (allSame) {
                        finalNote = firstNote;
                        finalColor = firstColor;
                    }
                }
            } catch (e) {
                // Error fetching annotations, use defaults
                console.error('Error fetching annotations:', e);
            }
        }

        showAnnotationDialogWithData(dialogRowIndices, finalNote, finalColor);
    }, [
        isLicensed,
        isWorkspaceOpen,
        tabState,
        gridOps,
        showAnnotationDialogWithData,
        showMessageDialog,
        addLog,
    ]);

    const handleSubmitAnnotation = useCallback(async (note: string, color: string) => {
        const currentTab = tabState.currentTab;
        if (!currentTab || !currentTab.fileHash) {
            return;
        }

        // Check for virtual select all
        const isVirtualSelectAll = tabState.virtualSelectAllRef.current;

        // If not virtual select all and no specific rows selected, return
        if (!isVirtualSelectAll && annotationRowIndices.length === 0) {
            return;
        }

        setAnnotationLoading(true);
        try {
            const WorkspaceAPI = AppAPI;

            if (isVirtualSelectAll || annotationRowIndices.length === 0) {
                // Use new selection-based method for virtual select all
                const payload: any = {
                    ranges: [] as { start: number; end: number }[],
                    virtualSelectAll: isVirtualSelectAll,
                    query: currentTab.appliedQuery || "",
                    timeField: currentTab.timeField || "",
                    note: note,
                    color: color,
                };

                if (!isVirtualSelectAll && annotationRowIndices.length > 0) {
                    // Convert row indices to ranges
                    payload.ranges = gridOps.indexesToRanges(annotationRowIndices);
                }

                const result = await WorkspaceAPI.AddAnnotationsToSelection(payload);
                const rowsAnnotated = result?.rowsAnnotated ?? 0;

                if (isVirtualSelectAll) {
                    addLog('info', `Annotation added to ${rowsAnnotated.toLocaleString()} row${rowsAnnotated === 1 ? '' : 's'} (all matching rows)`);
                } else {
                    addLog('info', `Annotation added to ${rowsAnnotated.toLocaleString()} row${rowsAnnotated === 1 ? '' : 's'}`);
                }
            } else {
                // Use existing method for specific row indices
                await WorkspaceAPI.AddAnnotationsByHash(
                    currentTab.fileHash,
                    currentTab.fileOptions || {},
                    annotationRowIndices,
                    currentTab.timeField || '',
                    note,
                    color,
                    currentTab.appliedQuery || ''
                );

                const rowText = annotationRowIndices.length === 1 ? 'row' : 'rows';
                addLog('info', `Annotation added to ${annotationRowIndices.length} ${rowText}`);
            }

            hideAnnotationDialog();

            // Optimistically update local annotations for instant UI feedback
            if (currentTab.gridApi) {
                if (isVirtualSelectAll) {
                    // For virtual select all, refresh cache
                    currentTab.gridApi.refreshInfiniteCache();
                } else if (annotationRowIndices.length > 0) {
                    // Update local annotations map
                    const currentTabState = tabState.getCurrentTabState();
                    const updatedAnnotations = new Map(currentTabState?.rowAnnotations || new Map());
                    for (const rowIdx of annotationRowIndices) {
                        updatedAnnotations.set(rowIdx, { color: color || 'white' });
                    }

                    // Update tab state
                    tabState.updateCurrentTab((prev: any) => ({
                        ...prev,
                        rowAnnotations: updatedAnnotations,
                    }));

                    // Redraw rows
                    requestAnimationFrame(() => {
                        currentTab.gridApi?.redrawRows();
                    });
                }
            }

            setAnnotationRowIndices([]);
            // Reset virtual select all after annotation
            tabState.setVirtualSelectAll(false);
        } catch (e: any) {
            const errorMsg = e?.message || String(e);
            if (errorMsg.includes('requires a valid license')) {
                showMessageDialog('Premium Feature', 'Annotations require a valid license. Please import a license file to use this feature.', true);
            } else if (errorMsg.includes('no workspace is open')) {
                showMessageDialog('No Workspace Open', 'Please open a workspace file first (File → Open workspace) to use annotations.', true);
            } else if (errorMsg.includes('annotation_update_failed')) {
                // Extract the failure count from the error message
                const match = errorMsg.match(/(\d+) of (\d+) annotation/);
                if (match) {
                    const failed = match[1];
                    const total = match[2];
                    showMessageDialog('Annotation Update Failed', `${failed} of ${total} annotation(s) failed to save.\n\nThis may be due to the annotation note exceeding the maximum size limit (512 characters).`, true);
                } else {
                    showMessageDialog('Annotation Update Failed', 'Some annotations failed to save. Please try again with a shorter note.', true);
                }
                addLog('error', errorMsg);
            } else {
                showMessageDialog('Annotation Error', `Failed to save annotation: ${errorMsg}`, true);
                addLog('error', 'Failed to add annotation: ' + errorMsg);
            }
            hideAnnotationDialog();
        } finally {
            setAnnotationLoading(false);
        }
    }, [
        tabState,
        gridOps,
        annotationRowIndices,
        setAnnotationRowIndices,
        hideAnnotationDialog,
        setAnnotationLoading,
        showMessageDialog,
        addLog,
    ]);

    const handleDeleteAnnotation = useCallback(async () => {
        const currentTab = tabState.currentTab;
        if (!currentTab || !currentTab.fileHash) {
            return;
        }

        // Check for virtual select all
        const isVirtualSelectAll = tabState.virtualSelectAllRef.current;

        // If not virtual select all and no specific rows selected, return
        if (!isVirtualSelectAll && annotationRowIndices.length === 0) {
            return;
        }

        setAnnotationLoading(true);
        try {
            const WorkspaceAPI = AppAPI;

            if (isVirtualSelectAll || annotationRowIndices.length === 0) {
                // Use new selection-based method
                const payload: any = {
                    ranges: [] as { start: number; end: number }[],
                    virtualSelectAll: isVirtualSelectAll,
                    query: currentTab.appliedQuery || "",
                    timeField: currentTab.timeField || "",
                };

                if (!isVirtualSelectAll && annotationRowIndices.length > 0) {
                    payload.ranges = gridOps.indexesToRanges(annotationRowIndices);
                }

                const result = await WorkspaceAPI.DeleteAnnotationsFromSelection(payload);
                const rowsDeleted = result?.rowsAnnotated ?? 0;

                if (isVirtualSelectAll) {
                    addLog('info', `Annotation deleted from ${rowsDeleted.toLocaleString()} row${rowsDeleted === 1 ? '' : 's'} (all matching rows)`);
                } else {
                    addLog('info', `Annotation deleted from ${rowsDeleted.toLocaleString()} row${rowsDeleted === 1 ? '' : 's'}`);
                }
            } else {
                // Use existing method for specific row indices
                await WorkspaceAPI.DeleteAnnotationsByHash(
                    currentTab.fileHash,
                    currentTab.fileOptions || {},
                    annotationRowIndices,
                    currentTab.timeField || '',
                    currentTab.appliedQuery || ''
                );

                const rowText = annotationRowIndices.length === 1 ? 'row' : 'rows';
                addLog('info', `Annotation deleted from ${annotationRowIndices.length} ${rowText}`);
            }

            hideAnnotationDialog();

            // Clear annotations from local state
            const currentTabState = tabState.getCurrentTabState();
            if (currentTabState?.rowAnnotations) {
                if (isVirtualSelectAll) {
                    // Clear all annotations
                    tabState.updateCurrentTab((prev: any) => ({
                        ...prev,
                        rowAnnotations: new Map(),
                    }));
                } else {
                    // Clear specific row annotations
                    const updatedAnnotations = new Map(currentTabState.rowAnnotations);
                    for (const rowIndex of annotationRowIndices) {
                        updatedAnnotations.delete(rowIndex);
                    }
                    tabState.updateCurrentTab((prev: any) => ({
                        ...prev,
                        rowAnnotations: updatedAnnotations,
                    }));
                }
            }

            setAnnotationRowIndices([]);
            // Reset virtual select all after deletion
            tabState.setVirtualSelectAll(false);

            // Redraw rows
            const gridApi = currentTab.gridApi;
            if (gridApi) {
                if (isVirtualSelectAll) {
                    gridApi.refreshInfiniteCache();
                } else {
                    requestAnimationFrame(() => {
                        gridApi.redrawRows();
                    });
                }
            }
        } catch (e: any) {
            const errorMsg = e?.message || String(e);
            if (errorMsg.includes('requires a valid license')) {
                showMessageDialog('Premium Feature', 'Annotations require a valid license. Please import a license file to use this feature.', true);
            } else if (errorMsg.includes('no workspace is open')) {
                showMessageDialog('No Workspace Open', 'Please open a workspace file first (File → Open workspace) to delete annotations.', true);
            } else {
                addLog('error', `Failed to delete annotations: ${errorMsg}`);
            }
            hideAnnotationDialog();
        } finally {
            setAnnotationLoading(false);
        }
    }, [
        tabState,
        gridOps,
        annotationRowIndices,
        setAnnotationRowIndices,
        hideAnnotationDialog,
        setAnnotationLoading,
        showMessageDialog,
        addLog,
    ]);

    return {
        handleAnnotateRow,
        handleSubmitAnnotation,
        handleDeleteAnnotation,
    };
}
