package cmd

import (
	"errors"
	"fmt"

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
