// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

type sinksOptions struct {
	opts      *Options
	ioStreams IOStreams
}

type workspaceSinksClient interface {
	List(ctx context.Context) ([]string, error)
	Get(ctx context.Context, name string) (*registry.RegistrySinkConfig, error)
	Create(ctx context.Context, cfg registry.RegistrySinkConfig, filePath, packageURL string) error
	Update(ctx context.Context, name string, cfg registry.RegistrySinkConfig, filePath, packageURL string, updateOptions *registry.UpdateOptionsImpl) error
	Delete(ctx context.Context, name string) error
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Restart(ctx context.Context, name string) error
	StartInstance(ctx context.Context, name, instanceID string) error
	StopInstance(ctx context.Context, name, instanceID string) error
	RestartInstance(ctx context.Context, name, instanceID string) error
	Status(ctx context.Context, name string) (*registry.SinkStatus, error)
	InstanceStatus(ctx context.Context, name, instanceID string) (*registry.SinkInstanceStatusData, error)
}

var newWorkspaceSinksClient = func() (workspaceSinksClient, error) {
	client, err := newWorkspaceRegistryClient()
	if err != nil {
		return nil, err
	}
	return registry.NewSinksClient(client), nil
}

type sinkApplyOptions struct {
	Tenant                       string
	Namespace                    string
	Name                         string
	SinkType                     string
	CleanupSubscription          bool
	Inputs                       string
	TopicsPattern                string
	SubsName                     string
	SubsPosition                 string
	CustomSerdeInputs            string
	CustomSchemaInputs           string
	InputSpecs                   string
	MaxMessageRetries            int
	DeadLetterTopic              string
	ProcessingGuarantees         string
	RetainOrdering               bool
	RetainKeyOrdering            bool
	Parallelism                  int
	Archive                      string
	ClassName                    string
	SinkConfigFile               string
	SinkConfig                   string
	LogTopic                     string
	AutoAck                      bool
	TimeoutMs                    int64
	CPU                          float64
	RAM                          int64
	Disk                         int64
	NegativeAckRedeliveryDelayMs int64
	CustomRuntimeOptions         string
	Secrets                      string
	TransformFunction            string
	TransformFunctionClassName   string
	TransformFunctionConfig      string
	Connection                   string
	UseConnection                bool
	SNServiceAccount             string
	UpdateAuthData               bool
}

// NewCmdSinks creates workspace sink commands.
func NewCmdSinks(opts *Options) *cobra.Command {
	opts = setCurrentOptions(opts)
	o := &sinksOptions{
		opts:      opts,
		ioStreams: opts.IOStreams,
	}

	cmd := &cobra.Command{
		Use:   "sinks",
		Short: "Manage workspace sinks",
		Long: "Manage Pulsar IO Sinks through the workspace registry API. " +
			"Requires the cloud.sn.io extension group; not available on a self-hosted engine.",
		PersistentPreRunE: requireCloudExtension,
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}

	cmd.AddCommand(
		o.newListCommand(),
		o.newGetCommand(),
		o.newAvailableSinksCommand(),
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

func (o *sinksOptions) newListCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace sinks",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newSinksClient()
			if err != nil {
				return err
			}
			items, err := client.List(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list sinks: %w", err)
			}
			return renderNameList(o.ioStreams.Out, output, "Sink Name", items)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *sinksOptions) newGetCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "get [name]",
		Short: "Get a workspace sink",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newSinksClient()
			if err != nil {
				return err
			}
			item, err := client.Get(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get sink %q: %w", args[0], err)
			}
			return renderSinkConfig(o.ioStreams.Out, output, *item)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *sinksOptions) newAvailableSinksCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "available-sinks",
		Short: "List available built-in sink connectors",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceCatalogClient()
			if err != nil {
				return err
			}
			items, err := client.ListSinks(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list available sinks: %w", err)
			}
			return renderConnectorDefinitions(o.ioStreams.Out, output, "sink", items)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *sinksOptions) newConfigDefinitionCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "config-definition [name]",
		Short: "Get configuration definition for a built-in sink connector",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceCatalogClient()
			if err != nil {
				return err
			}
			items, err := client.GetSinkConfigDefinition(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get sink config definition %q: %w", args[0], err)
			}
			return renderConfigFieldDefinitions(o.ioStreams.Out, output, items)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *sinksOptions) newCreateCommand() *cobra.Command {
	opts := &sinkApplyOptions{CleanupSubscription: true}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a workspace sink",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, filePath, packageURL, err := buildSinkConfig(cmd.Context(), opts, false)
			if err != nil {
				return err
			}
			client, err := o.newSinksClient()
			if err != nil {
				return err
			}
			if err := client.Create(cmd.Context(), cfg, filePath, packageURL); err != nil {
				return fmt.Errorf("failed to create sink %q: %w", cfg.Name, err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Created sink %q successfully\n", cfg.Name)
			return nil
		},
	}
	addSinkApplyFlags(cmd, opts, true)
	return cmd
}

