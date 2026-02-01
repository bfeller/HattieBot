package tools

import (
	"context"
	"encoding/json"

	"github.com/hattiebot/hattiebot/internal/skills"
)

func InstallSkillTool(ctx context.Context, configDir, argsJSON string) (string, error) {
	var args struct {
		Manager string `json:"manager"`
		Package string `json:"package"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return ErrJSON(err), nil
	}

	m := skills.NewManager(configDir)
	output, err := m.InstallPackage(ctx, args.Manager, args.Package)
	if err != nil {
		return ErrJSON(err), nil
	}
	
	// Create a structured success response
	resp := map[string]string{
		"status": "installed",
		"output": output,
	}
	b, _ := json.Marshal(resp)
	return string(b), nil
}

func ListSkillsTool(ctx context.Context, configDir string) (string, error) {
	m := skills.NewManager(configDir)
	installed, err := m.ListInstalled(ctx)
	if err != nil {
		return ErrJSON(err), nil
	}
	
	b, _ := json.Marshal(installed)
	return string(b), nil
}
