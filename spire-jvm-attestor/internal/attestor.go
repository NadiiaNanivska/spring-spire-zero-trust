package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	workloadattestorv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/plugin/agent/workloadattestor/v1"
	configv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/service/common/config/v1"
	"github.com/yourorg/spire-jvm-attestor/config"
	"github.com/yourorg/spire-jvm-attestor/internal/cache"
	"github.com/yourorg/spire-jvm-attestor/internal/checkers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type JVMAttestor struct {
	workloadattestorv1.UnsafeWorkloadAttestorServer
	configv1.UnimplementedConfigServer

	mu        sync.RWMutex
	config    *config.Config
	procFS    string
	pipeline  []checkers.Checker
	hashCache *cache.HashCache
	logger    hclog.Logger
}

func (p *JVMAttestor) SetLogger(logger hclog.Logger) {
	p.logger = logger
}

func (p *JVMAttestor) safeLogger() hclog.Logger {
	if p.logger != nil {
		return p.logger
	}
	return hclog.NewNullLogger()
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
		p.buildPipeline(cfg)
	}
	return p
}

func (p *JVMAttestor) buildPipeline(cfg *config.Config) {
	checkersToConfigure := []struct {
		enabled bool
		create  func() checkers.Checker
	}{
		{cfg.CheckJDK, func() checkers.Checker { return checkers.NewJDKChecker() }},
		{cfg.CheckLaunch, func() checkers.Checker {
			return checkers.NewLaunchFactsChecker(cfg.BlockOnAttachSocket)
		}},
		{cfg.CheckJarHash, func() checkers.Checker { return checkers.NewJarHashChecker() }},
		{cfg.CheckProvenance, func() checkers.Checker { return checkers.NewProvenanceChecker() }},
	}

	p.pipeline = make([]checkers.Checker, 0, len(checkersToConfigure))
	for _, candidate := range checkersToConfigure {
		if candidate.enabled {
			p.pipeline = append(p.pipeline, candidate.create())
		}
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

	enabledModules := make([]string, 0, len(p.pipeline))
	for _, checker := range p.pipeline {
		enabledModules = append(enabledModules, checker.Name())
	}
	p.safeLogger().Info("JVM attestor configured",
		"enabled_modules", strings.Join(enabledModules, ","),
		"block_on_attach_socket", cfg.BlockOnAttachSocket,
	)
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

	log := p.safeLogger()

	attestCtx := &checkers.AttestationContext{
		Context:    ctx,
		PID:        req.Pid,
		ProcRoot:   fmt.Sprintf("%s/%d", p.procFS, req.Pid),
		HashCache:  p.hashCache,
		Logger:     log,
		PublicKeys: cfg.PublicKeys,
		JARHashes:  make(map[string]string),
	}

	var allSelectors []string

	for _, checker := range pipeline {
		selectors, err := checker.Check(attestCtx)
		if err != nil {
			// No JARs: return empty selectors so other attestors can handle this process.
			if errors.Is(err, checkers.ErrNotJVM) {
				log.Debug("process is not a JVM; skipping JVM attestation", "pid", req.Pid)
				return &workloadattestorv1.AttestResponse{}, nil
			}
			log.Warn("checker failed",
				"checker", checker.Name(),
				"pid", req.Pid,
				"error", err,
			)
			switch checker.Name() {
			case "jdk", "launch-facts":
				return nil, status.Errorf(codes.Internal, "%s check: %v", checker.Name(), err)
			default:
				return nil, status.Errorf(codes.PermissionDenied, "%s check failed: %v", checker.Name(), err)
			}
		}

		allSelectors = append(allSelectors, selectors...)
	}

	return buildResponse(allSelectors), nil
}

func (p *JVMAttestor) Close() error {
	p.mu.RLock()
	pipeline := p.pipeline
	p.mu.RUnlock()

	var errs []error
	for _, checker := range pipeline {
		if closer, ok := checker.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
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
