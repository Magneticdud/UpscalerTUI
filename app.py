#!/usr/bin/env python3
import glob
import json
import os
import shutil
import signal
import subprocess
import sys
import tempfile
from pathlib import Path

import questionary
from questionary import Style
from rich.console import Console
from rich.table import Table

CUSTOM_STYLE = Style(
    [
        ("qmark", "fg:#00ff00 bold"),
        ("question", "fg:#ffffff bold"),
        ("answer", "fg:#00ff00"),
        ("pointer", "fg:#ffff00 bold"),
        ("highlighted", "fg:#ffff00 bold"),
        ("selected", "fg:#00ff00"),
    ]
)


BIN_DIR = Path(__file__).parent / "bin"
REALCUGAN_BIN = BIN_DIR / "realcugan-ncnn-vulkan"
REALESRGAN_BIN = BIN_DIR / "realesrgan-ncnn-vulkan"
JPEG2PNG_BIN = BIN_DIR / "jpeg2png"

PRESETS_FILE = Path(__file__).parent / "presets.json"

REALCUGAN_MODELS = ["models-se", "models-pro"]
REALESRGAN_MODELS = [
    "realesr-animevideov3",
    "realesrgan-x4plus",
    "realesrgan-x4plus-anime",
    "realesrnet-x4plus",
]

REALESRGAN_MODEL_SCALES = {
    "realesr-animevideov3": ["2", "3", "4"],
    "realesrgan-x4plus": ["4"],
    "realesrgan-x4plus-anime": ["4"],
    "realesrnet-x4plus": ["4"],
}

# These models crash the AMD iGPU with default tile size; use a smaller tile
REALESRGAN_TILE_SIZES = {
    "realesrgan-x4plus": 100,
    "realesrnet-x4plus": 100,
}

SCALES = ["2", "3", "4"]
NOISE_LEVELS = ["-1", "0", "1", "2", "3"]

NOISE_TO_MODEL = {
    "models-se": {
        "2": {
            "-1": "no-denoise",
            "0": "denoise1x",
            "1": "denoise2x",
            "2": "denoise3x",
            "3": "denoise3x",
        },
        "3": {"-1": "no-denoise", "0": "denoise1x", "3": "denoise3x"},
        "4": {"-1": "no-denoise", "0": "denoise1x", "3": "denoise3x"},
    },
    "models-pro": {
        "2": {"-1": "conservative", "3": "denoise3x"},
        "3": {"-1": "conservative", "3": "denoise3x"},
    },
}

# ── Readable labels for Expert mode ──────────────────────────────────────────

REALCUGAN_MODEL_LABELS = {
    "models-se": "Standard (SE)",
    "models-pro": "Professional (Pro) — more precise, slower",
}

REALESRGAN_MODEL_LABELS = {
    "realesr-animevideov3": "Anime video — for video frames",
    "realesrnet-x4plus": "Realistic photos — general purpose",
    "realesrgan-x4plus-anime": "Anime / illustrations — sharp lines",
    "realesrgan-x4plus": "Realistic photos — variant",
}

NOISE_LABELS = {
    "-1": "No noise reduction",
    "0": "Light reduction",
    "1": "Medium reduction",
    "2": "Strong reduction",
    "3": "Very strong reduction",
}

# ── jpeg2png applicability ────────────────────────────────────────────────────
# jpeg2png works well for flat/line art; poor for photographs

JPEG2PNG_GUIDED_TYPES = {"illustration", "anime"}

JPEG2PNG_MODELS = {
    "models-se",
    "models-pro",
    "realesrgan-x4plus-anime",
    "realesr-animevideov3",
}

# ── Guided mode presets ───────────────────────────────────────────────────────

IMAGE_TYPE_PRESETS = {
    "photo": {
        "engine": "realesrgan",
        "model": "realesrnet-x4plus",
        "scale": "4",
        "noise": None,
    },
    "illustration": {
        "engine": "realesrgan",
        "model": "realesrgan-x4plus-anime",
        "scale": "4",
        "noise": None,
    },
    "anime": {
        "engine": "realcugan",
        "model": "models-se",
        "scale": "2",
        "noise": "-1",
    },
    "not_sure": {
        "engine": "realesrgan",
        "model": "realesrnet-x4plus",
        "scale": "4",
        "noise": None,
    },
}

GUIDED_VALID_SCALES = {
    "photo": ["4"],
    "illustration": ["4"],
    "anime": ["2", "3", "4"],
    "not_sure": ["4"],
}

