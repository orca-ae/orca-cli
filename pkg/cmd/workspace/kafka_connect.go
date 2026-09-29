// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter"
	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

const (
	kafkaConnectConfigKeyConnection       = "sn.connection"
	kafkaConnectConfigKeyPulsarPackageURL = "sn.pulsar.package.url"
)

type kafkaConnectOptions struct {
	opts      *Options
	ioStreams IOStreams
}

type kafkaConnectorConfigFile struct {
	Name         string                 `json:"name" yaml:"name"`
	InitialState string                 `json:"initial_state,omitempty" yaml:"initial_state,omitempty"`
	Config       map[string]interface{} `json:"config" yaml:"config"`
}

type kafkaConnectConnectionFlags struct {
	Connection           string
	UseConnection        bool
	UsePackageConnection bool
}

// NewCmdKafkaConnect creates workspace Kafka Connect commands.
func NewCmdKafkaConnect(opts *Options) *cobra.Command {
	opts = setCurrentOptions(opts)
	o := &kafkaConnectOptions{
		opts:      opts,
		ioStreams: opts.IOStreams,
	}

	cmd := &cobra.Command{
		Use:   "kafka-connect",
		Short: "Manage workspace Kafka Connect resources",
		Long: "Manage Kafka Connect resources through the workspace registry API. " +
			"Request authentication continues to use the workspace root service-account flags. " +
			"Requires the cloud.sn.io extension group; not available on a self-hosted engine.",
		PersistentPreRunE: requireCloudExtension,
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}

	cmd.AddCommand(
		o.newInfoCmd(),
		o.newHealthCmd(),
		o.newAvailableKafkaConnectorsCmd(),
		o.newKafkaConnectorConfigDefinitionCmd(),
		o.newGetCmd(),
		o.newDescribeCmd(),
		o.newApplyCmd(),
		o.newPatchCmd(),
		o.newRestartCmd(),
		o.newPauseCmd(),
		o.newResumeCmd(),
		o.newStopCmd(),
		o.newResetCmd(),
		o.newDeleteCmd(),
	)

	return cmd
}

func (o *kafkaConnectOptions) newHealthCmd() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Display workspace Kafka Connect worker health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newClient()
			if err != nil {
				return err
			}
			health, err := client.GetHealth(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get Kafka Connect health: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, health)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *kafkaConnectOptions) newAvailableKafkaConnectorsCmd() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "available-connectors",
		Short: "List available built-in Kafka connectors",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceCatalogClient()
			if err != nil {
				return err
			}
			items, err := client.ListKafkaConnectors(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list available Kafka connectors: %w", err)
			}
			return renderConnectorDefinitions(o.ioStreams.Out, output, "kafka", items)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *kafkaConnectOptions) newKafkaConnectorConfigDefinitionCmd() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "config-definition [name]",
		Short: "Get configuration definition for a built-in Kafka connector",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceCatalogClient()
			if err != nil {
				return err
			}
			items, err := client.GetKafkaConfigDefinition(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get Kafka connector config definition %q: %w", args[0], err)
			}
			return renderConfigFieldDefinitions(o.ioStreams.Out, output, items)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *kafkaConnectOptions) newClient() (*registry.KafkaConnectClient, error) {
	client, err := newWorkspaceRegistryClient()
	if err != nil {
		return nil, err
	}

	return registry.NewKafkaConnectClient(client), nil
}

func (o *kafkaConnectOptions) newInfoCmd() *cobra.Command {
	output := "text"

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Display workspace Kafka Connect cluster information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}

			client, err := o.newClient()
			if err != nil {
				return err
			}

			info, err := client.GetInfo(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get Kafka Connect information: %w", err)
			}

			return renderKafkaConnectInfo(o.ioStreams.Out, output, *info)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json)")
	return cmd
}

func (o *kafkaConnectOptions) newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Display connector plugins, connectors, and offsets",
	}

	cmd.AddCommand(
		o.newGetConnectorsCmd(),
		o.newGetPluginsCmd(),
		o.newGetPluginCatalogCmd(),
		o.newGetConnectorConfigCmd(),
		o.newGetConnectorStatusCmd(),
		o.newGetConnectorTasksCmd(),
		o.newGetTaskStatusCmd(),
		o.newGetDeprecatedTasksConfigCmd(),
		o.newGetActiveTopicsCmd(),
		o.newGetOffsetsCmd(),
	)

	return cmd
}

