package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Retr0413/wataridori/internal/agent/domain"
)

type InstructionSet struct {
	Version string `json:"version"`
	Hash    string `json:"hash"`
	Content string `json:"content"`
}

func NewInstructionSet(version, content string) (InstructionSet, error) {
	if strings.TrimSpace(version) == "" || strings.TrimSpace(content) == "" {
		return InstructionSet{}, errors.New("agent application: instruction version and content are required")
	}
	sum := sha256.Sum256([]byte(content))
	return InstructionSet{Version: version, Hash: hex.EncodeToString(sum[:]), Content: content}, nil
}

// ToolDefinition is provider-neutral. Infrastructure adapters map it to the
// selected model API but may not alter its semantics or expand its schema.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

func (t ToolDefinition) Validate() error {
	if strings.TrimSpace(t.Name) == "" || strings.TrimSpace(t.Description) == "" {
		return errors.New("agent application: tool name and description are required")
	}
	if len(t.InputSchema) == 0 || !json.Valid(t.InputSchema) {
		return errors.New("agent application: tool input schema must be valid JSON")
	}
	return nil
}

// AgentInput is the complete normalized context presented to the model for one
// bounded recovery step.
type AgentInput struct {
	Instructions   InstructionSet        `json:"instructions"`
	RecoveryRunID  string                `json:"recoveryRunId"`
	Scope          domain.Scope          `json:"scope"`
	Policy         domain.PolicySnapshot `json:"policy"`
	Evidence       []domain.Evidence     `json:"evidence"`
	AllowedTools   []ToolDefinition      `json:"allowedTools"`
	AllowedActions []domain.Action       `json:"allowedActions"`
	Budget         domain.Budget         `json:"budget"`
}

// AgentInputAssembler is the single semantic context-injection boundary. Model
// adapters receive its output and must not add instructions or select evidence.
type AgentInputAssembler struct {
	instructions InstructionSet
	tools        []ToolDefinition
}

func NewAgentInputAssembler(instructions InstructionSet, tools []ToolDefinition) (*AgentInputAssembler, error) {
	if strings.TrimSpace(instructions.Version) == "" || strings.TrimSpace(instructions.Hash) == "" || strings.TrimSpace(instructions.Content) == "" {
		return nil, errors.New("agent application: valid instructions are required")
	}
	seen := make(map[string]struct{}, len(tools))
	cloned := make([]ToolDefinition, len(tools))
	for i, tool := range tools {
		if err := tool.Validate(); err != nil {
			return nil, err
		}
		if _, ok := seen[tool.Name]; ok {
			return nil, fmt.Errorf("agent application: duplicate tool %q", tool.Name)
		}
		seen[tool.Name] = struct{}{}
		cloned[i] = cloneTool(tool)
	}
	return &AgentInputAssembler{instructions: instructions, tools: cloned}, nil
}

func (a *AgentInputAssembler) Build(run domain.RecoveryRun, evidence []domain.Evidence) (AgentInput, error) {
	if a == nil {
		return AgentInput{}, errors.New("agent application: nil context assembler")
	}
	if err := run.Validate(); err != nil {
		return AgentInput{}, err
	}
	totalBytes := 0
	evidenceCopy := make([]domain.Evidence, len(evidence))
	seen := make(map[string]struct{}, len(evidence))
	for i, item := range evidence {
		if err := item.Validate(); err != nil {
			return AgentInput{}, err
		}
		if _, ok := seen[item.ID]; ok {
			return AgentInput{}, fmt.Errorf("agent application: duplicate evidence %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		totalBytes += len(item.Content)
		evidenceCopy[i] = item
	}
	if totalBytes > run.Policy.Budget.MaxEvidenceBytes {
		return AgentInput{}, fmt.Errorf(
			"agent application: evidence exceeds context budget: %d > %d bytes",
			totalBytes,
			run.Policy.Budget.MaxEvidenceBytes,
		)
	}
	tools := make([]ToolDefinition, len(a.tools))
	for i, tool := range a.tools {
		tools[i] = cloneTool(tool)
	}
	actions := append([]domain.Action(nil), run.Policy.AllowedActions...)
	return AgentInput{
		Instructions:   a.instructions,
		RecoveryRunID:  run.ID,
		Scope:          run.Scope,
		Policy:         run.Policy,
		Evidence:       evidenceCopy,
		AllowedTools:   tools,
		AllowedActions: actions,
		Budget:         run.Policy.Budget,
	}, nil
}

func cloneTool(tool ToolDefinition) ToolDefinition {
	tool.InputSchema = append(json.RawMessage(nil), tool.InputSchema...)
	return tool
}
