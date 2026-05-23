# BareClaw

A tool‑free agent pipeline.

## Design

BareClaw has two primitives: `shell` and `agent`.

- **`shell`** — executes a whitelisted shell command.
- **`agent`** — spawns a sub‑agent.

An agent is defined by a directory. Any directory that contains an `agent.md` file is an agent. Any directory that contains subdirectories can spawn those subdirectories as sub‑agents. Sub‑agents provide an `api.md` file to their parent agents and have their own `agent.md`. Sub‑agents can also have their own subdirectories. The tree can be arbitrarily deep.

## Configuration

See [sample/config.sample.toml](sample/config.sample.toml) for an example.

Pay attention to the `path_location` section. Three fields — `position`, `after`, and `prefix` — specify where the path argument appears.

The system checks whether the path is inside the workspace and rejects any modifications outside it.


## Quick start

### 1. Build

```bash
go build
```

### 2. Configure

Copy `sample/config.sample.toml` to `sample/config.toml`, then fill in `base_url`, `model`, and `api_key` under the `[llm]` section.

### 3. Run

```bash
./bareclaw -c sample/config.toml # or 'bareclaw.exe -c sample\config.toml' on Windows

> Express your tasks here
> /quit
```
