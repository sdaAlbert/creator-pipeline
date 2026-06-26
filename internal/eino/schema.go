package eino

type PromptInput struct {
	UserID string     `json:"user_id"`
	Prompt string     `json:"prompt"`
	Assets []AssetRef `json:"assets,omitempty"`
}

type AssetRef struct {
	ObjectKey string `json:"object_key"`
	Kind      string `json:"kind"`
}

type Role struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Scene struct {
	Name string `json:"name"`
	Mood string `json:"mood"`
}

type Shot struct {
	Index       int    `json:"index"`
	Description string `json:"description"`
	DurationMS  int    `json:"duration_ms"`
}

type Dialogue struct {
	ShotIndex int    `json:"shot_index"`
	Text      string `json:"text"`
}

type CreationPlan struct {
	RunID           string             `json:"run_id"`
	TraceVersion    string             `json:"trace_version"`
	Prompt          string             `json:"prompt"`
	PromptType      string             `json:"prompt_type"`
	Roles           []Role             `json:"roles"`
	Scenes          []Scene            `json:"scenes"`
	Shots           []Shot             `json:"shots"`
	Dialogues       []Dialogue         `json:"dialogues"`
	Assets          []AssetRef         `json:"assets,omitempty"`
	AssetMetadata   *AssetMetadata     `json:"asset_metadata,omitempty"`
	Duration        *DurationEstimate  `json:"duration,omitempty"`
	QualityScore    int                `json:"quality_score"`
	Quality         QualityBreakdown   `json:"quality"`
	RepairAttempts  int                `json:"repair_attempts"`
	PlanningTrace   []PlanningTrace    `json:"planning_trace,omitempty"`
	CallbackEvents  []CallbackEvent    `json:"callback_events,omitempty"`
	WorkflowSummary ProductionWorkflow `json:"workflow_summary"`
}

type PlanningTrace struct {
	Step       int    `json:"step"`
	Node       string `json:"node"`
	Source     string `json:"source"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

type CallbackEvent struct {
	Node       string `json:"node"`
	Component  string `json:"component"`
	Event      string `json:"event"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Error      string `json:"error,omitempty"`
}

type QualityBreakdown struct {
	StructureScore  int      `json:"structure_score"`
	SemanticScore   int      `json:"semantic_score"`
	LanguageScore   int      `json:"language_score"`
	KeywordHits     []string `json:"keyword_hits,omitempty"`
	MissingKeywords []string `json:"missing_keywords,omitempty"`
	Issues          []string `json:"issues,omitempty"`
}

type AssetMetadata struct {
	AssetCount      int  `json:"asset_count"`
	ImageCount      int  `json:"image_count"`
	VideoCount      int  `json:"video_count"`
	AudioCount      int  `json:"audio_count"`
	EstimatedBytes  int  `json:"estimated_bytes"`
	HasUserMaterial bool `json:"has_user_material"`
}

type DurationEstimate struct {
	ShotCount        int `json:"shot_count"`
	TotalDurationMS  int `json:"total_duration_ms"`
	TargetDurationMS int `json:"target_duration_ms"`
	DeltaMS          int `json:"delta_ms"`
}

type ProductionWorkflow struct {
	UsedStateGraph bool     `json:"used_state_graph"`
	Branches       []string `json:"branches"`
	Tools          []string `json:"tools"`
	RepairLoop     bool     `json:"repair_loop"`
	Callbacks      bool     `json:"callbacks"`
}

type workflowState struct {
	Input          PromptInput
	PromptType     string
	Plan           CreationPlan
	QualityScore   int
	RepairAttempts int
}