func (o *sinksOptions) newUpdateCommand() *cobra.Command {
	opts := &sinkApplyOptions{CleanupSubscription: true}
	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Update a workspace sink",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.Name) != "" && strings.TrimSpace(opts.Name) != args[0] {
				return fmt.Errorf("--name %q does not match requested sink %q", opts.Name, args[0])
			}
			opts.Name = args[0]
			cfg, filePath, packageURL, err := buildSinkConfig(cmd.Context(), opts, true)
			if err != nil {
				return err
			}
			client, err := o.newSinksClient()
			if err != nil {
				return err
			}
			current, err := client.Get(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get sink %q: %w", args[0], err)
			}
			if current == nil {
				return fmt.Errorf("failed to get sink %q: empty response", args[0])
			}
			applySinkUpdateImmutableFields(&cfg, *current)
			updateOptions := buildSinkUpdateOptions(cmd.Flags().Changed("update-auth-data"), opts.UpdateAuthData)
			if err := client.Update(cmd.Context(), args[0], cfg, filePath, packageURL, updateOptions); err != nil {
				return fmt.Errorf("failed to update sink %q: %w", args[0], err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Updated sink %q successfully\n", args[0])
			return nil
		},
	}
	addSinkApplyFlags(cmd, opts, false)
	cmd.Flags().BoolVar(&opts.UpdateAuthData, "update-auth-data", false, "Whether or not to update the auth data")
	_ = cmd.Flags().MarkDeprecated("auto-ack", "this value is immutable")
	_ = cmd.Flags().MarkDeprecated("processing-guarantees", "this value is immutable")
	_ = cmd.Flags().MarkDeprecated("retain-ordering", "this value is immutable")
	return cmd
}

func (o *sinksOptions) newDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a workspace sink",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := o.newSinksClient()
			if err != nil {
				return err
			}
			if err := client.Delete(cmd.Context(), args[0]); err != nil {
				return fmt.Errorf("failed to delete sink %q: %w", args[0], err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Deleted sink %q successfully\n", args[0])
			return nil
		},
	}
	return cmd
}

func (o *sinksOptions) newStartCommand() *cobra.Command {
	return o.newLifecycleCommand("start", "Start a workspace sink",
		func(ctx context.Context, client workspaceSinksClient, name string) error {
			return client.Start(ctx, name)
		},
		func(ctx context.Context, client workspaceSinksClient, name, instanceID string) error {
			return client.StartInstance(ctx, name, instanceID)
		})
}

func (o *sinksOptions) newStopCommand() *cobra.Command {
	return o.newLifecycleCommand("stop", "Stop a workspace sink",
		func(ctx context.Context, client workspaceSinksClient, name string) error {
			return client.Stop(ctx, name)
		},
		func(ctx context.Context, client workspaceSinksClient, name, instanceID string) error {
			return client.StopInstance(ctx, name, instanceID)
		})
}

func (o *sinksOptions) newRestartCommand() *cobra.Command {
	return o.newLifecycleCommand("restart", "Restart a workspace sink",
		func(ctx context.Context, client workspaceSinksClient, name string) error {
			return client.Restart(ctx, name)
		},
		func(ctx context.Context, client workspaceSinksClient, name, instanceID string) error {
			return client.RestartInstance(ctx, name, instanceID)
		})
}