func (o *kafkaConnectOptions) newGetConnectorsCmd() *cobra.Command {
	output := "text"

	cmd := &cobra.Command{
		Use:   "connectors",
		Short: "List workspace Kafka Connect connectors",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}

			client, err := o.newClient()
			if err != nil {
				return err
			}

			items, err := client.ListConnectors(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list connectors: %w", err)
			}

			return renderNameList(o.ioStreams.Out, output, "Connector Name", items)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json)")
	return cmd
}

func (o *kafkaConnectOptions) newGetPluginsCmd() *cobra.Command {
	output := "text"
	connectorsOnly := true

	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "List workspace Kafka Connect plugins",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}

			client, err := o.newClient()
			if err != nil {
				return err
			}

			plugins, err := client.ListPlugins(cmd.Context(), connectorsOnly)
			if err != nil {
				return fmt.Errorf("failed to list plugins: %w", err)
			}

			return renderKafkaConnectPlugins(o.ioStreams.Out, output, plugins)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json)")
	cmd.Flags().BoolVar(&connectorsOnly, "connectors-only", connectorsOnly, "List only connector plugins")
	return cmd
}

func (o *kafkaConnectOptions) newGetPluginCatalogCmd() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "plugin-catalog",
		Short: "List installed Function Mesh connector definitions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newClient()
			if err != nil {
				return err
			}
			items, err := client.ListPluginCatalog(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list Kafka Connect plugin catalog: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, items)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *kafkaConnectOptions) newGetConnectorConfigCmd() *cobra.Command {
	return o.newKafkaConnectGetValueCommand("config [connector-name]", "Get raw connector configuration", func(ctx context.Context, client *registry.KafkaConnectClient, args []string) (interface{}, error) {
		return client.GetConnectorConfig(ctx, args[0])
	})
}

func (o *kafkaConnectOptions) newGetConnectorStatusCmd() *cobra.Command {
	return o.newKafkaConnectGetValueCommand("status [connector-name]", "Get connector runtime status", func(ctx context.Context, client *registry.KafkaConnectClient, args []string) (interface{}, error) {
		return client.GetConnectorStatus(ctx, args[0])
	})
}

func (o *kafkaConnectOptions) newGetConnectorTasksCmd() *cobra.Command {
	return o.newKafkaConnectGetValueCommand("tasks [connector-name]", "Get connector task configurations", func(ctx context.Context, client *registry.KafkaConnectClient, args []string) (interface{}, error) {
		return client.GetConnectorTasks(ctx, args[0])
	})
}

func (o *kafkaConnectOptions) newGetTaskStatusCmd() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "task-status [connector-name] [task-id]",
		Short: "Get one connector task status",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			taskID, err := strconv.Atoi(args[1])
			if err != nil || taskID < 0 {
				return fmt.Errorf("invalid task id %q", args[1])
			}
			client, err := o.newClient()
			if err != nil {
				return err
			}
			status, err := client.GetTaskStatus(cmd.Context(), args[0], taskID)
			if err != nil {
				return fmt.Errorf("failed to get task %d status for connector %q: %w", taskID, args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, status)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *kafkaConnectOptions) newGetDeprecatedTasksConfigCmd() *cobra.Command {
	cmd := o.newKafkaConnectGetValueCommand("tasks-config [connector-name]", "Get deprecated connector task-config map", func(ctx context.Context, client *registry.KafkaConnectClient, args []string) (interface{}, error) {
		return client.GetConnectorTasksConfig(ctx, args[0])
	})
	cmd.Deprecated = "use get tasks instead"
	return cmd
}

func (o *kafkaConnectOptions) newGetActiveTopicsCmd() *cobra.Command {
	return o.newKafkaConnectGetValueCommand("topics [connector-name]", "Get connector active topics", func(ctx context.Context, client *registry.KafkaConnectClient, args []string) (interface{}, error) {
		return client.GetActiveTopics(ctx, args[0])
	})
}

