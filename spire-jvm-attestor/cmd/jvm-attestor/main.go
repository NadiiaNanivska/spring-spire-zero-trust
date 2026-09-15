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
