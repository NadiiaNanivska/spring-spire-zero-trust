package checkers

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const maxSignatureSize = 4096

type ProvenanceChecker struct{}
func NewProvenanceChecker() *ProvenanceChecker { return &ProvenanceChecker{} }
func (c *ProvenanceChecker) Name() string { return "provenance" }

// Check verifies raw Ed25519 signatures over the 32-byte SHA-256 digest emitted by JarHashChecker.
// Signature files are <jar>.sig, contain raw signature bytes, and are opened with O_NOFOLLOW.
func (c *ProvenanceChecker) Check(ctx *AttestationContext) ([]string, error) {
	checkStart := time.Now()
	if len(ctx.JARHashes) == 0 || len(ctx.PublicKeys) == 0 { return nil, nil }
	for _, jarPath := range ctx.JARPaths {
		hashHex, ok := ctx.JARHashes[jarPath]; if !ok { continue }
		digest, err := hex.DecodeString(hashHex); if err != nil || len(digest) != sha256.Size { continue }
		clean := filepath.Clean(jarPath)
		if filepath.IsAbs(clean) { clean = strings.TrimLeft(clean, string(filepath.Separator)) }
		if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) { continue }
		sigPath := filepath.Join(ctx.ProcRoot, "root", clean+".sig")
		fd, err := syscall.Open(sigPath, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
		if err != nil { continue } // absent, symlinked, or inaccessible signatures are simply untrusted
		f := os.NewFile(uintptr(fd), sigPath)
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxSignatureSize { f.Close(); continue }
		sig, err := io.ReadAll(io.LimitReader(f, maxSignatureSize+1)); f.Close()
		if err != nil || len(sig) != ed25519.SignatureSize { continue }
		for fingerprint, encoded := range ctx.PublicKeys {
			pub, err := base64.StdEncoding.DecodeString(encoded); if err != nil || len(pub) != ed25519.PublicKeySize { continue }
			actual := sha256.Sum256(pub)
			if fingerprint != "SHA256:"+hex.EncodeToString(actual[:]) { continue }
			if ed25519.Verify(ed25519.PublicKey(pub), digest, sig) {
				if ctx.Logger != nil {
					ctx.Logger.Debug("provenance stage timing",
						"pid", ctx.PID,
						"provenance_check_us", time.Since(checkStart).Microseconds(),
						"jar_count", len(ctx.JARPaths),
						"signed_by", fingerprint,
					)
				}
				return []string{SelectorSignedByPrefix+fingerprint, SelectorSubjectMatchPrefix+"true"}, nil
			}
		}
	}
	if ctx.Logger != nil {
		ctx.Logger.Debug("provenance stage timing",
			"pid", ctx.PID,
			"provenance_check_us", time.Since(checkStart).Microseconds(),
			"jar_count", len(ctx.JARPaths),
			"signed_by", "",
		)
	}
	return nil, nil
}