func (o *kafkaConnectOptions) newKafkaConnectGetValueCommand(
	use string,
	short string,
	get func(context.Context, *registry.KafkaConnectClient, []string) (interface{}, error),
) *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newClient()
			if err != nil {
				return err
			}
			value, err := get(cmd.Context(), client, args)
			if err != nil {
				return fmt.Errorf("failed to %s: %w", strings.ToLower(short), err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, value)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *kafkaConnectOptions) newGetOffsetsCmd() *cobra.Command {
	output := "text"

	cmd := &cobra.Command{
		Use:   "offsets [connector-name]",
		Short: "Get committed offsets for a connector",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}

			client, err := o.newClient()
			if err != nil {
				return err
			}

			offsets, err := client.GetOffsets(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get offsets for connector %q: %w", args[0], err)
			}

			return renderKafkaConnectOffsets(o.ioStreams.Out, output, args[0], offsets)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json)")
	return cmd
}

func (o *kafkaConnectOptions) newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Display detailed information about connectors or plugins",
	}

	cmd.AddCommand(
		o.newDescribeConnectorCmd(),
		o.newDescribePluginCmd(),
	)

	return cmd
}

func (o *kafkaConnectOptions) newDescribeConnectorCmd() *cobra.Command {
	output := "text"

	cmd := &cobra.Command{
		Use:   "connector [connector-name]",
		Short: "Display detailed information about one connector",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}

			client, err := o.newClient()
			if err != nil {
				return err
			}

			connector, err := client.GetConnector(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get connector %q: %w", args[0], err)
			}
			return renderKafkaConnectConnector(o.ioStreams.Out, output, connector)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json)")
	return cmd
}

func (o *kafkaConnectOptions) newDescribePluginCmd() *cobra.Command {
	output := "text"

	cmd := &cobra.Command{
		Use:   "plugin [plugin-class]",
		Short: "Display plugin configuration definitions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}

			client, err := o.newClient()
			if err != nil {
				return err
			}

			result, err := client.DescribePluginConfig(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get plugin configuration for %q: %w", args[0], err)
			}

			return renderJSONOrText(o.ioStreams.Out, output, result)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json)")
	return cmd
}

func (o *kafkaConnectOptions) newApplyCmd() *cobra.Command {
	output := "text"
	configFile := ""
	dryRun := false
	flags := kafkaConnectConnectionFlags{}

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Create or update a connector from a config file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}

			cfg, err := loadKafkaConnectorConfigFile(configFile)
			if err != nil {
				return err
			}

			connectorName, connectorConfig, err := normalizeKafkaConnectorFile(cfg)
			if err != nil {
				return err
			}

			resolved, err := resolveKafkaConnectConnectionFlags(cmd.Context(), flags)
			if err != nil {
				return err
			}

			client, err := o.newClient()
			if err != nil {
				return err
			}

			currentConfig, exists, err := getExistingKafkaConnectorConfig(cmd.Context(), client, connectorName)
			if err != nil {
				return err
			}

			finalConfig, err := prepareKafkaConnectConfigForApply(connectorConfig, currentConfig, exists, resolved)
			if err != nil {
				return err
			}

			if dryRun {
				_, err := fmt.Fprintf(o.ioStreams.Out, "Local configuration checks passed for connector %q; server validation was not performed\n", connectorName)
				return err
			}

			if exists {
				updated, err := client.UpdateConnectorConfig(cmd.Context(), connectorName, finalConfig)
				if err != nil {
					return fmt.Errorf("failed to update connector %q: %w", connectorName, err)
				}
				return renderKafkaConnectConnector(o.ioStreams.Out, output, updated)
			}

			created, err := client.CreateConnector(cmd.Context(), registry.CreateConnectorRequest{
				Name:         connectorName,
				InitialState: cfg.InitialState,
				Config:       finalConfig,
			})
			if err != nil {
				return fmt.Errorf("failed to create connector %q: %w", connectorName, err)
			}
			return renderKafkaConnectConnector(o.ioStreams.Out, output, created)
		},
	}

	cmd.Flags().StringVarP(&configFile, "config-file", "f", "", "Connector config file in JSON or YAML")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate the connector configuration without applying changes")
	addKafkaConnectConnectionFlags(cmd, &flags)
	_ = cmd.MarkFlagRequired("config-file")
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json)")
	return cmd
}

