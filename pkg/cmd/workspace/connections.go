// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/olekukonko/tablewriter"
	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

type connectionsOptions struct {
	opts      *Options
	ioStreams IOStreams
}

// NewCmdConnections creates workspace connection commands.
func NewCmdConnections(opts *Options) *cobra.Command {
	opts = setCurrentOptions(opts)
	o := &connectionsOptions{
		opts:      opts,
		ioStreams: opts.IOStreams,
	}

	cmd := &cobra.Command{
		Use:   "connections",
		Short: "Manage workspace connections",
		Long: "Manage Connection resources through the workspace registry API. " +
			"Requires the cloud.sn.io extension group; not available on a self-hosted engine.",
		PersistentPreRunE: requireCloudExtension,
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}

	cmd.AddCommand(
		o.newListCommand(),
		o.newGetCommand(),
		o.newCreateCommand(),
		o.newValidateCommand(),
		o.newUpdateCommand(),
		o.newDeleteCommand(),
		o.newTestCommand(),
	)

	return cmd
}

func (o *connectionsOptions) newValidateCommand() *cobra.Command {
	options := newConnectionMutationOptions(connectionMutationModeCreate)

	cmd := &cobra.Command{
		Use:   "validate --name <name> --type <type>",
		Short: "Validate a workspace connection configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := options.buildCreatePayload(cmd)
			if err != nil {
				return err
			}
			client, err := newWorkspaceConnectionsClient()
			if err != nil {
				return err
			}
			if err := client.Validate(cmd.Context(), payload); err != nil {
				return fmt.Errorf("failed to validate connection %q: %w", payload.Name, err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Connection %q configuration is valid\n", payload.Name)
			return nil
		},
	}

	options.addFlags(cmd)
	_ = cmd.Flags().MarkHidden(flagOutput)
	return cmd
}

func (o *connectionsOptions) newListCommand() *cobra.Command {
	output := "text"

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace connections",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateConnectionsOutput(output); err != nil {
				return err
			}

			client, err := newWorkspaceConnectionsClient()
			if err != nil {
				return err
			}

			items, err := client.List(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list connections: %w", err)
			}

			return renderConnectionsList(o.ioStreams.Out, output, items)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *connectionsOptions) newGetCommand() *cobra.Command {
	output := "text"

	cmd := &cobra.Command{
		Use:   "get [name]",
		Short: "Get a workspace connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateConnectionsOutput(output); err != nil {
				return err
			}

			client, err := newWorkspaceConnectionsClient()
			if err != nil {
				return err
			}

			item, err := client.Get(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get connection %q: %w", args[0], err)
			}

			return renderConnection(o.ioStreams.Out, output, *item)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *connectionsOptions) newCreateCommand() *cobra.Command {
	options := newConnectionMutationOptions(connectionMutationModeCreate)

	cmd := &cobra.Command{
		Use:   "create --name <name> --type <type>",
		Short: "Create a workspace connection",
		Example: strings.TrimSpace(`
ork connections create --name my-kafka --type kafka --kafka-bootstrap-servers broker:9092
ork connections create --name my-pulsar --type pulsar --pulsar-service-url pulsar://broker:6650 --pulsar-admin-url http://broker:8080
`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := options.buildCreatePayload(cmd)
			if err != nil {
				return err
			}

			client, err := newWorkspaceConnectionsClient()
			if err != nil {
				return err
			}

			if err := client.Create(cmd.Context(), payload); err != nil {
				return fmt.Errorf("failed to create connection %q: %w", payload.Name, err)
			}

			if options.output == "text" {
				_, _ = fmt.Fprintf(o.ioStreams.Out, "Created connection %q successfully\n", payload.Name)
				return nil
			}

			item, err := client.Get(cmd.Context(), payload.Name)
			if err != nil {
				return fmt.Errorf("failed to fetch created connection %q: %w", payload.Name, err)
			}

			return renderConnection(o.ioStreams.Out, options.output, *item)
		},
	}

	options.addFlags(cmd)
	return cmd
}

func (o *connectionsOptions) newUpdateCommand() *cobra.Command {
	options := newConnectionMutationOptions(connectionMutationModeUpdate)

	cmd := &cobra.Command{
		Use:   "update --name <name>",
		Short: "Update a workspace connection",
		Example: strings.TrimSpace(`
ork connections update --name my-kafka --kafka-bootstrap-servers broker-1:9092,broker-2:9092
ork connections update --name my-pulsar --pulsar-auth-type none --pulsar-tls-enabled=false
`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := options.validateCommon(cmd); err != nil {
				return err
			}

			client, err := newWorkspaceConnectionsClient()
			if err != nil {
				return err
			}

			existing, err := client.Get(cmd.Context(), options.name)
			if err != nil {
				return fmt.Errorf("failed to get connection %q: %w", options.name, err)
			}

			payload, err := options.buildUpdatedPayload(cmd, *existing)
			if err != nil {
				return err
			}

			if err := client.Update(cmd.Context(), options.name, payload); err != nil {
				return fmt.Errorf("failed to update connection %q: %w", options.name, err)
			}

			if options.output == "text" {
				_, _ = fmt.Fprintf(o.ioStreams.Out, "Updated connection %q successfully\n", options.name)
				return nil
			}

			item, err := client.Get(cmd.Context(), options.name)
			if err != nil {
				return fmt.Errorf("failed to fetch updated connection %q: %w", options.name, err)
			}

			return renderConnection(o.ioStreams.Out, options.output, *item)
		},
	}

	options.addFlags(cmd)
	return cmd
}

