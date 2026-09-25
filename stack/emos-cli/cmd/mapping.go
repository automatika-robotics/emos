package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/installer"
	"github.com/automatika-robotics/emos-cli/internal/mapping"
	"github.com/automatika-robotics/emos-cli/internal/runner"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var (
	mapExportDir    string
	mapRMW          string
	mapSetupRebuild bool
)

var mapCmd = &cobra.Command{
	Use:   "map",
	Short: "Build and manage maps of the robot's environment",
	Long: "Build and manage maps.\n\n" +
		"How a robot maps is declared in its plugin. EMOS drives the mapping\n" +
		"software that ships with the robot, or builds the map itself from the\n" +
		"robot's LiDAR when the robot ships none.",
}

func init() {
	newCmd := &cobra.Command{
		Use:   "new [name]",
		Short: "Build a new map by driving the robot",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runMapNew,
	}
	newCmd.Flags().StringVar(&mapRMW, "rmw", "",
		"RMW implementation for a map EMOS builds itself ("+runner.RMWChoices+"); "+
			"unset keeps the environment's, or ROS's default")
	mapCmd.AddCommand(newCmd)
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Install the mapping backend EMOS builds maps with",
		Long: "Install the mapping backend (GLIM) for a robot whose maps EMOS builds itself.\n\n" +
			"It is compiled from source into the EMOS workspace, which takes a while.\n" +
			"A robot that maps with its own software needs none of this.",
		Args: cobra.NoArgs,
		RunE: runMapSetup,
	}
	setupCmd.Flags().BoolVar(&mapSetupRebuild, "rebuild", false,
		"build again although the backend is installed, for one an update has broken")
	mapCmd.AddCommand(setupCmd)
	exportCmd := &cobra.Command{
		Use:   "export [name]",
		Short: "Package a map for copying off the robot (defaults to the active one)",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runMapExport,
	}
	exportCmd.Flags().StringVarP(&mapExportDir, "output", "o", "",
		"directory to put the archive in (default ~/emos/map-archives)")
	mapCmd.AddCommand(exportCmd)
	mapCmd.AddCommand(&cobra.Command{
		Use:   "import <archive>",
		Short: "Unpack an exported map into the store (bare names are looked up in ~/emos/map-archives)",
		Args:  cobra.ExactArgs(1),
		RunE:  runMapImport,
	})
	mapCmd.AddCommand(&cobra.Command{
		Use:   "use <name>",
		Short: "Make a map the one the robot localizes against",
		Args:  cobra.ExactArgs(1),
		RunE:  runMapUse,
	})
	mapCmd.AddCommand(&cobra.Command{
		Use:   "rm <name>",
		Short: "Delete a map",
		Args:  cobra.ExactArgs(1),
		RunE:  runMapRm,
	})
	mapCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the maps on this robot",
		RunE:  runMapList,
	})
}

// resolveMapping loads the active robot's mapping declaration.
func resolveMapping() (*mapping.Declaration, error) {
	decl, err := mapping.Resolve(config.LoadConfig())
	switch {
	case errors.Is(err, mapping.ErrNoPlugin):
		ui.Error("No robot plugin is installed, so there is no robot to map with.")
		ui.Faint("Install one with 'emos plugin install <plugin>'.")
		return nil, fmt.Errorf("no robot plugin installed")
	case errors.Is(err, mapping.ErrNotSupported):
		ui.Error("This robot's plugin declares no mapping support.")
		return nil, fmt.Errorf("mapping not supported by this robot")
	case err != nil:
		return nil, err
	}
	return decl, nil
}