func (o *kafkaConnectOptions) newPatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patch",
		Short: "Modify connector configuration or offsets",
	}

	cmd.AddCommand(
		o.newPatchConnectorCmd(),
		o.newPatchOffsetsCmd(),
	)

	return cmd
}

func (o *kafkaConnectOptions) newPatchConnectorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connector [connector-name]",
		Short: "Patch one connector configuration (unsupported by current runtime)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("Kafka Connect config PATCH is declared by OpenAPI but not implemented by current Java runtime; use apply with a complete config file")
		},
	}
}

func (o *kafkaConnectOptions) newPatchOffsetsCmd() *cobra.Command {
	force := false
	kafkaTopic := ""
	kafkaPartition := 0
	kafkaOffset := int64(0)
	sourcePartitionJSON := ""
	sourceOffsetJSON := ""

	cmd := &cobra.Command{
		Use:   "offsets [connector-name]",
		Short: "Alter offsets for a connector",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sinkFlagsChanged := cmd.Flags().Changed("kafka-topic") || cmd.Flags().Changed("kafka-partition") || cmd.Flags().Changed("kafka-offset")
			sourceFlagsChanged := cmd.Flags().Changed("source-partition") || cmd.Flags().Changed("source-offset")
			isSinkMode := cmd.Flags().Changed("kafka-topic") && cmd.Flags().Changed("kafka-partition") && cmd.Flags().Changed("kafka-offset")
			isSourceMode := cmd.Flags().Changed("source-partition") && cmd.Flags().Changed("source-offset")
			if sinkFlagsChanged && !isSinkMode {
				return fmt.Errorf("--kafka-topic, --kafka-partition, and --kafka-offset must be specified together")
			}
			if sourceFlagsChanged && !isSourceMode {
				return fmt.Errorf("--source-partition and --source-offset must be specified together")
			}
			if !isSinkMode && !isSourceMode {
				return fmt.Errorf("specify either sink offset flags or source offset flags")
			}
			if isSinkMode && isSourceMode {
				return fmt.Errorf("sink offset flags and source offset flags cannot be used together")
			}

			offsets, err := buildKafkaConnectOffsetsPayload(isSinkMode, kafkaTopic, kafkaPartition, kafkaOffset, sourcePartitionJSON, sourceOffsetJSON)
			if err != nil {
				return err
			}

			if !force {
				ok, err := confirmKafkaConnectAction(
					o.ioStreams.In,
					o.ioStreams.Out,
					fmt.Sprintf("WARNING: This operation will alter offsets for connector %q. Continue? (y/n): ", args[0]),
				)
				if err != nil {
					return err
				}
				if !ok {
					_, _ = fmt.Fprintln(o.ioStreams.Out, "Operation cancelled.")
					return nil
				}
			}

			client, err := o.newClient()
			if err != nil {
				return err
			}
			if err := client.AlterOffsets(cmd.Context(), args[0], offsets); err != nil {
				return fmt.Errorf("failed to alter offsets for connector %q: %w", args[0], err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Successfully altered offsets for connector %q.\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&kafkaTopic, "kafka-topic", "", "Kafka topic name for sink connector offsets")
	cmd.Flags().IntVar(&kafkaPartition, "kafka-partition", 0, "Kafka partition for sink connector offsets")
	cmd.Flags().Int64Var(&kafkaOffset, "kafka-offset", 0, "Kafka offset for sink connector offsets")
	cmd.Flags().StringVar(&sourcePartitionJSON, "source-partition", "", "Source partition as JSON")
	cmd.Flags().StringVar(&sourceOffsetJSON, "source-offset", "", "Source offset as JSON")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	return cmd
}

func (o *kafkaConnectOptions) newRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart connectors or tasks",
	}
	cmd.AddCommand(
		o.newRestartConnectorCmd(),
		o.newRestartTaskCmd(),
	)
	return cmd
}

