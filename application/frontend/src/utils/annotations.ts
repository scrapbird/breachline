// Shared annotation operations. Both the in-app annotation flow
// (useAnnotationHandlers) and the MCP bridge (useMcpBridge) call these so an
// AI-driven annotate/delete performs the exact same backend operation as a
// user clicking annotate on an explicit row selection. View feedback (dialogs,
// optimistic grid updates) stays at each call site; only the operation is
// shared here.
// @ts-ignore - Wails generated bindings
import * as AppAPI from '../../wailsjs/go/app/App';
import { FileOptions } from '../types/FileOptions';

// A tab's identity plus the query context that annotation row indices are
// resolved against. Callers build this from their tab state.
export interface AnnotationTarget {
  fileHash: string;
  fileOptions?: FileOptions;
  timeField?: string;
  appliedQuery?: string;
}

// Annotate specific rows (by position within the tab's applied query) with a
// note and color. Mirrors the annotate button's explicit-selection path.
export async function annotateRowsByHash(
  target: AnnotationTarget,
  rowIndices: number[],
  note: string,
  color: string,
): Promise<void> {
  await AppAPI.AddAnnotationsByHash(
    target.fileHash,
    target.fileOptions || {},
    rowIndices,
    target.timeField || '',
    note,
    color,
    target.appliedQuery || '',
  );
}

// Remove annotations from specific rows (by position within the tab's applied
// query). Mirrors the delete button's explicit-selection path.
export async function deleteRowAnnotationsByHash(
  target: AnnotationTarget,
  rowIndices: number[],
): Promise<void> {
  await AppAPI.DeleteAnnotationsByHash(
    target.fileHash,
    target.fileOptions || {},
    rowIndices,
    target.timeField || '',
    target.appliedQuery || '',
  );
}
