package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool is an executable capability the agent can offer to the model.
// Schema fields (Name, Description, Parameters) describe the tool to the LLM;
// Execute runs it. Throw (return error) on failure — do not encode errors in content.
type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage // JSON Schema object
	Execute     func(ctx context.Context, args json.RawMessage) (string, error)
}

// Find returns the first tool with the given name.
func Find(list []Tool, name string) (Tool, bool) {
	for _, t := range list {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

// Run calls Execute. Returns an error if Execute is nil.
func (t Tool) Run(ctx context.Context, args json.RawMessage) (string, error) {
	if t.Execute == nil {
		return "", fmt.Errorf("tool %q: Execute is nil", t.Name)
	}
	return t.Execute(ctx, args)
}
