// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
)

type packagesOptions struct {
	opts      *Options
	ioStreams IOStreams
}

type workspacePackagesClient interface {
	List(ctx context.Context, packageType string) ([]string, error)
	ListVersions(ctx context.Context, packageType, packageName string) ([]string, error)
	GetMetadata(ctx context.Context, packageType, packageName, version string) (*registry.PackageMetadata, error)
	UpdateMetadata(ctx context.Context, packageType, packageName, version string, metadata registry.PackageMetadata) error
	Upload(ctx context.Context, packageType, packageName, version, filePath string, metadata registry.PackageMetadata) error
	Download(ctx context.Context, packageType, packageName, version string, writer io.Writer) error
	Delete(ctx context.Context, packageType, packageName, version string) error
}

type workspacePackageRef struct {
	LogicalType string
	APIType     string
	Name        string
	Version     string
}

var newWorkspacePackagesClient = func() (workspacePackagesClient, error) {
	client, err := newWorkspaceRegistryClient()
	if err != nil {
		return nil, err
	}

	return registry.NewPackagesClient(client), nil
}

// NewCmdPackages creates workspace package commands.
func NewCmdPackages(opts *Options) *cobra.Command {
	opts = setCurrentOptions(opts)
	o := &packagesOptions{
		opts:      opts,
		ioStreams: opts.IOStreams,
	}

	cmd := &cobra.Command{
		Use:   "packages",
		Short: "Manage workspace packages",
		Long: "Manage versioned workspace package artifacts through the workspace registry API. " +
			"Package URLs use the form <type>://<package_name>[@<version>] where <type> is one of function, source, or sink. " +
			"Requires the cloud.sn.io extension group; not available on a self-hosted engine.",
		PersistentPreRunE: requireCloudExtension,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		o.newListCommand(),
		o.newListVersionsCommand(),
		o.newGetMetadataCommand(),
		o.newUpdateMetadataCommand(),
		o.newUploadCommand(),
		o.newDownloadCommand(),
		o.newDeleteCommand(),
	)

	return cmd
}

func (o *packagesOptions) newListCommand() *cobra.Command {
	output := "text"
	logicalType := ""

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace packages",
		Long: "List package URLs for uploaded workspace packages of the requested type. " +
			"Use the returned package URL without a version suffix as input to list-versions.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}

			refType, err := parseWorkspacePackageType(logicalType)
			if err != nil {
				return err
			}

			client, err := o.newPackagesClient()
			if err != nil {
				return err
			}

			items, err := client.List(cmd.Context(), refType.APIType)
			if err != nil {
				return fmt.Errorf("failed to list %s packages: %w", refType.LogicalType, err)
			}

			refs := make([]workspacePackageRef, 0, len(items))
			for _, item := range items {
				refs = append(refs, workspacePackageRef{
					LogicalType: refType.LogicalType,
					APIType:     refType.APIType,
					Name:        item,
				})
			}

			return renderWorkspacePackageRefs(o.ioStreams.Out, output, "Package URL", refs)
		},
	}

	cmd.Flags().StringVar(&logicalType, "type", logicalType, "Package type (function, source, sink)")
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	_ = cmd.MarkFlagRequired("type")
	return cmd
}

