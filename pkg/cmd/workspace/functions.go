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

type functionsOptions struct {
	opts      *Options
	ioStreams IOStreams
}

type functionApplyOptions struct {
	FQFN                                string
	Tenant                              string
	Namespace                           string
	Name                                string
	ClassName                           string
	FunctionType                        string
	CleanupSubscription                 bool
	Jar                                 string
	Py                                  string
	Go                                  string
	Inputs                              string
	TopicsPattern                       string
	Output                              string
	ProducerConfig                      string
	LogTopic                            string
	SchemaType                          string
	CustomSerdeInputs                   string
	CustomSchemaInputs                  string
	CustomSchemaOutputs                 string
	InputSpecs                          string
	InputTypeClassName                  string
	OutputSerdeClassName                string
	OutputTypeClassName                 string
	FunctionConfigFile                  string
	ProcessingGuarantees                string
	UserConfig                          string
	RetainOrdering                      bool
	RetainKeyOrdering                   bool
	BatchBuilder                        string
	ForwardSourceMessageProperty        bool
	ForwardSourceMessagePropertyChanged bool
	SubsName                            string
	SubsPosition                        string
	SkipToLatest                        bool
	Parallelism                         int
	CPU                                 float64
	RAM                                 int64
	Disk                                int64
	WindowLengthCount                   int
	WindowLengthDurationMs              int64
	SlidingIntervalCount                int
	SlidingIntervalDurationMs           int64
	TimeoutMs                           int64
	AutoAck                             bool
	AutoAckChanged                      bool
	MaxMessageRetries                   int
	DeadLetterTopic                     string
	CustomRuntimeOptions                string
	Secrets                             string
	CleanupSubscriptionChanged          bool
	Connection                          string
	UseConnection                       bool
	SNServiceAccount                    string
	UpdateAuthData                      bool
}

type loadedFunctionConfigFile struct {
	Config                            registry.RegistryFunctionConfig
	ForwardSourceMessagePropertyFound bool
	CleanupSubscriptionFound          bool
	AutoAckFound                      bool
}

// NewCmdFunctions creates workspace function commands.
func NewCmdFunctions(opts *Options) *cobra.Command {
	opts = setCurrentOptions(opts)
	o := &functionsOptions{
		opts:      opts,
		ioStreams: opts.IOStreams,
	}

	cmd := &cobra.Command{
		Use:   "functions",
		Short: "Manage workspace functions",
		Long: "Manage Pulsar Functions through the workspace registry API. " +
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
		o.newUpdateCommand(),
		o.newDeleteCommand(),
		o.newStartCommand(),
		o.newStopCommand(),
		o.newRestartCommand(),
		o.newStatusCommand(),
		o.newStatsCommand(),
		o.newTriggerCommand(),
		o.newStateCommand(),
	)

	return cmd
}

func (o *functionsOptions) newListCommand() *cobra.Command {
	output := "text"

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace functions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newFunctionsClient()
			if err != nil {
				return err
			}
			items, err := client.List(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list functions: %w", err)
			}
			return renderNameList(o.ioStreams.Out, output, "Function Name", items)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *functionsOptions) newGetCommand() *cobra.Command {
	output := "text"

	cmd := &cobra.Command{
		Use:   "get [name]",
		Short: "Get a workspace function",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newFunctionsClient()
			if err != nil {
				return err
			}
			item, err := client.Get(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get function %q: %w", args[0], err)
			}
			return renderFunctionConfig(o.ioStreams.Out, output, *item)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *functionsOptions) newCreateCommand() *cobra.Command {
	opts := newFunctionApplyOptions()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a workspace function",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.ForwardSourceMessagePropertyChanged = cmd.Flags().Changed("forward-source-message-property")
			opts.CleanupSubscriptionChanged = cmd.Flags().Changed("cleanup-subscription")
			opts.AutoAckChanged = cmd.Flags().Changed("auto-ack")
			cfg, filePath, packageURL, err := buildFunctionConfig(cmd.Context(), opts, false)
			if err != nil {
				return err
			}
			client, err := o.newFunctionsClient()
			if err != nil {
				return err
			}
			if err := client.Create(cmd.Context(), cfg, filePath, packageURL); err != nil {
				return fmt.Errorf("failed to create function %q: %w", cfg.Name, err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Created function %q successfully\n", cfg.Name)
			return nil
		},
	}

	addFunctionApplyFlags(cmd, opts, true)
	return cmd
}

func (o *functionsOptions) newUpdateCommand() *cobra.Command {
	opts := newFunctionApplyOptions()

	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Update a workspace function",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.Name) != "" && strings.TrimSpace(opts.Name) != args[0] {
				return fmt.Errorf("--name %q does not match requested function %q", opts.Name, args[0])
			}
			opts.Name = args[0]
			opts.ForwardSourceMessagePropertyChanged = cmd.Flags().Changed("forward-source-message-property")
			opts.CleanupSubscriptionChanged = cmd.Flags().Changed("cleanup-subscription")
			opts.AutoAckChanged = cmd.Flags().Changed("auto-ack")
			cfg, filePath, packageURL, err := buildFunctionConfig(cmd.Context(), opts, true)
			if err != nil {
				return err
			}
			client, err := o.newFunctionsClient()
			if err != nil {
				return err
			}
			updateOptions := buildFunctionUpdateOptions(cmd.Flags().Changed("update-auth-data"), opts.UpdateAuthData)
			if err := client.Update(cmd.Context(), args[0], cfg, filePath, packageURL, updateOptions); err != nil {
				return fmt.Errorf("failed to update function %q: %w", args[0], err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Updated function %q successfully\n", args[0])
			return nil
		},
	}

	addFunctionApplyFlags(cmd, opts, false)
	cmd.Flags().BoolVar(&opts.UpdateAuthData, "update-auth-data", false, "Whether or not to update the auth data")
	return cmd
}

