import { useReducer, useCallback, useMemo } from 'react';

// All dialog visibility states consolidated
export interface DialogState {
    // Simple show/hide dialogs
    showSettings: boolean;
    showAbout: boolean;
    showSyntax: boolean;
    showShortcuts: boolean;
    showSyncStatus: boolean;
    showFuzzyFinder: boolean;
    showWorkspaceNameDialog: boolean;
    showConsole: boolean;
    showHistogram: boolean;
    
    // Dialogs with associated data
    showMessageDialog: boolean;
    messageDialogTitle: string;
    messageDialogMessage: string;
    messageDialogIsError: boolean;
    
    showAnnotationDialog: boolean;
    annotationRowIndices: number[];
    annotationNote: string;
    annotationColor: string;
    isAnnotationLoading: boolean;
    
    showIngestExpression: boolean;
    ingestTabId: string;
    ingestFilePath: string;
    
    showAddToWorkspaceDialog: boolean;
    addToWorkspaceFilePath: string;
    addToWorkspaceIsJson: boolean;
    
    showAuthPinDialog: boolean;
    authPinMessage: string;
    isAuthPinLoading: boolean;
    
    showColumnJPathDialog: boolean;
    columnJPathTarget: {
        columnName: string;
        currentExpression: string;
        previewData: string[];
    };
    
    // Header context menu (technically not a dialog but similar pattern)
    headerContextMenu: {
        visible: boolean;
        x: number;
        y: number;
        columnName: string;
    };
    
    // Loading overlays
    isCreatingWorkspace: boolean;
    showExportLoading: boolean;
    isOpeningFile: boolean;
    isOpeningWorkspace: boolean;
    isChangingTimestamp: boolean;
}

// Action types
type DialogAction =
    // Simple toggle actions
    | { type: 'TOGGLE_SETTINGS'; show: boolean }
    | { type: 'TOGGLE_ABOUT'; show: boolean }
    | { type: 'TOGGLE_SYNTAX'; show: boolean }
    | { type: 'TOGGLE_SHORTCUTS'; show: boolean }
    | { type: 'TOGGLE_SYNC_STATUS'; show: boolean }
    | { type: 'TOGGLE_FUZZY_FINDER'; show: boolean }
    | { type: 'TOGGLE_WORKSPACE_NAME_DIALOG'; show: boolean }
    | { type: 'TOGGLE_CONSOLE'; show: boolean }
    | { type: 'TOGGLE_HISTOGRAM'; show: boolean }
    
    // Message dialog actions
    | { type: 'SHOW_MESSAGE_DIALOG'; title: string; message: string; isError: boolean }
    | { type: 'HIDE_MESSAGE_DIALOG' }
    
    // Annotation dialog actions
    | { type: 'SHOW_ANNOTATION_DIALOG'; rowIndices: number[]; note: string; color: string }
    | { type: 'HIDE_ANNOTATION_DIALOG' }
    | { type: 'SET_ANNOTATION_LOADING'; loading: boolean }
    | { type: 'SET_ANNOTATION_NOTE'; note: string }
    | { type: 'SET_ANNOTATION_COLOR'; color: string }
    | { type: 'SET_ANNOTATION_ROW_INDICES'; indices: number[] }
    
    // Ingest expression dialog actions
    | { type: 'SHOW_INGEST_EXPRESSION'; tabId: string; filePath: string }
    | { type: 'HIDE_INGEST_EXPRESSION' }
    
    // Add to workspace dialog actions
    | { type: 'SHOW_ADD_TO_WORKSPACE'; filePath: string; isJson: boolean }
    | { type: 'HIDE_ADD_TO_WORKSPACE' }
    
    // Auth PIN dialog actions
    | { type: 'SHOW_AUTH_PIN'; message: string }
    | { type: 'HIDE_AUTH_PIN' }
    | { type: 'SET_AUTH_PIN_LOADING'; loading: boolean }
    | { type: 'SET_AUTH_PIN_MESSAGE'; message: string }
    
    // Column JPath dialog actions
    | { type: 'SHOW_COLUMN_JPATH'; columnName: string; currentExpression: string; previewData: string[] }
    | { type: 'HIDE_COLUMN_JPATH' }
    
    // Header context menu actions
    | { type: 'SHOW_HEADER_CONTEXT_MENU'; x: number; y: number; columnName: string }
    | { type: 'HIDE_HEADER_CONTEXT_MENU' }
    
    // Loading overlay actions
    | { type: 'SET_CREATING_WORKSPACE'; loading: boolean }
    | { type: 'SET_EXPORT_LOADING'; loading: boolean }
    | { type: 'SET_OPENING_FILE'; loading: boolean }
    | { type: 'SET_OPENING_WORKSPACE'; loading: boolean }
    | { type: 'SET_CHANGING_TIMESTAMP'; loading: boolean };

