package internal

import (
	"context"
	"fmt"
	"strings"
	"sync"

	workloadattestorv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/plugin/agent/workloadattestor/v1"
	configv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/service/common/config/v1"
	"github.com/yourorg/spire-jvm-attestor/config"
	"github.com/yourorg/spire-jvm-attestor/internal/cache"
	"github.com/yourorg/spire-jvm-attestor/internal/checkers"
	"github.com/yourorg/spire-jvm-attestor/internal/hashsource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// JVMAttestor is a SPIRE WorkloadAttestor that verifies JVM process integrity
// via /proc/<PID>/*: ptrace state, dangerous flags, and JAR SHA-256.
type JVMAttestor struct {
	workloadattestorv1.UnsafeWorkloadAttestorServer
	configv1.UnsafeConfigServer

	mu        sync.RWMutex
	config    *config.Config
	procFS    string
	pipeline  []checkers.Checker
	hashCache *cache.HashCache
}

func New() *JVMAttestor {
	return &JVMAttestor{
		procFS:    "/proc",
		hashCache: cache.NewHashCache(),
	}
}

func newWithProcFS(procFS string, cfg *config.Config) *JVMAttestor {
	p := &JVMAttestor{
		procFS:    procFS,
		config:    cfg,
		hashCache: cache.NewHashCache(),
	}
	if cfg != nil {
		if err := p.buildPipeline(cfg); err != nil {
			panic(fmt.Sprintf("invalid test config: %v", err))
		}
	}
	return p
}

type hashSourceFactory func(cfg *config.Config) hashsource.HashSource

var hashSourceRegistry = map[string]hashSourceFactory{
	"artifactory": func(cfg *config.Config) hashsource.HashSource {
		return hashsource.NewArtifactorySource(cfg.ArtifactoryURL, cfg.ArtifactoryAPIKey)
	},
	"manifest": func(cfg *config.Config) hashsource.HashSource {
		return hashsource.NewLocalManifestSource(cfg.HashManifestPath)
	},
}

func resolveHashSourceType(cfg *config.Config) string {
	if cfg.HashSourceType != "" {
		return cfg.HashSourceType
	}
	if cfg.ArtifactoryURL != "" && cfg.ArtifactoryAPIKey != "" {
		return "artifactory"
	}
	return "manifest"
}

func (p *JVMAttestor) buildPipeline(cfg *config.Config) error {
	sourceType := resolveHashSourceType(cfg)

	factory, ok := hashSourceRegistry[sourceType]
	if !ok {
		return fmt.Errorf("unknown hash_source_type %q; valid values: %v", sourceType, registryKeys(hashSourceRegistry))
	}

	p.pipeline = []checkers.Checker{
		checkers.NewAntiDebugChecker(),
		checkers.NewAntiTamperChecker(cfg.BlockOnAttachSocket),
		checkers.NewJarHashChecker(factory(cfg)),
	}
	return nil
}

func registryKeys(m map[string]hashSourceFactory) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (p *JVMAttestor) Configure(
	_ context.Context,
	req *configv1.ConfigureRequest,
) (*configv1.ConfigureResponse, error) {
	cfg, err := config.Parse(req.HclConfiguration)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid plugin configuration: %v", err)
	}

	p.mu.Lock()
	p.config = cfg
	err = p.buildPipeline(cfg)
	p.mu.Unlock()

	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid plugin configuration: %v", err)
	}

	return &configv1.ConfigureResponse{}, nil
}

func (p *JVMAttestor) Attest(
	ctx context.Context,
	req *workloadattestorv1.AttestRequest,
) (*workloadattestorv1.AttestResponse, error) {
	p.mu.RLock()
	cfg := p.config
	pipeline := p.pipeline
	p.mu.RUnlock()

	if cfg == nil {
		return nil, status.Error(codes.FailedPrecondition, "plugin not configured; call Configure first")
	}

	attestCtx := &checkers.AttestationContext{
		Context:   ctx,
		PID:       req.Pid,
		ProcRoot:  fmt.Sprintf("%s/%d", p.procFS, req.Pid),
		HashCache: p.hashCache,
	}

	var allSelectors []string

	for _, checker := range pipeline {
		selectors, err := checker.Check(attestCtx)
		if err != nil {
			if checker.Name() == "anti-debug" {
				return nil, status.Errorf(codes.Internal, "anti-debug check: %v", err)
			}
			return nil, status.Errorf(codes.PermissionDenied, "%s check failed: %v", checker.Name(), err)
		}

		allSelectors = append(allSelectors, selectors...)

		if containsSelector(selectors, checkers.SelectorDebugCleanFalse) ||
			containsSelector(selectors, checkers.SelectorAgentFlagsCleanFalse) {
			break
		}
	}

	return buildResponse(allSelectors), nil
}

func buildResponse(selectors []string) *workloadattestorv1.AttestResponse {
	resp := &workloadattestorv1.AttestResponse{}
	for _, s := range selectors {
		idx := strings.Index(s, ":")
		if idx < 0 {
			continue
		}
		resp.SelectorValues = append(resp.SelectorValues, s[idx+1:])
	}
	return resp
}

func containsSelector(selectors []string, target string) bool {
	for _, s := range selectors {
		if s == target {
			return true
		}
	}
	return false
}
