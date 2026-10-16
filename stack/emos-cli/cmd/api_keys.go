package cmd

import (
	"os"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/runner"
	"github.com/spf13/cobra"
)

var (
	keyName    string
	keyScopes  string
	keyExpires string
)

var apiKeysCmd = &cobra.Command{
	Use:   "api-keys",
	Short: "Manage the API keys of recipe UIs",
	Long: `Manage the API keys other programs use to reach a recipe's UI API.

A recipe that calls enable_ui serves its API over HTTPS on its port and needs
a key on every request except the health check. Keys carry scopes: 'read'
covers data and streams, 'command' covers publishing, services, goals and
cancels. Keys are kept, hashed, in ~/emos/.ui-security; running recipes pick
up created and revoked keys without a restart.`,
}

func init() {
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a key and print it once",
		RunE: func(cmd *cobra.Command, args []string) error {
			toolArgs := []string{"keys", "create", "--name", keyName, "--scopes", keyScopes}
			if keyExpires != "" {
				toolArgs = append(toolArgs, "--expires", keyExpires)
			}
			return runUISecurity(toolArgs...)
		},
	}
	create.Flags().StringVar(&keyName, "name", "", "who the key is for")
	create.Flags().StringVar(&keyScopes, "scopes", "read", "comma separated: read, command")
	create.Flags().StringVar(&keyExpires, "expires", "", "last day the key is valid (YYYY-MM-DD)")
	create.MarkFlagRequired("name")
	apiKeysCmd.AddCommand(create)
	configCmd.AddCommand(apiKeysCmd)

	apiKeysCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUISecurity("keys", "list")
		},
	})
	apiKeysCmd.AddCommand(&cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke a key, by the id 'emos config api-keys list' shows",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUISecurity("keys", "revoke", args[0])
		},
	})
}

func runUISecurity(args ...string) error {
	return runner.RunUISecurity(config.LoadConfig(), os.Stdout, args...)
}
