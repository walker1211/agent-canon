package model

const (
	ScanSchemaVersion               = "agent-canon.scan.v1"
	PlanSchemaVersion               = "agent-canon.plan.v1"
	SnapshotSchemaVersion           = "agent-canon.snapshot.v1"
	CanonSnapshotSchemaVersion      = "agent-canon.canon-snapshot.v1"
	SyncStateSchemaVersion          = "agent-canon.sync-state.v1"
	LearnedResolutionsSchemaVersion = "agent-canon.learned-resolutions.v1"
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

type DiffKind string

const (
	DiffKindAdded     DiffKind = "added"
	DiffKindRemoved   DiffKind = "removed"
	DiffKindChanged   DiffKind = "changed"
	DiffKindUnchanged DiffKind = "unchanged"
)

type ConflictKind string

const (
	ConflictKindContent    ConflictKind = "ContentConflict"
	ConflictKindLocation   ConflictKind = "LocationConflict"
	ConflictKindCapability ConflictKind = "CapabilityConflict"
	ConflictKindSecurity   ConflictKind = "SecurityConflict"
	ConflictKindSemantic   ConflictKind = "SemanticConflict"
)

type ConflictStatus string

const (
	ConflictStatusOpen     ConflictStatus = "open"
	ConflictStatusResolved ConflictStatus = "resolved"
)

type ResolutionDecision string

const (
	ResolutionDecisionOurs       ResolutionDecision = "ours"
	ResolutionDecisionTheirs     ResolutionDecision = "theirs"
	ResolutionDecisionSuggestion ResolutionDecision = "suggestion"
	ResolutionDecisionManual     ResolutionDecision = "manual"
)

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

type ResourceState struct {
	ID             string       `json:"id"`
	Kind           ResourceKind `json:"kind"`
	Scope          Scope        `json:"scope"`
	Tool           string       `json:"tool"`
	Path           string       `json:"path,omitempty"`
	TargetPathHint string       `json:"targetPathHint,omitempty"`
	Status         Status       `json:"status"`
	Strategy       string       `json:"strategy"`
	ContentHash    string       `json:"contentHash,omitempty"`
	NormalizedText string       `json:"normalizedText,omitempty"`
	Warnings       []Warning    `json:"warnings"`
}

type SnapshotReport struct {
	SchemaVersion string          `json:"schemaVersion"`
	Tool          string          `json:"tool"`
	CreatedAt     string          `json:"createdAt"`
	Project       string          `json:"project"`
	Resources     []ResourceState `json:"resources"`
	Warnings      []Warning       `json:"warnings"`
}

type CanonSnapshotReport struct {
	SchemaVersion string          `json:"schemaVersion"`
	CreatedAt     string          `json:"createdAt"`
	Project       string          `json:"project"`
	Resources     []ResourceState `json:"resources"`
	Warnings      []Warning       `json:"warnings"`
}

type SemanticDiff struct {
	ResourceID string       `json:"resourceId"`
	Kind       ResourceKind `json:"kind"`
	Scope      Scope        `json:"scope"`
	DiffKind   DiffKind     `json:"diffKind"`
	BaseHash   string       `json:"baseHash,omitempty"`
	OursHash   string       `json:"oursHash,omitempty"`
	TheirsHash string       `json:"theirsHash,omitempty"`
	Summary    string       `json:"summary"`
}

type Conflict struct {
	ID                   string         `json:"id"`
	Kind                 ConflictKind   `json:"kind"`
	ResourceID           string         `json:"resourceId"`
	ResourceKind         ResourceKind   `json:"resourceKind"`
	Scope                Scope          `json:"scope"`
	Base                 *ResourceState `json:"base,omitempty"`
	Ours                 *ResourceState `json:"ours,omitempty"`
	Theirs               *ResourceState `json:"theirs,omitempty"`
	Suggestion           string         `json:"suggestion,omitempty"`
	SuggestionConfidence float64        `json:"suggestionConfidence,omitempty"`
	RequiresUserDecision bool           `json:"requiresUserDecision"`
	Status               ConflictStatus `json:"status"`
	ResolutionID         string         `json:"resolutionId,omitempty"`
	Fingerprint          string         `json:"fingerprint"`
	Warnings             []Warning      `json:"warnings"`
}

type SyncStateReport struct {
	SchemaVersion string            `json:"schemaVersion"`
	CreatedAt     string            `json:"createdAt"`
	Project       string            `json:"project"`
	Source        string            `json:"source"`
	Target        string            `json:"target"`
	BaseSnapshots map[string]string `json:"baseSnapshots"`
	Diffs         []SemanticDiff    `json:"diffs"`
	Conflicts     []Conflict        `json:"conflicts"`
	Summary       SyncSummary       `json:"summary"`
	Warnings      []Warning         `json:"warnings"`
}

type SyncSummary struct {
	Diffs             int `json:"diffs"`
	OpenConflicts     int `json:"openConflicts"`
	ResolvedConflicts int `json:"resolvedConflicts"`
	Warnings          int `json:"warnings"`
}

type LearnedResolution struct {
	ID                  string             `json:"id"`
	ConflictFingerprint string             `json:"conflictFingerprint"`
	ConflictKind        ConflictKind       `json:"conflictKind"`
	ResourceID          string             `json:"resourceId"`
	ResolvedAt          string             `json:"resolvedAt"`
	Decision            ResolutionDecision `json:"decision"`
	Value               string             `json:"value,omitempty"`
}

type LearnedResolutionReport struct {
	SchemaVersion string              `json:"schemaVersion"`
	Project       string              `json:"project"`
	Resolutions   []LearnedResolution `json:"resolutions"`
}
