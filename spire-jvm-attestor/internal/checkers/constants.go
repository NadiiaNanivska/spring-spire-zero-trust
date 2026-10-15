package checkers

const (
	SelectorType = "jvm"

	SelectorJavaVersionPrefix = "jvm:java_version="
	SelectorJDKVerifiedTrue = "jvm:jdk_verified=true"
	SelectorHasJavaAgentTrue = "jvm:has_javaagent=true"
	SelectorHasJavaAgentFalse = "jvm:has_javaagent=false"
	SelectorHasJDWPTrue = "jvm:has_jdwp=true"
	SelectorHasJDWPFalse = "jvm:has_jdwp=false"
	SelectorJavaToolOptionsSetTrue = "jvm:java_tool_options_set=true"
	SelectorJavaToolOptionsSetFalse = "jvm:java_tool_options_set=false"
	SelectorAttachSocket = "jvm:attach_socket=true"
	SelectorAttachSocketFalse = "jvm:attach_socket=false"

	SelectorMapsVerified = "jvm:maps_verified=true"
	SelectorMapsVerifiedFalse = "jvm:maps_verified=false"
	SelectorKernelHandleTrue = "jvm:hash_via_kernel_handle=true"
	SelectorKernelHandleFalse = "jvm:hash_via_kernel_handle=false"
	SelectorInodeConsistentTrue = "jvm:inode_consistent=true"
	SelectorInodeConsistentFalse = "jvm:inode_consistent=false"
	SelectorSuspiciousFlagPrefix = "jvm:suspicious_flag="
	SelectorSuspiciousEnvPrefix = "jvm:suspicious_env="
	SelectorJarSha256Prefix = "jvm:jar_sha256="
	SelectorJarSourcePrefix = "jvm:jar_source="
	SelectorJarSetSha256Prefix = "jvm:jar_set_sha256="
	SelectorSignedByPrefix = "jvm:signed_by="
	SelectorSubjectMatchPrefix = "jvm:subject_match="
)