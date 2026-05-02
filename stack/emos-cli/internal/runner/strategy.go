package runner

// RuntimeStrategy defines the interface for mode-specific recipe execution.
//
// ExecRecipe runs the recipe synchronously (used by the CLI `emos run` path).
// StartRecipe starts the recipe in the background and returns a handle the
// caller can Wait on or Cancel (used by the `emos serve` daemon).
type RuntimeStrategy interface {
	PrepareEnvironment() error
	SetRMWImpl(rmw string) error
	ConfigureZenoh(recipeName string, manifest *recipeManifest) error
	LaunchRobotHardware() error
	VerifySensorTopics(sensors []ExtractedTopic, distro string) error
	ExecRecipe(recipeName string, manifest *recipeManifest, logFile string) error
	StartRecipe(recipeName string, manifest *recipeManifest, logFile string) (*RunHandle, error)
	Cleanup() error
}