func (o *kafkaConnectOptions) newRestartConnectorCmd() *cobra.Command {
	includeTasks := false
	onlyFailed := false
	output := "text"
	cmd := &cobra.Command{
		Use:   "connector [connector-name]",
		Short: "Restart a connector",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newClient()
			if err != nil {
				return err
			}
			status, err := client.RestartConnectorWithOptions(cmd.Context(), args[0], registry.RestartConnectorOptions{
				IncludeTasks: includeTasks,
				OnlyFailed:   onlyFailed,
			})
			if err != nil {
				return fmt.Errorf("failed to restart connector %q: %w", args[0], err)
			}
			if status != nil {
				return renderJSONOrText(o.ioStreams.Out, output, status)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Connector %q restarted successfully\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&includeTasks, "include-tasks", false, "Restart connector tasks too")
	cmd.Flags().BoolVar(&onlyFailed, "only-failed", false, "Restart only failed connector or tasks")
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *kafkaConnectOptions) newRestartTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task [connector-name] [task-id]",
		Short: "Restart one connector task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid task id %q", args[1])
			}
			client, err := o.newClient()
			if err != nil {
				return err
			}
			if err := client.RestartTask(cmd.Context(), args[0], taskID); err != nil {
				return fmt.Errorf("failed to restart task %q for connector %q: %w", args[1], args[0], err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Task %s of connector %q restarted successfully\n", args[1], args[0])
			return nil
		},
	}
	return cmd
}

func (o *kafkaConnectOptions) newPauseCmd() *cobra.Command {
	return o.newConnectorLifecycleCommand("pause", "Pause a connector", func(ctx context.Context, client *registry.KafkaConnectClient, name string) error {
		return client.PauseConnector(ctx, name)
	})
}

func (o *kafkaConnectOptions) newResumeCmd() *cobra.Command {
	return o.newConnectorLifecycleCommand("resume", "Resume a connector", func(ctx context.Context, client *registry.KafkaConnectClient, name string) error {
		return client.ResumeConnector(ctx, name)
	})
}

func (o *kafkaConnectOptions) newStopCmd() *cobra.Command {
	return o.newConnectorLifecycleCommand("stop", "Stop a connector", func(ctx context.Context, client *registry.KafkaConnectClient, name string) error {
		return client.StopConnector(ctx, name)
	})
}

func (o *kafkaConnectOptions) newResetCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "reset", Short: "Reset connector runtime tracking data"}
	cmd.AddCommand(o.newResetActiveTopicsCmd())
	return cmd
}

func (o *kafkaConnectOptions) newResetActiveTopicsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "topics [connector-name]",
		Short: "Reset connector active-topic tracking",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := o.newClient()
			if err != nil {
				return err
			}
			if err := client.ResetActiveTopics(cmd.Context(), args[0]); err != nil {
				return fmt.Errorf("failed to reset active topics for connector %q: %w", args[0], err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Reset active topics for connector %q successfully\n", args[0])
			return nil
		},
	}
	return cmd
}

func (o *kafkaConnectOptions) newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete connectors or reset offsets",
	}
	cmd.AddCommand(
		o.newDeleteConnectorCmd(),
		o.newDeleteOffsetsCmd(),
	)
	return cmd
}

func (o *kafkaConnectOptions) newDeleteConnectorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connector [connector-name]",
		Short: "Delete a connector",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := o.newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteConnector(cmd.Context(), args[0]); err != nil {
				return fmt.Errorf("failed to delete connector %q: %w", args[0], err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Connector %q deleted successfully\n", args[0])
			return nil
		},
	}
	return cmd
}

