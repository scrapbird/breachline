package app

import (
	"fmt"

	"breachline/app/histogram"
)

// emitHistogramReady emits a histogram:ready event to the frontend
func (a *App) emitHistogramReady(event *histogram.HistogramReadyEvent) {
	a.EmitEvent("histogram:ready", event)
	a.Log("debug", fmt.Sprintf("[HISTOGRAM_EVENT] Emitted histogram:ready for tab %s, version %s",
		event.TabID, event.Version))
}

// emitHistogramError emits a histogram error event to the frontend
func (a *App) emitHistogramError(tabID, version, errorMsg string) {
	event := &histogram.HistogramReadyEvent{
		TabID:   tabID,
		Version: version,
		Error:   errorMsg,
	}
	a.emitHistogramReady(event)
	a.Log("error", fmt.Sprintf("[HISTOGRAM_EVENT] Emitted histogram error for tab %s: %s", tabID, errorMsg))
}