IMAGE_TYPE_LABELS = {
    "photo": "Photo / realistic image",
    "illustration": "Illustration / digital art",
    "anime": "Anime / manga",
    "not_sure": "Not sure — use the best",
}


# ── Signal handler ────────────────────────────────────────────────────────────


def _sigint_handler(_sig, _frame):
    print("\nAborted.")
    sys.exit(0)


# ── jpeg2png pre-processing ───────────────────────────────────────────────────


def run_jpeg2png(img_path):
    """Convert a JPEG to PNG via jpeg2png in a temp dir.

    Returns (tmp_dir, png_path) on success; caller must shutil.rmtree(tmp_dir).
    Returns (None, None) if the image is not a JPEG, the binary is missing,
    or conversion fails — caller should use the original path unchanged.
    """
    if img_path.suffix.lower() not in {".jpg", ".jpeg"}:
        return None, None
    if not JPEG2PNG_BIN.exists():
        return None, None

    tmp_dir = Path(tempfile.mkdtemp())
    tmp_jpg = tmp_dir / img_path.name
    shutil.copy2(img_path, tmp_jpg)

    result = subprocess.run(
        [str(JPEG2PNG_BIN), str(tmp_jpg)],
        capture_output=True,
        text=True,
        errors="replace",
    )

    tmp_png = tmp_dir / (img_path.stem + ".png")
    if result.returncode == 0 and tmp_png.exists():
        return tmp_dir, tmp_png

    shutil.rmtree(tmp_dir, ignore_errors=True)
    return None, None


# ── Pure helpers ──────────────────────────────────────────────────────────────


def get_valid_noise_levels(model_dir, scale):
    return list(NOISE_TO_MODEL.get(model_dir, {}).get(scale, {}).keys())


def get_available_realcugan_combinations(model_dir):
    combinations = []
    model_scales = NOISE_TO_MODEL.get(model_dir, {}).keys()
    for scale in model_scales:
        valid_noises = get_valid_noise_levels(model_dir, scale)
        for noise in valid_noises:
            combinations.append((scale, noise))
    return combinations


def get_realcugan_model_files(model_dir):
    model_path = BIN_DIR / model_dir
    if not model_path.exists():
        return []
    files = glob.glob(str(model_path / "*.bin"))
    models = set()
    for f in files:
        name = Path(f).stem
        for suffix in [
            "-conservative",
            "-denoise1x",
            "-denoise2x",
            "-denoise3x",
            "-no-denoise",
        ]:
            if name.endswith(suffix):
                models.add(name[: -len(suffix)])
                break
    return sorted(models)


def get_output_path(input_path, app_name, model_name, output_dir=None):
    input_path = Path(input_path)
    if output_dir:
        output_dir = Path(output_dir)
    else:
        output_dir = input_path.parent

    stem = input_path.stem
    ext = input_path.suffix

    output_name = f"{stem}_{app_name}_{model_name}{ext}"
    return output_dir / output_name


def get_image_files(input_path):
    input_path = Path(input_path)
    if input_path.is_file():
        return [input_path]

    extensions = {".jpg", ".jpeg", ".png", ".webp", ".bmp", ".tiff", ".tif"}
    files = []
    for ext in extensions:
        files.extend(input_path.glob(f"*{ext}"))
        files.extend(input_path.glob(f"*{ext.upper()}"))
    return sorted(files)


# ── Preset persistence ────────────────────────────────────────────────────────


def load_presets():
    try:
        with open(PRESETS_FILE) as f:
            return json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        return {}


def save_preset(name, preset):
    presets = load_presets()
    if name in presets:
        confirm = questionary.confirm(
            f"Preset '{name}' already exists. Overwrite?", style=CUSTOM_STYLE
        ).ask()
        if not confirm:
            return
    presets[name] = preset
    try:
        PRESETS_FILE.parent.mkdir(parents=True, exist_ok=True)
        with open(PRESETS_FILE, "w") as f:
            json.dump(presets, f, indent=2)
        print(f"Preset '{name}' saved.")
    except OSError as e:
        print(f"Warning: could not save preset: {e}")


# ── Command execution ─────────────────────────────────────────────────────────