func (o *kafkaConnectOptions) newDeleteOffsetsCmd() *cobra.Command {
	force := false

	cmd := &cobra.Command{
		Use:   "offsets [connector-name]",
		Short: "Reset connector offsets",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := o.newClient()
			if err != nil {
				return err
			}

			if !force {
				ok, err := confirmKafkaConnectAction(
					o.ioStreams.In,
					o.ioStreams.Out,
					fmt.Sprintf("WARNING: This operation will reset offsets for connector %q. Continue? (y/n): ", args[0]),
				)
				if err != nil {
					return err
				}
				if !ok {
					_, _ = fmt.Fprintln(o.ioStreams.Out, "Operation cancelled.")
					return nil
				}
			}

			if err := client.ResetOffsets(cmd.Context(), args[0]); err != nil {
				return fmt.Errorf("failed to reset offsets for connector %q: %w", args[0], err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Successfully reset offsets for connector %q.\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	return cmd
}

func (o *kafkaConnectOptions) newConnectorLifecycleCommand(
	use string,
	short string,
	run func(ctx context.Context, client *registry.KafkaConnectClient, name string) error,
) *cobra.Command {
	pastTense := map[string]string{
		"pause":   "paused",
		"resume":  "resumed",
		"stop":    "stopped",
		"restart": "restarted",
	}[use]
	if pastTense == "" {
		pastTense = use + "d"
	}

	cmd := &cobra.Command{
		Use:   use + " [connector-name]",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := o.newClient()
			if err != nil {
				return err
			}
			if err := run(cmd.Context(), client, args[0]); err != nil {
				return fmt.Errorf("failed to %s connector %q: %w", use, args[0], err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Connector %q %s successfully\n", args[0], pastTense)
			return nil
		},
	}
	return cmd
}

func addKafkaConnectConnectionFlags(cmd *cobra.Command, flags *kafkaConnectConnectionFlags) {
	cmd.Flags().StringVar(&flags.Connection, "connection", "", "Kafka connection name written to config key sn.connection")
	cmd.Flags().BoolVar(&flags.UseConnection, "use-connection", false, "Interactively select a Kafka connection for sn.connection")
}

func resolveKafkaConnectConnectionFlags(ctx context.Context, flags kafkaConnectConnectionFlags) (kafkaConnectConnectionFlags, error) {
	connectionName, err := resolveConnectionSelection(ctx, flags.Connection, flags.UseConnection)
	if err != nil {
		return kafkaConnectConnectionFlags{}, err
	}

	return kafkaConnectConnectionFlags{
		Connection:    connectionName,
		UseConnection: flags.UseConnection,
	}, nil
}

func loadKafkaConnectorConfigFile(filename string) (*kafkaConnectorConfigFile, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file %q: %w", filename, err)
	}

	var cfg kafkaConnectorConfigFile
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file %q: %w", filename, err)
	}
	if cfg.Config == nil {
		return nil, fmt.Errorf("configuration file %q must define a config section", filename)
	}
	return &cfg, nil
}

func normalizeKafkaConnectorFile(cfg *kafkaConnectorConfigFile) (string, map[string]string, error) {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		if rawName, ok := cfg.Config["name"].(string); ok {
			name = strings.TrimSpace(rawName)
		}
	}
	if err := validateKafkaConnectConnectorName(name); err != nil {
		return "", nil, err
	}
	if err := validateKafkaConnectInitialState(cfg.InitialState); err != nil {
		return "", nil, err
	}

	result := make(map[string]string, len(cfg.Config))
	for key, value := range cfg.Config {
		scalar, err := kafkaConnectConfigScalar(value)
		if err != nil {
			return "", nil, fmt.Errorf("config.%s: %w", key, err)
		}
		result[key] = scalar
	}
	delete(result, "name")

	if err := validatePreparedKafkaConnectConfig(name, result); err != nil {
		return "", nil, err
	}
	return name, result, nil
}

func kafkaConnectConfigScalar(value interface{}) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), nil
	case int:
		return strconv.Itoa(typed), nil
	case int8:
		return strconv.FormatInt(int64(typed), 10), nil
	case int16:
		return strconv.FormatInt(int64(typed), 10), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case uint:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint64:
		return strconv.FormatUint(typed, 10), nil
	case json.Number:
		return typed.String(), nil
	default:
		return "", fmt.Errorf("must be a scalar string, boolean, or number, got %T", value)
	}
}

func prepareKafkaConnectConfigForApply(
	desired map[string]string,
	current map[string]string,
	exists bool,
	resolved kafkaConnectConnectionFlags,
) (map[string]string, error) {
	result := copyKafkaConnectConfig(desired)

	if exists {
		if err := validateKafkaConnectImmutableConnectionKeys(current, resolved); err != nil {
			return nil, err
		}
		if err := validateKafkaConnectImmutableConnectionConfigValues(current, result); err != nil {
			return nil, err
		}

		if value := strings.TrimSpace(current[kafkaConnectConfigKeyConnection]); value != "" {
			result[kafkaConnectConfigKeyConnection] = value
		}
	} else {
		if resolved.Connection != "" {
			result[kafkaConnectConfigKeyConnection] = resolved.Connection
		}
	}

	if strings.TrimSpace(result[kafkaConnectConfigKeyConnection]) == "" {
		return nil, fmt.Errorf("connector config requires %q in registry mode; use --connection, --use-connection, or define it in config", kafkaConnectConfigKeyConnection)
	}

	return result, nil
}

