// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

type sourcesOptions struct {
	opts      *Options
	ioStreams IOStreams
}

type workspaceSourcesClient interface {
	List(ctx context.Context) ([]string, error)
	Get(ctx context.Context, name string) (*registry.RegistrySourceConfig, error)
	Create(ctx context.Context, cfg registry.RegistrySourceConfig, filePath, packageURL string) error
	Update(ctx context.Context, name string, cfg registry.RegistrySourceConfig, filePath, packageURL string, updateOptions *registry.UpdateOptionsImpl) error
	Delete(ctx context.Context, name string) error
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Restart(ctx context.Context, name string) error
	StartInstance(ctx context.Context, name, instanceID string) error
	StopInstance(ctx context.Context, name, instanceID string) error
	RestartInstance(ctx context.Context, name, instanceID string) error
	Status(ctx context.Context, name string) (*registry.SourceStatus, error)
	InstanceStatus(ctx context.Context, name, instanceID string) (*registry.SourceInstanceStatusData, error)
}

var newWorkspaceSourcesClient = func() (workspaceSourcesClient, error) {
	client, err := newWorkspaceRegistryClient()
	if err != nil {
		return nil, err
	}
	return registry.NewSourcesClient(client), nil
}

type sourceApplyOptions struct {
	Tenant               string
	Namespace            string
	Name                 string
	SourceType           string
	ProcessingGuarantees string
	DestinationTopicName string
	ProducerConfig       string
	BatchBuilder         string
	LogTopic             string
	DeserializationClass string
	SchemaType           string
	Parallelism          int
	Archive              string
	ClassName            string
	SourceConfigFile     string
	CPU                  float64
	RAM                  int64
	Disk                 int64
	SourceConfig         string
	BatchSourceConfig    string
	CustomRuntimeOptions string
	Secrets              string
	Connection           string
	UseConnection        bool
	SNServiceAccount     string
	UpdateAuthData       bool
}

// NewCmdSources creates workspace source commands.
func NewCmdSources(opts *Options) *cobra.Command {
	opts = setCurrentOptions(opts)
	o := &sourcesOptions{
		opts:      opts,
		ioStreams: opts.IOStreams,
	}

	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage workspace sources",
		Long: "Manage Pulsar IO Sources through the workspace registry API. " +
			"Requires the cloud.sn.io extension group; not available on a self-hosted engine.",
		PersistentPreRunE: requireCloudExtension,
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}

	cmd.AddCommand(
		o.newListCommand(),
		o.newGetCommand(),
		o.newAvailableSourcesCommand(),
		o.newConfigDefinitionCommand(),
		o.newCreateCommand(),
		o.newUpdateCommand(),
		o.newDeleteCommand(),
		o.newStartCommand(),
		o.newStopCommand(),
		o.newRestartCommand(),
		o.newStatusCommand(),
	)

	return cmd
}

