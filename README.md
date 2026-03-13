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
- Scale: 2x, 3x, 4x
- Noise level: -1 (no denoise), 0, 1, 2, 3
- Models: models-se, models-pro, models-nose
- TTA mode available

**Real-ESRGAN**
- Scale: 2x, 3x, 4x
- Models: realesr-animevideov3, realesrgan-x4plus, realesrgan-x4plus-anime, realesrnet-x4plus
- TTA mode available

**Try All**
- Runs all available programs with all model combinations
- Useful for comparing results

### Output

- Output files are saved in the same directory as input by default
- Filename format: `{original_name}_{app}_{model}.{extension}`
  - Example: `image_realcugan_models-se_n2_s2.png`

## Binary Requirements

The following binaries must be present in the `bin/` directory:
- `realcugan-ncnn-vulkan`
- `realesrgan-ncnn-vulkan`
- Model directories (`models-se/`, `models-pro/`, `models-nose/`)

## Keyboard Controls

- Use arrow keys to navigate menus
- Press Enter to select
- Use Space to toggle checkboxes

## License

This program is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.

See [LICENSE](LICENSE) for full text.
