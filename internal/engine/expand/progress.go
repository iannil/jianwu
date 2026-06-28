package expand

// ProgressPhase identifies which stage of the expand pipeline is running.
type ProgressPhase int

const (
	PhaseResearch  ProgressPhase = iota + 1 // Research iteration
	PhaseDraft                              // Draft iteration
	PhaseValidate                           // Validate iteration
)

// ProgressEvent describes a progress update from the expand pipeline.
// Fired at phase boundaries and at meaningful intermediate points.
type ProgressEvent struct {
	Phase   ProgressPhase // Which pipeline stage
	Percent int           // 0–100 estimated completion within this phase
	Message string        // Human-readable status
}

// ProgressCallback is an optional observer for expand progress.
// A nil callback is a no-op. The callback must not block.
type ProgressCallback func(ProgressEvent)
