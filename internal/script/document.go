package script

import (
	"fmt"
	"regexp"
	"strings"

	creator "creator-pipeline/internal/eino"
)

type Document struct {
	Title              string          `json:"title"`
	Logline            string          `json:"logline"`
	Synopsis           string          `json:"synopsis"`
	Format             string          `json:"format"`
	DurationSec        int             `json:"duration_sec"`
	PromptType         string          `json:"prompt_type"`
	Characters         []Character     `json:"characters"`
	Scenes             []Scene         `json:"scenes"`
	StoryboardRows     []StoryboardRow `json:"storyboard_rows"`
	ShootingNotes      []string        `json:"shooting_notes"`
	AssetUsage         []AssetUsage    `json:"asset_usage"`
	QualityReview      QualityReview   `json:"quality_review"`
	SourceRunID        string          `json:"source_run_id"`
	TraceVersion       string          `json:"trace_version"`
	PlannerPathSummary string          `json:"planner_path_summary"`
}

type Character struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Scene struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Mood  string `json:"mood"`
}

type StoryboardRow struct {
	Index     int    `json:"index"`
	TimeRange string `json:"time_range"`
	Visual    string `json:"visual"`
	Voiceover string `json:"voiceover"`
	AssetHint string `json:"asset_hint"`
	Purpose   string `json:"purpose"`
}

type AssetUsage struct {
	ObjectKey string `json:"object_key"`
	Kind      string `json:"kind"`
	Usage     string `json:"usage"`
}

type QualityReview struct {
	Score          int      `json:"score"`
	Passed         bool     `json:"passed"`
	Issues         []string `json:"issues"`
	SuggestedFixes []string `json:"suggested_fixes"`
}

type RewriteRequest struct {
	ShotIndex   int    `json:"shot_index"`
	Instruction string `json:"instruction"`
}

func FromPlan(plan creator.CreationPlan) Document {
	doc := Document{
		Title:              titleFromPlan(plan),
		Logline:            loglineFromPlan(plan),
		Synopsis:           synopsisFromPlan(plan),
		Format:             "short_video_script",
		DurationSec:        durationSec(plan),
		PromptType:         plan.PromptType,
		SourceRunID:        plan.RunID,
		TraceVersion:       plan.TraceVersion,
		PlannerPathSummary: plannerPathSummary(plan),
	}
	for _, role := range plan.Roles {
		doc.Characters = append(doc.Characters, Character{Name: role.Name, Description: role.Description})
	}
	for i, scene := range plan.Scenes {
		doc.Scenes = append(doc.Scenes, Scene{Index: i + 1, Name: scene.Name, Mood: scene.Mood})
	}
	for i, shot := range plan.Shots {
		doc.StoryboardRows = append(doc.StoryboardRows, StoryboardRow{
			Index:     shot.Index,
			TimeRange: timeRange(plan.Shots, i),
			Visual:    shot.Description,
			Voiceover: dialogueForShot(plan.Dialogues, shot.Index),
			AssetHint: assetHint(plan.Assets, i),
			Purpose:   purposeForShot(i, len(plan.Shots), plan.PromptType),
		})
	}
	for _, asset := range plan.Assets {
		doc.AssetUsage = append(doc.AssetUsage, AssetUsage{ObjectKey: asset.ObjectKey, Kind: asset.Kind, Usage: "Reference material for visual planning and storyboard grounding"})
	}
	doc.ShootingNotes = shootingNotes(plan)
	doc.QualityReview = qualityReview(plan)
	return doc
}

func RewriteShot(plan *creator.CreationPlan, shotIndex int, instruction string) error {
	if shotIndex <= 0 {
		return fmt.Errorf("shot_index must be positive")
	}
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		instruction = "Make this shot clearer and more production-ready"
	}
	for i := range plan.Shots {
		if plan.Shots[i].Index == shotIndex {
			plan.Shots[i].Description = deterministicRewrite(plan.Shots[i].Description, instruction)
			return nil
		}
	}
	return fmt.Errorf("shot_index %d not found", shotIndex)
}

