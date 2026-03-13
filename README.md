# Upscaler TUI

A simple Python TUI (Terminal User Interface) for using Real-CUGAN and Real-ESRGAN image upscaling tools.

## Prerequisites

1. Python 3.8+
2. Vulkan runtime (required by the binaries)

Install Python dependencies:

```bash
pip install -r requirements.txt
```

## Usage

Run the TUI:

```bash
python app.py
```

### Options

**Real-CUGAN**

| Model | Scale | Noise Levels |
|-------|-------|--------------|
| models-se | 2x, 3x, 4x | -1, 0, 1, 2, 3 (2x), -1, 0, 3 (3x, 4x) |
| models-pro | 2x, 3x | -1, 3 |

- Scale: 2x, 3x, 4x
- Noise level: -1 (no denoise), 0, 1, 2, 3 (denoise strength)
- TTA mode available

**Real-ESRGAN**

| Model | Supported Scales |
|-------|------------------|
| realesr-animevideov3 | 2x, 3x, 4x |
| realesrgan-x4plus | 4x only |
| realesrgan-x4plus-anime | 4x only |
| realesrnet-x4plus | 4x only |

Note: Models marked as "x4" are trained specifically for 4x upscaling. Using them with lower scales produces cropped output.

**Try All**
- Runs all available programs with all valid model combinations
- Useful for comparing results

### Output

- Output files are saved in the same directory as input by default
- Filename format: `{original_name}_{app}_{model}{suffix}.{extension}`
  - Example: `image_realcugan_models-se_n2_s2.png`

## Binary Requirements

The following binaries must be present in the `bin/` directory:
- `realcugan-ncnn-vulkan`
- `realesrgan-ncnn-vulkan`
- Model directories for Real-CUGAN: `models-se/`, `models-pro/`
- Model directories for Real-ESRGAN: `models/`

All models are already included in the repository. I downloaded them from the [Real-CUGAN releases](https://github.com/xinntao/Real-CUGAN-ncnn-vulkan/releases) and [Real-ESRGAN releases](https://github.com/xinntao/Real-ESRGAN-ncnn-vulkan/releases). If I missed any, please let me know.

## Keyboard Controls

- Use arrow keys to navigate menus
- Press Enter to select
- Use Space to toggle checkboxes

## License

This program is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.

See [LICENSE](LICENSE) for full text.
