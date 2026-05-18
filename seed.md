# SeedClaw Genesis Seed

## Project Overview
Generate a **complete, compilable, runnable** Go project named SeedClaw — an autonomous AI agent framework.

## Core Philosophy
- **Unified Agent**: Only one agent type exists. The root agent and all sub-agents use identical loading logic, tool sets, and termination conditions.
- **Recursive Directory Tree**: Each agent directory contains `system.md` + `agents/`. Agents can have their own `agents/`, forming arbitrarily deep nesting.
- **Two Tools**: Every agent has `shell` and `agent`. How they are used is entirely determined by the content of `system.md` and `api.md`.
- **Safety**: Shell command whitelist is read from config file, controlling command names only. File write operations are restricted to the session workspace.

## Code Structure

```
seedclaw/
├── main.go
├── go.mod
├── core/
│ ├── llm.go # LLM API client
│ ├── agent.go # Agent loop (the only type, shared by root and sub-agents)
│ ├── executor.go # Shell command executor + agent dispatcher
│ └── config.go # Configuration loader
```

## Runtime Directory Structure

```
./
├── config.toml # Configuration file
├── system.md # Root agent's system prompt (contains {{AGENTS}} placeholder)
├── agents/ # Sub-agents available to the root agent
│ ├── write_file/
│ │ ├── system.md # This agent's own system prompt
│ │ ├── api.md # Public interface description
│ │ └── agents/ # (optional) Deeper sub-agents
│ │ └── validate_path/
│ │ ├── system.md
│ │ └── api.md
│ └── send_request/
│ ├── system.md
│ └── api.md
└── workspace/ # Workspace root for all sessions
└── <session_id>/ # Per-session isolated workspace
```

## Implementation Requirements

### 1. Configuration (core/config.go)

Read `config.toml` with the following structure:
```toml
[llm]
api_key = "sk-xxx"
base_url = "https://api.openai.com/v1"
model = "gpt-4o"
max_steps = 10

[agent]
root = "."
workspace = "./workspace"

[shell]
allowed_cmd = ["curl"]
```

* Read from `config.toml` in the current directory by default
* Support -c `path/to/config.toml` flag to specify config file path
* Exit with error if `api_key`, `base_url`, or `model` is empty
* `max_steps` defaults to 10
* `root` is the agent root directory, containing the top-level `system.md` and `agents/`. Defaults to current directory.
* `workspace` is the workspace root path, defaults to `./workspace`. Each session gets its own subdirectory.
* `allowed_cmd` is `[]string`, listing allowed command names. Empty slice means deny all.

### 2. Unified Agent Loading (core/agent.go)

Implement LoadAgent(dir string, cfg *Config, sessionID string) (*Agent, error). Both the root agent and all sub-agents are created through this function:
1. Read `dir/system.md`
2. Scan all subdirectories under `dir/agents/`
3. For each subdirectory, read `api.md` inside it
4. Concatenate all `api.md` contents as:
  ```markdown
## agent_name
Full content of api.md
  ```

5. Replace the {{AGENTS}} placeholder in system.md with the concatenated result
6. If agents/ directory does not exist or is empty, replace {{AGENTS}} with "No sub-agents available."
7. Append the interaction protocol to the end of the system prompt
8. Return the Agent instance

All agents (root and sub-agents) are loaded through this same function. No special cases.

### 3. Interaction Protocol (hardcoded in agent.go, appended to SystemPrompt)

```markdown
## Interaction Protocol

### Tool Calls
You have two tools. Use OpenAI Function Calling format: return tool_calls in your response.

- shell: Execute a whitelisted shell command.
  Parameter: command (string)

- agent: Spawn a sub-agent to complete a task.
  Parameters: agent_name (string), task (string)

When calling a tool, function.arguments is a JSON string:
{"command": "ls -la"}
{"agent_name": "write_file", "task": "Write Hello World to test.txt"}

### Task Completion
When the task is complete or cannot proceed, output in content:
- Success: <FINAL>summary content</FINAL>
- Failure: <FINAL>Cannot complete: reason</FINAL>

### Important Rules
1. Never return both content and tool_calls simultaneously
2. When executing an action, return only tool_calls with content empty
3. When ending a task, return only content (with <FINAL>), no tool_calls
4. Call only one tool at a time. Wait for the result before deciding next step.
5. You may write your thinking in content (without <FINAL>) as an intermediate step to continue the loop.
```

