package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	// 1. Read the raw JSON from Bugsbunny via Standard Input
	inputJSON, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Plugin Error: Failed to read stdin: %v\n", err)
		os.Exit(1)
	}

	// 2. Unmarshal into a generic map
	var issue map[string]interface{}
	if err := json.Unmarshal(inputJSON, &issue); err != nil {
		fmt.Fprintf(os.Stderr, "Plugin Error: Invalid JSON: %v\n", err)
		os.Exit(1)
	}

	// 3. The Plugin Logic: Check priority and inject SLA
	customData, ok := issue["custom_data"].(map[string]interface{})
	if ok {
		priority, hasPriority := customData["priority"].(string)
		if hasPriority && priority == "critical" {
			// Inject a new field dynamically!
			customData["sla_deadline"] = "2 Hours"
			customData["plugin_notice"] = "Enforced by SLA-Checker WASM"
		}
	}

	// 4. Marshal the modified issue back to JSON
	outputJSON, err := json.Marshal(issue)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Plugin Error: Failed to marshal output: %v\n", err)
		os.Exit(1)
	}

	// 5. Send it back to Bugsbunny via Standard Output
	os.Stdout.Write(outputJSON)
}
