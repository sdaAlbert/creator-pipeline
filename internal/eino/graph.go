package eino

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	nodeInit          = "init_state"
	nodeClassify      = "classify_prompt"
	nodeAssetTools    = "asset_tools"
	nodeRoles         = "roles"
	nodeScenes        = "scenes"
	nodeCommercial    = "commercial_storyboard"
	nodeStory         = "story_storyboard"
	nodeTutorial      = "tutorial_storyboard"
	nodeDurationTools = "duration_tools"
	nodeQuality       = "semantic_quality_check"
	nodeRepair        = "repair_shots"
	nodeDialogues     = "dialogues"
	nodePlan          = "final_plan"

	traceVersion = "planning_trace.v1"
)

type Planner struct {
	runnable compose.Runnable[PromptInput, CreationPlan]
	callback callbacks.Handler
}

type JSONFiller interface {
	FillJSON(ctx context.Context, system string, user string, out any) error
}

type PlannerOption func(*plannerConfig)

type plannerConfig struct {
	requireLLM bool
}

func WithRequiredLLM(required bool) PlannerOption {
	return func(cfg *plannerConfig) {
		cfg.requireLLM = required
	}
}

func NewPlanner(ctx context.Context, filler JSONFiller, opts ...PlannerOption) (*Planner, error) {
	cfg := plannerConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	toolsNode, err := newPlanningTools(ctx)
	if err != nil {
		return nil, err
	}

	g := compose.NewGraph[PromptInput, CreationPlan](compose.WithGenLocalState(func(ctx context.Context) *workflowState {
		return &workflowState{}
	}))

	_ = g.AddLambdaNode(nodeInit, compose.InvokableLambda(initNode), compose.WithNodeName(nodeInit))
	_ = g.AddLambdaNode(nodeClassify, compose.InvokableLambda(classifyNode), compose.WithNodeName(nodeClassify))
	_ = g.AddLambdaNode(nodeAssetTools, compose.InvokableLambda(assetToolsNode(toolsNode)), compose.WithNodeName(nodeAssetTools))
	_ = g.AddLambdaNode(nodeRoles, compose.InvokableLambda(rolesNode(filler, cfg.requireLLM)), compose.WithNodeName(nodeRoles))
	_ = g.AddLambdaNode(nodeScenes, compose.InvokableLambda(scenesNode(filler, cfg.requireLLM)), compose.WithNodeName(nodeScenes))
	_ = g.AddLambdaNode(nodeCommercial, compose.InvokableLambda(shotsNode(filler, "commercial", cfg.requireLLM)), compose.WithNodeName(nodeCommercial))
	_ = g.AddLambdaNode(nodeStory, compose.InvokableLambda(shotsNode(filler, "story", cfg.requireLLM)), compose.WithNodeName(nodeStory))
	_ = g.AddLambdaNode(nodeTutorial, compose.InvokableLambda(shotsNode(filler, "tutorial", cfg.requireLLM)), compose.WithNodeName(nodeTutorial))
	_ = g.AddLambdaNode(nodeDurationTools, compose.InvokableLambda(durationToolsNode(toolsNode)), compose.WithNodeName(nodeDurationTools))
	_ = g.AddLambdaNode(nodeQuality, compose.InvokableLambda(qualityNode), compose.WithNodeName(nodeQuality))
	_ = g.AddLambdaNode(nodeRepair, compose.InvokableLambda(repairNode), compose.WithNodeName(nodeRepair))
	_ = g.AddLambdaNode(nodeDialogues, compose.InvokableLambda(dialoguesNode(filler, cfg.requireLLM)), compose.WithNodeName(nodeDialogues))
	_ = g.AddLambdaNode(nodePlan, compose.InvokableLambda(finalPlanNode), compose.WithNodeName(nodePlan))

	_ = g.AddEdge(compose.START, nodeInit)
	_ = g.AddEdge(nodeInit, nodeClassify)
	_ = g.AddBranch(nodeClassify, compose.NewGraphBranch(assetBranch, map[string]bool{
		nodeAssetTools: true,
		nodeRoles:      true,
	}))
	_ = g.AddEdge(nodeAssetTools, nodeRoles)
	_ = g.AddEdge(nodeRoles, nodeScenes)
	_ = g.AddBranch(nodeScenes, compose.NewGraphBranch(storyboardBranch, map[string]bool{
		nodeCommercial: true,
		nodeStory:      true,
		nodeTutorial:   true,
	}))
	_ = g.AddEdge(nodeCommercial, nodeDurationTools)
	_ = g.AddEdge(nodeStory, nodeDurationTools)
	_ = g.AddEdge(nodeTutorial, nodeDurationTools)
	_ = g.AddEdge(nodeDurationTools, nodeQuality)
	_ = g.AddBranch(nodeQuality, compose.NewGraphBranch(qualityBranch, map[string]bool{
		nodeRepair:    true,
		nodeDialogues: true,
	}))
	_ = g.AddEdge(nodeRepair, nodeDurationTools)
	_ = g.AddEdge(nodeDialogues, nodePlan)
	_ = g.AddEdge(nodePlan, compose.END)

	runnable, err := g.Compile(ctx, compose.WithGraphName("CreatorProductionWorkflow"), compose.WithMaxRunSteps(24))
	if err != nil {
		return nil, err
	}
	return &Planner{runnable: runnable, callback: planningCallback()}, nil
}