const initialState: DialogState = {
    showSettings: false,
    showAbout: false,
    showSyntax: false,
    showShortcuts: false,
    showSyncStatus: false,
    showFuzzyFinder: false,
    showWorkspaceNameDialog: false,
    showConsole: false,
    showHistogram: true,
    
    showMessageDialog: false,
    messageDialogTitle: '',
    messageDialogMessage: '',
    messageDialogIsError: false,
    
    showAnnotationDialog: false,
    annotationRowIndices: [],
    annotationNote: '',
    annotationColor: 'white',
    isAnnotationLoading: false,
    
    showIngestExpression: false,
    ingestTabId: '',
    ingestFilePath: '',
    
    showAddToWorkspaceDialog: false,
    addToWorkspaceFilePath: '',
    addToWorkspaceIsJson: false,
    
    showAuthPinDialog: false,
    authPinMessage: '',
    isAuthPinLoading: false,
    
    showColumnJPathDialog: false,
    columnJPathTarget: {
        columnName: '',
        currentExpression: '',
        previewData: [],
    },
    
    headerContextMenu: {
        visible: false,
        x: 0,
        y: 0,
        columnName: '',
    },
    
    isCreatingWorkspace: false,
    showExportLoading: false,
    isOpeningFile: false,
    isOpeningWorkspace: false,
    isChangingTimestamp: false,
};

function dialogReducer(state: DialogState, action: DialogAction): DialogState {
    switch (action.type) {
        // Simple toggles
        case 'TOGGLE_SETTINGS':
            return { ...state, showSettings: action.show };
        case 'TOGGLE_ABOUT':
            return { ...state, showAbout: action.show };
        case 'TOGGLE_SYNTAX':
            return { ...state, showSyntax: action.show };
        case 'TOGGLE_SHORTCUTS':
            return { ...state, showShortcuts: action.show };
        case 'TOGGLE_SYNC_STATUS':
            return { ...state, showSyncStatus: action.show };
        case 'TOGGLE_FUZZY_FINDER':
            return { ...state, showFuzzyFinder: action.show };
        case 'TOGGLE_WORKSPACE_NAME_DIALOG':
            return { ...state, showWorkspaceNameDialog: action.show };
        case 'TOGGLE_CONSOLE':
            return { ...state, showConsole: action.show };
        case 'TOGGLE_HISTOGRAM':
            return { ...state, showHistogram: action.show };
            
        // Message dialog
        case 'SHOW_MESSAGE_DIALOG':
            return {
                ...state,
                showMessageDialog: true,
                messageDialogTitle: action.title,
                messageDialogMessage: action.message,
                messageDialogIsError: action.isError,
            };
        case 'HIDE_MESSAGE_DIALOG':
            return { ...state, showMessageDialog: false };
            
        // Annotation dialog
        case 'SHOW_ANNOTATION_DIALOG':
            return {
                ...state,
                showAnnotationDialog: true,
                annotationRowIndices: action.rowIndices,
                annotationNote: action.note,
                annotationColor: action.color,
            };
        case 'HIDE_ANNOTATION_DIALOG':
            return { ...state, showAnnotationDialog: false };
        case 'SET_ANNOTATION_LOADING':
            return { ...state, isAnnotationLoading: action.loading };
        case 'SET_ANNOTATION_NOTE':
            return { ...state, annotationNote: action.note };
        case 'SET_ANNOTATION_COLOR':
            return { ...state, annotationColor: action.color };
        case 'SET_ANNOTATION_ROW_INDICES':
            return { ...state, annotationRowIndices: action.indices };
            
        // Ingest expression dialog
        case 'SHOW_INGEST_EXPRESSION':
            return {
                ...state,
                showIngestExpression: true,
                ingestTabId: action.tabId,
                ingestFilePath: action.filePath,
            };
        case 'HIDE_INGEST_EXPRESSION':
            return { ...state, showIngestExpression: false };
            
        // Add to workspace dialog
        case 'SHOW_ADD_TO_WORKSPACE':
            return {
                ...state,
                showAddToWorkspaceDialog: true,
                addToWorkspaceFilePath: action.filePath,
                addToWorkspaceIsJson: action.isJson,
            };
        case 'HIDE_ADD_TO_WORKSPACE':
            return {
                ...state,
                showAddToWorkspaceDialog: false,
                addToWorkspaceFilePath: '',
                addToWorkspaceIsJson: false,
            };
            
        // Auth PIN dialog
        case 'SHOW_AUTH_PIN':
            return {
                ...state,
                showAuthPinDialog: true,
                authPinMessage: action.message,
            };
        case 'HIDE_AUTH_PIN':
            return { ...state, showAuthPinDialog: false };
        case 'SET_AUTH_PIN_LOADING':
            return { ...state, isAuthPinLoading: action.loading };
        case 'SET_AUTH_PIN_MESSAGE':
            return { ...state, authPinMessage: action.message };
            
        // Column JPath dialog
        case 'SHOW_COLUMN_JPATH':
            return {
                ...state,
                showColumnJPathDialog: true,
                columnJPathTarget: {
                    columnName: action.columnName,
                    currentExpression: action.currentExpression,
                    previewData: action.previewData,
                },
            };
        case 'HIDE_COLUMN_JPATH':
            return { ...state, showColumnJPathDialog: false };
            
        // Header context menu
        case 'SHOW_HEADER_CONTEXT_MENU':
            return {
                ...state,
                headerContextMenu: {
                    visible: true,
                    x: action.x,
                    y: action.y,
                    columnName: action.columnName,
                },
            };
        case 'HIDE_HEADER_CONTEXT_MENU':
            return {
                ...state,
                headerContextMenu: { ...state.headerContextMenu, visible: false },
            };
            
        // Loading overlays
        case 'SET_CREATING_WORKSPACE':
            return { ...state, isCreatingWorkspace: action.loading };
        case 'SET_EXPORT_LOADING':
            return { ...state, showExportLoading: action.loading };
        case 'SET_OPENING_FILE':
            return { ...state, isOpeningFile: action.loading };
        case 'SET_OPENING_WORKSPACE':
            return { ...state, isOpeningWorkspace: action.loading };
        case 'SET_CHANGING_TIMESTAMP':
            return { ...state, isChangingTimestamp: action.loading };
            
        default:
            return state;
    }
}