func (o *sinksOptions) newLifecycleCommand(
	use string,
	short string,
	runAll func(context.Context, workspaceSinksClient, string) error,
	runInstance func(context.Context, workspaceSinksClient, string, string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use + " [name] [instance-id]",
		Short: short,
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := o.newSinksClient()
			if err != nil {
				return err
			}
			if len(args) == 2 {
				err = runInstance(cmd.Context(), client, args[0], args[1])
			} else {
				err = runAll(cmd.Context(), client, args[0])
			}
			if err != nil {
				return fmt.Errorf("failed to %s sink %q: %w", use, args[0], err)
			}
			if len(args) == 2 {
				_, _ = fmt.Fprintf(o.ioStreams.Out, "%s sink %q instance %q successfully\n", lifecycleActionLabel(use), args[0], args[1])
			} else {
				_, _ = fmt.Fprintf(o.ioStreams.Out, "%s sink %q successfully\n", lifecycleActionLabel(use), args[0])
			}
			return nil
		},
	}
	return cmd
}

func (o *sinksOptions) newStatusCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "status [name] [instance-id]",
		Short: "Get workspace sink status",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newSinksClient()
			if err != nil {
				return err
			}
			if len(args) == 2 {
				status, err := client.InstanceStatus(cmd.Context(), args[0], args[1])
				if err != nil {
					return fmt.Errorf("failed to get sink %q instance %q status: %w", args[0], args[1], err)
				}
				return renderJSONOrText(o.ioStreams.Out, output, status)
			}
			status, err := client.Status(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get sink status %q: %w", args[0], err)
			}
			return renderSinkStatus(o.ioStreams.Out, output, *status)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *sinksOptions) newSinksClient() (workspaceSinksClient, error) {
	return newWorkspaceSinksClient()
}

func addSinkApplyFlags(cmd *cobra.Command, opts *sinkApplyOptions, includeConnection bool) {
	cmd.Flags().StringVar(&opts.Tenant, "tenant", "", "The tenant of the sink")
	cmd.Flags().StringVar(&opts.Namespace, "namespace", "", "The namespace of the sink")
	cmd.Flags().StringVar(&opts.Name, "name", "", "The name of the sink")
	cmd.Flags().StringVarP(&opts.SinkType, "sink-type", "t", "", "The built-in sink type")
	cmd.Flags().BoolVar(&opts.CleanupSubscription, "cleanup-subscription", true, "Whether delete the subscription when the sink is deleted")
	cmd.Flags().StringVarP(&opts.Inputs, "inputs", "i", "", "Comma-separated input topics")
	cmd.Flags().StringVar(&opts.TopicsPattern, "topics-pattern", "", "Topic pattern to consume from")
	cmd.Flags().StringVar(&opts.SubsName, "subs-name", "", "Specific subscription name for input-topic consumer")
	cmd.Flags().StringVar(&opts.SubsPosition, "subs-position", "", "Subscription position to consume from")
	cmd.Flags().StringVar(&opts.CustomSerdeInputs, "custom-serde-inputs", "", "Map of input topics to SerDe class names as JSON")
	cmd.Flags().StringVar(&opts.CustomSchemaInputs, "custom-schema-inputs", "", "Map of input topics to schema type as JSON")
	cmd.Flags().StringVar(&opts.InputSpecs, "input-specs", "", "Map of inputs to custom configuration as JSON")
	cmd.Flags().IntVar(&opts.MaxMessageRetries, "max-redeliver-count", 0, "Maximum redelivery count before dead letter")
	cmd.Flags().StringVar(&opts.DeadLetterTopic, "dead-letter-topic", "", "Dead letter topic")
	cmd.Flags().StringVar(&opts.ProcessingGuarantees, "processing-guarantees", "", "Processing guarantees applied to the sink")
	cmd.Flags().BoolVar(&opts.RetainOrdering, "retain-ordering", false, "Consume and process messages in order")
	cmd.Flags().BoolVar(&opts.RetainKeyOrdering, "retain-key-ordering", false, "Consume and process messages in key order")
	cmd.Flags().IntVar(&opts.Parallelism, "parallelism", 0, "Parallelism factor")
	cmd.Flags().StringVarP(&opts.Archive, "archive", "a", "", "Path or URL to the sink NAR package")
	cmd.Flags().StringVar(&opts.ClassName, "classname", "", "The sink class name when archive is file or URL based")
	cmd.Flags().StringVar(&opts.SinkConfigFile, "sink-config-file", "", "Path to a sink config YAML file")
	cmd.Flags().StringVar(&opts.SinkConfig, "sink-config", "", "Sink config as JSON")
	cmd.Flags().StringVar(&opts.LogTopic, "log-topic", "", "Topic for sink logs")
	cmd.Flags().BoolVar(&opts.AutoAck, "auto-ack", false, "Enable auto ack")
	cmd.Flags().Int64Var(&opts.TimeoutMs, "timeout-ms", 0, "Sink timeout in milliseconds")
	cmd.Flags().Float64Var(&opts.CPU, "cpu", 0, "CPU cores per sink instance")
	cmd.Flags().Int64Var(&opts.RAM, "ram", 0, "RAM per sink instance")
	cmd.Flags().Int64Var(&opts.Disk, "disk", 0, "Disk per sink instance")
	cmd.Flags().Int64Var(&opts.NegativeAckRedeliveryDelayMs, "negative-ack-redelivery-delay-ms", 0, "Negative ack redelivery delay in ms")
	cmd.Flags().StringVar(&opts.CustomRuntimeOptions, "custom-runtime-options", "", "Custom runtime options")
	cmd.Flags().StringVar(&opts.Secrets, "secrets", "", "Sink secrets as JSON")
	cmd.Flags().StringVar(&opts.TransformFunction, "transform-function", "", "Transform function name")
	cmd.Flags().StringVar(&opts.TransformFunctionClassName, "transform-function-classname", "", "Transform function class name")
	cmd.Flags().StringVar(&opts.TransformFunctionConfig, "transform-function-config", "", "Transform function config")
	if includeConnection {
		cmd.Flags().StringVar(&opts.Connection, "connection", "", "Connection name used by the sink")
		cmd.Flags().BoolVar(&opts.UseConnection, "use-connection", false, "Interactively select a connection for the sink")
	}
	cmd.Flags().StringVar(&opts.SNServiceAccount, "sn-service-account", "", "Service account identity used by the sink")
}