func (p *Planner) Plan(ctx context.Context, in PromptInput) (CreationPlan, error) {
	collector := newCallbackCollector()
	ctx = context.WithValue(ctx, callbackCollectorKey{}, collector)
	plan, err := p.runnable.Invoke(ctx, in, compose.WithCallbacks(p.callback))
	if err != nil {
		return CreationPlan{}, err
	}
	plan.CallbackEvents = collector.snapshot()
	return plan, nil
}

func initNode(ctx context.Context, in PromptInput) (PromptInput, error) {
	in.Prompt = strings.TrimSpace(in.Prompt)
	if in.UserID == "" {
		return in, fmt.Errorf("user_id is required")
	}
	if in.Prompt == "" {
		return in, fmt.Errorf("prompt is required")
	}
	return in, compose.ProcessState[*workflowState](ctx, func(ctx context.Context, s *workflowState) error {
		s.Input = in
		s.Plan = CreationPlan{
			RunID:        newRunID(),
			TraceVersion: traceVersion,
			Prompt:       in.Prompt,
			Assets:       in.Assets,
			WorkflowSummary: ProductionWorkflow{
				UsedStateGraph: true,
				Branches:       []string{"asset-aware branch", "prompt-type branch", "quality repair branch"},
				Tools:          []string{"inspect_asset_metadata", "estimate_video_duration"},
				RepairLoop:     true,
				Callbacks:      true,
			},
		}
		addTrace(&s.Plan, nodeInit, "state_graph", 0, "")
		return nil
	})
}

func classifyNode(ctx context.Context, in PromptInput) (PromptInput, error) {
	return in, compose.ProcessState[*workflowState](ctx, func(ctx context.Context, s *workflowState) error {
		started := time.Now()
		promptType := classifyPrompt(in.Prompt)
		s.PromptType = promptType
		s.Plan.PromptType = promptType
		addTrace(&s.Plan, nodeClassify, "rule", time.Since(started), "")
		return nil
	})
}

func assetBranch(ctx context.Context, in PromptInput) (string, error) {
	if len(in.Assets) > 0 {
		return nodeAssetTools, nil
	}
	return nodeRoles, nil
}

func storyboardBranch(ctx context.Context, in PromptInput) (string, error) {
	var next string
	err := compose.ProcessState[*workflowState](ctx, func(ctx context.Context, s *workflowState) error {
		switch s.PromptType {
		case "tutorial":
			next = nodeTutorial
		case "story":
			next = nodeStory
		default:
			next = nodeCommercial
		}
		return nil
	})
	return next, err
}

func qualityBranch(ctx context.Context, in PromptInput) (string, error) {
	var next string
	err := compose.ProcessState[*workflowState](ctx, func(ctx context.Context, s *workflowState) error {
		if s.QualityScore >= 75 || s.RepairAttempts >= 1 {
			next = nodeDialogues
			return nil
		}
		next = nodeRepair
		return nil
	})
	return next, err
}