func RewriteDialogue(plan *creator.CreationPlan, shotIndex int, instruction string) error {
	if shotIndex <= 0 {
		return fmt.Errorf("shot_index must be positive")
	}
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		instruction = "Make this voice-over shorter and clearer"
	}
	for i := range plan.Dialogues {
		if plan.Dialogues[i].ShotIndex == shotIndex {
			plan.Dialogues[i].Text = deterministicRewrite(plan.Dialogues[i].Text, instruction)
			return nil
		}
	}
	if hasShot(*plan, shotIndex) {
		plan.Dialogues = append(plan.Dialogues, creator.Dialogue{ShotIndex: shotIndex, Text: deterministicRewrite("Concise voice-over for this shot.", instruction)})
		return nil
	}
	return fmt.Errorf("shot_index %d not found", shotIndex)
}

func Markdown(doc Document) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", doc.Title)
	fmt.Fprintf(&b, "## Logline\n%s\n\n", doc.Logline)
	fmt.Fprintf(&b, "## Synopsis\n%s\n\n", doc.Synopsis)
	fmt.Fprintf(&b, "## Characters\n")
	for _, c := range doc.Characters {
		fmt.Fprintf(&b, "- **%s**: %s\n", c.Name, c.Description)
	}
	if len(doc.Characters) == 0 {
		fmt.Fprintf(&b, "- No explicit characters generated.\n")
	}
	fmt.Fprintf(&b, "\n## Scenes\n")
	for _, s := range doc.Scenes {
		fmt.Fprintf(&b, "### Scene %d: %s\nMood: %s\n\n", s.Index, s.Name, s.Mood)
	}
	fmt.Fprintf(&b, "## Storyboard\n")
	fmt.Fprintf(&b, "| Time | Shot | Visual | Voice-over | Asset | Purpose |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | --- | --- | --- |\n")
	for _, row := range doc.StoryboardRows {
		fmt.Fprintf(&b, "| %s | %d | %s | %s | %s | %s |\n", esc(row.TimeRange), row.Index, esc(row.Visual), esc(row.Voiceover), esc(row.AssetHint), esc(row.Purpose))
	}
	fmt.Fprintf(&b, "\n## Shooting Notes\n")
	for _, note := range doc.ShootingNotes {
		fmt.Fprintf(&b, "- %s\n", note)
	}
	fmt.Fprintf(&b, "\n## Quality Review\n")
	fmt.Fprintf(&b, "- Score: %d\n", doc.QualityReview.Score)
	fmt.Fprintf(&b, "- Passed: %t\n", doc.QualityReview.Passed)
	fmt.Fprintf(&b, "- Issues: %s\n", joinOrNone(doc.QualityReview.Issues))
	fmt.Fprintf(&b, "- Suggested fixes: %s\n", joinOrNone(doc.QualityReview.SuggestedFixes))
	fmt.Fprintf(&b, "\n## Trace\n")
	fmt.Fprintf(&b, "- Run ID: %s\n", doc.SourceRunID)
	fmt.Fprintf(&b, "- Trace version: %s\n", doc.TraceVersion)
	fmt.Fprintf(&b, "- Planner path: %s\n", doc.PlannerPathSummary)
	return b.String()
}

func deterministicRewrite(original string, instruction string) string {
	base := strings.TrimSpace(original)
	if base == "" {
		base = "Updated creative beat"
	}
	return fmt.Sprintf("%s Revised note: %s.", strings.TrimRight(base, ".。"), strings.TrimRight(instruction, ".。"))
}

func titleFromPlan(plan creator.CreationPlan) string {
	words := keywordWords(plan.Prompt)
	if len(words) == 0 {
		return "Untitled Short Video Script"
	}
	if len(words) > 6 {
		words = words[:6]
	}
	return strings.Title(strings.Join(words, " "))
}

func loglineFromPlan(plan creator.CreationPlan) string {
	if len(plan.Shots) > 0 {
		return trimSentence(plan.Shots[0].Description, 180)
	}
	return trimSentence(plan.Prompt, 180)
}

func synopsisFromPlan(plan creator.CreationPlan) string {
	parts := make([]string, 0, len(plan.Scenes))
	for _, s := range plan.Scenes {
		parts = append(parts, s.Name)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("A %s short video developed from the prompt: %s", plan.PromptType, plan.Prompt)
	}
	return fmt.Sprintf("A %s short video moving through %s, designed around the original prompt: %s", plan.PromptType, strings.Join(parts, ", "), plan.Prompt)
}