func (o *packagesOptions) newListVersionsCommand() *cobra.Command {
	output := "text"

	cmd := &cobra.Command{
		Use:   "list-versions [package-url]",
		Short: "List all versions of a workspace package",
		Long: "List all uploaded versions for a workspace package URL. " +
			"The package URL must identify the package without an @<version> suffix.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}

			ref, err := parseWorkspacePackageReferenceArg(args[0], false)
			if err != nil {
				return err
			}

			client, err := o.newPackagesClient()
			if err != nil {
				return err
			}

			versions, err := client.ListVersions(cmd.Context(), ref.APIType, ref.Name)
			if err != nil {
				return fmt.Errorf("failed to list versions for package %q: %w", ref.String(), err)
			}

			refs := make([]workspacePackageRef, 0, len(versions))
			for _, version := range versions {
				refs = append(refs, workspacePackageRef{
					LogicalType: ref.LogicalType,
					APIType:     ref.APIType,
					Name:        ref.Name,
					Version:     version,
				})
			}

			return renderWorkspacePackageRefs(o.ioStreams.Out, output, "Package URL", refs)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *packagesOptions) newGetMetadataCommand() *cobra.Command {
	output := "text"

	cmd := &cobra.Command{
		Use:   "get-metadata [package-url]",
		Short: "Get metadata for a workspace package version",
		Long: "Get the stored metadata for a specific workspace package version. " +
			"The package URL must include both the package type and version.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}

			ref, err := parseWorkspacePackageReferenceArg(args[0], true)
			if err != nil {
				return err
			}

			client, err := o.newPackagesClient()
			if err != nil {
				return err
			}

			item, err := client.GetMetadata(cmd.Context(), ref.APIType, ref.Name, ref.Version)
			if err != nil {
				return fmt.Errorf("failed to get metadata for package %q: %w", ref.String(), err)
			}

			return renderPackageMetadata(o.ioStreams.Out, output, *item)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *packagesOptions) newUpdateMetadataCommand() *cobra.Command {
	var metadata registry.PackageMetadata
	clearProperties := false

	cmd := &cobra.Command{
		Use:   "update-metadata [package-url]",
		Short: "Update metadata for a workspace package version",
		Long: "Update description, contact, or custom properties for a specific workspace package version " +
			"without re-uploading the package artifact.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseWorkspacePackageReferenceArg(args[0], true)
			if err != nil {
				return err
			}
			descriptionChanged := cmd.Flags().Changed("description")
			contactChanged := cmd.Flags().Changed("contact")
			propertiesChanged := cmd.Flags().Changed("properties")
			if !descriptionChanged && !contactChanged && !propertiesChanged && !clearProperties {
				return fmt.Errorf("at least one metadata flag must be provided")
			}
			if propertiesChanged && clearProperties {
				return fmt.Errorf("--properties and --clear-properties cannot be used together")
			}

			client, err := o.newPackagesClient()
			if err != nil {
				return err
			}

			current, err := client.GetMetadata(cmd.Context(), ref.APIType, ref.Name, ref.Version)
			if err != nil {
				return fmt.Errorf("failed to get metadata for package %q: %w", ref.String(), err)
			}
			if current == nil {
				return fmt.Errorf("failed to get metadata for package %q: empty response", ref.String())
			}
			merged := *current
			if descriptionChanged {
				merged.Description = metadata.Description
			}
			if contactChanged {
				merged.Contact = metadata.Contact
			}
			if propertiesChanged {
				merged.Properties = metadata.Properties
			}
			if clearProperties {
				merged.Properties = map[string]string{}
			}

			if err := client.UpdateMetadata(cmd.Context(), ref.APIType, ref.Name, ref.Version, merged); err != nil {
				return fmt.Errorf("failed to update metadata for package %q: %w", ref.String(), err)
			}

			_, _ = fmt.Fprintf(o.ioStreams.Out, "Updated metadata for package %q successfully\n", ref.String())
			return nil
		},
	}

	addPackageMetadataFlags(cmd, &metadata)
	cmd.Flags().BoolVar(&clearProperties, "clear-properties", false, "Clear all package properties")
	return cmd
}

func (o *packagesOptions) newUploadCommand() *cobra.Command {
	filePath := ""
	var metadata registry.PackageMetadata

	cmd := &cobra.Command{
		Use:   "upload [package-url]",
		Short: "Upload a workspace package version",
		Long: "Upload a local package artifact to a specific workspace package version. " +
			"The package URL must include the package type, package name, and version, and --path must point to the local artifact file.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseWorkspacePackageReferenceArg(args[0], true)
			if err != nil {
				return err
			}

			client, err := o.newPackagesClient()
			if err != nil {
				return err
			}

			if err := client.Upload(cmd.Context(), ref.APIType, ref.Name, ref.Version, filePath, metadata); err != nil {
				return fmt.Errorf("failed to upload package %q: %w", ref.String(), err)
			}

			_, _ = fmt.Fprintf(o.ioStreams.Out, "Uploaded package %q successfully\n", ref.String())
			return nil
		},
	}

	cmd.Flags().StringVar(&filePath, "path", filePath, "Path to the local package file")
	_ = cmd.MarkFlagRequired("path")
	addPackageMetadataFlags(cmd, &metadata)
	return cmd
}

