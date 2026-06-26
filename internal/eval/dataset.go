package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func LoadDataset(path string) ([]Case, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return DecodeDataset(f)
}

func DecodeDataset(r io.Reader) ([]Case, error) {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var cases []Case
	lineNo := 0
	seen := map[string]bool{}
	for s.Scan() {
		lineNo++
		line := strings.TrimPrefix(strings.TrimSpace(s.Text()), "\ufeff")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var c Case
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, fmt.Errorf("decode dataset line %d: %w", lineNo, err)
		}
		if err := c.Validate(); err != nil {
			return nil, fmt.Errorf("dataset line %d: %w", lineNo, err)
		}
		if seen[c.ID] {
			return nil, fmt.Errorf("dataset line %d: duplicate id %q", lineNo, c.ID)
		}
		seen[c.ID] = true
		cases = append(cases, c)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("dataset is empty")
	}
	return cases, nil
}

func (c Case) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(c.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	if strings.TrimSpace(c.ExpectedType) == "" {
		return fmt.Errorf("expected_type is required")
	}
	if c.MinShots <= 0 {
		return fmt.Errorf("min_shots must be positive")
	}
	if c.TargetDurationMS <= 0 {
		return fmt.Errorf("target_duration_ms must be positive")
	}
	lang := strings.ToLower(strings.TrimSpace(c.Language))
	if lang != "en" && lang != "zh" {
		return fmt.Errorf("language must be en or zh")
	}
	return nil
}
