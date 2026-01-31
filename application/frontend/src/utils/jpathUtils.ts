import { JSONPath } from 'jsonpath-plus';

/**
 * Apply a JPath expression to a cell value.
 * Returns the original value if:
 * - The expression is empty or '$'
 * - The cell value is not valid JSON
 * - The JPath expression is invalid
 */
export function applyJPathToCell(cellValue: string, expression: string): string {
    if (!cellValue || !expression || expression === '$') {
        return cellValue;
    }

    try {
        // Try to parse as JSON
        const parsed = JSON.parse(cellValue);
        const result = JSONPath({ path: expression, json: parsed, wrap: false });

        // Handle different result types
        if (result === undefined || result === null) {
            return '';
        }
        if (typeof result === 'object') {
            return JSON.stringify(result);
        }
        return String(result);
    } catch (e) {
        // Not valid JSON or JPath error - return original
        return cellValue;
    }
}

/**
 * Preview JPath transformation on multiple values.
 * Returns an array of objects with original, transformed, and optional error.
 */
export interface JPathPreviewResult {
    original: string;
    transformed: string;
    error?: string;
}

export function previewJPath(values: string[], expression: string): JPathPreviewResult[] {
    return values.map((v) => {
        if (!v || !expression || expression === '$') {
            return { original: v, transformed: v };
        }

        try {
            const parsed = JSON.parse(v);
            const result = JSONPath({ path: expression, json: parsed, wrap: false });

            let transformed: string;
            if (result === undefined || result === null) {
                transformed = '';
            } else if (typeof result === 'object') {
                transformed = JSON.stringify(result);
            } else {
                transformed = String(result);
            }

            return { original: v, transformed };
        } catch (e: any) {
            // Check if it's a JSON parse error or JPath error
            const isJsonError = e.message?.includes('JSON') || e.message?.includes('Unexpected token');
            if (isJsonError) {
                // Not JSON - just return original (not an error for display)
                return { original: v, transformed: v };
            }
            return { original: v, transformed: '', error: e.message || 'Invalid expression' };
        }
    });
}

/**
 * Validate a JPath expression against sample JSON data.
 * Returns null if valid, or an error message if invalid.
 */
export function validateJPathExpression(expression: string, sampleJson?: string): string | null {
    if (!expression) {
        return 'Expression cannot be empty';
    }

    // Basic syntax check - must start with $ or @
    if (!expression.startsWith('$') && !expression.startsWith('@')) {
        return 'Expression must start with $ or @';
    }

    // Try to evaluate against sample data if provided
    if (sampleJson) {
        try {
            const parsed = JSON.parse(sampleJson);
            JSONPath({ path: expression, json: parsed, wrap: false });
        } catch (e: any) {
            if (e.message?.includes('JSON')) {
                // Sample JSON is invalid, but expression might be fine
                return null;
            }
            return e.message || 'Invalid expression';
        }
    }

    return null;
}

/**
 * Truncate a string for display, adding ellipsis if too long.
 */
export function truncateForPreview(value: string, maxLength: number = 50): string {
    if (!value || value.length <= maxLength) {
        return value;
    }
    return value.substring(0, maxLength - 3) + '...';
}
