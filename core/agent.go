package core

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var ToolAgentName = "agent"
var ToolShellName = "shell"

// Fixed tool definitions used by every agent.
var Tools = []Tool{
	{
		Type: "function",
		Function: ToolFunction{
			Name:        ToolAgentName,
			Description: "Spawn a sub-agent to complete a task. Sub-agents are independent agents with their own system prompt and capabilities.",
			Parameters: Parameters{
				Type: "object",
				Properties: map[string]Property{
					"name": {
						Type:        "string",
						Description: "Name of the sub-agent, corresponding to a subdirectory under agents/",
					},
					"task": {
						Type:        "string",
						Description: "Task description. The sub-agent will complete this independently.",
					},
				},
				Required: []string{"name", "task"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunction{
			Name:        ToolShellName,
			Description: "Execute a whitelisted shell command.",
			Parameters: Parameters{
				Type: "object",
				Properties: map[string]Property{
					"command": {
						Type:        "string",
						Description: "The shell command to execute",
					},
					"arguments": {
						Type:        "array",
						Description: "Arguments for the shell command",
						Items: &Property{
							Type: "string",
						},
					},
				},
				Required: []string{"command"},
			},
		},
	},
}

type ShellArguments struct {
	Command   string   `json:"command"`
	Arguments []string `json:"arguments,omitempty"`
}

type AgentArguments struct {
	Name string `json:"name"`
	Task string `json:"task"`
}

//go:embed protocol.md
var interactionProtocol string // Interaction protocol appended to every system prompt.

var subAgentsDir = "agents" // Relative path to sub-agents from each agent directory
var systemPromptFileName = "system.md"
var apiFileName = "api.md"

type Agent struct {
	SystemPrompt string
	Config       *Config
	SessionID    string
	AgentDir     string // directory containing system.md and agents/
	Workspace    string
}

// LoadAgent reads system.md, scans sub-agents, replaces {{AGENTS}}, and returns an Agent.
// It works identically for root and sub-agents.
func LoadAgent(dir string, cfg *Config, sessionID string, workspace string) (*Agent, error) {
	// Read system prompt
	systemPath := filepath.Join(dir, systemPromptFileName)
	systemBytes, err := os.ReadFile(systemPath)
	if err != nil {
		return nil, fmt.Errorf("read %s from %s: %w", systemPromptFileName, dir, err)
	}
	systemPrompt := string(systemBytes)

	// Look for sub-agents
	agentsDir := filepath.Join(dir, subAgentsDir)
	entries, err := os.ReadDir(agentsDir)
	var agentDescriptions string
	subAgents := []string{}
	if err == nil && len(entries) > 0 {
		var parts []string
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			apiPath := filepath.Join(agentsDir, e.Name(), apiFileName)
			apiBytes, err := os.ReadFile(apiPath)
			if err != nil {
				// Skip sub-agents without api.md
				continue
			}
			subAgents = append(subAgents, e.Name())
			parts = append(parts, fmt.Sprintf("## %s\n%s", e.Name(), string(apiBytes)))
		}
		if len(parts) > 0 {
			agentDescriptions = strings.Join(parts, "\n")
		}
	}
	if agentDescriptions == "" {
		agentDescriptions = "No sub-agents available."
	}

	// Replace placeholder and append protocol
	finalPrompt := strings.Replace(systemPrompt, "{{AGENTS}}", agentDescriptions, 1)
	protocol := strings.Replace(interactionProtocol, "{{ALLOWED_AGENTS}}", strings.Join(subAgents, ","), -1)
	protocol = strings.Replace(protocol, "{{ALLOWED_CMD}}", strings.Join(cfg.Shell.AllowedCmd, ","), -1)
	finalPrompt += protocol

	return &Agent{
		SystemPrompt: finalPrompt,
		Config:       cfg,
		SessionID:    sessionID,
		AgentDir:     dir,
		Workspace:    workspace,
	}, nil
}

// Run executes the main agent loop with the given user input.
func (a *Agent) Run(userInput string) (string, error) {
	if err := os.MkdirAll(a.Workspace, 0755); err != nil {
		return "", fmt.Errorf("create workspace: %w", err)
	}

	messages := []Message{
		{Role: RoleSystem, Content: a.SystemPrompt},
		{Role: RoleUser, Content: userInput},
	}
	log.Printf("[session=%s step=%d] User input: %s", a.SessionID, 0, truncate(userInput, 200))

	for step := 0; step < a.Config.LLM.MaxSteps; step++ {
		log.Printf("[session=%s step=%d] Calling LLM...", a.SessionID, step)
		request := ChatCompletionRequest{
			Messages: messages,
			Tools:    Tools,
		}
		response, err := Chat(&a.Config.LLM, request)
		if err != nil {
			return "", fmt.Errorf("LLM call failed: %w", err)
		}
		if len(response.Choices) == 0 {
			return "", fmt.Errorf("empty LLM response")
		}
		msg := response.Choices[0].Message
		log.Printf("[session=%s step=%d] LLM respond: %s", a.SessionID, step, truncate(msg.Content, 200))

		if len(msg.ToolCalls) > 0 {
			messages = append(messages, Message{
				Role:      RoleAssistant,
				Content:   msg.Content,
				ToolCalls: msg.ToolCalls,
			})
			for _, tc := range msg.ToolCalls {
				log.Printf("[session=%s step=%d] Tool call: %s(%s)", a.SessionID, step, tc.Function.Name, truncate(tc.Function.Arguments, 200))
				var result string
				switch tc.Function.Name {
				case ToolAgentName:
					var args AgentArguments
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
						result = fmt.Sprintf("Invalid agent arguments: %v", err)
					} else {
						subAgentDir := filepath.Join(a.AgentDir, subAgentsDir, args.Name)
						if _, err := os.Stat(subAgentDir); os.IsNotExist(err) {
							return fmt.Sprintf("Agent not found: %s", args.Name), nil
						}
						subWorkspace := filepath.Join(a.Workspace, args.Name)
						if _, err := os.Stat(subWorkspace); os.IsNotExist(err) {
							os.Mkdir(subWorkspace, 0755)
						}
						subAgent, err := LoadAgent(subAgentDir, a.Config, a.SessionID, subWorkspace)
						if err != nil {
							return "", fmt.Errorf("load sub-agent %s: %w", args.Name, err)
						}
						res, err := subAgent.Run(args.Task)
						if err != nil {
							result = fmt.Sprintf("Agent execution failed: %s", err.Error())
						} else {
							result = res
						}
					}
				case ToolShellName:
					var args ShellArguments
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
						result = fmt.Sprintf("Invalid shell arguments: %v", err)
					} else {
						subWorkspace := filepath.Join(a.Workspace, args.Command)
						if _, err := os.Stat(subWorkspace); os.IsNotExist(err) {
							os.Mkdir(subWorkspace, 0755)
						}
						res, err := ExecuteShell(args.Command, args.Arguments, a.SessionID, a.Config.Shell.AllowedCmd, subWorkspace)
						if err != nil {
							result = fmt.Sprintf("Execution failed: %s", err.Error())
						} else {
							result = res
						}
					}
				default:
					result = fmt.Sprintf("Unknown tool: %s", tc.Function.Name)
				}
				log.Printf("[session=%s step=%d] Tool result: %s", a.SessionID, step, truncate(result, 200))
				messages = append(messages, Message{
					Role:       RoleTool,
					ToolCallID: tc.ID,
					Content:    result,
				})
			}
		} else {
			if msg.Content != "" {
				log.Printf("[session=%s step=%d] Task complete", a.SessionID, step)
				messages = append(messages, Message{
					Role:    RoleAssistant,
					Content: msg.Content,
				})
				return msg.Content, nil
			} else {
				return "", fmt.Errorf("empty response from model")
			}
		}
	}
	return "", fmt.Errorf("reached max steps %d without completion", a.Config.LLM.MaxSteps)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
