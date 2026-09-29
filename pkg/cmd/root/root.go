// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"os"

	"github.com/orca-ae/orca-cli/pkg/cmd/local"
	"github.com/orca-ae/orca-cli/pkg/cmd/workspace"
	"github.com/spf13/cobra"
)

// NewCommand creates the standalone ork command with workspace and local commands.
func NewCommand(streams workspace.IOStreams) *cobra.Command {
	opts := &workspace.Options{IOStreams: streams}
	cmd := &cobra.Command{
		Use:          "ork",
		Short:        "Manage and interact with resources in Orca Agent Engine",
		SilenceUsage: true,
	}
	cmd.PersistentFlags().StringVar(&opts.RegistryURL, "registry-url", os.Getenv("ORCA_REGISTRY_URL"), "Deployment host root URL (e.g. https://host.example.com), not including a /v1 or /v1/registry suffix")
	// Keep credential defaults empty so Cobra does not render secret environment values in help.
	cmd.PersistentFlags().StringVar(&opts.AccessToken, "access-token", "", "Workspace registry bearer token")
	cmd.PersistentFlags().StringVar(&opts.APIKey, "api-key", "", "Managed Agents workspace API key")
	opts.AccessToken = os.Getenv("ORCA_ACCESS_TOKEN")
	opts.APIKey = os.Getenv("ORCA_API_KEY")
	cmd.MarkFlagsMutuallyExclusive("access-token", "api-key")

	cmd.AddCommand(
		local.NewCommand(streams.Out, streams.ErrOut),
		workspace.NewCmdAPIVersions(opts),
		workspace.NewCmdHealthz(opts),
		workspace.NewCmdReadyz(opts),
		workspace.NewCmdHealth(opts),
		workspace.NewCmdConnections(opts),
		workspace.NewCmdAgent(opts),
		workspace.NewCmdFunctions(opts),
		workspace.NewCmdKafkaConnect(opts),
		workspace.NewCmdPackages(opts),
		workspace.NewCmdSources(opts),
		workspace.NewCmdSinks(opts),
		workspace.NewCmdGuardrails(opts),
		workspace.NewCmdModelPrices(opts),
		workspace.NewCmdAPIGroups(opts),
		workspace.NewCmdAPIResources(opts),
	)
	return cmd
}
