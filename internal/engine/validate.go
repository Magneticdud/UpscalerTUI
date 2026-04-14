package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Magneticdud/UpscalerGUI/internal/config"
)

// MissingBinaries checks that the required binaries exist in bin/.
// Returns a list of missing binary names (empty = all present).
func MissingBinaries() ([]string, error) {
	binDir, err := BinDir()
	if err != nil {
		return nil, err
	}
	required := []string{
		BinaryName("realcugan-ncnn-vulkan"),
		BinaryName("realesrgan-ncnn-vulkan"),
	}
	var missing []string
	for _, b := range required {
		if _, err := os.Stat(filepath.Join(binDir, b)); err != nil {
			missing = append(missing, b)
		}
	}
	return missing, nil
}

// MissingModels checks that model directories referenced in constants exist in bin/.
// Returns a list of missing model directory names.
func MissingModels() ([]string, error) {
	binDir, err := BinDir()
	if err != nil {
		return nil, err
	}
	// Check RealCUGAN model dirs
	required := append([]string{}, config.RealcuganModels...)
	// Check RealESRGAN models dir (all models live in bin/models/)
	required = append(required, "models")

	var missing []string
	for _, m := range required {
		if _, err := os.Stat(filepath.Join(binDir, m)); err != nil {
			missing = append(missing, m)
		}
	}
	return missing, nil
}

// ValidateGPUID returns an error if the GPU ID string is not valid.
// Valid: empty string, or integer >= -1.
func ValidateGPUID(s string) error {
	if s == "" {
		return nil
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil || fmt.Sprintf("%d", n) != s {
		return fmt.Errorf("GPU ID must be an integer (e.g. 0, 1, -1 for CPU)")
	}
	if n < -1 {
		return fmt.Errorf("GPU ID must be >= -1")
	}
	return nil
}

// ValidateThreads returns an error if the threads string is not valid.
// Valid: empty string, or "load:proc:save" with all integers > 0.
func ValidateThreads(s string) error {
	if s == "" {
		return nil
	}
	var load, proc, save int
	n, err := fmt.Sscanf(s, "%d:%d:%d", &load, &proc, &save)
	if err != nil || n != 3 || fmt.Sprintf("%d:%d:%d", load, proc, save) != s {
		return fmt.Errorf("threads must be in format load:proc:save (e.g. 2:2:2)")
	}
	if load <= 0 || proc <= 0 || save <= 0 {
		return fmt.Errorf("thread counts must be > 0")
	}
	return nil
}
