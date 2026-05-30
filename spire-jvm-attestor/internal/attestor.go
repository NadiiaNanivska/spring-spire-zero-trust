package internal

import (
	"context"
	"fmt"
	"strings"
	"sync"

	workloadattestorv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/plugin/agent/workloadattestor/v1"
	configv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/service/common/config/v1"
	"github.com/yourorg/spire-jvm-attestor/config"
	"github.com/yourorg/spire-jvm-attestor/internal/hashsource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// JVMAttestor implements the SPIRE WorkloadAttestor plugin interface.
//
// It verifies three kernel-level properties of a JVM process before allowing
// the SPIRE Agent to issue an SVID:
//
//  1. Anti-debug  — no ptrace tracer attached (TracerPid == 0)
//  2. Anti-tamper — no dangerous JVM flags, env vars, or Attach API socket
//  3. Jar hash    — SHA-256 of the loaded jar matches the CI/CD manifest
//
// All three checks read from /proc/<PID>/* which is populated directly by
// the Linux kernel and cannot be spoofed from user-space without root.
type JVMAttestor struct {
	workloadattestorv1.UnsafeWorkloadAttestorServer
	configv1.UnsafeConfigServer

	mu       sync.RWMutex
	config   *config.Config
	procFS   string
	checkers []Checker

	hashCache     *HashCache
}

func New() *JVMAttestor {
	return &JVMAttestor{
		procFS:        "/proc",
		hashCache:     NewHashCache(),
	}
}

func newWithProcFS(procFS string, cfg *config.Config) *JVMAttestor {
	p := &JVMAttestor{
		procFS:        procFS,
		config:        cfg,
		hashCache:     NewHashCache(),
	}
	if cfg != nil {
		p.buildPipeline(cfg)
	}
	return p
}

func (p *JVMAttestor) buildPipeline(cfg *config.Config) {
	var source hashsource.HashSource

	if cfg.ArtifactoryURL != "" && cfg.ArtifactoryAPIKey != "" {
		source = hashsource.NewArtifactorySource(cfg.ArtifactoryURL, cfg.ArtifactoryAPIKey)
	} else {
		source = hashsource.NewLocalManifestSource(cfg.HashManifestPath)
	}

	p.checkers = []Checker{
		NewAntiDebugChecker(),
		NewAntiTamperChecker(cfg.BlockOnAttachSocket),
		NewJarHashChecker(source),
	}
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
	p.buildPipeline(cfg)
	p.mu.Unlock()

	return &configv1.ConfigureResponse{}, nil
}

func (p *JVMAttestor) Attest(
	ctx context.Context,
	req *workloadattestorv1.AttestRequest,
) (*workloadattestorv1.AttestResponse, error) {
	p.mu.RLock()
	cfg := p.config
	checkers := p.checkers
	p.mu.RUnlock()

	if cfg == nil {
		return nil, status.Error(codes.FailedPrecondition, "plugin not configured; call Configure first")
	}

	attestCtx := &AttestationContext{
		Context:       ctx,
		PID:           req.Pid,
		ProcRoot:      fmt.Sprintf("%s/%d", p.procFS, req.Pid),
		HashCache:     p.hashCache,
	}

	var allSelectors []string

	for _, checker := range checkers {
		selectors, err := checker.Check(attestCtx)
		if err != nil {
			if checker.Name() == "anti-debug" {
				return nil, status.Errorf(codes.Internal, "anti-debug check: %v", err)
			}
			return nil, status.Errorf(codes.PermissionDenied, "%s check failed: %v", checker.Name(), err)
		}

		allSelectors = append(allSelectors, selectors...)

		if containsSelector(selectors, SelectorDebugCleanFalse) ||
			containsSelector(selectors, SelectorAgentFlagsCleanFalse) {
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