func (o *functionsOptions) newDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a workspace function",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := o.newFunctionsClient()
			if err != nil {
				return err
			}
			if err := client.Delete(cmd.Context(), args[0]); err != nil {
				return fmt.Errorf("failed to delete function %q: %w", args[0], err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Deleted function %q successfully\n", args[0])
			return nil
		},
	}

	return cmd
}

func (o *functionsOptions) newStartCommand() *cobra.Command {
	return o.newLifecycleCommand("start", "Start a workspace function",
		func(ctx context.Context, client *registry.FunctionsClient, name string) error {
			return client.Start(ctx, name)
		},
		func(ctx context.Context, client *registry.FunctionsClient, name, instanceID string) error {
			return client.StartInstance(ctx, name, instanceID)
		})
}

func (o *functionsOptions) newStopCommand() *cobra.Command {
	return o.newLifecycleCommand("stop", "Stop a workspace function",
		func(ctx context.Context, client *registry.FunctionsClient, name string) error {
			return client.Stop(ctx, name)
		},
		func(ctx context.Context, client *registry.FunctionsClient, name, instanceID string) error {
			return client.StopInstance(ctx, name, instanceID)
		})
}

func (o *functionsOptions) newRestartCommand() *cobra.Command {
	return o.newLifecycleCommand("restart", "Restart a workspace function",
		func(ctx context.Context, client *registry.FunctionsClient, name string) error {
			return client.Restart(ctx, name)
		},
		func(ctx context.Context, client *registry.FunctionsClient, name, instanceID string) error {
			return client.RestartInstance(ctx, name, instanceID)
		})
}

func (o *functionsOptions) newLifecycleCommand(
	use string,
	short string,
	runAll func(context.Context, *registry.FunctionsClient, string) error,
	runInstance func(context.Context, *registry.FunctionsClient, string, string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use + " [name] [instance-id]",
		Short: short,
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := o.newFunctionsClient()
			if err != nil {
				return err
			}
			if len(args) == 2 {
				err = runInstance(cmd.Context(), client, args[0], args[1])
			} else {
				err = runAll(cmd.Context(), client, args[0])
			}
			if err != nil {
				return fmt.Errorf("failed to %s function %q: %w", use, args[0], err)
			}
			if len(args) == 2 {
				_, _ = fmt.Fprintf(o.ioStreams.Out, "%s function %q instance %q successfully\n", lifecycleActionLabel(use), args[0], args[1])
			} else {
				_, _ = fmt.Fprintf(o.ioStreams.Out, "%s function %q successfully\n", lifecycleActionLabel(use), args[0])
			}
			return nil
		},
	}
	return cmd
}

