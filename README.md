# Upscaler TUI

![Upscaler TUI](logo.webp)

A terminal UI for upscaling images with Real-CUGAN and Real-ESRGAN. Two modes: **Guided** (answer a few questions, get a great result) and **Expert** (full control over every parameter).

## Prerequisites

1. Python 3.8+
2. Vulkan runtime (required by the binaries)

Install dependencies:

```bash
pip install -r requirements.txt
```

## Usage

```bash
python app.py
```

Or, if installed via `pip install -e .`:

```bash
upscaler
```

## Modes

### Guided mode

The easiest way to get started. Answer four questions and the tool picks the right model for you.

```
What are you upscaling?
  ❯ Photo / realistic image
    Illustration / digital art
    Anime / manga
    Not sure — use the best

How much do you want to enlarge?
  ❯ 4x (recommended)

Quality:
  ❯ Balanced (faster)
    Maximum quality (slower, ~8x the time)
```

You can save your settings as a named preset and reload them next time.

Output filenames use a readable format: `photo_guided_photo_4x.png`

### Expert mode

Full control. Choose between Real-CUGAN and Real-ESRGAN, then configure model, scale, noise reduction, TTA, GPU, and thread count manually.

### Try All

Runs every valid model/scale/noise combination and shows a summary table at the end. Useful for comparing results and picking the best model for your image.

## Models

**Real-CUGAN**

| Model | Scale | Noise levels |
|-------|-------|--------------|
| Standard (SE) | 2x, 3x, 4x | No reduction, light, medium, strong, very strong (2x); no reduction, light, very strong (3x, 4x) |
| Professional (Pro) | 2x, 3x | No reduction, very strong |

Best for anime and manga.

**Real-ESRGAN**

| Model | Scale | Best for |
|-------|-------|----------|
| Realistic photos — general purpose | 4x | Photos, realistic images |
| Anime / illustrations — sharp lines | 4x | Digital art, illustrations |
| Anime video — for video frames | 2x, 3x, 4x | Anime video frames |
| Realistic photos — variant | 4x | Photos (alternative model) |

Note: models trained for 4x only will produce cropped output at lower scales.

## Output files

By default, output files are saved next to the input. You can specify a custom output directory in any mode.

Filename format:
- Guided: `{name}_guided_{type}_{scale}x.{ext}` — e.g. `photo_guided_photo_4x.png`
- Expert: `{name}_realcugan_{model}_n{noise}_s{scale}.{ext}` — e.g. `photo_realcugan_models-se_n-1_s4.png`

## Saved presets

Guided mode lets you save your settings (image type, scale, quality) as a named preset in `~/.upscaler/presets.json`. On the next run, you can load a preset to skip the questions.

## Keyboard controls

- Arrow keys to navigate
- Enter to select
- Ctrl+C to abort at any point

## Binary requirements

The `bin/` directory must contain:
- `realcugan-ncnn-vulkan`
- `realesrgan-ncnn-vulkan`
- Model directories: `models-se/`, `models-pro/` (Real-CUGAN), `models/` (Real-ESRGAN)

All models are already included in the repository, downloaded from the [Real-CUGAN releases](https://github.com/xinntao/Real-CUGAN-ncnn-vulkan/releases) and [Real-ESRGAN releases](https://github.com/xinntao/Real-ESRGAN-ncnn-vulkan/releases).

## License

This program is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.

See [LICENSE](LICENSE) for full text.