### 4. LLM Client (core/llm.go)
* Use only Go standard library `net/http`
* `Chat(messages []Message, tools []Tool) (*Response, error)`
* Message struct: `Role, Content, ToolCalls, ToolCallID` — must serialize to OpenAI API format
* Tool struct: Name, Description, Parameters
* Call `{base_url}/chat/completions` with non-streaming requests
* Include two tool definitions in the request:
```json
{
  "model": "gpt-4o",
  "messages": [...],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "shell",
        "description": "Execute a whitelisted shell command.",
        "parameters": {
          "type": "object",
          "properties": {
            "command": {"type": "string", "description": "The shell command to execute"}
          },
          "required": ["command"]
        }
      }
    },
    {
      "type": "function",
      "function": {
        "name": "agent",
        "description": "Spawn a sub-agent to complete a task. Sub-agents are independent agents with their own system prompt and capabilities.",
        "parameters": {
          "type": "object",
          "properties": {
            "agent_name": {"type": "string", "description": "Name of the sub-agent, corresponding to a subdirectory under agents/"},
            "task": {"type": "string", "description": "Task description. The sub-agent will complete this independently."}
          },
          "required": ["agent_name", "task"]
        }
      }
    }
  ]
}
```

### 5. Response Parsing (agent.go main loop)

**shell tool call response:**
```json
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": null,
      "tool_calls": [{
        "id": "call_xyz789",
        "type": "function",
        "function": {
          "name": "shell",
          "arguments": "{\"command\":\"ls -la\"}"
        }
      }]
    },
    "finish_reason": "tool_calls"
  }]
}
```

**agent tool call response:**
```json
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": null,
      "tool_calls": [{
        "id": "call_abc456",
        "type": "function",
        "function": {
          "name": "agent",
          "arguments": "{\"agent_name\":\"write_file\",\"task\":\"Write Hello World to test.txt\"}"
        }
      }]
    },
    "finish_reason": "tool_calls"
  }]
}
```

**Task completion response:**
```json
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": "Done.<FINAL>File test.txt written successfully.</FINAL>",
      "tool_calls": null
    },
    "finish_reason": "stop"
  }]
}
```

**Intermediate thinking response:**
```json
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": "I need to check what files are in the current directory before deciding the next step.",
      "tool_calls": null
    },
    "finish_reason": "stop"
  }]
}
```

**Main loop logic:**

* `message.tool_calls` is non-empty → execute the tool, append result as `role: "tool"`, continue loop
* `message.tool_calls` is empty, content contains `<FINAL>` → extract FINAL content, end loop
* `message.tool_calls` is empty, content does not contain `<FINAL>` → intermediate thinking, append as `role: "assistant"`, continue loop
* Step count reaches `max_steps` without termination → force stop, return error

### 6. Executor (core/executor.go)

**ExecuteShell(command string, sessionID string, allowedCmd []string, workspaceRoot string) (string, error):**

1. Parse the command string, extract the command name (first token before space, strip path prefix like `/usr/bin/`)
2. Check if command name exists in `allowedCmd`
3. Not found → return "Rejected: command not in whitelist"
4. Split command by spaces into command name and argument array
5. For write operations (`rm`, `mv`, `cp`, `echo >`, `echo >>`):
  * Extract file paths from arguments
  * Verify paths are under `workspaceRoot/<sessionID>/`
  * If not, reject and return error
6. Execute with `os/exec`, settings:
  * Working directory = `workspaceRoot/<sessionID>/` (create if not exists)
  * 30 second timeout
7. Return stdout content on success, stderr content on failure

**ExecuteAgent(agentName string, task string, cfg *Config, sessionID string) (string, error):**

1. Look up `agents/<agentName>/` directory
2. Not found → return "Agent not found: <agentName>"
3. Call `LoadAgent("agents/<agentName>", cfg, sessionID)` to create sub-agent
4. Sub-agent shares the same sessionID and workspace as the parent
5. Call `subAgent.Run(task)` to run the sub-agent loop
6. Return the sub-agent's final result (FINAL content)

### 7.Agent Main Loop (core/agent.go)