func (o *functionsOptions) newStatusCommand() *cobra.Command {
	output := "text"

	cmd := &cobra.Command{
		Use:   "status [name] [instance-id]",
		Short: "Get workspace function status",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newFunctionsClient()
			if err != nil {
				return err
			}
			if len(args) == 2 {
				status, err := client.InstanceStatus(cmd.Context(), args[0], args[1])
				if err != nil {
					return fmt.Errorf("failed to get function %q instance %q status: %w", args[0], args[1], err)
				}
				return renderJSONOrText(o.ioStreams.Out, output, status)
			}
			status, err := client.Status(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get function status %q: %w", args[0], err)
			}
			return renderFunctionStatus(o.ioStreams.Out, output, *status)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *functionsOptions) newStatsCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "stats [name] [instance-id]",
		Short: "Get workspace function statistics",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newFunctionsClient()
			if err != nil {
				return err
			}
			if len(args) == 2 {
				stats, err := client.InstanceStats(cmd.Context(), args[0], args[1])
				if err != nil {
					return fmt.Errorf("failed to get function %q instance %q stats: %w", args[0], args[1], err)
				}
				return renderJSONOrText(o.ioStreams.Out, output, stats)
			}
			stats, err := client.Stats(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get function stats %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, stats)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *functionsOptions) newTriggerCommand() *cobra.Command {
	var data, dataFile, topic string
	cmd := &cobra.Command{
		Use:   "trigger [name]",
		Short: "Trigger a workspace function",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if (data == "") == (dataFile == "") {
				return fmt.Errorf("exactly one of --data or --data-file is required")
			}
			client, err := o.newFunctionsClient()
			if err != nil {
				return err
			}
			result, err := client.Trigger(cmd.Context(), args[0], data, dataFile, topic)
			if err != nil {
				return fmt.Errorf("failed to trigger function %q: %w", args[0], err)
			}
			_, err = fmt.Fprintln(o.ioStreams.Out, result)
			return err
		},
	}
	cmd.Flags().StringVar(&data, "data", "", "Inline function input data")
	cmd.Flags().StringVar(&dataFile, "data-file", "", "File containing function input data")
	cmd.Flags().StringVar(&topic, "topic", "", "Input topic to use for the trigger")
	return cmd
}

func (o *functionsOptions) newStateCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "state", Short: "Manage workspace function state"}
	cmd.AddCommand(o.newGetStateCommand(), o.newPutStateCommand())
	return cmd
}

func (o *functionsOptions) newGetStateCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "get [name] [key]",
		Short: "Get a workspace function state value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := o.newFunctionsClient()
			if err != nil {
				return err
			}
			state, err := client.GetState(cmd.Context(), args[0], args[1])
			if err != nil {
				return fmt.Errorf("failed to get function %q state %q: %w", args[0], args[1], err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, state)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *functionsOptions) newPutStateCommand() *cobra.Command {
	var stateJSON, stateFile string
	cmd := &cobra.Command{
		Use:   "put [name] [key]",
		Short: "Update a workspace function state value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			state, err := loadFunctionState(stateJSON, stateFile, args[1])
			if err != nil {
				return err
			}
			client, err := o.newFunctionsClient()
			if err != nil {
				return err
			}
			if err := client.PutState(cmd.Context(), args[0], args[1], state); err != nil {
				return fmt.Errorf("failed to update function %q state %q: %w", args[0], args[1], err)
			}
			_, _ = fmt.Fprintf(o.ioStreams.Out, "Updated function %q state %q successfully\n", args[0], args[1])
			return nil
		},
	}
	cmd.Flags().StringVar(&stateJSON, "state-json", "", "Function state as a JSON object")
	cmd.Flags().StringVar(&stateFile, "state-file", "", "Path to a JSON or YAML function state file")
	return cmd
}

func (o *functionsOptions) newFunctionsClient() (*registry.FunctionsClient, error) {
	client, err := newWorkspaceRegistryClient()
	if err != nil {
		return nil, err
	}
	return registry.NewFunctionsClient(client), nil
}

func newFunctionApplyOptions() *functionApplyOptions {
	return &functionApplyOptions{
		CleanupSubscription:          true,
		ForwardSourceMessageProperty: true,
		AutoAck:                      true,
	}
}

func loadFunctionState(stateJSON, stateFile, key string) (registry.FunctionState, error) {
	stateJSON = strings.TrimSpace(stateJSON)
	stateFile = strings.TrimSpace(stateFile)
	if (stateJSON == "") == (stateFile == "") {
		return registry.FunctionState{}, fmt.Errorf("exactly one of --state-json or --state-file is required")
	}

	var (
		state registry.FunctionState
		err   error
	)
	if stateJSON != "" {
		err = json.Unmarshal([]byte(stateJSON), &state)
		if err != nil {
			return registry.FunctionState{}, fmt.Errorf("failed to parse --state-json: %w", err)
		}
	} else {
		data, readErr := os.ReadFile(stateFile)
		if readErr != nil {
			return registry.FunctionState{}, fmt.Errorf("failed to read --state-file %q: %w", stateFile, readErr)
		}
		if err = yaml.Unmarshal(data, &state); err != nil {
			return registry.FunctionState{}, fmt.Errorf("failed to parse --state-file %q: %w", stateFile, err)
		}
	}

	if state.Key != "" && state.Key != key {
		return registry.FunctionState{}, fmt.Errorf("state key %q does not match requested key %q", state.Key, key)
	}
	state.Key = key
	return state, nil
}

