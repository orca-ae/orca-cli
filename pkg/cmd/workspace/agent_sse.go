// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"io"

	registry "github.com/orca-ae/orca-sdk-go"
)

// streamManagedAgentSSE opens a managed-agent event stream and renders it as
// NDJSON. The Server-Sent Events decoder itself lives in the SDK
// (registry.DecodeSSE); this adapter only binds it to the workspace client.
func streamManagedAgentSSE(
	client workspaceManagedAgentsClient,
	ctx context.Context,
	path string,
	writer io.Writer,
) error {
	return client.GetStream(ctx, path, "text/event-stream", func(reader io.Reader) error {
		return registry.DecodeSSE(writer, reader)
	})
}