func buildSinkConfig(
	ctx context.Context,
	opts *sinkApplyOptions,
	isUpdate bool,
) (registry.RegistrySinkConfig, string, string, error) {
	cfg := registry.RegistrySinkConfig{}
	if strings.TrimSpace(opts.SinkConfigFile) != "" {
		data, err := os.ReadFile(opts.SinkConfigFile)
		if err != nil {
			return cfg, "", "", fmt.Errorf("failed to read sink config file %q: %w", opts.SinkConfigFile, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, "", "", fmt.Errorf("failed to parse sink config file %q: %w", opts.SinkConfigFile, err)
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
	if opts.ProcessingGuarantees != "" {
		cfg.ProcessingGuarantees = opts.ProcessingGuarantees
	}
	if opts.RetainOrdering {
		cfg.RetainOrdering = true
	}
	if opts.RetainKeyOrdering {
		cfg.RetainKeyOrdering = true
	}
	if opts.Inputs != "" {
		cfg.Inputs = splitCSV(opts.Inputs)
	}
	if opts.CustomSerdeInputs != "" {
		parsed, err := parseStringMapJSON(opts.CustomSerdeInputs)
		if err != nil {
			return cfg, "", "", fmt.Errorf("--custom-serde-inputs: %w", err)
		}
		cfg.TopicToSerdeClassName = parsed
	}
	if opts.CustomSchemaInputs != "" {
		parsed, err := parseStringMapJSON(opts.CustomSchemaInputs)
		if err != nil {
			return cfg, "", "", fmt.Errorf("--custom-schema-inputs: %w", err)
		}
		cfg.TopicToSchemaType = parsed
	}
	if opts.SubsName != "" {
		cfg.SourceSubscriptionName = opts.SubsName
	}
	if opts.SubsPosition != "" {
		cfg.SourceSubscriptionPosition = opts.SubsPosition
	}
	if opts.TopicsPattern != "" {
		cfg.TopicsPattern = &opts.TopicsPattern
	}
	if opts.Parallelism != 0 {
		cfg.Parallelism = opts.Parallelism
	}
	if opts.Archive != "" && opts.SinkType != "" {
		return cfg, "", "", fmt.Errorf("--archive and --sink-type cannot be used together")
	}
	if opts.Archive != "" {
		cfg.Archive = opts.Archive
	}
	if opts.SinkType != "" {
		cfg.Archive = "builtin://" + opts.SinkType
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
	if opts.SinkConfig != "" {
		parsed, err := parseInterfaceMapJSON(opts.SinkConfig)
		if err != nil {
			return cfg, "", "", fmt.Errorf("--sink-config: %w", err)
		}
		cfg.Configs = parsed
	}
	if opts.LogTopic != "" {
		cfg.LogTopic = opts.LogTopic
	}
	if opts.AutoAck {
		cfg.AutoAck = true
	}
	if opts.TimeoutMs != 0 {
		cfg.TimeoutMs = &opts.TimeoutMs
	}
	cfg.CleanupSubscription = opts.CleanupSubscription
	if opts.InputSpecs != "" {
		parsed, err := parseConsumerConfigMapJSON(opts.InputSpecs)
		if err != nil {
			return cfg, "", "", fmt.Errorf("--input-specs: %w", err)
		}
		cfg.InputSpecs = parsed
	}
	if opts.MaxMessageRetries != 0 {
		cfg.MaxMessageRetries = opts.MaxMessageRetries
	}
	if opts.DeadLetterTopic != "" {
		cfg.DeadLetterTopic = opts.DeadLetterTopic
	}
	if opts.NegativeAckRedeliveryDelayMs != 0 {
		cfg.NegativeAckRedeliveryDelayMs = opts.NegativeAckRedeliveryDelayMs
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
	if opts.TransformFunction != "" {
		cfg.TransformFunction = opts.TransformFunction
	}
	if opts.TransformFunctionClassName != "" {
		cfg.TransformFunctionClassName = opts.TransformFunctionClassName
	}
	if opts.TransformFunctionConfig != "" {
		cfg.TransformFunctionConfig = opts.TransformFunctionConfig
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
		return cfg, "", "", fmt.Errorf("you must specify a name for the sink")
	}
	if err := validateWorkspaceName("sink", cfg.Name); err != nil {
		return cfg, "", "", err
	}
	if cfg.Parallelism <= 0 {
		cfg.Parallelism = 1
	}
	filePath, packageURL := splitPackageLocation(cfg.Archive)
	if !isUpdate && filePath == "" && packageURL == "" {
		return cfg, "", "", fmt.Errorf("sink archive is required, specify --archive, --sink-type, or a config file that defines archive")
	}

	return cfg, filePath, packageURL, nil
}

func applySinkUpdateImmutableFields(cfg *registry.RegistrySinkConfig, current registry.RegistrySinkConfig) {
	cfg.AutoAck = current.AutoAck
	cfg.ProcessingGuarantees = current.ProcessingGuarantees
	cfg.RetainOrdering = current.RetainOrdering
}

func buildSinkUpdateOptions(flagChanged bool, updateAuthData bool) *registry.UpdateOptionsImpl {
	if !flagChanged {
		return nil
	}
	return &registry.UpdateOptionsImpl{UpdateAuthData: updateAuthData}
}

func renderSinkConfig(writer io.Writer, output string, item registry.RegistrySinkConfig) error {
	return renderWorkspaceResource(writer, output, item, func(writer io.Writer) error {
		_, _ = fmt.Fprintf(writer, "Name: %s\n", item.Name)
		_, _ = fmt.Fprintf(writer, "Tenant: %s\n", item.Tenant)
		_, _ = fmt.Fprintf(writer, "Namespace: %s\n", item.Namespace)
		_, _ = fmt.Fprintf(writer, "ClassName: %s\n", item.ClassName)
		_, _ = fmt.Fprintf(writer, "Connection: %s\n", item.Connection)
		_, _ = fmt.Fprintf(writer, "SN Service Account: %s\n", item.SNServiceAccount)
		_, _ = fmt.Fprintf(writer, "Log Topic: %s\n", item.LogTopic)
		_, _ = fmt.Fprintf(writer, "Archive: %s\n", item.Archive)
		_, _ = fmt.Fprintf(writer, "Parallelism: %d\n", item.Parallelism)
		if len(item.Inputs) > 0 {
			_, _ = fmt.Fprintf(writer, "Inputs: %s\n", strings.Join(item.Inputs, ","))
		}
		return nil
	})
}

func renderSinkStatus(writer io.Writer, output string, status registry.SinkStatus) error {
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
				fmt.Sprintf("%d", instance.Status.NumWrittenToSink),
				instance.Status.Err,
			})
		}
		table.Render()
		return nil
	}
}