func (o *sourcesOptions) newListCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace sources",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newSourcesClient()
			if err != nil {
				return err
			}
			items, err := client.List(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list sources: %w", err)
			}
			return renderNameList(o.ioStreams.Out, output, "Source Name", items)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *sourcesOptions) newGetCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "get [name]",
		Short: "Get a workspace source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newSourcesClient()
			if err != nil {
				return err
			}
			item, err := client.Get(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get source %q: %w", args[0], err)
			}
			return renderSourceConfig(o.ioStreams.Out, output, *item)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *sourcesOptions) newAvailableSourcesCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "available-sources",
		Short: "List available built-in source connectors",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceCatalogClient()
			if err != nil {
				return err
			}
			items, err := client.ListSources(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list available sources: %w", err)
			}
			return renderConnectorDefinitions(o.ioStreams.Out, output, "source", items)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *sourcesOptions) newConfigDefinitionCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "config-definition [name]",
		Short: "Get configuration definition for a built-in source connector",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceCatalogClient()
			if err != nil {
				return err
			}
			items, err := client.GetSourceConfigDefinition(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get source config definition %q: %w", args[0], err)
			}
			return renderConfigFieldDefinitions(o.ioStreams.Out, output, items)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *sourcesOptions) newCreateCommand() *cobra.Command {
	opts := &sourceApplyOptions{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a workspace source",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, filePath, packageURL, err := buildSourceConfig(cmd.Context(), opts, false)
			if err != nil {
				return err
			}
			client, err := o.newSourcesClient()
			if err != nil {
				return err
			}
			if err := client.Create(cmd.Context(), cfg, filePath, packageURL); err != nil {
				return fmt.Errorf("failed to create source %q: %w", cfg.Name, err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Created source %q successfully\n", cfg.Name)
			return nil
		},
	}
	addSourceApplyFlags(cmd, opts, true)
	return cmd
}

func (o *sourcesOptions) newUpdateCommand() *cobra.Command {
	opts := &sourceApplyOptions{}
	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Update a workspace source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.Name) != "" && strings.TrimSpace(opts.Name) != args[0] {
				return fmt.Errorf("--name %q does not match requested source %q", opts.Name, args[0])
			}
			opts.Name = args[0]
			cfg, filePath, packageURL, err := buildSourceConfig(cmd.Context(), opts, true)
			if err != nil {
				return err
			}
			client, err := o.newSourcesClient()
			if err != nil {
				return err
			}
			updateOptions := buildSourceUpdateOptions(cmd.Flags().Changed("update-auth-data"), opts.UpdateAuthData)
			if err := client.Update(cmd.Context(), args[0], cfg, filePath, packageURL, updateOptions); err != nil {
				return fmt.Errorf("failed to update source %q: %w", args[0], err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Updated source %q successfully\n", args[0])
			return nil
		},
	}
	addSourceApplyFlags(cmd, opts, false)
	cmd.Flags().BoolVar(&opts.UpdateAuthData, "update-auth-data", false, "Whether or not to update the auth data")
	return cmd
}

func (o *sourcesOptions) newDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a workspace source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := o.newSourcesClient()
			if err != nil {
				return err
			}
			if err := client.Delete(cmd.Context(), args[0]); err != nil {
				return fmt.Errorf("failed to delete source %q: %w", args[0], err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Deleted source %q successfully\n", args[0])
			return nil
		},
	}
	return cmd
}

func (o *sourcesOptions) newStartCommand() *cobra.Command {
	return o.newLifecycleCommand("start", "Start a workspace source",
		func(ctx context.Context, client workspaceSourcesClient, name string) error {
			return client.Start(ctx, name)
		},
		func(ctx context.Context, client workspaceSourcesClient, name, instanceID string) error {
			return client.StartInstance(ctx, name, instanceID)
		})
}

func (o *sourcesOptions) newStopCommand() *cobra.Command {
	return o.newLifecycleCommand("stop", "Stop a workspace source",
		func(ctx context.Context, client workspaceSourcesClient, name string) error {
			return client.Stop(ctx, name)
		},
		func(ctx context.Context, client workspaceSourcesClient, name, instanceID string) error {
			return client.StopInstance(ctx, name, instanceID)
		})
}

func (o *sourcesOptions) newRestartCommand() *cobra.Command {
	return o.newLifecycleCommand("restart", "Restart a workspace source",
		func(ctx context.Context, client workspaceSourcesClient, name string) error {
			return client.Restart(ctx, name)
		},
		func(ctx context.Context, client workspaceSourcesClient, name, instanceID string) error {
			return client.RestartInstance(ctx, name, instanceID)
		})
}

func (o *sourcesOptions) newLifecycleCommand(
	use string,
	short string,
	runAll func(context.Context, workspaceSourcesClient, string) error,
	runInstance func(context.Context, workspaceSourcesClient, string, string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use + " [name] [instance-id]",
		Short: short,
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := o.newSourcesClient()
			if err != nil {
				return err
			}
			if len(args) == 2 {
				err = runInstance(cmd.Context(), client, args[0], args[1])
			} else {
				err = runAll(cmd.Context(), client, args[0])
			}
			if err != nil {
				return fmt.Errorf("failed to %s source %q: %w", use, args[0], err)
			}
			if len(args) == 2 {
				_, _ = fmt.Fprintf(o.ioStreams.Out, "%s source %q instance %q successfully\n", lifecycleActionLabel(use), args[0], args[1])
			} else {
				_, _ = fmt.Fprintf(o.ioStreams.Out, "%s source %q successfully\n", lifecycleActionLabel(use), args[0])
			}
			return nil
		},
	}
	return cmd
}