func (o *packagesOptions) newDownloadCommand() *cobra.Command {
	filePath := ""

	cmd := &cobra.Command{
		Use:   "download [package-url]",
		Short: "Download a workspace package version",
		Long: "Download the artifact for a specific workspace package version to a local file. " +
			"The package URL must include a version, and --path selects where the downloaded artifact is written.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseWorkspacePackageReferenceArg(args[0], true)
			if err != nil {
				return err
			}

			client, err := o.newPackagesClient()
			if err != nil {
				return err
			}

			var destinationMode *os.FileMode
			if info, statErr := os.Stat(filePath); statErr == nil {
				mode := info.Mode().Perm()
				destinationMode = &mode
			} else if !os.IsNotExist(statErr) {
				return fmt.Errorf("failed to inspect output file %q: %w", filePath, statErr)
			}
			destinationDir := filepath.Dir(filePath)
			tempFile, err := createPackageDownloadTemp(destinationDir, filepath.Base(filePath))
			if err != nil {
				return fmt.Errorf("failed to create temporary output file for %q: %w", filePath, err)
			}
			tempPath := tempFile.Name()
			keepTemp := true
			defer func() {
				if keepTemp {
					_ = tempFile.Close()
					_ = os.Remove(tempPath)
				}
			}()

			if err := client.Download(cmd.Context(), ref.APIType, ref.Name, ref.Version, tempFile); err != nil {
				return fmt.Errorf("failed to download package %q: %w", ref.String(), err)
			}
			if err := tempFile.Close(); err != nil {
				return fmt.Errorf("failed to close temporary output file for %q: %w", filePath, err)
			}
			if destinationMode != nil {
				if err := os.Chmod(tempPath, *destinationMode); err != nil {
					return fmt.Errorf("failed to set output file permissions for %q: %w", filePath, err)
				}
			}
			if err := os.Rename(tempPath, filePath); err != nil {
				return fmt.Errorf("failed to install downloaded package at %q: %w", filePath, err)
			}
			keepTemp = false

			_, _ = fmt.Fprintf(o.ioStreams.Out, "Downloaded package %q successfully\n", ref.String())
			return nil
		},
	}

	cmd.Flags().StringVar(&filePath, "path", filePath, "Path to write the package file to")
	_ = cmd.MarkFlagRequired("path")
	return cmd
}

func createPackageDownloadTemp(directory, destinationBase string) (*os.File, error) {
	for range 100 {
		var suffix [8]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return nil, fmt.Errorf("failed to generate temporary file name: %w", err)
		}
		path := filepath.Join(directory, "."+destinationBase+".tmp-"+hex.EncodeToString(suffix[:]))
		// Let kernel apply process umask, matching normal destination creation.
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
		if err == nil {
			return file, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("failed to allocate unique temporary file name")
}

func (o *packagesOptions) newDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [package-url]",
		Short: "Delete a workspace package version",
		Long: "Delete a specific workspace package version from the workspace registry. " +
			"The package URL must include the version to remove.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseWorkspacePackageReferenceArg(args[0], true)
			if err != nil {
				return err
			}

			client, err := o.newPackagesClient()
			if err != nil {
				return err
			}

			if err := client.Delete(cmd.Context(), ref.APIType, ref.Name, ref.Version); err != nil {
				return fmt.Errorf("failed to delete package %q: %w", ref.String(), err)
			}

			_, _ = fmt.Fprintf(o.ioStreams.Out, "Deleted package %q successfully\n", ref.String())
			return nil
		},
	}

	return cmd
}

func (o *packagesOptions) newPackagesClient() (workspacePackagesClient, error) {
	return newWorkspacePackagesClient()
}

func addPackageMetadataFlags(cmd *cobra.Command, metadata *registry.PackageMetadata) {
	cmd.Flags().StringVar(&metadata.Description, "description", metadata.Description, "Package description")
	cmd.Flags().StringVar(&metadata.Contact, "contact", metadata.Contact, "Package contact")
	cmd.Flags().StringToStringVarP(&metadata.Properties, "properties", "P", metadata.Properties, "Package properties")
}

func parseWorkspacePackageReferenceArg(raw string, requireVersion bool) (workspacePackageRef, error) {
	ref, err := parseWorkspacePackageReference(raw)
	if err != nil {
		return workspacePackageRef{}, err
	}

	if requireVersion && strings.TrimSpace(ref.Version) == "" {
		return workspacePackageRef{}, fmt.Errorf("package URL %q must include a version", raw)
	}
	if !requireVersion && strings.TrimSpace(ref.Version) != "" {
		return workspacePackageRef{}, fmt.Errorf("package URL %q must not include a version", raw)
	}

	return ref, nil
}

