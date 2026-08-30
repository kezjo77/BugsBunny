package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type PluginManager struct {
	runtime  wazero.Runtime
	compiled []wazero.CompiledModule
}

// NewPluginManager discovers and pre-compiles all .wasm files
func NewPluginManager(ctx context.Context, pluginsDir string) (*PluginManager, error) {
	// Initialize the Wazero runtime
	r := wazero.NewRuntime(ctx)
	
	// Enable WASI so plugins can use Stdin/Stdout
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	pm := &PluginManager{runtime: r}

	// Scan the plugins directory
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No plugins directory found, running core engine only.")
			return pm, nil
		}
		return nil, err
	}

	// Pre-compile every .wasm file found
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".wasm" {
			wasmBytes, err := os.ReadFile(filepath.Join(pluginsDir, entry.Name()))
			if err != nil {
				return nil, fmt.Errorf("failed to read plugin %s: %w", entry.Name(), err)
			}
			
			// Compiling ahead of time means execution takes microseconds
			compiled, err := r.CompileModule(ctx, wasmBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to compile plugin %s: %w", entry.Name(), err)
			}
			
			pm.compiled = append(pm.compiled, compiled)
			fmt.Printf("🔌 Loaded plugin: %s\n", entry.Name())
		}
	}
	return pm, nil
}

// RunMutatingHook passes the issue JSON through every loaded plugin sequentially
func (pm *PluginManager) RunMutatingHook(ctx context.Context, payload []byte) ([]byte, error) {
	currentPayload := payload

	for _, module := range pm.compiled {
		var stdout bytes.Buffer
		stdin := bytes.NewReader(currentPayload)

		// Map the plugin's Stdin to our JSON, and its Stdout to our buffer
		config := wazero.NewModuleConfig().
			WithStdin(stdin).
			WithStdout(&stdout).
			WithStderr(os.Stderr) // Pipe plugin errors to the server console

		// Execute the plugin's main() function
		_, err := pm.runtime.InstantiateModule(ctx, module, config)
		if err != nil {
			return nil, fmt.Errorf("plugin execution failed: %v", err)
		}

		// The output of this plugin becomes the input for the next one
		currentPayload = stdout.Bytes()
	}

	return currentPayload, nil
}