func (o *sourcesOptions) newStatusCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "status [name] [instance-id]",
		Short: "Get workspace source status",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newSourcesClient()
			if err != nil {
				return err
			}
			if len(args) == 2 {
				status, err := client.InstanceStatus(cmd.Context(), args[0], args[1])
				if err != nil {
					return fmt.Errorf("failed to get source %q instance %q status: %w", args[0], args[1], err)
				}
				return renderJSONOrText(o.ioStreams.Out, output, status)
			}
			status, err := client.Status(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get source status %q: %w", args[0], err)
			}
			return renderSourceStatus(o.ioStreams.Out, output, *status)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *sourcesOptions) newSourcesClient() (workspaceSourcesClient, error) {
	return newWorkspaceSourcesClient()
}

func addSourceApplyFlags(cmd *cobra.Command, opts *sourceApplyOptions, includeConnection bool) {
	cmd.Flags().StringVar(&opts.Tenant, "tenant", "", "The tenant of the source")
	cmd.Flags().StringVar(&opts.Namespace, "namespace", "", "The namespace of the source")
	cmd.Flags().StringVar(&opts.Name, "name", "", "The name of the source")
	cmd.Flags().StringVarP(&opts.SourceType, "source-type", "t", "", "The built-in source type")
	cmd.Flags().StringVar(&opts.ProcessingGuarantees, "processing-guarantees", "", "Processing guarantees applied to the source")
	cmd.Flags().StringVar(&opts.DestinationTopicName, "destination-topic-name", "", "The destination Pulsar topic")
	cmd.Flags().StringVar(&opts.ProducerConfig, "producer-config", "", "Custom producer configuration as JSON")
	cmd.Flags().StringVar(&opts.BatchBuilder, "batch-builder", "", "Batch builder strategy")
	cmd.Flags().StringVar(&opts.LogTopic, "log-topic", "", "Topic for source logs")
	cmd.Flags().StringVar(&opts.DeserializationClass, "deserialization-classname", "", "The SerDe class name for the source")
	cmd.Flags().StringVar(&opts.SchemaType, "schema-type", "", "The schema type for messages emitted from the source")
	cmd.Flags().IntVar(&opts.Parallelism, "parallelism", 0, "The source parallelism factor")
	cmd.Flags().StringVarP(&opts.Archive, "archive", "a", "", "Path or URL to the source NAR package")
	cmd.Flags().StringVar(&opts.ClassName, "classname", "", "The source class name when archive is file or URL based")
	cmd.Flags().StringVar(&opts.SourceConfigFile, "source-config-file", "", "Path to a source config YAML file")
	cmd.Flags().Float64Var(&opts.CPU, "cpu", 0, "CPU cores per source instance")
	cmd.Flags().Int64Var(&opts.RAM, "ram", 0, "RAM per source instance")
	cmd.Flags().Int64Var(&opts.Disk, "disk", 0, "Disk per source instance")
	cmd.Flags().StringVar(&opts.SourceConfig, "source-config", "", "Source config as JSON")
	cmd.Flags().StringVar(&opts.BatchSourceConfig, "batch-source-config", "", "Batch source config as JSON")
	cmd.Flags().StringVar(&opts.CustomRuntimeOptions, "custom-runtime-options", "", "Custom runtime options")
	cmd.Flags().StringVar(&opts.Secrets, "secrets", "", "Source secrets as JSON")
	if includeConnection {
		cmd.Flags().StringVar(&opts.Connection, "connection", "", "Connection name used by the source")
		cmd.Flags().BoolVar(&opts.UseConnection, "use-connection", false, "Interactively select a connection for the source")
	}
	cmd.Flags().StringVar(&opts.SNServiceAccount, "sn-service-account", "", "Service account identity used by the source")
}