func assetToolsNode(toolsNode *compose.ToolsNode) func(context.Context, PromptInput) (PromptInput, error) {
	return func(ctx context.Context, in PromptInput) (PromptInput, error) {
		started := time.Now()
		var metadata AssetMetadata
		err := invokePlanningTool(ctx, toolsNode, "inspect_asset_metadata", assetMetadataRequest{Assets: in.Assets}, &metadata)
		return in, compose.ProcessState[*workflowState](ctx, func(ctx context.Context, s *workflowState) error {
			if err != nil {
				addTrace(&s.Plan, nodeAssetTools, "tools_node", time.Since(started), err.Error())
				return nil
			}
			s.Plan.AssetMetadata = &metadata
			addTrace(&s.Plan, nodeAssetTools, "tools_node", time.Since(started), "")
			return nil
		})
	}
}

func rolesNode(filler JSONFiller, requireLLM bool) func(context.Context, PromptInput) (PromptInput, error) {
	return func(ctx context.Context, in PromptInput) (PromptInput, error) {
		return in, compose.ProcessState[*workflowState](ctx, func(ctx context.Context, s *workflowState) error {
			started := time.Now()
			if filler != nil {
				var out struct {
					Roles []Role `json:"roles"`
				}
				err := filler.FillJSON(ctx, systemPrompt(), fmt.Sprintf(`Prompt: %s
Prompt type: %s
Asset metadata: %+v

Generate 2-3 concise creative roles for this exact prompt. Keep the same language as the prompt when possible. Do not introduce unrelated mythology or unrelated products. JSON shape:
{"roles":[{"name":"...","description":"..."}]}`, s.Input.Prompt, s.PromptType, s.Plan.AssetMetadata), &out)
				if err == nil && len(out.Roles) > 0 {
					s.Plan.Roles = out.Roles
					addTrace(&s.Plan, nodeRoles, "llm", time.Since(started), "")
					return nil
				}
				addTrace(&s.Plan, nodeRoles, "fallback", time.Since(started), classifyErr(err))
				if requireLLM {
					return requiredLLMError(nodeRoles, err)
				}
			} else if requireLLM {
				addTrace(&s.Plan, nodeRoles, "fallback", time.Since(started), "llm_required_but_not_configured")
				return requiredLLMError(nodeRoles, fmt.Errorf("llm is required but not configured"))
			}
			s.Plan.Roles = []Role{
				{Name: "main subject", Description: "Primary product or character extracted from the prompt"},
				{Name: "target audience", Description: "Viewer group implied by the creative brief"},
			}
			addTrace(&s.Plan, nodeRoles, "fallback", time.Since(started), "")
			return nil
		})
	}
}

func scenesNode(filler JSONFiller, requireLLM bool) func(context.Context, PromptInput) (PromptInput, error) {
	return func(ctx context.Context, in PromptInput) (PromptInput, error) {
		return in, compose.ProcessState[*workflowState](ctx, func(ctx context.Context, s *workflowState) error {
			started := time.Now()
			if filler != nil {
				var out struct {
					Scenes []Scene `json:"scenes"`
				}
				err := filler.FillJSON(ctx, systemPrompt(), fmt.Sprintf(`Prompt: %s
Prompt type: %s
Roles: %+v

Generate 3 scenes for this exact prompt and these roles. Keep the same language as the prompt when possible. JSON shape:
{"scenes":[{"name":"...","mood":"..."}]}`, s.Input.Prompt, s.PromptType, s.Plan.Roles), &out)
				if err == nil && len(out.Scenes) > 0 {
					s.Plan.Scenes = out.Scenes
					addTrace(&s.Plan, nodeScenes, "llm", time.Since(started), "")
					return nil
				}
				addTrace(&s.Plan, nodeScenes, "fallback", time.Since(started), classifyErr(err))
				if requireLLM {
					return requiredLLMError(nodeScenes, err)
				}
			} else if requireLLM {
				addTrace(&s.Plan, nodeScenes, "fallback", time.Since(started), "llm_required_but_not_configured")
				return requiredLLMError(nodeScenes, fmt.Errorf("llm is required but not configured"))
			}
			s.Plan.Scenes = fallbackScenes(s.PromptType)
			addTrace(&s.Plan, nodeScenes, "fallback", time.Since(started), "")
			return nil
		})
	}
}

