package checkers

const (
	SelectorType = "jvm"

	SelectorDebugCleanTrue       = "jvm:debug_clean=true"
	SelectorDebugCleanFalse      = "jvm:debug_clean=false"
	SelectorAgentFlagsCleanTrue  = "jvm:agent_flags_clean=true"
	SelectorAgentFlagsCleanFalse = "jvm:agent_flags_clean=false"
	SelectorAttachSocketExposed  = "jvm:attach_socket_exposed=true"
	SelectorAttachSocketClean    = "jvm:attach_socket_exposed=false"

	// SelectorMapsVerified covers both maps and fd discovery, but excludes cmdline.
	SelectorMapsVerified      = "jvm:maps_verified=true"
	SelectorMapsVerifiedFalse = "jvm:maps_verified=false"

	SelectorKernelHandleTrue  = "jvm:hash_via_kernel_handle=true"
	SelectorKernelHandleFalse = "jvm:hash_via_kernel_handle=false"

	SelectorInodeConsistentTrue  = "jvm:inode_consistent=true"
	SelectorInodeConsistentFalse = "jvm:inode_consistent=false"

	SelectorTracerPidPrefix      = "jvm:tracer_pid="
	SelectorSuspiciousFlagPrefix = "jvm:suspicious_flag="
	SelectorSuspiciousEnvPrefix  = "jvm:suspicious_env="
	SelectorJarSha256Prefix      = "jvm:jar_sha256="
	SelectorJarSourcePrefix      = "jvm:jar_source="

	SelectorJarSetSha256Prefix = "jvm:jar_set_sha256="
)