func qualityReview(plan creator.CreationPlan) QualityReview {
	issues := append([]string(nil), plan.Quality.Issues...)
	if plan.QualityScore < 75 {
		issues = append(issues, "quality_score_below_threshold")
	}
	if len(plan.Shots) == 0 {
		issues = append(issues, "missing_storyboard")
	}
	fixes := make([]string, 0)
	for _, issue := range issues {
		switch issue {
		case "missing_prompt_keywords":
			fixes = append(fixes, "Rewrite affected shots to include missing prompt keywords.")
		case "language_mismatch":
			fixes = append(fixes, "Regenerate dialogue in the same language as the user prompt.")
		case "duration_out_of_range":
			fixes = append(fixes, "Rebalance shot durations to match the target runtime.")
		default:
			fixes = append(fixes, "Review and revise the storyboard before production.")
		}
	}
	return QualityReview{Score: plan.QualityScore, Passed: plan.QualityScore >= 75 && len(issues) == 0, Issues: dedupe(issues), SuggestedFixes: dedupe(fixes)}
}

func durationSec(plan creator.CreationPlan) int {
	total := 0
	if plan.Duration != nil && plan.Duration.TotalDurationMS > 0 {
		total = plan.Duration.TotalDurationMS
	} else {
		for _, shot := range plan.Shots {
			total += shot.DurationMS
		}
	}
	return (total + 999) / 1000
}

func timeRange(shots []creator.Shot, idx int) string {
	start := 0
	for i := 0; i < idx; i++ {
		start += shots[i].DurationMS
	}
	end := start + shots[idx].DurationMS
	return fmt.Sprintf("%ds-%ds", start/1000, (end+999)/1000)
}

func dialogueForShot(dialogues []creator.Dialogue, shotIndex int) string {
	for _, d := range dialogues {
		if d.ShotIndex == shotIndex {
			return d.Text
		}
	}
	return ""
}

func assetHint(assets []creator.AssetRef, idx int) string {
	if len(assets) == 0 {
		return "No uploaded asset"
	}
	asset := assets[idx%len(assets)]
	return fmt.Sprintf("%s (%s)", asset.ObjectKey, asset.Kind)
}

func purposeForShot(idx int, total int, promptType string) string {
	if idx == 0 {
		return "Hook"
	}
	if idx == total-1 {
		if promptType == "commercial" {
			return "CTA"
		}
		return "Resolution"
	}
	return "Development"
}

func shootingNotes(plan creator.CreationPlan) []string {
	notes := []string{"Keep each shot visually concrete and easy to produce.", "Review voice-over length against the planned shot duration."}
	if len(plan.Assets) > 0 {
		notes = append(notes, "Use uploaded assets as visual references instead of inventing unrelated details.")
	}
	if plan.RepairAttempts > 0 {
		notes = append(notes, "Storyboard was repaired automatically; review semantic anchors before production.")
	}
	return notes
}

func hasShot(plan creator.CreationPlan, shotIndex int) bool {
	for _, shot := range plan.Shots {
		if shot.Index == shotIndex {
			return true
		}
	}
	return false
}

var wordPattern = regexp.MustCompile(`[\p{L}\p{N}]+`)

func keywordWords(s string) []string {
	return wordPattern.FindAllString(s, -1)
}

func trimSentence(s string, limit int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= limit {
		return s
	}
	r := []rune(s)
	return string(r[:limit]) + "..."
}

func esc(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, "; ")
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func plannerPathSummary(plan creator.CreationPlan) string {
	if len(plan.PlanningTrace) == 0 {
		if plan.PromptType == "" {
			return "unknown"
		}
		return fmt.Sprintf("classify -> %s storyboard -> quality review -> dialogue", plan.PromptType)
	}
	parts := make([]string, 0, len(plan.PlanningTrace))
	for _, event := range plan.PlanningTrace {
		node := strings.TrimSpace(event.Node)
		if node == "" {
			continue
		}
		source := strings.TrimSpace(event.Source)
		status := strings.TrimSpace(event.Status)
		label := node
		if source != "" {
			label += ":" + source
		}
		if status != "" {
			label += ":" + status
		}
		parts = append(parts, label)
	}
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.Join(parts, " -> ")
}