func addFunctionApplyFlags(cmd *cobra.Command, opts *functionApplyOptions, includeConnection bool) {
	cmd.Flags().StringVar(&opts.FQFN, "fqfn", "", "The Fully Qualified Function Name (tenant/namespace/name)")
	cmd.Flags().StringVar(&opts.Tenant, "tenant", "", "The tenant of the function")
	cmd.Flags().StringVar(&opts.Namespace, "namespace", "", "The namespace of the function")
	cmd.Flags().StringVar(&opts.Name, "name", "", "The name of the function")
	cmd.Flags().StringVar(&opts.ClassName, "classname", "", "The class name of the function")
	cmd.Flags().StringVarP(&opts.FunctionType, "function-type", "t", "", "The built-in function type")
	cmd.Flags().BoolVar(&opts.CleanupSubscription, "cleanup-subscription", true, "Whether delete the subscription when the function is deleted")
	cmd.Flags().StringVar(&opts.Jar, "jar", "", "Path or URL to the function JAR package")
	cmd.Flags().StringVar(&opts.Py, "py", "", "Path or URL to the function Python package")
	cmd.Flags().StringVar(&opts.Go, "go", "", "Path or URL to the function Go binary")
	cmd.Flags().StringVarP(&opts.Inputs, "inputs", "i", "", "Comma-separated input topics")
	cmd.Flags().StringVar(&opts.TopicsPattern, "topics-pattern", "", "Topic pattern to consume from")
	cmd.Flags().StringVarP(&opts.Output, "output", "o", "", "The output topic of the function")
	cmd.Flags().StringVar(&opts.ProducerConfig, "producer-config", "", "Custom producer configuration as JSON")
	cmd.Flags().StringVar(&opts.LogTopic, "log-topic", "", "Topic for function logs")
	cmd.Flags().StringVar(&opts.SchemaType, "schema-type", "", "Builtin schema type or output schema class name")
	cmd.Flags().StringVar(&opts.CustomSerdeInputs, "custom-serde-inputs", "", "Map of input topics to SerDe class names as JSON")
	cmd.Flags().StringVar(&opts.CustomSchemaInputs, "custom-schema-inputs", "", "Map of input topics to schema class names as JSON")
	cmd.Flags().StringVar(&opts.CustomSchemaOutputs, "custom-schema-outputs", "", "Map of output topics to schema properties as JSON")
	cmd.Flags().StringVar(&opts.InputSpecs, "input-specs", "", "Map of inputs to custom consumer configuration as JSON")
	cmd.Flags().StringVar(&opts.InputTypeClassName, "input-type-class-name", "", "The input type class name")
	cmd.Flags().StringVar(&opts.OutputSerdeClassName, "output-serde-classname", "", "The output SerDe class name")
	cmd.Flags().StringVar(&opts.OutputTypeClassName, "output-type-class-name", "", "The output type class name")
	cmd.Flags().StringVar(&opts.FunctionConfigFile, "function-config-file", "", "Path to a function config YAML file")
	cmd.Flags().StringVar(&opts.ProcessingGuarantees, "processing-guarantees", "", "Processing guarantees for the function")
	cmd.Flags().StringVar(&opts.UserConfig, "user-config", "", "User-defined config as JSON")
	cmd.Flags().BoolVar(&opts.RetainOrdering, "retain-ordering", false, "Consume and process messages in order")
	cmd.Flags().BoolVar(&opts.RetainKeyOrdering, "retain-key-ordering", false, "Consume and process messages in key order")
	cmd.Flags().StringVar(&opts.BatchBuilder, "batch-builder", "", "Batch builder strategy")
	cmd.Flags().BoolVar(&opts.ForwardSourceMessageProperty, "forward-source-message-property", true, "Forward input message properties to output topic")
	cmd.Flags().StringVar(&opts.SubsName, "subs-name", "", "Specific subscription name for input-topic consumer")
	cmd.Flags().StringVar(&opts.SubsPosition, "subs-position", "", "Subscription position to consume from")
	cmd.Flags().BoolVar(&opts.SkipToLatest, "skip-to-latest", false, "Skip to the latest message upon function instance restart")
	cmd.Flags().IntVar(&opts.Parallelism, "parallelism", 0, "Parallelism factor")
	cmd.Flags().Float64Var(&opts.CPU, "cpu", 0, "CPU cores per function instance")
	cmd.Flags().Int64Var(&opts.RAM, "ram", 0, "RAM per function instance")
	cmd.Flags().Int64Var(&opts.Disk, "disk", 0, "Disk per function instance")
	cmd.Flags().IntVar(&opts.WindowLengthCount, "window-length-count", 0, "The number of messages per window")
	cmd.Flags().Int64Var(&opts.WindowLengthDurationMs, "window-length-duration-ms", 0, "The time duration of the window in milliseconds")
	cmd.Flags().IntVar(&opts.SlidingIntervalCount, "sliding-interval-count", 0, "The number of messages after which the window slides")
	cmd.Flags().Int64Var(&opts.SlidingIntervalDurationMs, "sliding-interval-duration-ms", 0, "The time duration after which the window slides")
	cmd.Flags().Int64Var(&opts.TimeoutMs, "timeout-ms", 0, "Function timeout in milliseconds")
	cmd.Flags().BoolVar(&opts.AutoAck, "auto-ack", true, "Enable auto ack")
	cmd.Flags().IntVar(&opts.MaxMessageRetries, "max-message-retries", 0, "Max message retries before sending to dead letter topic")
	cmd.Flags().StringVar(&opts.DeadLetterTopic, "dead-letter-topic", "", "Dead letter topic")
	cmd.Flags().StringVar(&opts.CustomRuntimeOptions, "custom-runtime-options", "", "Custom runtime options")
	cmd.Flags().StringVar(&opts.Secrets, "secrets", "", "Function secrets as JSON")
	if includeConnection {
		cmd.Flags().StringVar(&opts.Connection, "connection", "", "Connection name used by the function")
		cmd.Flags().BoolVar(&opts.UseConnection, "use-connection", false, "Interactively select a connection for the function")
	}
	cmd.Flags().StringVar(&opts.SNServiceAccount, "sn-service-account", "", "Service account identity used by the function")
}