def run_command(cmd, cwd=None, show_command=True):
    if cwd is None:
        cwd = BIN_DIR.parent
    if show_command:
        print(f"\n{'=' * 60}")
        print(f"Running: {' '.join(str(x) for x in cmd)}")
        print(f"CWD: {cwd}")
        print("=" * 60)
        result = subprocess.run(
            cmd, capture_output=True, text=True, cwd=cwd, errors="replace"
        )
        if result.stdout:
            print(result.stdout)
        if result.stderr:
            print(result.stderr)
    else:
        # Guided mode: stream directly to terminal, no buffering
        result = subprocess.run(cmd, cwd=cwd, errors="replace")

    return result.returncode == 0


def process_realcugan(
    input_path,
    output_path,
    scale,
    noise_level,
    model_dir,
    use_tta=False,
    gpu_id=None,
    threads=None,
    show_command=True,
):
    cmd = [
        str(REALCUGAN_BIN),
        "-i",
        str(input_path),
        "-o",
        str(output_path),
        "-s",
        scale,
        "-n",
        noise_level,
        "-m",
        model_dir,
    ]

    if use_tta:
        cmd.append("-x")
    if gpu_id is not None:
        cmd.extend(["-g", str(gpu_id)])
    if threads:
        cmd.extend(["-j", threads])

    return run_command(cmd, show_command=show_command)


def process_realesrgan(
    input_path,
    output_path,
    scale,
    model_name,
    use_tta=False,
    gpu_id=None,
    threads=None,
    show_command=True,
):
    cmd = [
        str(REALESRGAN_BIN),
        "-i",
        str(input_path),
        "-o",
        str(output_path),
        "-s",
        scale,
        "-n",
        model_name,
    ]

    tile_size = REALESRGAN_TILE_SIZES.get(model_name)
    if tile_size is not None:
        cmd.extend(["-t", str(tile_size)])
    if use_tta:
        cmd.append("-x")
    if gpu_id is not None:
        cmd.extend(["-g", str(gpu_id)])
    if threads:
        cmd.extend(["-j", threads])

    return run_command(cmd, show_command=show_command)


# ── Shared prompt helpers ─────────────────────────────────────────────────────


def ask_advanced_options():
    """Ask GPU and thread options. Returns (gpu_id, threads) or (None, None) on abort."""
    use_custom_gpu = questionary.confirm("Custom GPU?", style=CUSTOM_STYLE).ask()
    if use_custom_gpu is None:
        return None, None
    gpu_id = None
    if use_custom_gpu:
        raw = questionary.text(
            "GPU ID (-1 for CPU, 0, 1, 2...):", style=CUSTOM_STYLE
        ).ask()
        if raw is None:
            return None, None
        gpu_id = int(raw)

    use_custom_threads = questionary.confirm(
        "Custom thread count?", style=CUSTOM_STYLE
    ).ask()
    if use_custom_threads is None:
        return gpu_id, None
    threads = None
    if use_custom_threads:
        threads = questionary.text(
            "Thread count (load:proc:save, e.g. 2:2:2):", style=CUSTOM_STYLE
        ).ask()
        if threads is None:
            return gpu_id, None

    return gpu_id, threads


def ask_output_dir():
    """Ask for optional custom output directory. Returns Path or None."""
    use_custom = questionary.confirm(
        "Custom output directory?", style=CUSTOM_STYLE
    ).ask()
    if not use_custom:
        return None
    raw = questionary.text("Output directory:", style=CUSTOM_STYLE).ask()
    if not raw:
        return None
    output_dir = Path(raw).expanduser()
    output_dir.mkdir(parents=True, exist_ok=True)
    return output_dir


# ── Guided flow ───────────────────────────────────────────────────────────────


