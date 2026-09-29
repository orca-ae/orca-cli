// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import "github.com/spf13/cobra"

// NewGroupCommand creates a grouped workspace command for embedders that want
// the resource commands under a workspace parent.
func NewGroupCommand(opts *Options) *cobra.Command {
	opts = setCurrentOptions(opts)
	rootCmd := &cobra.Command{
		Use:   "workspace [command]",
		Short: "Manage workspace-scoped registry resources",
		Long:  "Manage workspace-scoped registry resources exposed by the workspace registry API.",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}
	rootCmd.AddCommand(
		NewCmdAPIVersions(opts),
		NewCmdHealthz(opts),
		NewCmdReadyz(opts),
		NewCmdHealth(opts),
		NewCmdConnections(opts),
		NewCmdAgent(opts),
		NewCmdFunctions(opts),
		NewCmdKafkaConnect(opts),
		NewCmdPackages(opts),
		NewCmdSources(opts),
		NewCmdSinks(opts),
		NewCmdGuardrails(opts),
		NewCmdModelPrices(opts),
		NewCmdAPIGroups(opts),
		NewCmdAPIResources(opts),
	)
	return rootCmd
}