// explain prints the operator-facing account of a mapping error and returns the
// short error cobra reports. Other errors pass through unchanged.
func explain(err error) error {
	var (
		missing    *mapping.ErrNoSuchMap
		unreadable *mapping.ErrStoreUnreadable
		noArchive  *mapping.ErrNoSuchArchive
		active     *mapping.ErrMapIsActive
		noGrid     *mapping.ErrNoGrid
		exists     *mapping.ErrMapExists
		notArchive *mapping.ErrNotAMapArchive
	)
	switch {
	case errors.As(err, &noGrid):
		ui.Error(fmt.Sprintf("'%s' has no occupancy grid, so a recipe could not load it.", noGrid.Name))
		ui.Faint(fmt.Sprintf("A mapping session that did not finish leaves such a map behind; "+
			"'emos map rm %s' removes it.", noGrid.Name))
		return fmt.Errorf("map has no occupancy grid")
	case errors.As(err, &exists):
		ui.Error(fmt.Sprintf("A map named '%s' is already in the store.", exists.Name))
		ui.Faint(fmt.Sprintf("Remove it first with 'emos map rm %s' to replace it.", exists.Name))
		return fmt.Errorf("map already exists")
	case errors.As(err, &notArchive):
		ui.Error(fmt.Sprintf("%s is not a map archive exported by EMOS: %s.", notArchive.Path, notArchive.Reason))
		return fmt.Errorf("not a map archive")
	case errors.As(err, &missing):
		ui.Error(fmt.Sprintf("No map named '%s'.", missing.Name))
		ui.Faint("Run 'emos map list' to see what this robot has.")
		return fmt.Errorf("map not found")
	case errors.As(err, &unreadable):
		ui.Error(fmt.Sprintf("Cannot read the map store at %s.", unreadable.Store))
		ui.Faint("The robot's own software owns that directory and EMOS runs as a " +
			"normal user. A recipe would not be able to load these maps either.")
		return fmt.Errorf("map store not readable")
	case errors.As(err, &noArchive):
		ui.Error(fmt.Sprintf("No archive at %s.", noArchive.Path))
		ui.Faint(fmt.Sprintf("Give a path, or the name of a file in %s.", config.MapArchivesDir))
		return fmt.Errorf("archive not found")
	case errors.As(err, &active):
		ui.Error(fmt.Sprintf("'%s' is the map the robot is currently using.", active.Name))
		ui.Faint("Switch to another map first with 'emos map use <name>'; deleting " +
			"the active one would leave localization with nothing to localize against.")
		return fmt.Errorf("refusing to delete the active map")
	}
	return err
}

func runMapList(cmd *cobra.Command, args []string) error {
	decl, err := resolveMapping()
	if err != nil {
		return err
	}
	maps, err := decl.List()
	if err != nil {
		return explain(err)
	}

	if len(maps) == 0 {
		ui.Info(fmt.Sprintf("No maps yet in %s.", decl.Store()))
		return nil
	}

	ui.Header("MAPS")
	for _, m := range maps {
		line := m.Name
		if m.Active {
			line += "  ● active"
		}
		if m.Grid == "" {
			line += "  (no occupancy grid)"
		}
		ui.Info(line)
	}
	ui.Faint(fmt.Sprintf("%d map(s) in %s", len(maps), decl.Store()))
	return nil
}

func runMapNew(cmd *cobra.Command, args []string) error {
	decl, err := resolveMapping()
	if err != nil {
		return err
	}
	if err := decl.CanStart(); err != nil {
		return err
	}
	if !term.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("'emos map new' needs an interactive terminal: mapping is stopped from it")
	}
	cfg := config.LoadConfig()
	if decl.Kind == mapping.KindNative {
		if err := nativeMappingReady(cfg); err != nil {
			return err
		}
	}
	name := "map"
	if len(args) == 1 {
		name = args[0]
	} else if name, err = ui.Prompt("Name for this map", "map"); err != nil {
		ui.Info("Cancelled.")
		return nil
	}
	if err := mapping.CheckName(name); err != nil {
		return err
	}
	if decl.Kind == mapping.KindNative {
		return runNativeMapNew(cfg, decl, name)
	}
	return runVendorMapNew(decl, name)
}

// loopAdvice is what makes a map line up with itself, whoever builds it.
const loopAdvice = "Plan a route that closes loops -- make sure to revisit places you " +
	"have already scanned, or the map will not line up with itself."