def guided_flow():
    # Offer to load a saved preset
    presets = load_presets()
    image_type = None
    scale = None
    use_tta = None

    if presets:
        use_preset = questionary.confirm(
            f"You have {len(presets)} saved preset(s). Load one?", style=CUSTOM_STYLE
        ).ask()
        if use_preset is None:
            return
        if use_preset:
            preset_names = list(presets.keys())
            chosen = questionary.select(
                "Choose preset:", choices=preset_names, style=CUSTOM_STYLE
            ).ask()
            if chosen is None:
                return
            config = presets[chosen]
            image_type = config["image_type"]
            scale = config["scale"]
            use_tta = config["tta"]

    input_path = questionary.text("Input file or directory:", style=CUSTOM_STYLE).ask()
    if not input_path:
        return
    input_path = Path(input_path).expanduser()
    if not input_path.exists():
        print(f"Error: {input_path} does not exist")
        return

    if image_type is None:
        type_choices = [
            questionary.Choice(IMAGE_TYPE_LABELS[k], value=k)
            for k in IMAGE_TYPE_PRESETS
        ]
        image_type = questionary.select(
            "What are you upscaling?", choices=type_choices, style=CUSTOM_STYLE
        ).ask()
        if image_type is None:
            return

    if scale is None:
        valid_scales = GUIDED_VALID_SCALES[image_type]
        default_scale = IMAGE_TYPE_PRESETS[image_type]["scale"]
        scale_choices = [
            questionary.Choice(
                f"{s}x" + (" (recommended)" if s == default_scale else ""), value=s
            )
            for s in valid_scales
        ]
        scale = questionary.select(
            "How much do you want to enlarge?",
            choices=scale_choices,
            style=CUSTOM_STYLE,
        ).ask()
        if scale is None:
            return

    if use_tta is None:
        quality_choices = [
            questionary.Choice("Balanced (faster)", value=False),
            questionary.Choice("Maximum quality (slower, ~8x the time)", value=True),
        ]
        use_tta = questionary.select(
            "Quality:", choices=quality_choices, style=CUSTOM_STYLE
        ).ask()
        if use_tta is None:
            return

    output_dir = ask_output_dir()

    # Offer to save as preset
    save_as = questionary.confirm("Save as preset?", style=CUSTOM_STYLE).ask()
    if save_as is None:
        return
    if save_as:
        preset_name = questionary.text("Preset name:", style=CUSTOM_STYLE).ask()
        if preset_name:
            save_preset(
                preset_name, {"image_type": image_type, "scale": scale, "tta": use_tta}
            )

    preset = IMAGE_TYPE_PRESETS[image_type]
    engine = preset["engine"]
    model = preset["model"]
    noise = preset["noise"]

    images = get_image_files(input_path)

    print(f"\nFound {len(images)} image(s)")
    print(f"Type:    {IMAGE_TYPE_LABELS[image_type]}")
    print(f"Scale:   {scale}x")
    print(f"Quality: {'Maximum (TTA)' if use_tta else 'Balanced'}")
    print(f"Engine:  {engine} / {model}\n")

    use_jpeg2png = image_type in JPEG2PNG_GUIDED_TYPES

    for img in images:
        tmp_dir, src = run_jpeg2png(img) if use_jpeg2png else (None, None)
        src = src or img

        output = get_output_path(img, "guided", f"{image_type}_{scale}x", output_dir)
        if tmp_dir:
            output = output.with_suffix(".png")
        if output_dir:
            output.parent.mkdir(parents=True, exist_ok=True)

        if tmp_dir:
            print(f"  jpeg2png: {img.name} → {src.name}")
        print(f"Processing: {img.name} -> {output.name}")

        try:
            if engine == "realcugan":
                process_realcugan(
                    src, output, scale, noise, model, use_tta, show_command=False
                )
            else:
                process_realesrgan(
                    src, output, scale, model, use_tta, show_command=False
                )
        finally:
            if tmp_dir:
                shutil.rmtree(tmp_dir, ignore_errors=True)

    print("\nDone!")


# ── Expert flow ───────────────────────────────────────────────────────────────


def expert_flow():
    engine_choices = [
        questionary.Choice("Real-CUGAN", value="realcugan"),
        questionary.Choice("Real-ESRGAN", value="realesrgan"),
    ]
    engine = questionary.select(
        "Which program?", choices=engine_choices, style=CUSTOM_STYLE
    ).ask()
    if engine is None:
        return
    if engine == "realcugan":
        realcugan_flow()
    else:
        realesrgan_flow()


# ── Try All ───────────────────────────────────────────────────────────────────


