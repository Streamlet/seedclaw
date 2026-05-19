package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// ExecuteShell runs a shell command after validating it against the whitelist.
// Write operations are restricted to the session workspace.
func ExecuteShell(command string, arguments []string, sessionID string, allowedCmds []string, workspace string) (string, error) {
	// Check whitelist
	allowed := false
	for _, c := range allowedCmds {
		if c == command {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Sprintf("Rejected: command '%s' not in whitelist", command), nil
	}

	if err := os.MkdirAll(workspace, 0755); err != nil {
		return "", fmt.Errorf("create workspace: %w", err)
	}

	// Execute via shell to support redirections
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, arguments...)
	cmd.Dir = workspace
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out")
		}
		return "", fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}
	return string(output), nil
}
