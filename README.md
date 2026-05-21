# BareClaw

An agent pipeline without tools.

BareClaw has two primitives: `shell` and `agent`.

- **`shell`** — executes a whitelisted shell command.
- **`agent`** — spawns a sub‑agent.

Agents are defined by directories. A directory containing an `agent.md` file is an agent. A directory containing an `agents/` subdirectory can spawn sub‑agents. Sub‑agents provide an `api.md` file to their parent agents. Sub‑agents can also have their own `agents/` subdirectory. The tree can be arbitrarily deep.

## Configuration

Sample:

```toml
[llm]
base_url = "https://api.openai.com/v1"  # API endpoint
model = "gpt-4o"                        # Model name
api_key = "sk-..."                      # LLM API key
max_steps = 10                          # Max steps per agent loop

[agent]
system_dir = "sample/system"    # Root agent path
history_dir = "sample/history"  # Root history path
work_dir = "sample/workspace"   # Root workspace path

[shell]
commands = [ "echo", "cat", "curl" ]    # Allowed shell commands

[shell.path_location.cat]   # Specifies where the path appears in `cat`'s arguments
position = [0]              # The first argument is the path

[shell.path_location.curl]  # Specifies where the path appears in `curl`'s arguments
after = ["-o", "--output"]  # The argument after `-o` or `--output` is the path
prefix = ["--output="]      # The text following `--output=` is the path
```

Pay attention to the `path_location` section. Three methods — `position`, `after`, and `prefix` — can be used to specify where the path argument is.
The system will check whether the path is inside the workspace and reject any changes outside the workspace.


## Quick start

### 1. Build

```bash
go build
```

### 2. Configure

Please refer to the `Configuration` section.
Save the content as `config.toml`, which is the default config file name.

### 3. Create your first agent

Create a `system` directory for the main agent, and place `agent.md` and `rules.md` inside it. `rules.md` can be shared by all sub-agents.
Inside `system/agents`, create subdirectories for sub‑agents as needed; each sub‑agent should contain `api.md` and `agent.md`.

### 4. Run

```bash
./bareclaw
> Express your tasks here
> /quit
```