func runVendorMapNew(decl *mapping.Declaration, name string) error {
	ui.Header("MAPPING")
	if limit := decl.Vendor.AreaLimitM; limit > 0 {
		ui.Info(fmt.Sprintf("This robot maps areas up to %.0f x %.0f m.", limit, limit))
	}
	if mapping.Escalates(decl.Vendor.Start) || mapping.Escalates(decl.Vendor.Stop) {
		ui.Info("Mapping runs the robot's own tool as root; sudo may prompt.")
	}
	ui.Faint(loopAdvice)

	// Caught from before the start command, so no signal can end the CLI while
	// the robot keeps mapping.
	sigs, release := catchSignals()
	defer release()

	session, err := decl.StartVendor(name, mapping.SystemRunner)
	if err != nil {
		return err
	}
	if interrupted := driveUntilStopped(sigs, nil); interrupted {
		ui.Warn("Interrupted -- stopping mapping so the robot does not keep going.")
	}

	ui.Info("Stopping mapping and saving...")
	stopCtx, cancelStop := cancelOnSignal(sigs)
	defer cancelStop()
	built, err := session.Stop(stopCtx)
	if errors.Is(err, mapping.ErrStopInterrupted) {
		ui.Error("Stopping was interrupted, so the robot may still be mapping.")
		ui.Faint("Stop it by hand: " + session.StopCommand())
		return err
	}
	if err != nil {
		ui.Error(err.Error())
		ui.Faint("The robot may still be finishing. Check with 'emos map list'.")
		return fmt.Errorf("mapping did not complete")
	}
	reportSaved(built)
	return nil
}

// catchSignals takes over the signals that would end the CLI, so it decides
// what they mean while mapping goes on. release hands them back.
func catchSignals() (sigs chan os.Signal, release func()) {
	sigs = make(chan os.Signal, 2)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	return sigs, func() { signal.Stop(sigs) }
}