func shotsNode(filler JSONFiller, path string, requireLLM bool) func(context.Context, PromptInput) (PromptInput, error) {
	return func(ctx context.Context, in PromptInput) (PromptInput, error) {
		return in, compose.ProcessState[*workflowState](ctx, func(ctx context.Context, s *workflowState) error {
			started := time.Now()
			nodeName := path + "_storyboard"
			if filler != nil {
				var out struct {
					Shots []Shot `json:"shots"`
				}
				err := filler.FillJSON(ctx, systemPrompt(), fmt.Sprintf(`Prompt: %s
Planning path: %s
Roles: %+v
Scenes: %+v
Asset metadata: %+v

Generate 3 storyboard shots for this exact prompt and path. Keep the same language as the prompt when possible. duration_ms must be integer milliseconds. JSON shape:
{"shots":[{"index":1,"description":"...","duration_ms":2500}]}`, s.Input.Prompt, path, s.Plan.Roles, s.Plan.Scenes, s.Plan.AssetMetadata), &out)
				if err == nil && len(out.Shots) > 0 {
					s.Plan.Shots = normalizeShots(out.Shots)
					addTrace(&s.Plan, nodeName, "llm", time.Since(started), "")
					return nil
				}
				addTrace(&s.Plan, nodeName, "fallback", time.Since(started), classifyErr(err))
				if requireLLM {
					return requiredLLMError(nodeName, err)
				}
			} else if requireLLM {
				addTrace(&s.Plan, nodeName, "fallback", time.Since(started), "llm_required_but_not_configured")
				return requiredLLMError(nodeName, fmt.Errorf("llm is required but not configured"))
			}
			s.Plan.Shots = fallbackShots(path)
			addTrace(&s.Plan, nodeName, "fallback", time.Since(started), "")
			return nil
		})
	}
}

func durationToolsNode(toolsNode *compose.ToolsNode) func(context.Context, PromptInput) (PromptInput, error) {
	return func(ctx context.Context, in PromptInput) (PromptInput, error) {
		started := time.Now()
		var shots []Shot
		err := compose.ProcessState[*workflowState](ctx, func(ctx context.Context, s *workflowState) error {
			shots = append([]Shot(nil), s.Plan.Shots...)
			return nil
		})
		if err != nil {
			return in, err
		}

		var estimate DurationEstimate
		err = invokePlanningTool(ctx, toolsNode, "estimate_video_duration", durationRequest{Shots: shots, TargetDurationMS: 10000}, &estimate)
		return in, compose.ProcessState[*workflowState](ctx, func(ctx context.Context, s *workflowState) error {
			if err != nil {
				addTrace(&s.Plan, nodeDurationTools, "tools_node", time.Since(started), err.Error())
				return nil
			}
			s.Plan.Duration = &estimate
			addTrace(&s.Plan, nodeDurationTools, "tools_node", time.Since(started), "")
			return nil
		})
	}
}

func qualityNode(ctx context.Context, in PromptInput) (PromptInput, error) {
	return in, compose.ProcessState[*workflowState](ctx, func(ctx context.Context, s *workflowState) error {
		started := time.Now()
		quality := evaluateQuality(s.Input.Prompt, s.Plan)
		score := quality.StructureScore + quality.SemanticScore + quality.LanguageScore
		if score > 100 {
			score = 100
		}
		s.Plan.Quality = quality
		s.QualityScore = score
		s.Plan.QualityScore = score
		errText := ""
		if len(quality.Issues) > 0 {
			errText = strings.Join(quality.Issues, "; ")
		}
		addTrace(&s.Plan, nodeQuality, "semantic_rule", time.Since(started), errText)
		return nil
	})
}