func validateKafkaConnectImmutableConnectionKeys(current map[string]string, resolved kafkaConnectConnectionFlags) error {
	if currentConnection := strings.TrimSpace(current[kafkaConnectConfigKeyConnection]); resolved.Connection != "" && currentConnection != resolved.Connection {
		return fmt.Errorf("%q cannot be changed for an existing connector: current=%q requested=%q", kafkaConnectConfigKeyConnection, currentConnection, resolved.Connection)
	}
	return nil
}

func validateKafkaConnectImmutableConnectionConfigValues(current, desired map[string]string) error {
	for _, key := range []string{kafkaConnectConfigKeyConnection} {
		currentValue := strings.TrimSpace(current[key])
		desiredValue := strings.TrimSpace(desired[key])
		if desiredValue != "" && currentValue != desiredValue {
			return fmt.Errorf("%q cannot be changed for an existing connector: current=%q requested=%q", key, currentValue, desiredValue)
		}
	}
	return nil
}

func validatePreparedKafkaConnectConfig(name string, config map[string]string) error {
	if err := validateKafkaConnectConnectorName(name); err != nil {
		return err
	}
	if strings.TrimSpace(config["connector.class"]) == "" {
		return fmt.Errorf("config.connector.class is required")
	}
	if tasksMax, ok := config["tasks.max"]; ok && strings.TrimSpace(tasksMax) != "" {
		val, err := strconv.Atoi(strings.TrimSpace(tasksMax))
		if err != nil || val <= 0 {
			return fmt.Errorf("config.tasks.max must be a positive integer")
		}
	}
	return nil
}

func validateKafkaConnectConnectorName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("connector name cannot be empty")
	}
	if len(name) > 29 {
		return fmt.Errorf("connector name cannot exceed 29 characters")
	}
	return nil
}

func validateKafkaConnectInitialState(initialState string) error {
	if initialState == "" {
		return nil
	}
	switch strings.ToUpper(initialState) {
	case "RUNNING", "PAUSED", "STOPPED":
		return nil
	default:
		return fmt.Errorf("initial_state must be one of RUNNING, PAUSED, or STOPPED")
	}
}

func getExistingKafkaConnectorConfig(ctx context.Context, client *registry.KafkaConnectClient, name string) (map[string]string, bool, error) {
	config, err := client.GetConnectorConfig(ctx, name)
	if err == nil {
		return config, true, nil
	}

	var httpErr *registry.HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode == 404 {
		return nil, false, nil
	}

	return nil, false, fmt.Errorf("failed to read existing connector config for %q: %w", name, err)
}