func buildFunctionConfig(
	ctx context.Context,
	opts *functionApplyOptions,
	isUpdate bool,
) (registry.RegistryFunctionConfig, string, string, error) {
	cfg := registry.RegistryFunctionConfig{}
	loadedConfig := &loadedFunctionConfigFile{}
	if strings.TrimSpace(opts.FunctionConfigFile) != "" {
		loaded, err := loadFunctionConfigFile(opts.FunctionConfigFile)
		if err != nil {
			return cfg, "", "", err
		}
		loadedConfig = loaded
		cfg = loaded.Config
	}

	if err := validateFunctionPackageSelectors(cfg, opts); err != nil {
		return cfg, "", "", err
	}
	if err := applyFunctionIdentity(&cfg, opts); err != nil {
		return cfg, "", "", err
	}
	if err := applyFunctionFlags(&cfg, opts); err != nil {
		return cfg, "", "", err
	}
	cfg.AutoAck = resolveFunctionBool(
		cfg.AutoAck,
		loadedConfig.AutoAckFound,
		opts.AutoAckChanged,
		opts.AutoAck,
		true,
	)
	cfg.ForwardSourceMessageProperty = resolveFunctionBool(
		cfg.ForwardSourceMessageProperty,
		loadedConfig.ForwardSourceMessagePropertyFound,
		opts.ForwardSourceMessagePropertyChanged,
		opts.ForwardSourceMessageProperty,
		true,
	)
	cfg.CleanupSubscription = resolveFunctionBool(
		cfg.CleanupSubscription,
		loadedConfig.CleanupSubscriptionFound,
		opts.CleanupSubscriptionChanged,
		opts.CleanupSubscription,
		true,
	)

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
		return cfg, "", "", fmt.Errorf("you must specify a name for the function or a Fully Qualified Function Name (FQFN)")
	}
	if err := validateWorkspaceName("function", cfg.Name); err != nil {
		return cfg, "", "", err
	}
	if cfg.Parallelism <= 0 {
		cfg.Parallelism = 1
	}
	if err := validateFunctionPackageFields(cfg); err != nil {
		return cfg, "", "", err
	}

	filePath, packageURL := splitFunctionPackageLocation(cfg)
	if !isUpdate && filePath == "" && packageURL == "" {
		return cfg, "", "", fmt.Errorf("function package is required, specify one of --jar, --py, --go, --function-type, or a config file that defines a package")
	}

	return cfg, filePath, packageURL, nil
}