func evaluateQuality(prompt string, plan CreationPlan) QualityBreakdown {
	quality := QualityBreakdown{}

	if len(plan.Shots) >= 3 {
		quality.StructureScore += 25
	} else {
		quality.Issues = append(quality.Issues, "insufficient_shots")
	}
	if len(plan.Scenes) >= 3 {
		quality.StructureScore += 15
	} else {
		quality.Issues = append(quality.Issues, "insufficient_scenes")
	}
	if plan.Duration != nil && abs(plan.Duration.DeltaMS) <= 2500 {
		quality.StructureScore += 15
	} else {
		quality.Issues = append(quality.Issues, "duration_out_of_range")
	}

	keywords := extractPromptKeywords(prompt)
	text := strings.ToLower(planText(plan))
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			quality.KeywordHits = append(quality.KeywordHits, keyword)
		} else {
			quality.MissingKeywords = append(quality.MissingKeywords, keyword)
		}
	}
	if len(keywords) == 0 {
		quality.SemanticScore = 20
	} else {
		quality.SemanticScore = int(float64(len(quality.KeywordHits)) / float64(len(keywords)) * 35)
	}
	if len(quality.MissingKeywords) > 0 {
		quality.Issues = append(quality.Issues, "missing_prompt_keywords")
	}

	if languageAligned(prompt, text) {
		quality.LanguageScore = 10
	} else {
		quality.Issues = append(quality.Issues, "language_mismatch")
	}

	return quality
}

func repairNode(ctx context.Context, in PromptInput) (PromptInput, error) {
	return in, compose.ProcessState[*workflowState](ctx, func(ctx context.Context, s *workflowState) error {
		started := time.Now()
		s.RepairAttempts++
		s.Plan.RepairAttempts = s.RepairAttempts
		if len(s.Plan.Shots) == 0 {
			s.Plan.Shots = fallbackShots(s.PromptType)
		}
		keywords := extractPromptKeywords(s.Input.Prompt)
		anchor := promptAnchor(s.Input.Prompt, keywords)
		target := 10000
		perShot := target / len(s.Plan.Shots)
		for i := range s.Plan.Shots {
			s.Plan.Shots[i].Index = i + 1
			s.Plan.Shots[i].DurationMS = perShot
			if anchor != "" && !containsAnyKeyword(s.Plan.Shots[i].Description, keywords) {
				if hasCJK(s.Input.Prompt) {
					s.Plan.Shots[i].Description = fmt.Sprintf("%s。围绕「%s」重写镜头，避免偏离用户 prompt", s.Plan.Shots[i].Description, anchor)
				} else {
					s.Plan.Shots[i].Description = fmt.Sprintf("%s. Rewritten around %s to stay aligned with the user prompt", s.Plan.Shots[i].Description, anchor)
				}
			} else if !strings.Contains(strings.ToLower(s.Plan.Shots[i].Description), "repair") {
				s.Plan.Shots[i].Description = s.Plan.Shots[i].Description + " [duration repaired]"
			}
		}
		s.Plan.Quality.Issues = append(s.Plan.Quality.Issues, "repaired_semantic_anchor")
		addTrace(&s.Plan, nodeRepair, "repair_loop", time.Since(started), strings.Join(s.Plan.Quality.MissingKeywords, ","))
		return nil
	})
}

func dialoguesNode(filler JSONFiller, requireLLM bool) func(context.Context, PromptInput) (PromptInput, error) {
	return func(ctx context.Context, in PromptInput) (PromptInput, error) {
		return in, compose.ProcessState[*workflowState](ctx, func(ctx context.Context, s *workflowState) error {
			started := time.Now()
			if filler != nil {
				var out struct {
					Dialogues []Dialogue `json:"dialogues"`
				}
				err := filler.FillJSON(ctx, systemPrompt(), fmt.Sprintf(`Prompt: %s
Shots: %+v

Generate one short dialogue or voice-over line per shot for this exact prompt. Keep the same language as the prompt when possible. JSON shape:
{"dialogues":[{"shot_index":1,"text":"..."}]}`, s.Input.Prompt, s.Plan.Shots), &out)
				if err == nil && len(out.Dialogues) > 0 {
					s.Plan.Dialogues = out.Dialogues
					addTrace(&s.Plan, nodeDialogues, "llm", time.Since(started), "")
					return nil
				}
				addTrace(&s.Plan, nodeDialogues, "fallback", time.Since(started), classifyErr(err))
				if requireLLM {
					return requiredLLMError(nodeDialogues, err)
				}
			} else if requireLLM {
				addTrace(&s.Plan, nodeDialogues, "fallback", time.Since(started), "llm_required_but_not_configured")
				return requiredLLMError(nodeDialogues, fmt.Errorf("llm is required but not configured"))
			}
			s.Plan.Dialogues = fallbackDialogues(s.Plan.Shots)
			addTrace(&s.Plan, nodeDialogues, "fallback", time.Since(started), "")
			return nil
		})
	}
}

