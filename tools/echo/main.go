// Echo is a minimal agent-created tool: reads JSON {"message": "..."} from stdin, outputs {"reply": "..."}.
package main

import (
	"encoding/json"
	"os"
)

func main() {
	var input struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"reply": input.Message})
}
