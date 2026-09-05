package checkers

const (
	SelectorType = "jvm"

	SelectorDebugCleanTrue       = "jvm:debug_clean=true"
	SelectorDebugCleanFalse      = "jvm:debug_clean=false"
	SelectorAgentFlagsCleanTrue  = "jvm:agent_flags_clean=true"
	SelectorAgentFlagsCleanFalse = "jvm:agent_flags_clean=false"
	SelectorAttachSocketExposed  = "jvm:attach_socket_exposed=true"
	SelectorAttachSocketClean    = "jvm:attach_socket_exposed=false"

	// SelectorMapsVerified reports that every jar was discovered through a
	// kernel-attested source (the address space or the descriptor table) rather
	// than through the process' own command line.
	SelectorMapsVerified      = "jvm:maps_verified=true"
	SelectorMapsVerifiedFalse = "jvm:maps_verified=false"

	// SelectorKernelHandle reports that the hashed bytes were read through a
	// /proc handle (map_files or fd) that resolves straight to the inode the
	// kernel associated with the process, with no pathname resolution involved.
	SelectorKernelHandleTrue  = "jvm:hash_via_kernel_handle=true"
	SelectorKernelHandleFalse = "jvm:hash_via_kernel_handle=false"

	SelectorInodeConsistentTrue  = "jvm:inode_consistent=true"
	SelectorInodeConsistentFalse = "jvm:inode_consistent=false"

	SelectorTracerPidPrefix      = "jvm:tracer_pid="
	SelectorSuspiciousFlagPrefix = "jvm:suspicious_flag="
	SelectorSuspiciousEnvPrefix  = "jvm:suspicious_env="
	SelectorJarSha256Prefix      = "jvm:jar_sha256="
	SelectorJarSourcePrefix      = "jvm:jar_source="

	// SelectorJarSetSha256Prefix carries a digest over the whole sorted set of
	// discovered jars. Per-jar jar_sha256 selectors alone are not sufficient to
	// pin a workload: SPIRE matches an entry when its selectors are a SUBSET of
	// the workload's, so an attacker who additionally opens a clean jar would
	// still satisfy an entry pinned on that clean hash. The set digest changes
	// whenever any jar is added, removed or altered.
	SelectorJarSetSha256Prefix = "jvm:jar_set_sha256="
)