// Hook return type for better TypeScript inference
export interface DialogActions {
    // Simple toggles
    setShowSettings: (show: boolean) => void;
    setShowAbout: (show: boolean) => void;
    setShowSyntax: (show: boolean) => void;
    setShowShortcuts: (show: boolean) => void;
    setShowSyncStatus: (show: boolean) => void;
    setShowFuzzyFinder: (show: boolean) => void;
    setShowWorkspaceNameDialog: (show: boolean) => void;
    setShowConsole: (show: boolean) => void;
    setShowHistogram: (show: boolean) => void;
    
    // Message dialog
    showMessageDialog: (title: string, message: string, isError: boolean) => void;
    hideMessageDialog: () => void;
    
    // Annotation dialog
    showAnnotationDialogWithData: (rowIndices: number[], note: string, color: string) => void;
    hideAnnotationDialog: () => void;
    setAnnotationLoading: (loading: boolean) => void;
    setAnnotationNote: (note: string) => void;
    setAnnotationColor: (color: string) => void;
    setAnnotationRowIndices: (indices: number[]) => void;
    
    // Ingest expression dialog
    showIngestExpressionDialog: (tabId: string, filePath: string) => void;
    hideIngestExpressionDialog: () => void;
    
    // Add to workspace dialog
    showAddToWorkspaceDialog: (filePath: string, isJson: boolean) => void;
    hideAddToWorkspaceDialog: () => void;
    
    // Auth PIN dialog
    showAuthPinDialogWithMessage: (message: string) => void;
    hideAuthPinDialog: () => void;
    setAuthPinLoading: (loading: boolean) => void;
    setAuthPinMessage: (message: string) => void;
    
    // Column JPath dialog
    showColumnJPathDialogWithData: (columnName: string, currentExpression: string, previewData: string[]) => void;
    hideColumnJPathDialog: () => void;
    
    // Header context menu
    showHeaderContextMenuAt: (x: number, y: number, columnName: string) => void;
    hideHeaderContextMenu: () => void;
    
    // Loading overlays
    setCreatingWorkspace: (loading: boolean) => void;
    setExportLoading: (loading: boolean) => void;
    setOpeningFile: (loading: boolean) => void;
    setOpeningWorkspace: (loading: boolean) => void;
    setChangingTimestamp: (loading: boolean) => void;
}

