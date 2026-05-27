package model

const (
	ScanSchemaVersion = "agent-canon.scan.v1"
	PlanSchemaVersion = "agent-canon.plan.v1"
)

type Status string

const (
	StatusCompatible  Status = "compatible"
	StatusPartial     Status = "partial"
	StatusUnsupported Status = "unsupported"
	StatusDangerous   Status = "dangerous"
)

type ResourceKind string

const (
	KindInstruction ResourceKind = "Instruction"
	KindRule        ResourceKind = "Rule"
	KindSkill       ResourceKind = "Skill"
	KindCommand     ResourceKind = "Command"
	KindAgent       ResourceKind = "Agent"
	KindMCPServer   ResourceKind = "MCPServer"
	KindHook        ResourceKind = "Hook"
	KindMemoryItem  ResourceKind = "MemoryItem"
	KindSession     ResourceKind = "Session"
	KindConfig      ResourceKind = "Config"
)

type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
	ScopeLocal   Scope = "local"
)

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Resource struct {
	ID             string       `json:"id"`
	Kind           ResourceKind `json:"kind"`
	Scope          Scope        `json:"scope"`
	SourceTool     string       `json:"sourceTool"`
	SourcePath     string       `json:"sourcePath"`
	TargetTool     string       `json:"targetTool,omitempty"`
	TargetPathHint string       `json:"targetPathHint,omitempty"`
	Status         Status       `json:"status"`
	Strategy       string       `json:"strategy"`
	Warnings       []Warning    `json:"warnings"`
}

type ScanReport struct {
	SchemaVersion string      `json:"schemaVersion"`
	Source        string      `json:"source"`
	Target        string      `json:"target"`
	Project       string      `json:"project"`
	ClaudeHome    string      `json:"claudeHome"`
	CodexHome     string      `json:"codexHome"`
	Resources     []Resource  `json:"resources"`
	Warnings      []Warning   `json:"warnings"`
	Summary       ScanSummary `json:"summary"`
}

type ScanSummary struct {
	Compatible  int `json:"compatible"`
	Partial     int `json:"partial"`
	Unsupported int `json:"unsupported"`
	Dangerous   int `json:"dangerous"`
}

type Operation struct {
	ID             string       `json:"id"`
	Action         string       `json:"action"`
	ResourceID     string       `json:"resourceId"`
	Kind           ResourceKind `json:"kind"`
	SourcePath     string       `json:"sourcePath"`
	TargetPath     string       `json:"targetPath"`
	Status         Status       `json:"status"`
	Strategy       string       `json:"strategy"`
	RequiresReview bool         `json:"requiresReview"`
	Warnings       []Warning    `json:"warnings"`
}

type PlanReport struct {
	SchemaVersion string      `json:"schemaVersion"`
	Source        string      `json:"source"`
	Target        string      `json:"target"`
	Project       string      `json:"project"`
	Operations    []Operation `json:"operations"`
	Warnings      []Warning   `json:"warnings"`
	NonGoals      []string    `json:"nonGoals"`
	Summary       PlanSummary `json:"summary"`
}

type PlanSummary struct {
	Create    int `json:"create"`
	Modify    int `json:"modify"`
	Skip      int `json:"skip"`
	Manual    int `json:"manual"`
	Dangerous int `json:"dangerous"`
}