def try_all(input_path, output_dir=None):
    input_path = Path(input_path)
    images = get_image_files(input_path)

    if not images:
        print(f"No images found in {input_path}")
        return

    print(f"\nFound {len(images)} image(s) to process")

    realcugan_combos = []
    for model_dir in REALCUGAN_MODELS:
        combos = get_available_realcugan_combinations(model_dir)
        for scale, noise in combos:
            realcugan_combos.append((model_dir, scale, noise))

    realesrgan_combos = []
    for model_name in REALESRGAN_MODELS:
        scales = REALESRGAN_MODEL_SCALES.get(model_name, SCALES)
        for scale in scales:
            realesrgan_combos.append((model_name, scale))

    print(f"Will try {len(realcugan_combos)} Real-CUGAN combinations")
    print(f"Will try {len(realesrgan_combos)} Real-ESRGAN combinations")
    total_possible = len(images) * (len(realcugan_combos) + len(realesrgan_combos))
    print(f"Total: {total_possible} outputs\n")

    if not questionary.confirm("Continue?").ask():
        return

    total = 0
    success = 0
    results = []

    for img in images:
        print(f"\n{'#' * 60}")
        print(f"Processing: {img.name}")
        print(f"{'#' * 60}")

        # Run jpeg2png once per image; reuse for all compatible models
        j2p_dir, j2p_src = run_jpeg2png(img)
        if j2p_dir:
            print(f"  jpeg2png: {img.name} → {j2p_src.name}")

        try:
            for model_dir, scale, noise in realcugan_combos:
                src = j2p_src if j2p_dir else img
                output = get_output_path(
                    img, "realcugan", f"{model_dir}_n{noise}_s{scale}", output_dir
                )
                if j2p_dir:
                    output = output.with_suffix(".png")
                total += 1
                print(
                    f"\n[{total}] Real-CUGAN | {model_dir} | noise={noise} | scale={scale}x"
                )
                ok = process_realcugan(src, output, scale, noise, model_dir)
                if ok:
                    success += 1
                noise_label = NOISE_LABELS.get(noise, noise)
                model_label = (
                    f"{REALCUGAN_MODEL_LABELS.get(model_dir, model_dir)}, {noise_label}"
                )
                results.append((output.name, model_label, f"{scale}x", ok))

            for model_name, scale in realesrgan_combos:
                use_j2p = j2p_dir and model_name in JPEG2PNG_MODELS
                src = j2p_src if use_j2p else img
                output = get_output_path(
                    img, "realesrgan", f"{model_name}_s{scale}", output_dir
                )
                if use_j2p:
                    output = output.with_suffix(".png")
                total += 1
                print(f"\n[{total}] Real-ESRGAN | {model_name} | scale={scale}x")
                ok = process_realesrgan(src, output, scale, model_name)
                if ok:
                    success += 1
                model_label = REALESRGAN_MODEL_LABELS.get(model_name, model_name)
                results.append((output.name, model_label, f"{scale}x", ok))

        finally:
            if j2p_dir:
                shutil.rmtree(j2p_dir, ignore_errors=True)

    # Rich summary table (after all questionary prompts are done)
    console = Console()
    table = Table(title=f"Results — {success}/{total} completed")
    table.add_column("Output file", style="cyan", no_wrap=False)
    table.add_column("Model", style="white")
    table.add_column("Scale", style="yellow", justify="center")
    table.add_column("Status", justify="center")

    for fname, model_label, scale, ok in results:
        status_str = "[green]OK[/green]" if ok else "[red]FAIL[/red]"
        table.add_row(fname, model_label, scale, status_str)

    console.print(table)


# ── Individual expert flows ───────────────────────────────────────────────────


def try_all_flow():
    input_path = questionary.text("Input file or directory:", style=CUSTOM_STYLE).ask()
    if not input_path:
        return

    input_path = Path(input_path).expanduser()
    if not input_path.exists():
        print(f"Error: {input_path} does not exist")
        return

    output_dir = ask_output_dir()
    try_all(input_path, output_dir)


def realcugan_flow():
    input_path = questionary.text("Input file or directory:", style=CUSTOM_STYLE).ask()
    if not input_path:
        return

    input_path = Path(input_path).expanduser()
    if not input_path.exists():
        print(f"Error: {input_path} does not exist")
        return

    model_choices = [
        questionary.Choice(REALCUGAN_MODEL_LABELS[m], value=m) for m in REALCUGAN_MODELS
    ]
    model = questionary.select(
        "Model:", choices=model_choices, style=CUSTOM_STYLE
    ).ask()
    if model is None:
        return

    valid_scales = list(NOISE_TO_MODEL.get(model, {}).keys())
    scale = questionary.select("Scale:", choices=valid_scales, style=CUSTOM_STYLE).ask()
    if scale is None:
        return

    valid_noises = get_valid_noise_levels(model, scale)
    noise_choices = [
        questionary.Choice(NOISE_LABELS.get(n, n), value=n) for n in valid_noises
    ]
    noise = questionary.select(
        "Noise reduction:", choices=noise_choices, style=CUSTOM_STYLE
    ).ask()
    if noise is None:
        return

    use_tta = questionary.confirm(
        "Enable TTA (slower, maximum quality)?", style=CUSTOM_STYLE
    ).ask()
    if use_tta is None:
        return

    gpu_id, threads = ask_advanced_options()

    output_dir = ask_output_dir()

    images = get_image_files(input_path)

    print(f"\nFound {len(images)} image(s)")
    print(f"Model: {REALCUGAN_MODEL_LABELS[model]}")
    print(f"Scale: {scale}x")
    print(f"Noise: {NOISE_LABELS.get(noise, noise)}")
    print(f"TTA: {'Yes' if use_tta else 'No'}")

    for img in images:
        tmp_dir, src = run_jpeg2png(img)
        src = src or img

        output = get_output_path(
            img, "realcugan", f"{model}_n{noise}_s{scale}", output_dir
        )
        if tmp_dir:
            output = output.with_suffix(".png")
        if output_dir:
            output.parent.mkdir(parents=True, exist_ok=True)

        if tmp_dir:
            print(f"\n  jpeg2png: {img.name} → {src.name}")
        print(f"\nProcessing: {img.name} -> {output.name}")

        try:
            process_realcugan(
                src, output, scale, noise, model, use_tta, gpu_id, threads
            )
        finally:
            if tmp_dir:
                shutil.rmtree(tmp_dir, ignore_errors=True)

    print("\nDone!")


