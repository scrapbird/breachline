import React, { useEffect, useState } from 'react';
import TimezoneSelector from './TimezoneSelector';
import PluginSettingsTab from './PluginSettingsTab';

interface SettingsProps {
    show: boolean;
    settings: {
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
    };
    onCancel: () => void;
    onSave: () => void;
    onSettingsChange: (updater: (prev: any) => any) => void;
    onLog?: (level: 'info' | 'warn' | 'error', message: string) => void;
}

const Settings: React.FC<SettingsProps> = ({
    show,
    settings,
    onCancel,
    onSave,
    onSettingsChange,
    onLog = () => {},
}) => {
    // Tab state
    const [activeTab, setActiveTab] = useState<'general' | 'cache' | 'plugins'>('general');
    
    // Local state for cache size input to allow smooth editing
    const [cacheSizeInput, setCacheSizeInput] = useState<string>(settings.cache_size_limit_mb.toString());
    const [maxDirFilesInput, setMaxDirFilesInput] = useState<string>(settings.max_directory_files.toString());

    // Sync local inputs with settings
    useEffect(() => {
        setCacheSizeInput(settings.cache_size_limit_mb.toString());
    }, [settings.cache_size_limit_mb]);
    
    useEffect(() => {
        setMaxDirFilesInput(settings.max_directory_files.toString());
    }, [settings.max_directory_files]);

    // Handle Escape key to close settings modal
    useEffect(() => {
        if (!show) return;
        
        const handleEscape = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.preventDefault();
                onCancel();
            }
        };
        
        document.addEventListener('keydown', handleEscape);
        return () => document.removeEventListener('keydown', handleEscape);
    }, [show, onCancel]);

    if (!show) return null;

    return (
        <div className="modal-overlay" onClick={onCancel}>
            <div className="modal-content" onClick={(e) => e.stopPropagation()} style={{ height: '80vh', display: 'flex', flexDirection: 'column' }}>
                <div className="modal-header">
                    <h2>Settings</h2>
                    <button onClick={onCancel} className="close-button">
                        <i className="fa-solid fa-xmark" />
                    </button>
                </div>
                
                {/* Tab Navigation */}
                <div style={{ 
                    display: 'flex', 
                    gap: 4, 
                    borderBottom: '1px solid #444',
                    padding: '0 20px',
                }}>
                    <button
                        onClick={() => setActiveTab('general')}
                        style={{
                            padding: '10px 16px',
                            border: 'none',
                            background: activeTab === 'general' ? '#333' : 'transparent',
                            color: activeTab === 'general' ? '#fff' : '#999',
                            borderBottom: activeTab === 'general' ? '2px solid #3a6' : '2px solid transparent',
                            cursor: 'pointer',
                            fontSize: 14,
                            fontWeight: activeTab === 'general' ? 500 : 400,
                        }}
                    >
                        General
                    </button>
                    <button
                        onClick={() => setActiveTab('cache')}
                        style={{
                            padding: '10px 16px',
                            border: 'none',
                            background: activeTab === 'cache' ? '#333' : 'transparent',
                            color: activeTab === 'cache' ? '#fff' : '#999',
                            borderBottom: activeTab === 'cache' ? '2px solid #3a6' : '2px solid transparent',
                            cursor: 'pointer',
                            fontSize: 14,
                            fontWeight: activeTab === 'cache' ? 500 : 400,
                        }}
                    >
                        Cache
                    </button>
                    <button
                        onClick={() => setActiveTab('plugins')}
                        style={{
                            padding: '10px 16px',
                            border: 'none',
                            background: activeTab === 'plugins' ? '#333' : 'transparent',
                            color: activeTab === 'plugins' ? '#fff' : '#999',
                            borderBottom: activeTab === 'plugins' ? '2px solid #3a6' : '2px solid transparent',
                            cursor: 'pointer',
                            fontSize: 14,
                            fontWeight: activeTab === 'plugins' ? 500 : 400,
                        }}
                    >
                        Plugins
                    </button>
                </div>
                
                <div className="modal-body" style={{ overflowY: 'auto', flex: 1 }}>
                    {activeTab === 'general' && (
                    <>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                        <label style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                            <input
                                type="checkbox"
                                checked={settings.sort_by_time}
                                onChange={(e) => onSettingsChange(s => ({ ...s, sort_by_time: e.target.checked }))}
                                style={{ marginTop: '2px' }}
                            />
                            <span>Sort by time</span>
                        </label>
                        
                        <label style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                            <input
                                type="checkbox"
                                checked={settings.pin_timestamp_column}
                                onChange={(e) => onSettingsChange(s => ({ ...s, pin_timestamp_column: e.target.checked }))}
                                style={{ marginTop: '2px' }}
                            />
                            <span>Pin timestamp column</span>
                        </label>
                        <div style={{ fontSize: 11, opacity: 0.65, textAlign: 'left', marginLeft: 24, marginTop: -4 }}>
                            When enabled, the timestamp column is always shown as the first column.
                        </div>
                        
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                            <div style={{ fontSize: 13, opacity: 0.85, textAlign: 'left' }}>Sort order</div>
                            <label style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                                <input
                                    type="radio"
                                    name="sort_order"
                                    value="asc"
                                    checked={!!settings.sort_ascending}
                                    onChange={() => onSettingsChange(s => ({ ...s, sort_ascending: true }))}
                                    style={{ marginTop: '2px' }}
                                />
                                <span>Sort ascending</span>
                            </label>
                            <label style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                                <input
                                    type="radio"
                                    name="sort_order"
                                    value="desc"
                                    checked={!settings.sort_ascending}
                                    onChange={() => onSettingsChange(s => ({ ...s, sort_ascending: false }))}
                                    style={{ marginTop: '2px' }}
                                />
                                <span>Sort descending</span>
                            </label>
                        </div>
                        
                        {/* Separator between sort and file settings */}
                        <div style={{ height: '1px', backgroundColor: '#444', margin: '8px 0' }}></div>
                        
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                            <div style={{ fontSize: 13, opacity: 0.85, textAlign: 'left' }}>Maximum files when opening directory</div>
                            <input
                                type="number"
                                min="10"
                                max="10000"
                                step="100"
                                value={maxDirFilesInput}
                                onChange={(e) => {
                                    const value = e.target.value;
                                    setMaxDirFilesInput(value);
                                    
                                    // Only update settings if it's a valid number
                                    const numValue = parseInt(value);
                                    if (!isNaN(numValue) && numValue >= 10) {
                                        onSettingsChange(s => ({ ...s, max_directory_files: numValue }));
                                    }
                                }}
                                onBlur={(e) => {
                                    // Apply fallback when field loses focus if invalid
                                    const value = parseInt(e.target.value);
                                    if (isNaN(value) || value < 10) {
                                        setMaxDirFilesInput('500');
                                        onSettingsChange(s => ({ ...s, max_directory_files: 500 }));
                                    }
                                }}
                                style={{
                                    padding: '6px 8px',
                                    border: '1px solid #444',
                                    borderRadius: '4px',
                                    backgroundColor: '#2a2a2a',
                                    color: '#fff',
                                    fontSize: '13px',
                                    width: '100px'
                                }}
                            />
                            <div style={{ fontSize: 11, opacity: 0.65, textAlign: 'left' }}>
                                Maximum number of files to load when opening a directory as a virtual file.
                            </div>
                        </div>
                    </div>
                    
                    {/* Separator between cache and timezone settings */}
                    <div style={{ height: '1px', backgroundColor: '#444', margin: '16px 0' }}></div>
                    
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                            <div style={{ fontSize: 13, opacity: 0.85, textAlign: 'left' }}>Default ingest timezone</div>
                            <TimezoneSelector
                                value={settings.default_ingest_timezone}
                                onChange={(value) => onSettingsChange(s => ({ ...s, default_ingest_timezone: value }))}
                                placeholder="Search timezones..."
                            />
                            <div style={{ fontSize: 11, opacity: 0.65, textAlign: 'left' }}>
                                Used to interpret timestamps without explicit timezone.
                            </div>
                        </div>
                        
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                            <div style={{ fontSize: 13, opacity: 0.85, textAlign: 'left' }}>Display timezone</div>
                            <TimezoneSelector
                                value={settings.display_timezone}
                                onChange={(value) => onSettingsChange(s => ({ ...s, display_timezone: value }))}
                                placeholder="Search timezones..."
                            />
                            <div style={{ fontSize: 11, opacity: 0.65, textAlign: 'left' }}>
                                Affects how times are shown in the grid and charts.
                            </div>
                        </div>
                        
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                            <div style={{ fontSize: 13, opacity: 0.85, textAlign: 'left' }}>Timestamp display format</div>
                            <input
                                type="text"
                                className="themed-input"
                                placeholder="yyyy-MM-dd HH:mm:ss"
                                value={settings.timestamp_display_format}
                                onChange={(e) => onSettingsChange(s => ({ ...s, timestamp_display_format: e.target.value }))}
                                style={{ padding: '6px 8px', borderRadius: 6, border: '1px solid #444', background: '#222', color: '#eee' }}
                            />
                            <div style={{ fontSize: 11, opacity: 0.65, textAlign: 'left' }}>
                                Example pattern: yyyy-MM-dd HH:mm:ss zzz. Seconds and milliseconds are supported (use SSS for milliseconds).
                            </div>
                        </div>
                    </div>
                    </>
                    )}
                    
                    {activeTab === 'cache' && (
                    <>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                        <label style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                            <input
                                type="checkbox"
                                checked={!!settings.enable_query_cache}
                                onChange={(e) => onSettingsChange(s => ({ ...s, enable_query_cache: e.target.checked }))}
                                style={{ marginTop: '2px' }}
                            />
                            <span>Enable query cache</span>
                        </label>
                        <div style={{ fontSize: 11, opacity: 0.65, textAlign: 'left', marginLeft: 24, marginTop: -4 }}>
                            When enabled, query results are cached to improve performance for repeated searches.
                        </div>
                        
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                            <div style={{ fontSize: 13, opacity: 0.85, textAlign: 'left' }}>Cache size limit (MB)</div>
                            <input
                                type="number"
                                min="10"
                                max="1000"
                                step="100"
                                value={cacheSizeInput}
                                onChange={(e) => {
                                    const value = e.target.value;
                                    setCacheSizeInput(value);
                                    
                                    // Only update settings if it's a valid number
                                    const numValue = parseInt(value);
                                    if (!isNaN(numValue) && numValue >= 10) {
                                        onSettingsChange(s => ({ ...s, cache_size_limit_mb: numValue }));
                                    }
                                }}
                                onBlur={(e) => {
                                    // Apply fallback when field loses focus if invalid
                                    const value = parseInt(e.target.value);
                                    if (isNaN(value) || value < 10) {
                                        setCacheSizeInput('100');
                                        onSettingsChange(s => ({ ...s, cache_size_limit_mb: 100 }));
                                    }
                                }}
                                style={{
                                    padding: '6px 8px',
                                    border: '1px solid #444',
                                    borderRadius: '4px',
                                    backgroundColor: '#2a2a2a',
                                    color: '#fff',
                                    fontSize: '13px',
                                    width: '100px'
                                }}
                            />
                            <div style={{ fontSize: 11, opacity: 0.65, textAlign: 'left' }}>
                                Memory limit for query cache. Applies to all cache types (pipeline, stage, overall).
                            </div>
                        </div>
                    </div>
                    </>
                    )}
                    
                    {activeTab === 'plugins' && (
                        <PluginSettingsTab
                            enablePlugins={settings.enable_plugins}
                            onEnablePluginsChange={(enabled) => onSettingsChange(s => ({ ...s, enable_plugins: enabled }))}
                            onLog={onLog}
                        />
                    )}
                </div>
                
                {/* Modal Footer - Fixed at bottom */}
                <div style={{ 
                    padding: '16px 20px', 
                    borderTop: '1px solid #444',
                    display: 'flex', 
                    justifyContent: 'flex-end', 
                    gap: 8,
                    backgroundColor: '#2E3436',
                }}>
                    <button
                        onClick={onCancel}
                        style={{ padding: '8px 12px', borderRadius: 8, border: '1px solid #444', background: '#333', color: '#eee', cursor: 'pointer' }}
                    >
                        Cancel
                    </button>
                    <button
                        onClick={onSave}
                        style={{ padding: '8px 12px', borderRadius: 8, border: '1px solid #3a6', background: '#2d4733', color: '#cfe', cursor: 'pointer' }}
                    >
                        Save
                    </button>
                </div>
            </div>
        </div>
    );
};

export default Settings;