func finalPlanNode(ctx context.Context, in PromptInput) (CreationPlan, error) {
	var plan CreationPlan
	err := compose.ProcessState[*workflowState](ctx, func(ctx context.Context, s *workflowState) error {
		plan = s.Plan
		return nil
	})
	if err != nil {
		return CreationPlan{}, err
	}
	return plan, nil
}

func newPlanningTools(ctx context.Context) (*compose.ToolsNode, error) {
	assetTool, err := utils.InferTool("inspect_asset_metadata", "inspect uploaded asset metadata for planning", inspectAssetMetadata)
	if err != nil {
		return nil, err
	}
	durationTool, err := utils.InferTool("estimate_video_duration", "estimate total storyboard duration", estimateVideoDuration)
	if err != nil {
		return nil, err
	}
	return compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{assetTool, durationTool},
	})
}

func invokePlanningTool(ctx context.Context, toolsNode *compose.ToolsNode, name string, args any, out any) error {
	b, err := json.Marshal(args)
	if err != nil {
		return err
	}
	msg := schema.AssistantMessage("", nil)
	msg.ToolCalls = []schema.ToolCall{{
		ID:   "call_" + name,
		Type: "function",
		Function: schema.FunctionCall{
			Name:      name,
			Arguments: string(b),
		},
	}}
	outs, err := toolsNode.Invoke(ctx, msg)
	if err != nil {
		return err
	}
	if len(outs) == 0 {
		return fmt.Errorf("tool %s returned no output", name)
	}
	return json.Unmarshal([]byte(outs[0].Content), out)
}

type assetMetadataRequest struct {
	Assets []AssetRef `json:"assets"`
}

type durationRequest struct {
	Shots            []Shot `json:"shots"`
	TargetDurationMS int    `json:"target_duration_ms"`
}

func inspectAssetMetadata(ctx context.Context, in *assetMetadataRequest) (*AssetMetadata, error) {
	meta := &AssetMetadata{AssetCount: len(in.Assets), HasUserMaterial: len(in.Assets) > 0}
	for _, asset := range in.Assets {
		switch strings.ToLower(asset.Kind) {
		case "image", "photo", "png", "jpg", "jpeg":
			meta.ImageCount++
			meta.EstimatedBytes += 2 * 1024 * 1024
		case "video", "mp4", "mov":
			meta.VideoCount++
			meta.EstimatedBytes += 20 * 1024 * 1024
		case "audio", "wav", "mp3":
			meta.AudioCount++
			meta.EstimatedBytes += 5 * 1024 * 1024
		default:
			meta.EstimatedBytes += 1024 * 1024
		}
	}
	return meta, nil
}

func estimateVideoDuration(ctx context.Context, in *durationRequest) (*DurationEstimate, error) {
	total := 0
	for _, shot := range in.Shots {
		total += shot.DurationMS
	}
	target := in.TargetDurationMS
	if target == 0 {
		target = 10000
	}
	return &DurationEstimate{
		ShotCount:        len(in.Shots),
		TotalDurationMS:  total,
		TargetDurationMS: target,
		DeltaMS:          total - target,
	}, nil
}

