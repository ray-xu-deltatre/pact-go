// Package v3 contains the main Pact DSL used in the Consumer
// collaboration test cases, and Provider contract test verification.
package v3

// TODO: setup a proper state machine to prevent actions
// Current issues
// 1. Setup needs to be initialised to get a port -> should be resolved by creating the server at the point of verification
// 2. Ensure that interactions are properly cleared
// 3. Need to ensure only v2 or v3 matchers are added

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pact-foundation/pact-go/utils"
	"github.com/pact-foundation/pact-go/v3/internal/installer"
	"github.com/pact-foundation/pact-go/v3/internal/native"
)

func init() {
	initLogging()
	native.Init()
	i := installer.NewInstaller()
	i.CheckInstallation()
}

type PluginProviderConfig struct {
	// Plugin name
	// TODO: for later
	// Name string

	// Command to start the plugin
	// TODO: for later. For now, just have the plugin running before the test

	// Consumer is the name of the Consumer/Client.
	Consumer string

	// Provider is the name of the Providing service.
	Provider string

	// Location of Pact external service invocation output logging.
	// Defaults to `<cwd>/logs`.
	LogDir string

	// Pact files will be saved in this folder.
	// Defaults to `<cwd>/pacts`.
	PactDir string

	// Host is the address of the Mock and Verification Service runs on
	// Examples include 'localhost', '127.0.0.1', '[::1]'
	// Defaults to 'localhost'
	Host string

	// Port for the mock provider to run on
	Port int
}

// NewPluginProvider creates an instance of an external plugin based consumer
func NewPluginProvider(config PluginProviderConfig) (*PluginProvider, error) {
	provider := &PluginProvider{
		config: config,
	}
	err := provider.validateConfig()

	if err != nil {
		return nil, err
	}

	return provider, err
}

// PluginProvider is the entrypoint for plugin based consumer tests
type PluginProvider struct {
	config       PluginProviderConfig
	Interactions []interface{} `json:"interactions"`
}

// validateConfig validates the configuration for the consumer test
func (p *PluginProvider) validateConfig() error {
	log.Println("[DEBUG] pact setup")
	dir, _ := os.Getwd()

	if p.config.Host == "" {
		p.config.Host = "127.0.0.1"
	}

	if p.config.LogDir == "" {
		p.config.LogDir = fmt.Sprintf(filepath.Join(dir, "logs"))
	}

	if p.config.PactDir == "" {
		p.config.PactDir = fmt.Sprintf(filepath.Join(dir, "pacts"))
	}

	var pErr error
	if p.config.Port <= 0 {
		p.config.Port, pErr = utils.GetFreePort()
	}

	if pErr != nil {
		return fmt.Errorf("error: unable to find free port, mock server will fail to start")
	}

	return nil
}

func (p *PluginProvider) cleanInteractions() {
	p.Interactions = make([]interface{}, 0)
}

// ExecuteTest runs the current test case against a Mock Service.
// Will cleanup interactions between tests within a suite
// and write the pact file if successful
func (p *PluginProvider) ExecuteTest(integrationTest func(MockServerConfig) error) error {
	log.Println("[DEBUG] pact verify")

	log.Println("[DEBUG] starting plugin provider")
	port := p.config.Port // admin port
	clientPort, err := native.CreatePluginMockServer(port, "test")

	if err != nil {
		return err
	}

	// Cleanup processes at the end of the test session
	defer native.CleanupPluginMockServer(port)

	// Wait for plugin server to start on port
	err = waitForPort(port, "tcp", "localhost", 10*time.Second, fmt.Sprintf(`Timed out waiting for plugin to start on port %d:`, port))
	if err != nil {
		return err
	}

	log.Println("[DEBUG] started plugin provider on port", port)

	// TODO: Generate interactions for Pact file
	fmt.Println("[INFO] sending interactions to plugin", p.Interactions)

	// Send the interactions - note for this purpose, we assume the plugin already knows the interactions
	err = native.AddPluginInteractions(port, p.Interactions)
	if err != nil {
		return err
	}

	// Clean out interactions before next run
	defer p.cleanInteractions()

	// Run the integration test
	err = integrationTest(MockServerConfig{
		Port:      clientPort,
		Host:      p.config.Host,
		TLSConfig: GetTLSConfigForTLSMockServer(),
	})

	if err != nil {
		return err
	}

	// Run Verification Process
	fmt.Println("[INFO] verifying interactions with Plugin provider")
	res, mismatches := native.VerifyPlugin(port)

	log.Println("[INFO] mismatches:", mismatches, "res", res)

	if !res {
		return fmt.Errorf("pact validation failed: %+v %+v", res, mismatches)
	}

	return p.WritePact()
}

// WritePact should be called when all tests have been performed for a
// given Consumer <-> Provider pair. It will write out the Pact to the
// configured file. This is safe to call multiple times as the service is smart
// enough to merge pacts and avoid duplicates.
func (p *PluginProvider) WritePact() error {
	log.Println("[DEBUG] write pact file")
	return nil
}

// AddInteraction creates a new Pact interaction, initialising all
// required things. Will automatically start a Mock Service if none running.
func (p *PluginProvider) AddInteraction(i interface{}) {
	log.Println("[DEBUG] plugin add interaction", i)
	p.Interactions = append(p.Interactions, i)
	log.Println("[DEBUG] plugin current interaction", p.Interactions)
}
