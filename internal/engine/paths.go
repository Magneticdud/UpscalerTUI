package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// BinDir returns the absolute path to the bin/ directory next to the executable.
func BinDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "bin"), nil
}

// BinaryName returns the OS-appropriate binary name (appends .exe on Windows).
func BinaryName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// GetOutputPath builds the output file path, mirroring the Python app's naming.
//
// For guided mode:   {stem}_guided_{tag}{ext}
// For expert mode:   {stem}_{appName}_{modelTag}{ext}
func GetOutputPath(inputPath, appName, modelTag string, outputDir string) string {
	stem := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	ext := strings.ToLower(filepath.Ext(inputPath))
	name := stem + "_" + appName + "_" + modelTag + ext

	dir := outputDir
	if dir == "" {
		dir = filepath.Dir(inputPath)
	}
	return filepath.Join(dir, name)
}

// GetImageFiles returns all image files at path (file or directory).
// Extensions are normalised to lowercase. Duplicates (e.g. foo.jpg + foo.JPG
// on case-insensitive filesystems) are deduplicated by canonical lowercase path.
func GetImageFiles(inputPath string) ([]string, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if isImageExt(filepath.Ext(inputPath)) {
			return []string{inputPath}, nil
		}
		return []string{}, nil
	}

	entries, err := os.ReadDir(inputPath)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !isImageExt(ext) {
			continue
		}
		// Dedup key: lowercase full path
		key := strings.ToLower(filepath.Join(inputPath, e.Name()))
		if seen[key] {
			continue
		}
		seen[key] = true
		files = append(files, filepath.Join(inputPath, e.Name()))
	}
	return files, nil
}

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true,
	".webp": true, ".bmp": true, ".tiff": true, ".tif": true,
}

func isImageExt(ext string) bool {
	return imageExts[strings.ToLower(ext)]
}