func planningCallback() callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
			if c := getCallbackCollector(ctx); c != nil {
				c.start(runName(info), runComponent(info))
			}
			return ctx
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			if c := getCallbackCollector(ctx); c != nil {
				c.end(runName(info), runComponent(info))
			}
			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			if c := getCallbackCollector(ctx); c != nil {
				c.fail(runName(info), runComponent(info), err)
			}
			return ctx
		}).
		Build()
}

type callbackCollectorKey struct{}

type callbackCollector struct {
	mu      sync.Mutex
	starts  map[string]time.Time
	records []CallbackEvent
}

func newCallbackCollector() *callbackCollector {
	return &callbackCollector{starts: make(map[string]time.Time)}
}

func getCallbackCollector(ctx context.Context) *callbackCollector {
	c, _ := ctx.Value(callbackCollectorKey{}).(*callbackCollector)
	return c
}

func (c *callbackCollector) start(node string, component string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.starts[node] = time.Now()
	c.records = append(c.records, CallbackEvent{Node: node, Component: component, Event: "start"})
}

func (c *callbackCollector) end(node string, component string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	started := c.starts[node]
	duration := int64(0)
	if !started.IsZero() {
		duration = time.Since(started).Milliseconds()
	}
	c.records = append(c.records, CallbackEvent{Node: node, Component: component, Event: "end", DurationMS: duration})
}

func (c *callbackCollector) fail(node string, component string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, CallbackEvent{Node: node, Component: component, Event: "error", Error: classifyErr(err)})
}

func (c *callbackCollector) snapshot() []CallbackEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]CallbackEvent(nil), c.records...)
}

func runName(info *callbacks.RunInfo) string {
	if info == nil || info.Name == "" {
		return "unknown"
	}
	return info.Name
}

func runComponent(info *callbacks.RunInfo) string {
	if info == nil || info.Component == "" {
		return "unknown"
	}
	return fmt.Sprint(info.Component)
}

func classifyPrompt(prompt string) string {
	lower := strings.ToLower(prompt)
	switch {
	case strings.Contains(lower, "tutorial") || strings.Contains(prompt, "教程") || strings.Contains(prompt, "教学"):
		return "tutorial"
	case strings.Contains(lower, "story") || strings.Contains(prompt, "剧情") || strings.Contains(prompt, "故事"):
		return "story"
	default:
		return "commercial"
	}
}

func fallbackScenes(promptType string) []Scene {
	switch promptType {
	case "tutorial":
		return []Scene{{Name: "problem setup", Mood: "clear"}, {Name: "step demonstration", Mood: "focused"}, {Name: "final result", Mood: "confident"}}
	case "story":
		return []Scene{{Name: "character setup", Mood: "curious"}, {Name: "turning point", Mood: "tense"}, {Name: "resolution", Mood: "warm"}}
	default:
		return []Scene{{Name: "opening hook", Mood: "commercial"}, {Name: "product reveal", Mood: "focused"}, {Name: "ending callout", Mood: "clean"}}
	}
}

func fallbackShots(path string) []Shot {
	switch path {
	case "tutorial":
		return []Shot{{Index: 1, Description: "Show the user's pain point clearly", DurationMS: 2500}, {Index: 2, Description: "Demonstrate the key operation step by step", DurationMS: 4500}, {Index: 3, Description: "Show the final result and next action", DurationMS: 3000}}
	case "story":
		return []Shot{{Index: 1, Description: "Introduce the main character and setting", DurationMS: 3000}, {Index: 2, Description: "Reveal the conflict through action", DurationMS: 4000}, {Index: 3, Description: "Resolve with a memorable final frame", DurationMS: 3000}}
	default:
		return []Shot{{Index: 1, Description: "Fast product or visual hook", DurationMS: 2500}, {Index: 2, Description: "Show product value through action", DurationMS: 3500}, {Index: 3, Description: "Close with brand or callout frame", DurationMS: 3000}}
	}
}

func fallbackDialogues(shots []Shot) []Dialogue {
	dialogues := make([]Dialogue, 0, len(shots))
	for _, shot := range shots {
		dialogues = append(dialogues, Dialogue{ShotIndex: shot.Index, Text: "Concise voice-over aligned with this shot."})
	}
	return dialogues
}

