package query

import (
	"fmt"
	"sync"
	"time"
)

// ProgressTracker manages progress reporting across multiple pipeline stages
type ProgressTracker struct {
	callback     ProgressCallback
	stages       map[string]*StageProgress
	totalStages  int
	currentStage int
	startTime    time.Time
	mutex        sync.RWMutex
}

// StageProgress tracks progress for a single pipeline stage
type StageProgress struct {
	Name      string
	Current   int64
	Total     int64
	StartTime time.Time
	Message   string
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(callback ProgressCallback, totalStages int) *ProgressTracker {
	return &ProgressTracker{
		callback:    callback,
		stages:      make(map[string]*StageProgress),
		totalStages: totalStages,
		startTime:   time.Now(),
	}
}

// StartStage begins tracking a new pipeline stage
func (p *ProgressTracker) StartStage(name string, estimatedRows int64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	p.currentStage++
	p.stages[name] = &StageProgress{
		Name:      name,
		Total:     estimatedRows,
		StartTime: time.Now(),
		Message:   fmt.Sprintf("Starting %s", name),
	}
	
	if p.callback != nil {
		p.callback(name, 0, estimatedRows, fmt.Sprintf("Stage %d/%d: %s", p.currentStage, p.totalStages, name))
	}
}

// UpdateStage updates progress for the current stage
func (p *ProgressTracker) UpdateStage(name string, current int64, message string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	stage, exists := p.stages[name]
	if !exists {
		return
	}
	
	stage.Current = current
	if message != "" {
		stage.Message = message
	}
	
	if p.callback != nil {
		// Only report progress if we have meaningful numbers
		if stage.Total > MinRowsForProgress && current%ProgressUpdateInterval == 0 {
			elapsed := time.Since(stage.StartTime)
			rate := float64(current) / elapsed.Seconds()
			
			progressMsg := fmt.Sprintf("Stage %d/%d: %s (%d/%d rows, %.0f rows/sec)", 
				p.currentStage, p.totalStages, stage.Message, current, stage.Total, rate)
			
			p.callback(name, current, stage.Total, progressMsg)
		}
	}
}

// CompleteStage marks a stage as completed
func (p *ProgressTracker) CompleteStage(name string, finalCount int64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	stage, exists := p.stages[name]
	if !exists {
		return
	}
	
	stage.Current = finalCount
	stage.Total = finalCount
	elapsed := time.Since(stage.StartTime)
	
	if p.callback != nil {
		rate := float64(finalCount) / elapsed.Seconds()
		message := fmt.Sprintf("Stage %d/%d: %s completed (%d rows, %.0f rows/sec, %v)", 
			p.currentStage, p.totalStages, name, finalCount, rate, elapsed.Truncate(time.Millisecond))
		
		p.callback(name, finalCount, finalCount, message)
	}
}

// GetOverallProgress returns the overall progress across all stages
func (p *ProgressTracker) GetOverallProgress() (current, total int64) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	for _, stage := range p.stages {
		current += stage.Current
		total += stage.Total
	}
	
	return current, total
}

// GetStageProgress returns progress for a specific stage
func (p *ProgressTracker) GetStageProgress(name string) *StageProgress {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	stage, exists := p.stages[name]
	if !exists {
		return nil
	}
	
	// Return a copy to avoid race conditions
	return &StageProgress{
		Name:      stage.Name,
		Current:   stage.Current,
		Total:     stage.Total,
		StartTime: stage.StartTime,
		Message:   stage.Message,
	}
}

// EstimateTimeRemaining estimates time remaining based on current progress
func (p *ProgressTracker) EstimateTimeRemaining() time.Duration {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	current, total := p.GetOverallProgress()
	if current == 0 || total == 0 {
		return 0
	}
	
	elapsed := time.Since(p.startTime)
	rate := float64(current) / elapsed.Seconds()
	remaining := float64(total-current) / rate
	
	return time.Duration(remaining) * time.Second
}

// NoOpProgressCallback is a progress callback that does nothing
func NoOpProgressCallback(stage string, current, total int64, message string) {
	// Do nothing
}

// LogProgressCallback creates a progress callback that logs to a provided function
func LogProgressCallback(logFunc func(level, message string)) ProgressCallback {
	return func(stage string, current, total int64, message string) {
		if logFunc != nil {
			logFunc("info", fmt.Sprintf("[QUERY_PROGRESS] %s", message))
		}
	}
}

// ThrottledProgressCallback wraps another callback with throttling to prevent spam
func ThrottledProgressCallback(callback ProgressCallback, minInterval time.Duration) ProgressCallback {
	var lastCall time.Time
	var mutex sync.Mutex
	
	return func(stage string, current, total int64, message string) {
		mutex.Lock()
		defer mutex.Unlock()
		
		now := time.Now()
		if now.Sub(lastCall) >= minInterval {
			lastCall = now
			if callback != nil {
				callback(stage, current, total, message)
			}
		}
	}
}

// ProgressEstimator helps estimate row counts for different pipeline stages
type ProgressEstimator struct {
	inputRows int64
}

// NewProgressEstimator creates a new progress estimator
func NewProgressEstimator(inputRows int64) *ProgressEstimator {
	return &ProgressEstimator{inputRows: inputRows}
}

// EstimateStageOutput estimates the output size for a pipeline stage
func (e *ProgressEstimator) EstimateStageOutput(stage PipelineStage) int64 {
	if e.inputRows <= 0 {
		return -1 // Unknown
	}
	
	ratio := stage.EstimateOutputSize()
	if ratio < 0 {
		return -1 // Unknown
	}
	
	return int64(float64(e.inputRows) * ratio)
}
