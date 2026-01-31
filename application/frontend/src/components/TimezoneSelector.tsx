import React, { useState, useRef, useEffect, useMemo } from 'react';
import { createPortal } from 'react-dom';
import { getTimezoneOptions } from '../utils/timezones';
import { fuzzyMatch } from '../utils/fuzzySearch';
import './TimezoneSelector.css';

interface TimezoneSelectorProps {
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
    showEmptyOption?: boolean;
    emptyOptionLabel?: string;
}

const TimezoneSelector: React.FC<TimezoneSelectorProps> = ({
    value,
    onChange,
    placeholder = 'Search timezones...',
    showEmptyOption = false,
    emptyOptionLabel = 'Use default setting',
}) => {
    const [isOpen, setIsOpen] = useState(false);
    const [searchQuery, setSearchQuery] = useState('');
    const [highlightedIndex, setHighlightedIndex] = useState(0);
    const [dropdownPosition, setDropdownPosition] = useState({ top: 0, left: 0, width: 0 });
    const triggerRef = useRef<HTMLDivElement>(null);
    const dropdownRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLInputElement>(null);
    const listRef = useRef<HTMLDivElement>(null);
    
    // Get all timezone options
    const allOptions = useMemo(() => getTimezoneOptions(), []);
    
    // Filter and sort options based on search query
    // Keep 'Local' and 'UTC' at the top, sort rest alphabetically
    const filteredOptions = useMemo(() => {
        const priorityItems = ['Local', 'UTC'];
        const priorityResults: string[] = [];
        const otherResults: Array<{ option: string; score: number }> = [];
        
        for (const option of allOptions) {
            const { matches, score } = fuzzyMatch(searchQuery, option);
            if (matches) {
                if (priorityItems.includes(option)) {
                    priorityResults.push(option);
                } else {
                    otherResults.push({ option, score });
                }
            }
        }
        
        // Sort other results by score descending, then alphabetically for ties
        otherResults.sort((a, b) => {
            if (b.score !== a.score) {
                return b.score - a.score;
            }
            return a.option.localeCompare(b.option);
        });
        
        // Keep priority items in their original order at the top
        const sortedPriority = priorityItems.filter(p => priorityResults.includes(p));
        
        return [...sortedPriority, ...otherResults.map(r => r.option)];
    }, [allOptions, searchQuery]);
    
    // Reset highlighted index when filtered options change
    useEffect(() => {
        setHighlightedIndex(0);
    }, [filteredOptions]);
    
    // Calculate dropdown position when opening
    useEffect(() => {
        if (isOpen && triggerRef.current) {
            const updatePosition = () => {
                const rect = triggerRef.current!.getBoundingClientRect();
                setDropdownPosition({
                    top: rect.bottom,
                    left: rect.left,
                    width: rect.width,
                });
            };
            
            updatePosition();
            
            // Update position on scroll or resize
            window.addEventListener('scroll', updatePosition, true);
            window.addEventListener('resize', updatePosition);
            
            return () => {
                window.removeEventListener('scroll', updatePosition, true);
                window.removeEventListener('resize', updatePosition);
            };
        }
    }, [isOpen]);
    
    // Handle click outside to close dropdown
    useEffect(() => {
        if (!isOpen) return;
        
        const handleClickOutside = (event: MouseEvent) => {
            const target = event.target as Node;
            const clickedTrigger = triggerRef.current?.contains(target);
            const clickedDropdown = dropdownRef.current?.contains(target);
            
            if (!clickedTrigger && !clickedDropdown) {
                setIsOpen(false);
                setSearchQuery('');
            }
        };
        
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, [isOpen]);
    
    // Scroll highlighted item into view
    useEffect(() => {
        if (isOpen && listRef.current) {
            const highlightedElement = listRef.current.children[highlightedIndex + (showEmptyOption ? 1 : 0)] as HTMLElement;
            if (highlightedElement) {
                highlightedElement.scrollIntoView({ block: 'nearest' });
            }
        }
    }, [highlightedIndex, isOpen, showEmptyOption]);
    
    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setSearchQuery(e.target.value);
        if (!isOpen) {
            setIsOpen(true);
        }
    };
    
    const handleKeyDown = (e: React.KeyboardEvent) => {
        const maxIndex = filteredOptions.length - 1;
        
        switch (e.key) {
            case 'ArrowDown':
                e.preventDefault();
                setHighlightedIndex(prev => Math.min(prev + 1, maxIndex));
                break;
            case 'ArrowUp':
                e.preventDefault();
                setHighlightedIndex(prev => Math.max(prev - 1, 0));
                break;
            case 'Enter':
                e.preventDefault();
                if (filteredOptions[highlightedIndex]) {
                    handleSelect(filteredOptions[highlightedIndex]);
                }
                break;
            case 'Escape':
                e.preventDefault();
                e.stopPropagation(); // Prevent closing parent dialog
                setIsOpen(false);
                setSearchQuery('');
                break;
            case 'Tab':
                setIsOpen(false);
                setSearchQuery('');
                break;
        }
    };
    
    const handleSelect = (option: string) => {
        onChange(option);
        setIsOpen(false);
        setSearchQuery('');
    };
    
    const handleSelectEmpty = () => {
        onChange('');
        setIsOpen(false);
        setSearchQuery('');
    };
    
    const handleToggle = () => {
        const newIsOpen = !isOpen;
        setIsOpen(newIsOpen);
        if (newIsOpen) {
            // Focus input when opening
            setTimeout(() => inputRef.current?.focus(), 0);
        } else {
            setSearchQuery('');
        }
    };
    
    const displayValue = value || (showEmptyOption ? emptyOptionLabel : 'Select timezone...');
    
    const dropdown = isOpen ? createPortal(
        <div 
            className="timezone-selector-dropdown"
            ref={dropdownRef}
            style={{
                position: 'fixed',
                top: dropdownPosition.top,
                left: dropdownPosition.left,
                width: dropdownPosition.width,
            }}
        >
            <div className="timezone-selector-search">
                <i className="fa-solid fa-magnifying-glass" />
                <input
                    ref={inputRef}
                    type="text"
                    value={searchQuery}
                    onChange={handleInputChange}
                    onKeyDown={handleKeyDown}
                    placeholder={placeholder}
                    autoFocus
                />
            </div>
            
            <div className="timezone-selector-list" ref={listRef}>
                {showEmptyOption && (
                    <div
                        className={`timezone-selector-option empty-option ${value === '' ? 'selected' : ''}`}
                        onClick={handleSelectEmpty}
                    >
                        {emptyOptionLabel}
                    </div>
                )}
                
                {filteredOptions.length === 0 ? (
                    <div className="timezone-selector-no-results">
                        No timezones match "{searchQuery}"
                    </div>
                ) : (
                    filteredOptions.map((option, index) => (
                        <div
                            key={option}
                            className={`timezone-selector-option ${option === value ? 'selected' : ''} ${index === highlightedIndex ? 'highlighted' : ''}`}
                            onClick={() => handleSelect(option)}
                            onMouseEnter={() => setHighlightedIndex(index)}
                        >
                            {option}
                        </div>
                    ))
                )}
            </div>
        </div>,
        document.body
    ) : null;
    
    return (
        <div className="timezone-selector">
            <div 
                className={`timezone-selector-trigger ${isOpen ? 'open' : ''}`}
                onClick={handleToggle}
                ref={triggerRef}
            >
                <span className={`timezone-selector-value ${!value && showEmptyOption ? 'placeholder' : ''}`}>
                    {displayValue}
                </span>
                <i className={`fa-solid fa-chevron-${isOpen ? 'up' : 'down'}`} />
            </div>
            {dropdown}
        </div>
    );
};

export default TimezoneSelector;
