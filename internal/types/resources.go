package types

import "time"

// ResourceKind represents the type of async resource
type ResourceKind string

const (
	KindJob          ResourceKind = "Job"
	KindCronJob      ResourceKind = "CronJob"
	KindWorkflow     ResourceKind = "Workflow"
	KindCronWorkflow ResourceKind = "CronWorkflow"
	KindSensor       ResourceKind = "Sensor"
	KindEventSource  ResourceKind = "EventSource"
)

// ResourceStatus represents the status of an async resource
type ResourceStatus string

const (
	StatusRunning   ResourceStatus = "Running"
	StatusSucceeded ResourceStatus = "Succeeded"
	StatusFailed    ResourceStatus = "Failed"
	StatusPending   ResourceStatus = "Pending"
	StatusUnknown   ResourceStatus = "Unknown"
)

// DAGNode represents a node in a workflow DAG
type DAGNode struct {
	Name   string
	Type   string // DAG, Pod, Retry, etc.
	Phase  string // Running, Succeeded, Failed, Pending, Error
}

// AsyncResource represents a unified view of async processing resources
type AsyncResource struct {
	Kind       ResourceKind
	Name       string
	Namespace  string
	Status     ResourceStatus
	StartTime  *time.Time
	EndTime    *time.Time
	Duration   time.Duration
	Message    string
	Retries    int
	MaxRetries int

	// Metrics
	SuccessCount int
	FailureCount int
	Throughput   float64 // per minute

	// Additional info
	Schedule   string // for CronJob/CronWorkflow
	Timezone   string // timezone for schedule (e.g., "Asia/Tokyo")
	LastRun    *time.Time
	NextRun    *time.Time
	QueueDepth int // for queue workers

	// Parent relationship (for Workflow spawned by CronWorkflow)
	ParentKind string
	ParentName string

	// DAG nodes (for Workflow)
	DAGNodes []DAGNode

	// Event info (for Sensor/EventSource)
	EventSourceName string   // EventSource name that Sensor listens to
	EventNames      []string // Event names that Sensor listens to
	EventType       string   // Type of EventSource (webhook, sqs, kafka, etc.)
	TriggerNames    []string // Trigger names in Sensor
}

// ViewMode represents the current view mode
type ViewMode int

const (
	ViewAll ViewMode = iota
	ViewJobs
	ViewWorkflows
	ViewEvents
)

func (v ViewMode) String() string {
	switch v {
	case ViewJobs:
		return "Jobs"
	case ViewWorkflows:
		return "Workflows"
	case ViewEvents:
		return "Events"
	default:
		return "All"
	}
}
