package engine

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Magneticdud/UpscalerGUI/internal/config"
)

// RealesrganOptions configures a Real-ESRGAN upscaling job.
type RealesrganOptions struct {
	InputPath  string
	OutputPath string
	Scale      string
	ModelName  string
	TTA        bool
	GPUID      string // empty = default
	Threads    string // empty = default
}

// ProcessRealesrgan runs realesrgan-ncnn-vulkan and streams output to logCh.
// Returns true on success.
func ProcessRealesrgan(ctx context.Context, logCh chan<- string, opts RealesrganOptions) bool {
	binDir, err := BinDir()
	if err != nil {
		return false
	}
	bin := filepath.Join(binDir, BinaryName("realesrgan-ncnn-vulkan"))

	args := []string{
		"-i", opts.InputPath,
		"-o", opts.OutputPath,
		"-s", opts.Scale,
		"-n", opts.ModelName,
	}
	if tileSize, ok := config.RealesrganTileSizes[opts.ModelName]; ok {
		args = append(args, "-t", fmt.Sprintf("%d", tileSize))
	}
	if opts.TTA {
		args = append(args, "-x")
	}
	if opts.GPUID != "" {
		args = append(args, "-g", opts.GPUID)
	}
	if opts.Threads != "" {
		args = append(args, "-j", opts.Threads)
	}

	return runBinary(ctx, logCh, bin, args...)
}
