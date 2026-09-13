package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/mapping"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var mapExportDir string

var mapCmd = &cobra.Command{
	Use:   "map",
	Short: "Build and manage maps of the robot's environment",
	Long: "Build and manage maps.\n\n" +
		"How a robot maps is its plugin's to declare. EMOS drives the mapping\n" +
		"software that ships with the robot.",
}

func init() {
	mapCmd.AddCommand(&cobra.Command{
		Use:   "new [name]",
		Short: "Build a new map by driving the robot",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runMapNew,
	})
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
		ui.Faint("If the plugin was installed before mapping support was added, " +
			"'emos plugin update' refreshes what EMOS knows about it.")
		return nil, fmt.Errorf("mapping not supported by this robot")
	case errors.Is(err, mapping.ErrNativeNotSupported):
		ui.Error("This robot's plugin expects EMOS to build the map itself, " +
			"which this version of EMOS does not support.")
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
	)
	switch {
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
	name := "map"
	if len(args) == 1 {
		name = args[0]
	} else if name, err = ui.Prompt("Name for this map", "map"); err != nil {
		ui.Info("Cancelled.")
		return nil
	}

	ui.Header("MAPPING")
	if limit := decl.Vendor.AreaLimitM; limit > 0 {
		ui.Info(fmt.Sprintf("This robot maps areas up to %.0f x %.0f m.", limit, limit))
	}
	if mapping.Escalates(decl.Vendor.Start) || mapping.Escalates(decl.Vendor.Stop) {
		ui.Info("Mapping runs the robot's own tool as root; sudo may prompt.")
	}
	ui.Faint("Plan a route that closes loops -- make sure to revisit places you " +
		"have already scanned, or the map will not line up with itself.")

	// Caught from before the start command, so no signal can end the CLI while
	// the robot keeps mapping. The prompt below reads Ctrl+C as a key.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigs)

	session, err := decl.Start(name, mapping.SystemRunner)
	if err != nil {
		return err
	}

	ui.Success("Mapping started.")
	ui.Info("Drive the robot along your route with its own controller.")
	promptCtx, cancelPrompt := cancelOnSignal(sigs)
	err = ui.Continue(promptCtx, "Press Enter when you have finished driving", "Stop mapping")
	cancelPrompt()
	if err != nil {
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

	ui.Success(fmt.Sprintf("Map '%s' saved.", built.Name))
	if built.Grid == "" {
		ui.Warn("It has no occupancy grid yet, so a recipe cannot load it.")
		ui.Faint("The robot may still be post-processing; re-check with 'emos map list'.")
	} else {
		ui.Faint(built.Grid)
	}
	return nil
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
	if v := decl.Vendor; mapping.Escalates(v.Remove) || (len(v.Remove) == 0 && v.RequiresRoot) {
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

	if mapping.Escalates(decl.Vendor.Export) {
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
	return nil
}

func runMapImport(cmd *cobra.Command, args []string) error {
	decl, err := resolveMapping()
	if err != nil {
		return err
	}
	if mapping.Escalates(decl.Vendor.Import) {
		ui.Info("Importing writes into the robot's own map store; sudo may prompt.")
	}
	built, err := decl.Import(args[0], config.MapArchivesDir, mapping.SystemRunner)
	if err != nil {
		return explain(err)
	}
	ui.Success(fmt.Sprintf("Imported '%s'.", built.Name))
	if built.Grid == "" {
		ui.Warn("It has no occupancy grid, so a recipe cannot load it.")
	}
	if !built.Active {
		ui.Faint(fmt.Sprintf("Make it active with 'emos map use %s'.", built.Name))
	}
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
	if target.Grid == "" {
		ui.Warn("It has no occupancy grid, so a recipe cannot load it.")
	}

	ui.Warn("The robot will localize against this map from now on. It needs " +
		"relocalizing, and a running recipe will lose its position.")
	if !ui.Confirm(fmt.Sprintf("Make '%s' the active map?", name)) {
		ui.Info("Left alone.")
		return nil
	}
	if mapping.Escalates(decl.Vendor.Apply) {
		ui.Info("Switching runs the robot's own tool as root; sudo may prompt.")
	}
	if err := decl.Use(name, mapping.SystemRunner); err != nil {
		return explain(err)
	}
	ui.Success(fmt.Sprintf("'%s' is now the active map.", name))
	ui.Faint("Relocalize the robot at its standard starting spot before running a recipe.")
	return nil
}
