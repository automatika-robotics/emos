package runner

// RuntimeStrategy defines the interface for mode-specific recipe execution.
type RuntimeStrategy interface {
	PrepareEnvironment() error
	SetRMWImpl(rmw string) error
	ConfigureZenoh(recipeName string, manifest *recipeManifest) error
	LaunchRobotHardware() error
	VerifySensorTopics(sensors []ExtractedTopic, distro string) error
	ExecRecipe(recipeName string, manifest *recipeManifest, logFile string) error
	Cleanup() error
}