func parseWorkspacePackageReference(raw string) (workspacePackageRef, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return workspacePackageRef{}, fmt.Errorf("package URL cannot be empty")
	}

	scheme, rest, found := strings.Cut(trimmed, "://")
	if !found {
		return workspacePackageRef{}, fmt.Errorf("invalid package URL %q: expected <type>://<package_name>[@<version>]", raw)
	}

	refType, err := parseWorkspacePackageType(scheme)
	if err != nil {
		return workspacePackageRef{}, err
	}

	if strings.TrimSpace(rest) == "" {
		return workspacePackageRef{}, fmt.Errorf("invalid package URL %q: package name cannot be empty", raw)
	}
	if strings.ContainsAny(rest, "/ \t\r\n") {
		return workspacePackageRef{}, fmt.Errorf("invalid package URL %q: package name must not contain slashes or whitespace", raw)
	}

	name, version, found := strings.Cut(rest, "@")
	if strings.TrimSpace(name) == "" {
		return workspacePackageRef{}, fmt.Errorf("invalid package URL %q: package name cannot be empty", raw)
	}
	if found {
		if strings.Contains(version, "@") {
			return workspacePackageRef{}, fmt.Errorf("invalid package URL %q: package version must contain at most one @ separator", raw)
		}
		if strings.TrimSpace(version) == "" {
			return workspacePackageRef{}, fmt.Errorf("invalid package URL %q: package version cannot be empty", raw)
		}
		if strings.ContainsAny(version, "/ \t\r\n") {
			return workspacePackageRef{}, fmt.Errorf("invalid package URL %q: package version must not contain slashes or whitespace", raw)
		}
	}

	return workspacePackageRef{
		LogicalType: refType.LogicalType,
		APIType:     refType.APIType,
		Name:        name,
		Version:     version,
	}, nil
}

func parseWorkspacePackageType(raw string) (workspacePackageRef, error) {
	switch strings.TrimSpace(raw) {
	case "function":
		return workspacePackageRef{LogicalType: "function", APIType: "function"}, nil
	case "source":
		return workspacePackageRef{LogicalType: "source", APIType: "source"}, nil
	case "sink":
		return workspacePackageRef{LogicalType: "sink", APIType: "sink"}, nil
	default:
		return workspacePackageRef{}, fmt.Errorf("invalid package type %q: must be one of function, source, sink", raw)
	}
}

func renderWorkspacePackageRefs(writer io.Writer, output string, header string, refs []workspacePackageRef) error {
	items := make([]string, 0, len(refs))
	for _, ref := range refs {
		items = append(items, ref.String())
	}

	return renderNameList(writer, output, header, items)
}

func renderPackageMetadata(writer io.Writer, output string, metadata registry.PackageMetadata) error {
	return renderWorkspaceResource(writer, output, metadata, func(writer io.Writer) error {
		if strings.TrimSpace(metadata.Description) != "" {
			_, _ = fmt.Fprintf(writer, "Description: %s\n", metadata.Description)
		}
		if strings.TrimSpace(metadata.Contact) != "" {
			_, _ = fmt.Fprintf(writer, "Contact: %s\n", metadata.Contact)
		}
		if metadata.CreateTime != 0 {
			_, _ = fmt.Fprintf(writer, "CreateTime: %d\n", metadata.CreateTime)
		}
		if metadata.ModificationTime != 0 {
			_, _ = fmt.Fprintf(writer, "ModificationTime: %d\n", metadata.ModificationTime)
		}

		if len(metadata.Properties) > 0 {
			keys := make([]string, 0, len(metadata.Properties))
			for key := range metadata.Properties {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			_, _ = fmt.Fprintln(writer, "Properties:")
			for _, key := range keys {
				_, _ = fmt.Fprintf(writer, "  %s=%s\n", key, metadata.Properties[key])
			}
		}

		return nil
	})
}

func (r workspacePackageRef) String() string {
	if strings.TrimSpace(r.Version) == "" {
		return fmt.Sprintf("%s://%s", r.LogicalType, r.Name)
	}

	return fmt.Sprintf("%s://%s@%s", r.LogicalType, r.Name, r.Version)
}
