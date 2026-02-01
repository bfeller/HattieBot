package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Input struct {
	Name string `json:"name"`
}

func main() {
	var input Input
	decoder := json.NewDecoder(os.Stdin)
	err := decoder.Decode(&input)
	if err != nil {
		fmt.Println(json.Marshal(map[string]interface{}{"error": err.Error()}))
		os.Exit(1)
	}

	output := map[string]string{"message": fmt.Sprintf("Hello %s", input.Name)}
	outputJSON, err := json.Marshal(output)
	if err != nil {
		fmt.Println(json.Marshal(map[string]interface{}{"error": err.Error()}))
		os.Exit(1)
	}
	fmt.Println(string(outputJSON))
}