func planText(plan CreationPlan) string {
	var b strings.Builder
	for _, role := range plan.Roles {
		b.WriteString(role.Name)
		b.WriteString(" ")
		b.WriteString(role.Description)
		b.WriteString(" ")
	}
	for _, scene := range plan.Scenes {
		b.WriteString(scene.Name)
		b.WriteString(" ")
		b.WriteString(scene.Mood)
		b.WriteString(" ")
	}
	for _, shot := range plan.Shots {
		b.WriteString(shot.Description)
		b.WriteString(" ")
	}
	for _, dialogue := range plan.Dialogues {
		b.WriteString(dialogue.Text)
		b.WriteString(" ")
	}
	return b.String()
}

var englishWordPattern = regexp.MustCompile(`[A-Za-z0-9]+`)

func extractPromptKeywords(prompt string) []string {
	lower := strings.ToLower(prompt)
	seen := make(map[string]bool)
	var keywords []string
	add := func(keyword string) {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" || seen[keyword] {
			return
		}
		seen[keyword] = true
		keywords = append(keywords, keyword)
	}

	for _, word := range englishWordPattern.FindAllString(lower, -1) {
		if len(word) >= 4 && !isStopWord(word) {
			add(word)
		}
	}

	domainTerms := []string{"广告", "安全灯", "骑行", "夜间", "安全", "城市", "夜景", "科技", "教程", "剧情", "故事", "咖啡", "赛博朋克", "短视频"}
	for _, term := range domainTerms {
		if strings.Contains(prompt, term) {
			add(term)
		}
	}
	return keywords
}

func isStopWord(word string) bool {
	switch word {
	case "generate", "create", "make", "with", "that", "this", "video", "short":
		return true
	default:
		return false
	}
}

func languageAligned(prompt string, text string) bool {
	promptHasCJK := hasCJK(prompt)
	textHasCJK := hasCJK(text)
	if promptHasCJK {
		return textHasCJK
	}
	return true
}

func hasCJK(s string) bool {
	for _, r := range s {
		if (r >= '\u4e00' && r <= '\u9fff') || (r >= '\u3040' && r <= '\u30ff') || (r >= '\uac00' && r <= '\ud7af') {
			return true
		}
	}
	return false
}

func containsAnyKeyword(text string, keywords []string) bool {
	lower := strings.ToLower(text)
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func promptAnchor(prompt string, keywords []string) string {
	if len(keywords) > 0 {
		if len(keywords) > 8 {
			keywords = keywords[:8]
		}
		if hasCJK(prompt) {
			return strings.Join(keywords, "、")
		}
		return strings.Join(keywords, ", ")
	}
	if len([]rune(prompt)) > 24 {
		return string([]rune(prompt)[:24])
	}
	return prompt
}

func normalizeShots(shots []Shot) []Shot {
	for i := range shots {
		if shots[i].Index == 0 {
			shots[i].Index = i + 1
		}
		if shots[i].DurationMS <= 0 {
			shots[i].DurationMS = 3000
		}
	}
	return shots
}

func systemPrompt() string {
	return "You are a structured planning node in an AI video creation backend. Follow the user's prompt closely. Keep outputs concise, concrete, and production-ready."
}

func addTrace(plan *CreationPlan, node string, source string, duration time.Duration, errText string) {
	status := "success"
	if errText != "" {
		status = "error"
	}
	plan.PlanningTrace = append(plan.PlanningTrace, PlanningTrace{
		Step:       len(plan.PlanningTrace) + 1,
		Node:       node,
		Source:     source,
		Status:     status,
		DurationMS: duration.Milliseconds(),
		Error:      errText,
	})
}

func newRunID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("run-%d", time.Now().UTC().UnixNano())
	}
	return "run_" + hex.EncodeToString(b[:])
}

func classifyErr(err error) string {
	if err == nil {
		return "empty_output"
	}
	msg := err.Error()
	if len(msg) > 120 {
		msg = msg[:120]
	}
	return msg
}

func requiredLLMError(node string, err error) error {
	if err == nil {
		err = fmt.Errorf("empty model output")
	}
	return fmt.Errorf("llm required for %s: %w", node, err)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