func copyKafkaConnectConfig(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func buildKafkaConnectOffsetsPayload(
	isSinkMode bool,
	kafkaTopic string,
	kafkaPartition int,
	kafkaOffset int64,
	sourcePartitionJSON string,
	sourceOffsetJSON string,
) (registry.ConnectorOffsets, error) {
	if isSinkMode {
		return registry.ConnectorOffsets{
			Offsets: []map[string]interface{}{
				{
					"partition": map[string]interface{}{
						"kafka_topic":     kafkaTopic,
						"kafka_partition": kafkaPartition,
					},
					"offset": map[string]interface{}{
						"kafka_offset": kafkaOffset,
					},
				},
			},
		}, nil
	}

	var sourcePartition map[string]interface{}
	if err := json.Unmarshal([]byte(sourcePartitionJSON), &sourcePartition); err != nil {
		return registry.ConnectorOffsets{}, fmt.Errorf("failed to parse --source-partition: %w", err)
	}
	var sourceOffset map[string]interface{}
	if err := json.Unmarshal([]byte(sourceOffsetJSON), &sourceOffset); err != nil {
		return registry.ConnectorOffsets{}, fmt.Errorf("failed to parse --source-offset: %w", err)
	}

	return registry.ConnectorOffsets{
		Offsets: []map[string]interface{}{
			{
				"partition": sourcePartition,
				"offset":    sourceOffset,
			},
		},
	}, nil
}

func renderKafkaConnectInfo(writer io.Writer, output string, info registry.ServerInfo) error {
	switch output {
	case "json":
		return renderJSON(writer, info)
	case "yaml":
		return renderYAML(writer, info)
	default:
		table := tablewriter.NewWriter(writer)
		table.SetHeader([]string{"Version", "Commit", "Kafka Cluster ID"})
		table.Append([]string{info.Version, info.Commit, info.KafkaClusterID})
		table.Render()
		return nil
	}
}

func renderKafkaConnectPlugins(writer io.Writer, output string, plugins []registry.PluginInfo) error {
	switch output {
	case "json":
		return renderJSON(writer, plugins)
	case "yaml":
		return renderYAML(writer, plugins)
	default:
		table := tablewriter.NewWriter(writer)
		table.SetHeader([]string{"Class", "Type", "Version"})
		for _, plugin := range plugins {
			table.Append([]string{plugin.Class, plugin.Type, plugin.Version})
		}
		table.Render()
		return nil
	}
}

func renderKafkaConnectConnector(writer io.Writer, output string, connector *registry.ConnectorInfo) error {
	if connector == nil {
		return nil
	}

	switch output {
	case "json":
		return renderJSON(writer, connector)
	case "yaml":
		return renderYAML(writer, connector)
	default:
		_, _ = fmt.Fprintf(writer, "Connector: %s\n", connector.Name)
		_, _ = fmt.Fprintf(writer, "Type: %s\n", connector.Type)
		if len(connector.Config) > 0 {
			_, _ = fmt.Fprintln(writer, "\nConfiguration:")
			keys := make([]string, 0, len(connector.Config))
			for key := range connector.Config {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				_, _ = fmt.Fprintf(writer, "  %s: %s\n", key, connector.Config[key])
			}
		}
		if len(connector.Tasks) > 0 {
			_, _ = fmt.Fprintln(writer, "\nTasks:")
			table := tablewriter.NewWriter(writer)
			table.SetHeader([]string{"Connector", "Task ID"})
			for _, task := range connector.Tasks {
				table.Append([]string{task.Connector, strconv.Itoa(task.Task)})
			}
			table.Render()
		}
		return nil
	}
}

func renderKafkaConnectOffsets(writer io.Writer, output string, connectorName string, offsets *registry.ConnectorOffsets) error {
	switch output {
	case "json":
		return renderJSON(writer, offsets)
	case "yaml":
		return renderYAML(writer, offsets)
	default:
		_, _ = fmt.Fprintf(writer, "CONNECTOR OFFSETS: %s\n", connectorName)
		if offsets == nil || len(offsets.Offsets) == 0 {
			_, _ = fmt.Fprintln(writer, "No offsets available")
			return nil
		}
		for index, entry := range offsets.Offsets {
			_, _ = fmt.Fprintf(writer, "Offset #%d:\n", index+1)
			if partition, ok := entry["partition"]; ok {
				partitionJSON, _ := json.MarshalIndent(partition, "", "  ")
				_, _ = fmt.Fprintf(writer, "  Partition: %s\n", string(partitionJSON))
			}
			if value, ok := entry["offset"]; ok {
				offsetJSON, _ := json.MarshalIndent(value, "", "  ")
				_, _ = fmt.Fprintf(writer, "  Offset: %s\n", string(offsetJSON))
			}
		}
		return nil
	}
}

func renderJSONOrText(writer io.Writer, output string, value interface{}) error {
	switch output {
	case "json":
		return renderJSON(writer, value)
	case "yaml":
		return renderYAML(writer, value)
	default:
		encoded, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(writer, string(encoded))
		return err
	}
}

func confirmKafkaConnectAction(reader io.Reader, writer io.Writer, prompt string) (bool, error) {
	if _, err := fmt.Fprint(writer, prompt); err != nil {
		return false, err
	}
	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