// driveUntilStopped tells the operator to drive, and waits for Enter. The wait
// also ends when ended closes, which is nil for mapping that cannot end by
// itself. It reports whether the operator interrupted instead.
func driveUntilStopped(sigs <-chan os.Signal, ended <-chan struct{}) (interrupted bool) {
	ui.Success("Mapping started.")
	ui.Info("Drive the robot along your route with its own controller.")
	ctx, cancel := cancelOnSignal(sigs)
	defer cancel()
	go func() {
		select {
		case <-ended:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ui.WaitForEnter(ctx, "Press Enter when you have finished driving.") != nil
}

// reportSaved tells the operator about the map a session left.
func reportSaved(built *mapping.Map) {
	ui.Success(fmt.Sprintf("Map '%s' saved.", built.Name))
	if built.Grid == "" {
		ui.Warn("It has no occupancy grid yet, so a recipe cannot load it.")
		ui.Faint("The robot may still be post-processing; re-check with 'emos map list'.")
		return
	}
	ui.Faint(built.Grid)
}

// hintUse says how to make a map the active one, unless it already is.
func hintUse(m *mapping.Map) {
	if !m.Active {
		ui.Faint(fmt.Sprintf("Make it active with 'emos map use %s'.", m.Name))
	}
}

// warnWithoutGrid warns about a map a recipe cannot load.
func warnWithoutGrid(m *mapping.Map) {
	if m.Grid == "" {
		ui.Warn("It has no occupancy grid, so a recipe cannot load it.")
	}
}

// cancelOnSignal returns a context cancelled by the next signal on sigs.
func cancelOnSignal(sigs <-chan os.Signal) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-sigs:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// refuseInContainer explains that a container install cannot build maps itself.
func refuseInContainer(cfg *config.EMOSConfig) error {
	err := mapping.NativeSupported(cfg.Mode)
	if err != nil {
		ui.Error("EMOS builds this robot's maps itself, which a container install cannot do in this version.")
		ui.Faint("Map from a pixi or native EMOS install on the robot.")
	}
	return err
}

func runMapSetup(cmd *cobra.Command, args []string) error {
	decl, err := resolveMapping()
	if err != nil {
		return err
	}
	if decl.Kind != mapping.KindNative {
		ui.Info("This robot maps with its own software, so there is nothing to set up.")
		return nil
	}
	cfg := config.LoadConfig()
	if err := refuseInContainer(cfg); err != nil {
		return err
	}
	if err := refuseWhilePluginsBusy("the build"); err != nil {
		return err
	}
	inEnvironment := func(shell string, out io.Writer) (mapping.Process, error) {
		return runner.StartTool(cfg, shell, out)
	}
	installed, err := mapping.BackendInstalled(inEnvironment)
	if err != nil {
		return err
	}
	if installed && !mapSetupRebuild {
		ui.Success("The mapping backend (GLIM) is already installed.")
		ui.Faint("Build a map with 'emos map new'. If an update has broken it, 'emos map setup --rebuild' builds it again.")
		return nil
	}

	if cfg.Mode != config.ModePixi {
		ui.Error("Installing the mapping backend is automated for a pixi install only.")
		ui.Faint("On this install, build GTSAM, gtsam_points, GLIM and glim_ros2 into your ROS workspace " +
			"yourself. The versions and build options EMOS uses are in " +
			"stack/emos-cli/scripts/install_mapping_backend_pixi.sh of the EMOS repository.")
		return fmt.Errorf("mapping backend not installed")
	}

	ui.Header("MAPPING BACKEND")
	ui.Info("GTSAM, gtsam_points, GLIM and glim_ros2 are compiled from source into the EMOS workspace.")
	ui.Info("ROS packages they need are added to the pixi environment: " +
		strings.Join(installer.MappingBackendROSPackages, ", ") + ".")
	env := pixiBuildEnv()
	if cuda := installer.DetectCUDA(); cuda != nil {
		ui.Info(fmt.Sprintf("CUDA %s was detected at %s, so the CUDA-optimized version of GLIM can be used.",
			cuda.Version, cuda.Root))
		if ui.Confirm(fmt.Sprintf("Build GLIM for CUDA %s?", cuda.Version)) {
			env = append(env, "EMOS_MAPPING_CUDA="+cuda.Root)
		} else {
			ui.Info("Building for the CPU alone.")
		}
	}
	ui.Faint("This may take several minutes.")
	if !ui.Confirm("Build the mapping backend now?") {
		ui.Info("Left alone.")
		return nil
	}
	const retry = "Run 'emos map setup' again: it continues from what is already fetched and built."
	if err := installer.InstallMappingBackend(cfg.PixiProjectDir, cfg.ROSDistro, env); err != nil {
		ui.Error("The mapping backend did not build.")
		ui.Faint(retry)
		return err
	}
	if installed, err = mapping.BackendInstalled(inEnvironment); err != nil {
		return err
	} else if !installed {
		ui.Error("The build finished, but GLIM is still not found in the EMOS environment.")
		ui.Faint(retry)
		return fmt.Errorf("mapping backend not installed")
	}
	ui.Success("The mapping backend (GLIM) is installed.")
	ui.Faint("Build a map with 'emos map new'.")
	return nil
}

// nativeMappingReady says why EMOS cannot build a map itself right now, before
// the operator is asked for anything.
func nativeMappingReady(cfg *config.EMOSConfig) error {
	if err := refuseInContainer(cfg); err != nil {
		return err
	}
	if err := runner.CheckRMW(mapRMW); err != nil {
		return err
	}
	return refuseWhilePluginsBusy("mapping")
}

// runNativeMapNew builds a map with EMOS's own mapping session.
func runNativeMapNew(cfg *config.EMOSConfig, decl *mapping.Declaration, name string) error {
	logFile := runner.LogFilePath("map-" + name)
	log, err := runner.OpenLog(logFile)
	if err != nil {
		return err
	}
	defer log.Close()

	ui.Header("MAPPING")
	ui.Info("EMOS builds this map itself from the robot's LiDAR.")
	ui.Info("RMW Implementation: " + runner.RMWLabel(mapRMW))

	// Caught from here on, so the session is never left running behind a CLI
	// that a signal ended. The session is in its own process group.
	sigs, release := catchSignals()
	defer release()

	run, err := runner.Prepare(cfg, mapRMW, nil,
		runner.StopOnSignal(sigs, "interrupted before mapping started"))
	if err != nil {
		return err
	}
	defer run.Close()
	start := func(shell string, out io.Writer) (mapping.Process, error) {
		return run.Start("starting mapping", shell, out)
	}

	if installed, err := mapping.BackendInstalled(start); err != nil {
		return err
	} else if !installed {
		ui.Error("The mapping backend (GLIM) is not installed in this EMOS environment.")
		ui.Faint("Install it with 'emos map setup'.")
		return fmt.Errorf("mapping backend not installed")
	}

	ui.Header("BUILDING MAP: " + name)
	ui.Faint(loopAdvice)
	ui.Faint("Finish in an area you have already covered.")
	ui.Info("The session's output is saved to: " + logFile)

	startCtx, cancelStart := cancelOnSignal(sigs)
	session, err := decl.StartNative(startCtx, cfg.Plugin.EntryPoint, name, start, log, ui.Warn)
	cancelStart()
	if err != nil {
		return explainSession(err, logFile)
	}

	interrupted := driveUntilStopped(sigs, session.Done())

	var built *mapping.Map
	select {
	case <-session.Done():
		ui.Warn("The mapping session ended on its own.")
		built, err = session.Result()
	default:
		if interrupted {
			ui.Warn("Interrupted -- stopping mapping and saving what it has.")
		}
		ui.Faint("Ctrl+C gives up on the map.")
		stopCtx, cancelStop := cancelOnSignal(sigs)
		err = ui.Spinner("Stopping mapping and saving the map", func() error {
			var e error
			built, e = session.Stop(stopCtx)
			return e
		})
		cancelStop()
	}
	if err != nil {
		return explainSession(err, logFile)
	}
	reportSaved(built)
	hintUse(built)
	return nil
}

// explainSession prints the operator-facing account of a native mapping
// session that produced no map, and returns the short error cobra reports.
func explainSession(err error, logFile string) error {
	var exited *mapping.ErrSessionExited
	switch {
	case errors.Is(err, mapping.ErrStopInterrupted):
		ui.Error("Stopping was interrupted, so the session was ended and no map was saved.")
		return err
	case errors.Is(err, context.Canceled):
		ui.Error("Interrupted before mapping started.")
		return err
	case errors.Is(err, mapping.ErrNoMapBuilt):
		ui.Error("The mapping backend published no map.")
		ui.Faint("The first one comes some seconds after the robot starts moving. " +
			"Drive for longer, and check that the LiDAR is running.")
	case errors.Is(err, mapping.ErrStopTimedOut):
		ui.Error("The mapping session did not stop in time, so it was killed.")
	case errors.As(err, &exited):
		ui.Error("Mapping failed: " + err.Error() + ".")
	default:
		return err
	}
	for _, line := range lastLines(logFile, 15) {
		ui.Faint("  " + line)
	}
	ui.Faint("Full output: " + logFile)
	ui.Faint("A map directory without a map may be left behind; 'emos map list' shows it and 'emos map rm' removes it.")
	return fmt.Errorf("mapping did not complete")
}

// lastLines returns up to n of the last non-empty lines of the file at path.
func lastLines(path string, n int) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

func runMapRm(cmd *cobra.Command, args []string) error {
	name := args[0]
	decl, err := resolveMapping()
	if err != nil {
		return err
	}

	// Resolve first, so the prompt names the directory that will actually be
	// removed.
	target, err := decl.Find(name)
	if err != nil {
		return explain(err)
	}
	if target.Active {
		return explain(&mapping.ErrMapIsActive{Name: name})
	}

	if !ui.Confirm(fmt.Sprintf("Delete %s? This cannot be undone", target.Path)) {
		ui.Info("Left alone.")
		return nil
	}
	if v := decl.Vendor; v != nil && (mapping.Escalates(v.Remove) || (len(v.Remove) == 0 && v.RequiresRoot)) {
		ui.Info("The map store is owned by the robot's own software; sudo may prompt.")
	}

	if err := decl.Remove(name, mapping.SystemRunner); err != nil {
		return explain(err)
	}
	ui.Success(fmt.Sprintf("Deleted %s.", target.Path))
	return nil
}

func runMapExport(cmd *cobra.Command, args []string) error {
	decl, err := resolveMapping()
	if err != nil {
		return err
	}
	name := ""
	if len(args) == 1 {
		name = args[0]
	} else if name, err = decl.ActiveName(); err != nil {
		return explain(err)
	} else if name == "" {
		ui.Error("No map is marked active, so there is nothing to export.")
		ui.Faint("Name one explicitly: 'emos map export <name>'.")
		return fmt.Errorf("no active map")
	}

	if v := decl.Vendor; v != nil && mapping.Escalates(v.Export) {
		ui.Info("Exporting runs the robot's own tool as root; sudo may prompt.")
	}
	dest := mapExportDir
	if dest == "" {
		dest = config.MapArchivesDir
	}
	archive, err := decl.Export(name, dest, mapping.SystemRunner)
	if err != nil && archive != "" {
		ui.Error(err.Error())
		ui.Faint(fmt.Sprintf("The archive is still at %s.", archive))
		return fmt.Errorf("could not move the archive")
	}
	if err != nil {
		return explain(err)
	}
	if archive == "" {
		ui.Success(fmt.Sprintf("Exported '%s'.", name))
		ui.Faint("The robot's tool chose where to put the archive; check its output above.")
		return nil
	}
	ui.Success(fmt.Sprintf("Exported '%s' to %s", name, archive))
	if decl.Kind == mapping.KindNative {
		ui.Faint("It holds the map's files; the mapping backend's working data stays on the robot.")
	}
	return nil
}

func runMapImport(cmd *cobra.Command, args []string) error {
	decl, err := resolveMapping()
	if err != nil {
		return err
	}
	if v := decl.Vendor; v != nil && mapping.Escalates(v.Import) {
		ui.Info("Importing writes into the robot's own map store; sudo may prompt.")
	}
	built, err := decl.Import(args[0], config.MapArchivesDir, mapping.SystemRunner)
	if err != nil {
		return explain(err)
	}
	ui.Success(fmt.Sprintf("Imported '%s'.", built.Name))
	warnWithoutGrid(built)
	hintUse(built)
	return nil
}

func runMapUse(cmd *cobra.Command, args []string) error {
	name := args[0]
	decl, err := resolveMapping()
	if err != nil {
		return err
	}
	target, err := decl.Find(name)
	if err != nil {
		return explain(err)
	}
	if target.Active {
		ui.Info(fmt.Sprintf("'%s' is already the active map.", name))
		return nil
	}
	if decl.Kind == mapping.KindNative {
		return useNativeMap(decl, target)
	}
	return useVendorMap(decl, target)
}

// useNativeMap repoints the store's active link. Nothing that is running is
// affected, so it does not ask first. A map without a grid is refused.
func useNativeMap(decl *mapping.Declaration, target *mapping.Map) error {
	if err := decl.Use(target.Name, mapping.SystemRunner); err != nil {
		return explain(err)
	}
	ui.Success(fmt.Sprintf("'%s' is now the active map.", target.Name))
	ui.Faint("Recipes load it the next time they start.")
	return nil
}

// useVendorMap has the robot's own software switch maps, which changes what the
// robot localizes against at once. A map without a grid is
// only warned about as the vendor's tool may still be processing it.
func useVendorMap(decl *mapping.Declaration, target *mapping.Map) error {
	warnWithoutGrid(target)
	ui.Warn("The robot will localize against this map from now on. It needs " +
		"relocalizing, and a running recipe will lose its position.")
	if !ui.Confirm(fmt.Sprintf("Make '%s' the active map?", target.Name)) {
		ui.Info("Left alone.")
		return nil
	}
	if mapping.Escalates(decl.Vendor.Apply) {
		ui.Info("Switching runs the robot's own tool as root; sudo may prompt.")
	}
	if err := decl.Use(target.Name, mapping.SystemRunner); err != nil {
		return explain(err)
	}
	ui.Success(fmt.Sprintf("'%s' is now the active map.", target.Name))
	ui.Faint("Relocalize the robot at its standard starting spot before running a recipe.")
	return nil
}
