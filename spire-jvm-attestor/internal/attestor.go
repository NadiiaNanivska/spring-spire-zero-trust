package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	workloadattestorv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/plugin/agent/workloadattestor/v1"
	configv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/service/common/config/v1"
	"github.com/yourorg/spire-jvm-attestor/config"
	"github.com/yourorg/spire-jvm-attestor/internal/cache"
	"github.com/yourorg/spire-jvm-attestor/internal/checkers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// JVMAttestor is a SPIRE WorkloadAttestor that verifies JVM process integrity
// via /proc/<PID>/*: ptrace state, dangerous flags, and JAR SHA-256.
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
	p.pipeline = []checkers.Checker{
		checkers.NewAntiDebugChecker(),
		checkers.NewAntiTamperChecker(cfg.BlockOnAttachSocket),
		checkers.NewJarHashChecker(),
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

	p.safeLogger().Info("plugin configured", "block_on_attach_socket", cfg.BlockOnAttachSocket)
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

	attestStart := time.Now()
	log := p.safeLogger()
	log.Debug("jvm attestation started",
		"pid", req.Pid,
		"started_at", attestStart.Format(time.RFC3339Nano),
	)

	attestCtx := &checkers.AttestationContext{
		Context:   ctx,
		PID:       req.Pid,
		ProcRoot:  fmt.Sprintf("%s/%d", p.procFS, req.Pid),
		HashCache: p.hashCache,
		Logger:    log,
	}

	var allSelectors []string
	checkerDurations := make(map[string]time.Duration, len(pipeline))

	for _, checker := range pipeline {
		checkStart := time.Now()
		selectors, err := checker.Check(attestCtx)
		checkerDurations[checker.Name()] = time.Since(checkStart)
		if err != nil {
			// ErrNotJVM means the process has no JAR files — it is simply not a JVM
			// workload. Return empty selectors so that other attestors (k8s, unix)
			// can still issue SVIDs for this process.
			if errors.Is(err, checkers.ErrNotJVM) {
				log.Debug("not a JVM process, skipping attestation",
					"pid", req.Pid,
					"duration_us", time.Since(attestStart).Microseconds(),
				)
				return &workloadattestorv1.AttestResponse{}, nil
			}
			log.Warn("checker failed",
				"checker", checker.Name(),
				"pid", req.Pid,
				"duration_us", checkerDurations[checker.Name()].Microseconds(),
				"error", err,
			)
			switch checker.Name() {
			case "anti-debug":
				return nil, status.Errorf(codes.Internal, "anti-debug check: %v", err)
			case "anti-tamper":
				return nil, status.Errorf(codes.FailedPrecondition, "%s check failed: %v", checker.Name(), err)
			default:
				return nil, status.Errorf(codes.PermissionDenied, "%s check failed: %v", checker.Name(), err)
			}
		}

		allSelectors = append(allSelectors, selectors...)

		if containsSelector(selectors, checkers.SelectorDebugCleanFalse) ||
			containsSelector(selectors, checkers.SelectorAgentFlagsCleanFalse) {
			break
		}
	}

	log.Info("jvm attestation timing",
		"pid", req.Pid,
		"total_us", time.Since(attestStart).Microseconds(),
		"anti_debug_us", checkerDurations["anti-debug"].Microseconds(),
		"anti_tamper_us", checkerDurations["anti-tamper"].Microseconds(),
		"jar_hash_us", checkerDurations["jar-hash"].Microseconds(),
		"selectors", len(allSelectors),
	)
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