func loadFunctionConfigFile(path string) (*loadedFunctionConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read function config file %q: %w", path, err)
	}

	var cfg registry.RegistryFunctionConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse function config file %q: %w", path, err)
	}

	var presence struct {
		ForwardSourceMessageProperty *bool `json:"forwardSourceMessageProperty,omitempty" yaml:"forwardSourceMessageProperty,omitempty"`
		CleanupSubscription          *bool `json:"cleanupSubscription,omitempty" yaml:"cleanupSubscription,omitempty"`
		AutoAck                      *bool `json:"autoAck,omitempty" yaml:"autoAck,omitempty"`
	}
	if err := yaml.Unmarshal(data, &presence); err != nil {
		return nil, fmt.Errorf("failed to parse function config file %q: %w", path, err)
	}

	return &loadedFunctionConfigFile{
		Config:                            cfg,
		ForwardSourceMessagePropertyFound: presence.ForwardSourceMessageProperty != nil,
		CleanupSubscriptionFound:          presence.CleanupSubscription != nil,
		AutoAckFound:                      presence.AutoAck != nil,
	}, nil
}

func resolveFunctionBool(current bool, foundInConfigFile bool, flagChanged bool, flagValue bool, defaultValue bool) bool {
	if flagChanged {
		return flagValue
	}
	if foundInConfigFile {
		return current
	}
	return defaultValue
}

func buildFunctionUpdateOptions(flagChanged bool, updateAuthData bool) *registry.UpdateOptionsImpl {
	if !flagChanged {
		return nil
	}
	return &registry.UpdateOptionsImpl{UpdateAuthData: updateAuthData}
}