func buildSourceConfig(
	ctx context.Context,
	opts *sourceApplyOptions,
	isUpdate bool,
) (registry.RegistrySourceConfig, string, string, error) {
	cfg := registry.RegistrySourceConfig{}
	if strings.TrimSpace(opts.SourceConfigFile) != "" {
		data, err := os.ReadFile(opts.SourceConfigFile)
		if err != nil {
			return cfg, "", "", fmt.Errorf("failed to read source config file %q: %w", opts.SourceConfigFile, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, "", "", fmt.Errorf("failed to parse source config file %q: %w", opts.SourceConfigFile, err)
		}
	}

	if opts.Tenant != "" {
		cfg.Tenant = opts.Tenant
	}
	if opts.Namespace != "" {
		cfg.Namespace = opts.Namespace
	}
	if opts.Name != "" {
		cfg.Name = opts.Name
	}
	if opts.ClassName != "" {
		cfg.ClassName = opts.ClassName
	}
	if opts.DestinationTopicName != "" {
		cfg.TopicName = opts.DestinationTopicName
	}
	if opts.DeserializationClass != "" {
		cfg.SerdeClassName = opts.DeserializationClass
	}
	if opts.SchemaType != "" {
		cfg.SchemaType = opts.SchemaType
	}
	if opts.ProcessingGuarantees != "" {
		cfg.ProcessingGuarantees = opts.ProcessingGuarantees
	}
	if opts.Parallelism != 0 {
		cfg.Parallelism = opts.Parallelism
	}
	if opts.Archive != "" && opts.SourceType != "" {
		return cfg, "", "", fmt.Errorf("--archive and --source-type cannot be used together")
	}
	if opts.Archive != "" {
		cfg.Archive = opts.Archive
	}
	if opts.SourceType != "" {
		cfg.Archive = "builtin://" + opts.SourceType
		cfg.SourceType = opts.SourceType
	}
	if opts.CPU != 0 || opts.RAM != 0 || opts.Disk != 0 {
		if cfg.Resources == nil {
			cfg.Resources = &registry.Resources{}
		}
		if opts.CPU != 0 {
			cfg.Resources.CPU = opts.CPU
		}
		if opts.RAM != 0 {
			cfg.Resources.RAM = opts.RAM
		}
		if opts.Disk != 0 {
			cfg.Resources.Disk = opts.Disk
		}
	}
	if opts.SourceConfig != "" {
		parsed, err := parseInterfaceMapJSON(opts.SourceConfig)
		if err != nil {
			return cfg, "", "", fmt.Errorf("--source-config: %w", err)
		}
		cfg.Configs = parsed
	}
	if opts.ProducerConfig != "" {
		parsed, err := parseProducerConfigJSON(opts.ProducerConfig)
		if err != nil {
			return cfg, "", "", fmt.Errorf("--producer-config: %w", err)
		}
		cfg.ProducerConfig = parsed
	}
	if opts.BatchBuilder != "" {
		cfg.BatchBuilder = opts.BatchBuilder
	}
	if opts.LogTopic != "" {
		cfg.LogTopic = opts.LogTopic
	}
	if opts.BatchSourceConfig != "" {
		parsed, err := parseBatchSourceConfigJSON(opts.BatchSourceConfig)
		if err != nil {
			return cfg, "", "", fmt.Errorf("--batch-source-config: %w", err)
		}
		cfg.BatchSourceConfig = parsed
	}
	if opts.CustomRuntimeOptions != "" {
		cfg.CustomRuntimeOptions = opts.CustomRuntimeOptions
	}
	if opts.Secrets != "" {
		parsed, err := parseInterfaceMapJSON(opts.Secrets)
		if err != nil {
			return cfg, "", "", fmt.Errorf("--secrets: %w", err)
		}
		cfg.Secrets = parsed
	}

	if isUpdate {
		cfg.Connection = ""
	} else {
		connectionName, err := resolveConnectionSelection(ctx, opts.Connection, opts.UseConnection)
		if err != nil {
			return cfg, "", "", err
		}
		if connectionName != "" {
			cfg.Connection = connectionName
		}
		if strings.TrimSpace(cfg.Connection) == "" {
			return cfg, "", "", fmt.Errorf("--connection or --use-connection is required")
		}
	}
	if strings.TrimSpace(opts.SNServiceAccount) != "" {
		cfg.SNServiceAccount = strings.TrimSpace(opts.SNServiceAccount)
	}

	if cfg.Tenant == "" {
		cfg.Tenant = defaultPulsarTenant
	}
	if cfg.Namespace == "" {
		cfg.Namespace = defaultPulsarNamespace
	}
	if cfg.Name == "" {
		return cfg, "", "", fmt.Errorf("you must specify a name for the source")
	}
	if err := validateWorkspaceName("source", cfg.Name); err != nil {
		return cfg, "", "", err
	}
	if cfg.Parallelism <= 0 {
		cfg.Parallelism = 1
	}
	filePath, packageURL := splitPackageLocation(cfg.Archive)
	if !isUpdate && filePath == "" && packageURL == "" {
		return cfg, "", "", fmt.Errorf("source archive is required, specify --archive, --source-type, or a config file that defines archive")
	}

	return cfg, filePath, packageURL, nil
}

func buildSourceUpdateOptions(flagChanged bool, updateAuthData bool) *registry.UpdateOptionsImpl {
	if !flagChanged {
		return nil
	}
	return &registry.UpdateOptionsImpl{UpdateAuthData: updateAuthData}
}

func renderSourceConfig(writer io.Writer, output string, item registry.RegistrySourceConfig) error {
	return renderWorkspaceResource(writer, output, item, func(writer io.Writer) error {
		_, _ = fmt.Fprintf(writer, "Name: %s\n", item.Name)
		_, _ = fmt.Fprintf(writer, "Tenant: %s\n", item.Tenant)
		_, _ = fmt.Fprintf(writer, "Namespace: %s\n", item.Namespace)
		_, _ = fmt.Fprintf(writer, "ClassName: %s\n", item.ClassName)
		_, _ = fmt.Fprintf(writer, "Connection: %s\n", item.Connection)
		_, _ = fmt.Fprintf(writer, "SN Service Account: %s\n", item.SNServiceAccount)
		_, _ = fmt.Fprintf(writer, "Log Topic: %s\n", item.LogTopic)
		_, _ = fmt.Fprintf(writer, "Topic: %s\n", item.TopicName)
		_, _ = fmt.Fprintf(writer, "Archive: %s\n", item.Archive)
		_, _ = fmt.Fprintf(writer, "Parallelism: %d\n", item.Parallelism)
		return nil
	})
}

func renderSourceStatus(writer io.Writer, output string, status registry.SourceStatus) error {
	if err := validateWorkspaceOutput(output); err != nil {
		return err
	}

	switch output {
	case "json":
		return renderJSON(writer, status)
	case "yaml":
		return renderYAML(writer, status)
	default:
		_, _ = fmt.Fprintf(writer, "Instances: %d\n", status.NumInstances)
		_, _ = fmt.Fprintf(writer, "Running: %d\n", status.NumRunning)

		table := tablewriter.NewWriter(writer)
		table.SetHeader([]string{"Instance ID", "Running", "Restarts", "Worker", "Written", "Error"})
		for _, instance := range status.Instances {
			if instance == nil {
				continue
			}
			table.Append([]string{
				fmt.Sprintf("%d", instance.InstanceID),
				fmt.Sprintf("%t", instance.Status.Running),
				fmt.Sprintf("%d", instance.Status.NumRestarts),
				instance.Status.WorkerID,
				fmt.Sprintf("%d", instance.Status.NumWritten),
				instance.Status.Err,
			})
		}
		table.Render()
		return nil
	}
}

func parseBatchSourceConfigJSON(raw string) (*registry.BatchSourceConfig, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var value registry.BatchSourceConfig
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("invalid JSON for batch source config: %w", err)
	}
	return &value, nil
}
