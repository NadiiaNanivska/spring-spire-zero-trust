package checkers

const (
	SelectorType = "jvm"

	SelectorDebugCleanTrue       = "jvm:debug_clean=true"
	SelectorDebugCleanFalse      = "jvm:debug_clean=false"
	SelectorAgentFlagsCleanTrue  = "jvm:agent_flags_clean=true"
	SelectorAgentFlagsCleanFalse = "jvm:agent_flags_clean=false"
	SelectorAttachSocketExposed  = "jvm:attach_socket_exposed=true"
	SelectorAttachSocketClean    = "jvm:attach_socket_exposed=false"
	SelectorMapsVerified         = "jvm:maps_verified=true"
	SelectorInodeConsistentTrue  = "jvm:inode_consistent=true"
	SelectorInodeConsistentFalse = "jvm:inode_consistent=false"

	SelectorTracerPidPrefix      = "jvm:tracer_pid="
	SelectorSuspiciousFlagPrefix = "jvm:suspicious_flag="
	SelectorSuspiciousEnvPrefix  = "jvm:suspicious_env="
	SelectorJarSha256Prefix      = "jvm:jar_sha256:"
)
