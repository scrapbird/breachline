import { useState, useCallback, useEffect } from 'react';
import { EventsOn } from '../../wailsjs/runtime';

export type LogLevel = 'info' | 'warn' | 'error';

export interface LogEntry {
    ts: number;
    level: LogLevel;
    message: string;
}

const MAX_LOGS = 500;

export interface ConsoleLoggerState {
    logs: LogEntry[];
    consoleHeight: number;
}

export interface ConsoleLoggerActions {
    addLog: (level: LogLevel, message: string) => void;
    clearLogs: () => void;
    setConsoleHeight: (height: number) => void;
}

/**
 * Hook for managing console logs and backend log event listener.
 * 
 * Consolidates:
 * - logs state array
 * - consoleHeight state
 * - addLog function
 * - Backend log event listener
 */
export function useConsoleLogger(): [ConsoleLoggerState, ConsoleLoggerActions] {
    const [logs, setLogs] = useState<LogEntry[]>([]);
    const [consoleHeight, setConsoleHeight] = useState<number>(160);
    
    // Add a log entry
    const addLog = useCallback((level: LogLevel, message: string) => {
        setLogs(prev => {
            const next = [...prev, { ts: Date.now(), level, message }];
            // Keep only the last MAX_LOGS entries
            if (next.length > MAX_LOGS) {
                return next.slice(next.length - MAX_LOGS);
            }
            return next;
        });
    }, []);
    
    // Clear all logs
    const clearLogs = useCallback(() => {
        setLogs([]);
    }, []);
    
    // Listen for backend log events
    useEffect(() => {
        const unsub = EventsOn("log", (data: any) => {
            const level = (data?.level || 'info') as LogLevel;
            const message = data?.message || '';
            if (message) {
                addLog(level, message);
            }
        });
        return () => {
            if (unsub) unsub();
        };
    }, [addLog]);
    
    return [
        { logs, consoleHeight },
        { addLog, clearLogs, setConsoleHeight }
    ];
}
