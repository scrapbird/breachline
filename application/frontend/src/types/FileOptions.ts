// FileOptions contains all options that define a virtual file variant.
// Two files with the same hash but different options are considered different virtual files.
// Fields are optional to match Wails-generated types (Go omitempty).
export interface FileOptions {
    jpath?: string;
    noHeaderRow?: boolean;
    ingestTimezoneOverride?: string;
    // Plugin options
    pluginId?: string;  // UUID of the plugin to use (from plugin.yml)
    pluginName?: string;  // Display name for UI
    // Directory loading options
    isDirectory?: boolean;
    filePattern?: string;  // Required for directories - glob pattern like *.json.gz
    includeSourceColumn?: boolean;
    detectedFileType?: string;  // Detected from actual file loader: "csv", "json", "xlsx"
}

// createDefaultFileOptions returns a FileOptions with all default values
export const createDefaultFileOptions = (): FileOptions => ({
    jpath: '',
    noHeaderRow: false,
    ingestTimezoneOverride: '',
    pluginId: '',
    isDirectory: false,
    filePattern: '',
    includeSourceColumn: false,
});

// fileOptionsKey returns a unique string key for this options combination.
// Used for composite keys and map lookups.
export const fileOptionsKey = (opts: FileOptions): string => {
    const noHeader = opts.noHeaderRow ? 'true' : 'false';
    const tz = opts.ingestTimezoneOverride || 'default';
    const plugin = opts.pluginId || 'default';
    // Include directory options in key
    let dirStr = 'file';
    if (opts.isDirectory) {
        dirStr = 'dir';
        if (opts.filePattern) {
            dirStr += ':' + opts.filePattern;
        }
        if (opts.includeSourceColumn) {
            dirStr += ':src';
        }
    }
    return `${opts.jpath || ''}::${noHeader}::${tz}::${plugin}::${dirStr}`;
};

// fileOptionsEqual returns true if two FileOptions are equivalent.
export const fileOptionsEqual = (a: FileOptions, b: FileOptions): boolean => {
    return fileOptionsKey(a) === fileOptionsKey(b);
};

// fileOptionsIsEmpty returns true if all options are at default values.
export const fileOptionsIsEmpty = (opts: FileOptions): boolean => {
    return (!opts.jpath || opts.jpath === '') && !opts.noHeaderRow && 
           (!opts.ingestTimezoneOverride || opts.ingestTimezoneOverride === '') &&
           (!opts.pluginId || opts.pluginId === '') &&
           !opts.isDirectory && (!opts.filePattern || opts.filePattern === '') &&
           !opts.includeSourceColumn;
};
