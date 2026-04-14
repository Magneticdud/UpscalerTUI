package config

// RealCUGAN models
var RealcuganModels = []string{"models-se", "models-pro"}

// RealESRGAN models
var RealesrganModels = []string{
	"realesr-animevideov3",
	"realesrgan-x4plus",
	"realesrgan-x4plus-anime",
	"realesrnet-x4plus",
}

// Valid scales per RealESRGAN model
var RealesrganModelScales = map[string][]string{
	"realesr-animevideov3":    {"2", "3", "4"},
	"realesrgan-x4plus":      {"4"},
	"realesrgan-x4plus-anime": {"4"},
	"realesrnet-x4plus":      {"4"},
}

// Tile size override for models that crash AMD iGPU with default tile
var RealesrganTileSizes = map[string]int{
	"realesrgan-x4plus":  100,
	"realesrnet-x4plus":  100,
}

// Noise level → model file suffix mapping for RealCUGAN
// noise level → model suffix
var NoisToModel = map[string]map[string]map[string]string{
	"models-se": {
		"2": {
			"-1": "no-denoise",
			"0":  "denoise1x",
			"1":  "denoise2x",
			"2":  "denoise3x",
			"3":  "denoise3x",
		},
		"3": {"-1": "no-denoise", "0": "denoise1x", "3": "denoise3x"},
		"4": {"-1": "no-denoise", "0": "denoise1x", "3": "denoise3x"},
	},
	"models-pro": {
		"2": {"-1": "conservative", "3": "denoise3x"},
		"3": {"-1": "conservative", "3": "denoise3x"},
	},
}

// Human-readable labels
var RealcuganModelLabels = map[string]string{
	"models-se":  "Standard (SE)",
	"models-pro": "Professional (Pro) — more precise, slower",
}

var RealesrganModelLabels = map[string]string{
	"realesr-animevideov3":    "Anime video — for video frames",
	"realesrnet-x4plus":      "Realistic photos — general purpose",
	"realesrgan-x4plus-anime": "Anime / illustrations — sharp lines",
	"realesrgan-x4plus":      "Realistic photos — variant",
}

var NoiseLevels = []string{"-1", "0", "1", "2", "3"}

var NoiseLabels = map[string]string{
	"-1": "No noise reduction",
	"0":  "Light reduction",
	"1":  "Medium reduction",
	"2":  "Strong reduction",
	"3":  "Very strong reduction",
}

// jpeg2png applicability
var Jpeg2pngGuidedTypes = map[string]bool{
	"illustration": true,
	"anime":        true,
}

var Jpeg2pngModels = map[string]bool{
	"models-se":               true,
	"models-pro":              true,
	"realesrgan-x4plus-anime": true,
	"realesr-animevideov3":    true,
}

// Guided mode presets
type GuidedPreset struct {
	Engine string
	Model  string
	Scale  string
	Noise  string // empty string means not applicable
}

var ImageTypePresets = map[string]GuidedPreset{
	"photo": {
		Engine: "realesrgan",
		Model:  "realesrnet-x4plus",
		Scale:  "4",
	},
	"illustration": {
		Engine: "realesrgan",
		Model:  "realesrgan-x4plus-anime",
		Scale:  "4",
	},
	"anime": {
		Engine: "realcugan",
		Model:  "models-se",
		Scale:  "2",
		Noise:  "-1",
	},
	"not_sure": {
		Engine: "realesrgan",
		Model:  "realesrnet-x4plus",
		Scale:  "4",
	},
}

var GuidedValidScales = map[string][]string{
	"photo":        {"4"},
	"illustration": {"4"},
	"anime":        {"2", "3", "4"},
	"not_sure":     {"4"},
}

// Order matters for display
var ImageTypeOrder = []string{"photo", "illustration", "anime", "not_sure"}

var ImageTypeLabels = map[string]string{
	"photo":        "Photo / realistic image",
	"illustration": "Illustration / digital art",
	"anime":        "Anime / manga",
	"not_sure":     "Not sure — use the best",
}

// GetValidNoiseLevels returns the valid noise levels for a RealCUGAN model+scale combo.
func GetValidNoiseLevels(modelDir, scale string) []string {
	scaleMap, ok := NoisToModel[modelDir]
	if !ok {
		return nil
	}
	noiseMap, ok := scaleMap[scale]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(noiseMap))
	// Preserve order
	for _, n := range NoiseLevels {
		if _, ok := noiseMap[n]; ok {
			result = append(result, n)
		}
	}
	return result
}

// GetRealcuganScales returns the valid scales for a RealCUGAN model.
func GetRealcuganScales(modelDir string) []string {
	scaleMap, ok := NoisToModel[modelDir]
	if !ok {
		return nil
	}
	scales := make([]string, 0, len(scaleMap))
	for _, s := range []string{"2", "3", "4"} {
		if _, ok := scaleMap[s]; ok {
			scales = append(scales, s)
		}
	}
	return scales
}