func applyFunctionIdentity(cfg *registry.RegistryFunctionConfig, opts *functionApplyOptions) error {
	if strings.TrimSpace(opts.FQFN) != "" {
		parts := strings.Split(opts.FQFN, "/")
		if len(parts) != 3 {
			return fmt.Errorf("fully qualified function names (FQFNs) must be of the form tenant/namespace/name")
		}
		cfg.Tenant = parts[0]
		cfg.Namespace = parts[1]
		cfg.Name = parts[2]
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
	return nil
}

func applyFunctionFlags(cfg *registry.RegistryFunctionConfig, opts *functionApplyOptions) error {
	if opts.ClassName != "" {
		cfg.ClassName = opts.ClassName
	}
	if opts.FunctionType != "" {
		builtin := "builtin://" + opts.FunctionType
		cfg.Jar = &builtin
	}
	if opts.Jar != "" {
		cfg.Jar = &opts.Jar
	}
	if opts.Py != "" {
		cfg.Py = &opts.Py
	}
	if opts.Go != "" {
		cfg.Go = &opts.Go
	}
	if opts.Inputs != "" {
		cfg.Inputs = splitCSV(opts.Inputs)
	}
	if opts.TopicsPattern != "" {
		cfg.TopicsPattern = &opts.TopicsPattern
	}
	if opts.Output != "" {
		cfg.Output = opts.Output
	}
	if opts.ProducerConfig != "" {
		parsed, err := parseProducerConfigJSON(opts.ProducerConfig)
		if err != nil {
			return fmt.Errorf("--producer-config: %w", err)
		}
		cfg.ProducerConfig = parsed
	}
	if opts.LogTopic != "" {
		cfg.LogTopic = opts.LogTopic
	}
	if opts.SchemaType != "" {
		cfg.OutputSchemaType = opts.SchemaType
	}
	if opts.CustomSerdeInputs != "" {
		parsed, err := parseStringMapJSON(opts.CustomSerdeInputs)
		if err != nil {
			return fmt.Errorf("--custom-serde-inputs: %w", err)
		}
		cfg.CustomSerdeInputs = parsed
	}
	if opts.CustomSchemaInputs != "" {
		parsed, err := parseStringMapJSON(opts.CustomSchemaInputs)
		if err != nil {
			return fmt.Errorf("--custom-schema-inputs: %w", err)
		}
		cfg.CustomSchemaInputs = parsed
	}
	if opts.CustomSchemaOutputs != "" {
		parsed, err := parseStringMapJSON(opts.CustomSchemaOutputs)
		if err != nil {
			return fmt.Errorf("--custom-schema-outputs: %w", err)
		}
		cfg.CustomSchemaOutputs = parsed
	}
	if opts.InputSpecs != "" {
		parsed, err := parseConsumerConfigMapJSON(opts.InputSpecs)
		if err != nil {
			return fmt.Errorf("--input-specs: %w", err)
		}
		cfg.InputSpecs = parsed
	}
	if opts.InputTypeClassName != "" {
		cfg.InputTypeClassName = opts.InputTypeClassName
	}
	if opts.OutputSerdeClassName != "" {
		cfg.OutputSerdeClassName = opts.OutputSerdeClassName
	}
	if opts.OutputTypeClassName != "" {
		cfg.OutputTypeClassName = opts.OutputTypeClassName
	}
	if opts.ProcessingGuarantees != "" {
		cfg.ProcessingGuarantees = opts.ProcessingGuarantees
	}
	if opts.UserConfig != "" {
		parsed, err := parseInterfaceMapJSON(opts.UserConfig)
		if err != nil {
			return fmt.Errorf("--user-config: %w", err)
		}
		cfg.UserConfig = parsed
	}
	if opts.RetainOrdering {
		cfg.RetainOrdering = true
	}
	if opts.RetainKeyOrdering {
		cfg.RetainKeyOrdering = true
	}
	if opts.BatchBuilder != "" {
		cfg.BatchBuilder = opts.BatchBuilder
	}
	if opts.SubsName != "" {
		cfg.SubName = opts.SubsName
	}
	if opts.SubsPosition != "" {
		cfg.SubscriptionPosition = opts.SubsPosition
	}
	if opts.SkipToLatest {
		cfg.SkipToLatest = true
	}
	if opts.Parallelism != 0 {
		cfg.Parallelism = opts.Parallelism
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
	if opts.WindowLengthCount != 0 || opts.WindowLengthDurationMs != 0 || opts.SlidingIntervalCount != 0 || opts.SlidingIntervalDurationMs != 0 {
		if cfg.WindowConfig == nil {
			cfg.WindowConfig = &registry.WindowConfig{}
		}
		if opts.WindowLengthCount != 0 {
			cfg.WindowConfig.WindowLengthCount = &opts.WindowLengthCount
		}
		if opts.WindowLengthDurationMs != 0 {
			cfg.WindowConfig.WindowLengthDurationMs = &opts.WindowLengthDurationMs
		}
		if opts.SlidingIntervalCount != 0 {
			cfg.WindowConfig.SlidingIntervalCount = &opts.SlidingIntervalCount
		}
		if opts.SlidingIntervalDurationMs != 0 {
			cfg.WindowConfig.SlidingIntervalDurationMs = &opts.SlidingIntervalDurationMs
		}
	}
	if opts.TimeoutMs != 0 {
		cfg.TimeoutMs = &opts.TimeoutMs
	}
	if opts.MaxMessageRetries != 0 {
		cfg.MaxMessageRetries = &opts.MaxMessageRetries
	}
	if opts.DeadLetterTopic != "" {
		cfg.DeadLetterTopic = opts.DeadLetterTopic
	}
	if opts.CustomRuntimeOptions != "" {
		cfg.CustomRuntimeOptions = opts.CustomRuntimeOptions
	}
	if opts.Secrets != "" {
		parsed, err := parseInterfaceMapJSON(opts.Secrets)
		if err != nil {
			return fmt.Errorf("--secrets: %w", err)
		}
		cfg.Secrets = parsed
	}
	return nil
}

func validateFunctionPackageSelectors(cfg registry.RegistryFunctionConfig, opts *functionApplyOptions) error {
	selectorsByKind := map[string][]string{}
	order := make([]string, 0, 4)

	add := func(kind, source string) {
		if _, ok := selectorsByKind[kind]; !ok {
			order = append(order, kind)
		}
		selectorsByKind[kind] = append(selectorsByKind[kind], source)
	}

	if strings.TrimSpace(opts.FunctionType) != "" {
		add("function-type", "--function-type")
	}
	if strings.TrimSpace(opts.Jar) != "" {
		add("jar", "--jar")
	}
	if strings.TrimSpace(opts.Py) != "" {
		add("py", "--py")
	}
	if strings.TrimSpace(opts.Go) != "" {
		add("go", "--go")
	}
	if cfg.Jar != nil && strings.TrimSpace(*cfg.Jar) != "" {
		add("jar", "function config file jar")
	}
	if cfg.Py != nil && strings.TrimSpace(*cfg.Py) != "" {
		add("py", "function config file py")
	}
	if cfg.Go != nil && strings.TrimSpace(*cfg.Go) != "" {
		add("go", "function config file go")
	}

	if len(order) <= 1 {
		return nil
	}

	conflicts := make([]string, 0, len(order))
	for _, kind := range order {
		conflicts = append(conflicts, selectorsByKind[kind]...)
	}

	return fmt.Errorf(
		"function package selectors are mutually exclusive; found %s. Specify only one of --function-type, --jar, --py, --go, or a config file package field",
		strings.Join(conflicts, ", "),
	)
}

func validateFunctionPackageFields(cfg registry.RegistryFunctionConfig) error {
	selectedFields := make([]string, 0, 3)

	if cfg.Go != nil && strings.TrimSpace(*cfg.Go) != "" {
		selectedFields = append(selectedFields, "go")
	}
	if cfg.Py != nil && strings.TrimSpace(*cfg.Py) != "" {
		selectedFields = append(selectedFields, "py")
	}
	if cfg.Jar != nil && strings.TrimSpace(*cfg.Jar) != "" {
		selectedFields = append(selectedFields, "jar")
	}
	if len(selectedFields) <= 1 {
		return nil
	}

	return fmt.Errorf(
		"function config contains multiple package fields (%s); specify only one package source",
		strings.Join(selectedFields, ", "),
	)
}

func splitFunctionPackageLocation(cfg registry.RegistryFunctionConfig) (string, string) {
	if cfg.Go != nil && strings.TrimSpace(*cfg.Go) != "" {
		return splitPackageLocation(*cfg.Go)
	}
	if cfg.Py != nil && strings.TrimSpace(*cfg.Py) != "" {
		return splitPackageLocation(*cfg.Py)
	}
	if cfg.Jar != nil && strings.TrimSpace(*cfg.Jar) != "" {
		return splitPackageLocation(*cfg.Jar)
	}
	return "", ""
}

func renderFunctionConfig(writer io.Writer, output string, item registry.RegistryFunctionConfig) error {
	return renderWorkspaceResource(writer, output, item, func(writer io.Writer) error {
		_, _ = fmt.Fprintf(writer, "Name: %s\n", item.Name)
		_, _ = fmt.Fprintf(writer, "Tenant: %s\n", item.Tenant)
		_, _ = fmt.Fprintf(writer, "Namespace: %s\n", item.Namespace)
		_, _ = fmt.Fprintf(writer, "ClassName: %s\n", item.ClassName)
		_, _ = fmt.Fprintf(writer, "Connection: %s\n", item.Connection)
		_, _ = fmt.Fprintf(writer, "SN Service Account: %s\n", item.SNServiceAccount)
		_, _ = fmt.Fprintf(writer, "Parallelism: %d\n", item.Parallelism)
		_, _ = fmt.Fprintf(writer, "Output: %s\n", item.Output)
		_, _ = fmt.Fprintf(writer, "Log Topic: %s\n", item.LogTopic)
		if len(item.Inputs) > 0 {
			_, _ = fmt.Fprintf(writer, "Inputs: %s\n", strings.Join(item.Inputs, ","))
		}
		if ref := functionPackageReference(item); ref != "" {
			_, _ = fmt.Fprintf(writer, "Package: %s\n", ref)
		}
		return nil
	})
}

func renderFunctionStatus(writer io.Writer, output string, status registry.FunctionStatus) error {
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
		table.SetHeader([]string{"Instance ID", "Running", "Restarts", "Worker", "Error"})
		for _, instance := range status.Instances {
			table.Append([]string{
				fmt.Sprintf("%d", instance.InstanceID),
				fmt.Sprintf("%t", instance.Status.Running),
				fmt.Sprintf("%d", instance.Status.NumRestarts),
				instance.Status.WorkerID,
				instance.Status.Err,
			})
		}
		table.Render()
		return nil
	}
}

func functionPackageReference(item registry.RegistryFunctionConfig) string {
	if item.Go != nil && *item.Go != "" {
		return *item.Go
	}
	if item.Py != nil && *item.Py != "" {
		return *item.Py
	}
	if item.Jar != nil && *item.Jar != "" {
		return *item.Jar
	}
	return ""
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseStringMapJSON(raw string) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var value map[string]string
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("invalid JSON for string map: %w", err)
	}
	return value, nil
}

func parseInterfaceMapJSON(raw string) (map[string]interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var value map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("invalid JSON for config map: %w", err)
	}
	return value, nil
}

func parseProducerConfigJSON(raw string) (*registry.ProducerConfig, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var value registry.ProducerConfig
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("invalid JSON for producer config: %w", err)
	}
	return &value, nil
}

func parseConsumerConfigMapJSON(raw string) (map[string]registry.ConsumerConfig, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var value map[string]registry.ConsumerConfig
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("invalid JSON for consumer config map: %w", err)
	}
	return value, nil
}
