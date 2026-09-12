package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/mapping"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var mapCmd = &cobra.Command{
	Use:   "map",
	Short: "Build and manage maps of the robot's environment",
	Long: "Build and manage maps.\n\n" +
		"How a robot maps is its plugin's to declare: some ship their own SLAM and\n" +
		"EMOS drives it, others expose a LiDAR and EMOS builds the map itself.",
}

func init() {
	mapCmd.AddCommand(&cobra.Command{
		Use:   "new [name]",
		Short: "Build a new map by driving the robot",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runMapNew,
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
	case err != nil:
		return nil, err
	}
	return decl, nil
}

func runMapList(cmd *cobra.Command, args []string) error {
	decl, err := resolveMapping()
	if err != nil {
		return err
	}

	maps, err := decl.List()
	var unreadable *mapping.ErrStoreUnreadable
	if errors.As(err, &unreadable) {
		ui.Error(fmt.Sprintf("Cannot read the map store at %s.", unreadable.Store))
		ui.Faint("The robot's own software owns that directory and EMOS runs as a " +
			"normal user. A recipe would not be able to load these maps either.")
		return fmt.Errorf("map store not readable")
	}
	if err != nil {
		return err
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
	name := ""
	if len(args) == 1 {
		name = args[0]
	} else {
		name = ui.Input("Name for this map", "map")
	}

	ui.Header("MAPPING")
	if decl.Vendor != nil {
		if limit := decl.Vendor.AreaLimitM; limit > 0 {
			ui.Info(fmt.Sprintf("This robot maps areas up to %.0f x %.0f m.", limit, limit))
		}
		if decl.Vendor.RequiresRoot {
			ui.Info("Mapping needs root on this robot; sudo will prompt.")
		}
	}
	ui.Faint("Plan a route that closes loops -- make sure to revisit places you" + " have already scanned, or the map will not line up with itself.")

	session, err := decl.Start(name, mapping.SystemRunner)
	if err != nil {
		return err
	}

	// From here the robot is mapping. Any exit must stop it, including Ctrl+C
	stopped := false
	stop := func() (*mapping.Map, error) {
		if stopped {
			return nil, nil
		}
		stopped = true
		ui.Info("Stopping mapping and saving...")
		return session.Stop()
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	aborted := make(chan struct{})
	go func() {
		<-sigs
		close(aborted)
	}()

	ui.Success("Mapping started.")
	ui.Info("Drive the robot along your route with its own controller.")
	done := make(chan struct{})
	go func() {
		fmt.Print("\n  Press Enter when you have finished driving... ")
		fmt.Scanln()
		close(done)
	}()

	select {
	case <-done:
	case <-aborted:
		fmt.Println()
		ui.Warn("Interrupted -- stopping mapping so the robot does not keep going.")
	}

	built, err := stop()
	if err != nil {
		ui.Error(err.Error())
		ui.Faint("The robot may still be finishing. Check with 'emos map list'.")
		return fmt.Errorf("mapping did not complete")
	}
	if built == nil {
		return fmt.Errorf("mapping stopped but produced no map")
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