def realesrgan_flow():
    input_path = questionary.text("Input file or directory:", style=CUSTOM_STYLE).ask()
    if not input_path:
        return

    input_path = Path(input_path).expanduser()
    if not input_path.exists():
        print(f"Error: {input_path} does not exist")
        return

    model_choices = [
        questionary.Choice(REALESRGAN_MODEL_LABELS[m], value=m)
        for m in REALESRGAN_MODELS
    ]
    model = questionary.select(
        "Model:", choices=model_choices, style=CUSTOM_STYLE
    ).ask()
    if model is None:
        return

    valid_scales = REALESRGAN_MODEL_SCALES.get(model, SCALES)
    scale = questionary.select("Scale:", choices=valid_scales, style=CUSTOM_STYLE).ask()
    if scale is None:
        return

    use_tta = questionary.confirm(
        "Enable TTA (slower, maximum quality)?", style=CUSTOM_STYLE
    ).ask()
    if use_tta is None:
        return

    gpu_id, threads = ask_advanced_options()

    output_dir = ask_output_dir()

    images = get_image_files(input_path)

    print(f"\nFound {len(images)} image(s)")
    print(f"Model: {REALESRGAN_MODEL_LABELS[model]}")
    print(f"Scale: {scale}x")
    print(f"TTA: {'Yes' if use_tta else 'No'}")

    use_jpeg2png = model in JPEG2PNG_MODELS

    for img in images:
        tmp_dir, src = run_jpeg2png(img) if use_jpeg2png else (None, None)
        src = src or img

        output = get_output_path(img, "realesrgan", f"{model}_s{scale}", output_dir)
        if tmp_dir:
            output = output.with_suffix(".png")
        if output_dir:
            output.parent.mkdir(parents=True, exist_ok=True)

        if tmp_dir:
            print(f"\n  jpeg2png: {img.name} → {src.name}")
        print(f"\nProcessing: {img.name} -> {output.name}")

        try:
            process_realesrgan(src, output, scale, model, use_tta, gpu_id, threads)
        finally:
            if tmp_dir:
                shutil.rmtree(tmp_dir, ignore_errors=True)

    print("\nDone!")


# ── Main menu ─────────────────────────────────────────────────────────────────


def main_menu():
    while True:
        choice = questionary.select(
            "Upscaler TUI",
            choices=[
                questionary.Choice(
                    "Guided mode  — choose what you're upscaling", value="guided"
                ),
                questionary.Choice(
                    "Expert mode  — full control over parameters", value="expert"
                ),
                questionary.Choice(
                    "Try All      — run all models and compare", value="try_all"
                ),
                questionary.Choice("Exit", value="exit"),
            ],
            style=CUSTOM_STYLE,
        ).ask()

        if choice == "exit" or choice is None:
            print("Goodbye!")
            break
        elif choice == "guided":
            guided_flow()
        elif choice == "expert":
            expert_flow()
        elif choice == "try_all":
            try_all_flow()


# ── Entry point ───────────────────────────────────────────────────────────────


def main():
    if not REALCUGAN_BIN.exists() or not REALESRGAN_BIN.exists():
        print("Error: Binary files not found in bin/")
        print(f"Expected: {REALCUGAN_BIN}")
        print(f"Expected: {REALESRGAN_BIN}")
        sys.exit(1)

    signal.signal(signal.SIGINT, _sigint_handler)
    main_menu()


if __name__ == "__main__":
    main()