export function useDialogState(): [DialogState, DialogActions] {
    const [state, dispatch] = useReducer(dialogReducer, initialState);
    
    const actions: DialogActions = useMemo(() => ({
        // Simple toggles
        setShowSettings: (show: boolean) => dispatch({ type: 'TOGGLE_SETTINGS', show }),
        setShowAbout: (show: boolean) => dispatch({ type: 'TOGGLE_ABOUT', show }),
        setShowSyntax: (show: boolean) => dispatch({ type: 'TOGGLE_SYNTAX', show }),
        setShowShortcuts: (show: boolean) => dispatch({ type: 'TOGGLE_SHORTCUTS', show }),
        setShowSyncStatus: (show: boolean) => dispatch({ type: 'TOGGLE_SYNC_STATUS', show }),
        setShowFuzzyFinder: (show: boolean) => dispatch({ type: 'TOGGLE_FUZZY_FINDER', show }),
        setShowWorkspaceNameDialog: (show: boolean) => dispatch({ type: 'TOGGLE_WORKSPACE_NAME_DIALOG', show }),
        setShowConsole: (show: boolean) => dispatch({ type: 'TOGGLE_CONSOLE', show }),
        setShowHistogram: (show: boolean) => dispatch({ type: 'TOGGLE_HISTOGRAM', show }),
        
        // Message dialog
        showMessageDialog: (title: string, message: string, isError: boolean) =>
            dispatch({ type: 'SHOW_MESSAGE_DIALOG', title, message, isError }),
        hideMessageDialog: () => dispatch({ type: 'HIDE_MESSAGE_DIALOG' }),
        
        // Annotation dialog
        showAnnotationDialogWithData: (rowIndices: number[], note: string, color: string) =>
            dispatch({ type: 'SHOW_ANNOTATION_DIALOG', rowIndices, note, color }),
        hideAnnotationDialog: () => dispatch({ type: 'HIDE_ANNOTATION_DIALOG' }),
        setAnnotationLoading: (loading: boolean) => dispatch({ type: 'SET_ANNOTATION_LOADING', loading }),
        setAnnotationNote: (note: string) => dispatch({ type: 'SET_ANNOTATION_NOTE', note }),
        setAnnotationColor: (color: string) => dispatch({ type: 'SET_ANNOTATION_COLOR', color }),
        setAnnotationRowIndices: (indices: number[]) => dispatch({ type: 'SET_ANNOTATION_ROW_INDICES', indices }),
        
        // Ingest expression dialog
        showIngestExpressionDialog: (tabId: string, filePath: string) =>
            dispatch({ type: 'SHOW_INGEST_EXPRESSION', tabId, filePath }),
        hideIngestExpressionDialog: () => dispatch({ type: 'HIDE_INGEST_EXPRESSION' }),
        
        // Add to workspace dialog
        showAddToWorkspaceDialog: (filePath: string, isJson: boolean) =>
            dispatch({ type: 'SHOW_ADD_TO_WORKSPACE', filePath, isJson }),
        hideAddToWorkspaceDialog: () => dispatch({ type: 'HIDE_ADD_TO_WORKSPACE' }),
        
        // Auth PIN dialog
        showAuthPinDialogWithMessage: (message: string) => dispatch({ type: 'SHOW_AUTH_PIN', message }),
        hideAuthPinDialog: () => dispatch({ type: 'HIDE_AUTH_PIN' }),
        setAuthPinLoading: (loading: boolean) => dispatch({ type: 'SET_AUTH_PIN_LOADING', loading }),
        setAuthPinMessage: (message: string) => dispatch({ type: 'SET_AUTH_PIN_MESSAGE', message }),
        
        // Column JPath dialog
        showColumnJPathDialogWithData: (columnName: string, currentExpression: string, previewData: string[]) =>
            dispatch({ type: 'SHOW_COLUMN_JPATH', columnName, currentExpression, previewData }),
        hideColumnJPathDialog: () => dispatch({ type: 'HIDE_COLUMN_JPATH' }),
        
        // Header context menu
        showHeaderContextMenuAt: (x: number, y: number, columnName: string) =>
            dispatch({ type: 'SHOW_HEADER_CONTEXT_MENU', x, y, columnName }),
        hideHeaderContextMenu: () => dispatch({ type: 'HIDE_HEADER_CONTEXT_MENU' }),
        
        // Loading overlays
        setCreatingWorkspace: (loading: boolean) => dispatch({ type: 'SET_CREATING_WORKSPACE', loading }),
        setExportLoading: (loading: boolean) => dispatch({ type: 'SET_EXPORT_LOADING', loading }),
        setOpeningFile: (loading: boolean) => dispatch({ type: 'SET_OPENING_FILE', loading }),
        setOpeningWorkspace: (loading: boolean) => dispatch({ type: 'SET_OPENING_WORKSPACE', loading }),
        setChangingTimestamp: (loading: boolean) => dispatch({ type: 'SET_CHANGING_TIMESTAMP', loading }),
    }), []);
    
    return [state, actions];
}