func (o *connectionsOptions) newDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a workspace connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newWorkspaceConnectionsClient()
			if err != nil {
				return err
			}

			if err := client.Delete(cmd.Context(), args[0]); err != nil {
				return fmt.Errorf("failed to delete connection %q: %w", args[0], err)
			}

			_, _ = fmt.Fprintf(o.ioStreams.Out, "Deleted connection %q successfully\n", args[0])
			return nil
		},
	}

	return cmd
}

func (o *connectionsOptions) newTestCommand() *cobra.Command {
	output := "text"

	cmd := &cobra.Command{
		Use:   "test [name]",
		Short: "Test a workspace connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateConnectionsOutput(output); err != nil {
				return err
			}

			client, err := newWorkspaceConnectionsClient()
			if err != nil {
				return err
			}

			health, err := client.Test(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to test connection %q: %w", args[0], err)
			}

			return renderConnectionHealth(o.ioStreams.Out, output, *health)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func validateConnectionsOutput(output string) error {
	switch output {
	case "text", "json", "yaml":
		return nil
	default:
		return fmt.Errorf("--output must be one of: text, json, yaml")
	}
}

func renderConnectionsList(writer io.Writer, output string, items []registry.ConnectionConfig) error {
	switch output {
	case "json":
		return renderJSON(writer, items)
	case "yaml":
		return renderYAML(writer, items)
	default:
		table := tablewriter.NewWriter(writer)
		table.SetHeader([]string{"Name", "Type", "Phase", "Internal"})
		for _, item := range items {
			phase := ""
			if item.Status != nil {
				phase = string(item.Status.Phase)
			}
			table.Append([]string{
				item.Name,
				string(item.Spec.Type),
				phase,
				fmt.Sprintf("%t", item.Internal),
			})
		}
		table.Render()
		return nil
	}
}

func renderConnection(writer io.Writer, output string, item registry.ConnectionConfig) error {
	switch output {
	case "json":
		return renderJSON(writer, item)
	case "yaml":
		return renderYAML(writer, item)
	default:
		phase := ""
		lastTestedAt := ""
		message := ""
		if item.Status != nil {
			phase = string(item.Status.Phase)
			lastTestedAt = string(item.Status.LastTestedAt)
			message = item.Status.Message
		}

		_, _ = fmt.Fprintf(writer, "Name: %s\n", item.Name)
		_, _ = fmt.Fprintf(writer, "Type: %s\n", item.Spec.Type)
		_, _ = fmt.Fprintf(writer, "Cluster Ref: %s\n", item.ClusterRef)
		_, _ = fmt.Fprintf(writer, "Phase: %s\n", phase)
		_, _ = fmt.Fprintf(writer, "Internal: %t\n", item.Internal)
		if lastTestedAt != "" {
			_, _ = fmt.Fprintf(writer, "Last Tested At: %s\n", lastTestedAt)
		}
		if strings.TrimSpace(message) != "" {
			_, _ = fmt.Fprintf(writer, "Message: %s\n", message)
		}
		return nil
	}
}

func renderConnectionHealth(writer io.Writer, output string, health registry.ConnectionHealthStatus) error {
	switch output {
	case "json":
		return renderJSON(writer, health)
	case "yaml":
		return renderYAML(writer, health)
	default:
		_, _ = fmt.Fprintf(writer, "Name: %s\n", health.Name)
		_, _ = fmt.Fprintf(writer, "Healthy: %t\n", health.Healthy)
		_, _ = fmt.Fprintf(writer, "Phase: %s\n", health.Phase)
		if strings.TrimSpace(health.LastTestedAt) != "" {
			_, _ = fmt.Fprintf(writer, "Last Tested At: %s\n", health.LastTestedAt)
		}
		if strings.TrimSpace(health.Message) != "" {
			_, _ = fmt.Fprintf(writer, "Message: %s\n", health.Message)
		}
		return nil
	}
}

func renderJSON(writer io.Writer, value interface{}) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(writer, string(encoded))
	return nil
}

func renderYAML(writer io.Writer, value interface{}) error {
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(writer, string(encoded))
	return nil
}
