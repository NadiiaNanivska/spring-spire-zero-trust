// Command jvm-attestor is a SPIRE WorkloadAttestor plugin that verifies
// JVM process integrity at the Linux kernel level via /proc.
//
// It is invoked by the SPIRE Agent as an external plugin binary. The agent
// communicates with it over a gRPC connection established via stdin/stdout
// using the SPIRE plugin SDK protocol.
//
// Usage — agent.conf:
//
//	plugins {
//	  WorkloadAttestor "jvm" {
//	    plugin_cmd      = "/opt/spire/plugins/jvm-attestor"
//	    plugin_checksum = "sha256:<hex>"
//
//	    plugin_data {
//	      block_on_attach_socket = true
//	    }
//	  }
//	}
package main

import (
	"github.com/spiffe/spire-plugin-sdk/pluginmain"
	workloadattestorv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/plugin/agent/workloadattestor/v1"
	configv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/service/common/config/v1"

	"github.com/yourorg/spire-jvm-attestor/internal"
)

func main() {
	plugin := internal.New()
	pluginmain.Serve(
		workloadattestorv1.WorkloadAttestorPluginServer(plugin),
		configv1.ConfigServiceServer(plugin),
	)
}
