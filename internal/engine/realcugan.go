package engine

import (
	"context"
	"path/filepath"
)

// RealcuganOptions configures a Real-CUGAN upscaling job.
type RealcuganOptions struct {
	InputPath  string
	OutputPath string
	Scale      string
	NoiseLevel string
	ModelDir   string
	TTA        bool
	GPUID      string // empty = default
	Threads    string // empty = default, format "load:proc:save"
}

// ProcessRealcugan runs realcugan-ncnn-vulkan and streams output to logCh.
// Returns true on success.
func ProcessRealcugan(ctx context.Context, logCh chan<- string, opts RealcuganOptions) bool {
	binDir, err := BinDir()
	if err != nil {
		return false
	}
	bin := filepath.Join(binDir, BinaryName("realcugan-ncnn-vulkan"))

	args := []string{
		"-i", opts.InputPath,
		"-o", opts.OutputPath,
		"-s", opts.Scale,
		"-n", opts.NoiseLevel,
		"-m", opts.ModelDir,
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