```go
type Agent struct {
    SystemPrompt string
    Tools        []Tool
    Config       *Config
    SessionID    string
    AgentDir     string
}

func (a *Agent) Run(userInput string) (string, error) {
    // Create workspace directory
    workDir := filepath.Join(a.Config.Workspace, a.SessionID)
    os.MkdirAll(workDir, 0755)
    
    // Initialize message history
    messages := []Message{
        {Role: "system", Content: a.SystemPrompt},
        {Role: "user", Content: userInput},
    }
    
    for step := 0; step < a.Config.MaxSteps; step++ {
        log.Printf("[session=%s step=%d] Calling LLM...", a.SessionID, step)
        
        resp, err := Chat(messages, a.Tools)
        if err != nil {
            return "", fmt.Errorf("LLM call failed: %w", err)
        }
        
        msg := resp.Choices[0].Message
        
        // Case 1: model wants to call a tool
        if len(msg.ToolCalls) > 0 {
            tc := msg.ToolCalls[0]
            log.Printf("[session=%s step=%d] Tool call: %s", a.SessionID, step, tc.Function.Name)
            
            var result string
            switch tc.Function.Name {
            case "shell":
                var args struct{ Command string }
                json.Unmarshal([]byte(tc.Function.Arguments), &args)
                result, err = ExecuteShell(args.Command, a.SessionID, a.Config.Shell.AllowedCmd, a.Config.Agent.Workspace)
            case "agent":
                var args struct {
                    AgentName string `json:"agent_name"`
                    Task      string `json:"task"`
                }
                json.Unmarshal([]byte(tc.Function.Arguments), &args)
                result, err = ExecuteAgent(args.AgentName, args.Task, a.Config, a.SessionID)
            default:
                result = fmt.Sprintf("Unknown tool: %s", tc.Function.Name)
            }
            if err != nil {
                result = fmt.Sprintf("Execution failed: %s", err.Error())
            }
            
            log.Printf("[session=%s step=%d] Tool result: %s", a.SessionID, step, truncate(result, 200))
            
            // Append assistant message (with tool_calls)
            messages = append(messages, Message{
                Role:      "assistant",
                ToolCalls: msg.ToolCalls,
            })
            // Append tool result message
            messages = append(messages, Message{
                Role:       "tool",
                ToolCallID: tc.ID,
                Content:    result,
            })
            continue
        }
        
        // Case 2: model returns text
        if msg.Content != "" {
            if strings.Contains(msg.Content, "<FINAL>") {
                log.Printf("[session=%s step=%d] Task complete", a.SessionID, step)
                return extractFinal(msg.Content), nil
            }
            
            log.Printf("[session=%s step=%d] Thinking: %s", a.SessionID, step, truncate(msg.Content, 200))
            messages = append(messages, Message{
                Role:    "assistant",
                Content: msg.Content,
            })
            continue
        }
        
        return "", fmt.Errorf("empty response")
    }
    
    return "", fmt.Errorf("reached max steps %d without completion", a.Config.MaxSteps)
}
```

### Session Management

* Generate `session_id` on each run (format: `YYYYMMDD-HHMMSS-6random`)
* Create `workspace/<session_id>/` directory
* Root agent and all sub-agents share the same sessionID and workspace
* Log format: `[session=xxx step=n] message`

### 9. Entry Point (main.go)

* Parse command line arguments:
  * -c path/to/config.toml: specify config file
  * First non-flag argument as single-run task description; no argument enters interactive mode
* Interactive mode: read input line by line, /quit to exit
* Single-run mode: execute and print result, then exit
* Print session_id on startup
* Root agent loaded via LoadAgent(config.Agent.Root, config, sessionID)

### 10. Example Files

system.md (root agent):

```markdown
You are SeedClaw, an autonomous AI agent. Complete tasks by orchestrating sub-agents or executing commands directly.

## Available Sub-Agents
{{AGENTS}}

## Guidelines
1. Prefer sub-agents for complex tasks.
2. Use shell for direct command execution when needed.
3. Analyze results after each step and adjust your plan.
4. When encountering errors, try alternative approaches.
```

agents/write_file/api.md:

```markdown
# write_file
Write content to a file.

Example:
agent("write_file", "Write Hello World to test.txt")
```

agents/write_file/system.md:

```markdown
You are a file writing specialist. Write content to the specified file.

## Available Sub-Agents
{{AGENTS}}

## Method
Use echo to write files:
echo "content" > filepath

Output `<FINAL>Written: filepath</FINAL>` when done.
```

agents/send_request/api.md:

```markdown
# send_request
Send an HTTP request and return the response.

Example:
agent("send_request", "GET https://api.example.com/data")
```

agents/send_request/system.md:


```markdown
You are an HTTP request specialist. Send HTTP requests and return response content.

## Available Sub-Agents
{{AGENTS}}

## Method
Use curl:
curl -s <URL>

Output `<FINAL>response content</FINAL>` when done.
```

# Key Constraints

* Use only Go standard library
* All agents created via the same LoadAgent function
* All agents have identical tool sets: shell + agent
* Whitelist read from config.toml, only checks command names, no regex on arguments
* Interaction protocol hardcoded in agent.go, including tool_calls format and FINAL marker
* Module name: seedclaw
* Must compile with go build
