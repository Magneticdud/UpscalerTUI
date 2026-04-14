package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// Jpeg2pngResult holds the temp dir and converted PNG path.
// If conversion was skipped or failed, both fields are empty string.
type Jpeg2pngResult struct {
	TmpDir  string
	PngPath string
}

// RunJpeg2png converts a JPEG to PNG via jpeg2png in a temp dir.
// Returns a result with TmpDir+PngPath on success; both empty on skip/failure.
// Caller must call result.Cleanup() when done.
//
// Skips (returns empty) if:
//   - input is not a JPEG
//   - jpeg2png binary is missing
//   - conversion fails
func RunJpeg2png(ctx context.Context, inputPath string) (Jpeg2pngResult, error) {
	ext := strings.ToLower(filepath.Ext(inputPath))
	if ext != ".jpg" && ext != ".jpeg" {
		return Jpeg2pngResult{}, nil
	}

	binDir, err := BinDir()
	if err != nil {
		return Jpeg2pngResult{}, nil
	}
	bin := filepath.Join(binDir, BinaryName("jpeg2png"))
	if _, err := os.Stat(bin); err != nil {
		return Jpeg2pngResult{}, nil
	}

	tmpDir, err := os.MkdirTemp("", "upscaler-j2p-*")
	if err != nil {
		return Jpeg2pngResult{}, err
	}

	// Copy the input JPEG into the temp dir
	base := filepath.Base(inputPath)
	tmpJpg := filepath.Join(tmpDir, base)
	if err := copyFile(inputPath, tmpJpg); err != nil {
		os.RemoveAll(tmpDir)
		return Jpeg2pngResult{}, err
	}

	stem := strings.TrimSuffix(base, filepath.Ext(base))
	tmpPng := filepath.Join(tmpDir, stem+".png")

	cmd := newCommand(ctx, bin, tmpJpg)
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return Jpeg2pngResult{}, nil // treat failure as skip
	}

	if _, err := os.Stat(tmpPng); err != nil {
		os.RemoveAll(tmpDir)
		return Jpeg2pngResult{}, nil
	}

	return Jpeg2pngResult{TmpDir: tmpDir, PngPath: tmpPng}, nil
}

// Cleanup removes the temporary directory created by RunJpeg2png.
func (r Jpeg2pngResult) Cleanup() {
	if r.TmpDir != "" {
		os.RemoveAll(r.TmpDir)
	}
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
