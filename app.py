#!/usr/bin/env python3
import os
import sys
import subprocess
import glob
import shutil
from pathlib import Path

import questionary
from questionary import Style

CUSTOM_STYLE = Style([
    ('qmark', 'fg:#00ff00 bold'),
    ('question', 'fg:#ffffff bold'),
    ('answer', 'fg:#00ff00'),
    ('pointer', 'fg:#ffff00 bold'),
    ('highlighted', 'fg:#ffff00 bold'),
    ('selected', 'fg:#00ff00'),
])


BIN_DIR = Path(__file__).parent / "bin"
REALCUGAN_BIN = BIN_DIR / "realcugan-ncnn-vulkan"
REALESRGAN_BIN = BIN_DIR / "realesrgan-ncnn-vulkan"

REALCUGAN_MODELS = ["models-se", "models-pro"]
REALESRGAN_MODELS = ["realesr-animevideov3", "realesrgan-x4plus", "realesrgan-x4plus-anime", "realesrnet-x4plus"]

SCALES = ["2", "3", "4"]
NOISE_LEVELS = ["-1", "0", "1", "2", "3"]

NOISE_TO_MODEL = {
    "models-se": {
        "2": {"-1": "no-denoise", "0": "denoise1x", "1": "denoise2x", "2": "denoise3x", "3": "denoise3x"},
        "3": {"-1": "no-denoise", "0": "denoise1x", "3": "denoise3x"},
        "4": {"-1": "no-denoise", "0": "denoise1x", "3": "denoise3x"},
    },
    "models-pro": {
        "2": {"-1": "conservative", "3": "denoise3x"},
        "3": {"-1": "conservative", "3": "denoise3x"},
    },
}


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
        for suffix in ["-conservative", "-denoise1x", "-denoise2x", "-denoise3x", "-no-denoise"]:
            if name.endswith(suffix):
                models.add(name.replace(suffix, "").replace("up", "up"))
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


def run_command(cmd, verbose=True, cwd=None):
    if cwd is None:
        cwd = BIN_DIR.parent
    if verbose:
        print(f"\n{'='*60}")
        print(f"Running: {' '.join(str(x) for x in cmd)}")
        print(f"CWD: {cwd}")
        print('='*60)
    
    result = subprocess.run(cmd, capture_output=True, text=True, cwd=cwd)
    
    if result.stdout:
        print(result.stdout)
    if result.stderr:
        print(result.stderr)
    
    return result.returncode == 0


def process_realcugan(input_path, output_path, scale, noise_level, model_dir, use_tta=False, gpu_id=None, threads=None):
    cmd = [str(REALCUGAN_BIN), "-i", str(input_path), "-o", str(output_path), 
           "-s", scale, "-n", noise_level, "-m", model_dir]
    
    if use_tta:
        cmd.append("-x")
    if gpu_id is not None:
        cmd.extend(["-g", str(gpu_id)])
    if threads:
        cmd.extend(["-j", threads])
    
    return run_command(cmd)


def process_realesrgan(input_path, output_path, scale, model_name, use_tta=False, gpu_id=None, threads=None):
    cmd = [str(REALESRGAN_BIN), "-i", str(input_path), "-o", str(output_path),
           "-s", scale, "-n", model_name]
    
    if use_tta:
        cmd.append("-x")
    if gpu_id is not None:
        cmd.extend(["-g", str(gpu_id)])
    if threads:
        cmd.extend(["-j", threads])
    
    return run_command(cmd)


def get_image_files(input_path):
    input_path = Path(input_path)
    if input_path.is_file():
        return [input_path]
    
    extensions = {'.jpg', '.jpeg', '.png', '.webp', '.bmp', '.tiff', '.tif'}
    files = []
    for ext in extensions:
        files.extend(input_path.glob(f"*{ext}"))
        files.extend(input_path.glob(f"*{ext.upper()}"))
    return sorted(files)


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
    
    print(f"Will try {len(realcugan_combos)} Real-CUGAN combinations")
    print(f"Will try {len(REALESRGAN_MODELS) * len(SCALES)} Real-ESRGAN combinations")
    total_possible = len(images) * (len(realcugan_combos) + len(REALESRGAN_MODELS) * len(SCALES))
    print(f"Total: {total_possible} outputs\n")
    
    if not questionary.confirm("Continue?").ask():
        return
    
    total = 0
    success = 0
    
    for img in images:
        print(f"\n{'#'*60}")
        print(f"Processing: {img.name}")
        print(f"{'#'*60}")
        
        for model_dir, scale, noise in realcugan_combos:
            output = get_output_path(img, "realcugan", f"{model_dir}_n{noise}_s{scale}", output_dir)
            total += 1
            print(f"\n[{total}] Real-CUGAN | {model_dir} | noise={noise} | scale={scale}x")
            if process_realcugan(img, output, scale, noise, model_dir):
                success += 1
        
        for model_name in REALESRGAN_MODELS:
            for scale in SCALES:
                output = get_output_path(img, "realesrgan", f"{model_name}_s{scale}", output_dir)
                total += 1
                print(f"\n[{total}] Real-ESRGAN | {model_name} | scale={scale}x")
                if process_realesrgan(img, output, scale, model_name):
                    success += 1
    
    print(f"\n{'='*60}")
    print(f"COMPLETED: {success}/{total} successful")
    print(f"{'='*60}")


def main_menu():
    while True:
        choice = questionary.select(
            "Select operation:",
            choices=[
                "Real-CUGAN",
                "Real-ESRGAN",
                "Try All (all programs + all models)",
                "Exit",
            ],
            style=CUSTOM_STYLE,
        ).ask()
        
        if choice == "Exit" or choice is None:
            print("Goodbye!")
            break
        elif choice == "Try All (all programs + all models)":
            try_all_flow()
        elif choice == "Real-CUGAN":
            realcugan_flow()
        elif choice == "Real-ESRGAN":
            realesrgan_flow()


def try_all_flow():
    input_path = questionary.text("Input file or directory:", style=CUSTOM_STYLE).ask()
    if not input_path:
        return
    
    input_path = Path(input_path).expanduser()
    if not input_path.exists():
        print(f"Error: {input_path} does not exist")
        return
    
    use_custom_output = questionary.confirm("Custom output directory?", style=CUSTOM_STYLE).ask()
    output_dir = None
    if use_custom_output:
        output_dir = questionary.text("Output directory:", style=CUSTOM_STYLE).ask()
        if output_dir:
            output_dir = Path(output_dir).expanduser()
    
    try_all(input_path, output_dir)


def realcugan_flow():
    input_path = questionary.text("Input file or directory:", style=CUSTOM_STYLE).ask()
    if not input_path:
        return
    
    input_path = Path(input_path).expanduser()
    if not input_path.exists():
        print(f"Error: {input_path} does not exist")
        return
    
    scale = questionary.select("Scale:", choices=SCALES, style=CUSTOM_STYLE).ask()
    noise = questionary.select("Noise level (-1=no denoise, 0-3=denoise strength):", 
                                choices=NOISE_LEVELS, style=CUSTOM_STYLE).ask()
    model = questionary.select("Model:", choices=REALCUGAN_MODELS, style=CUSTOM_STYLE).ask()
    
    use_tta = questionary.confirm("Enable TTA (Test-Time Augmentation, slower but better quality)?", 
                                  style=CUSTOM_STYLE).ask()
    
    use_custom_gpu = questionary.confirm("Custom GPU?", style=CUSTOM_STYLE).ask()
    gpu_id = None
    if use_custom_gpu:
        gpu_id = questionary.text("GPU ID (-1 for CPU, 0, 1, 2...):", style=CUSTOM_STYLE).ask()
        if gpu_id:
            gpu_id = int(gpu_id)
    
    use_custom_threads = questionary.confirm("Custom thread count?", style=CUSTOM_STYLE).ask()
    threads = None
    if use_custom_threads:
        threads = questionary.text("Thread count (load:proc:save, e.g. 2:2:2):", style=CUSTOM_STYLE).ask()
    
    images = get_image_files(input_path)
    
    print(f"\nFound {len(images)} image(s)")
    print(f"Model: {model}")
    print(f"Scale: {scale}x")
    print(f"Noise level: {noise}")
    print(f"TTA: {'Yes' if use_tta else 'No'}")
    
    for img in images:
        output = get_output_path(img, "realcugan", f"{model}_n{noise}_s{scale}")
        print(f"\nProcessing: {img.name} -> {output.name}")
        process_realcugan(img, output, scale, noise, model, use_tta, gpu_id, threads)
    
    print("\nDone!")


def realesrgan_flow():
    input_path = questionary.text("Input file or directory:", style=CUSTOM_STYLE).ask()
    if not input_path:
        return
    
    input_path = Path(input_path).expanduser()
    if not input_path.exists():
        print(f"Error: {input_path} does not exist")
        return
    
    scale = questionary.select("Scale:", choices=SCALES, style=CUSTOM_STYLE).ask()
    model = questionary.select("Model:", choices=REALESRGAN_MODELS, style=CUSTOM_STYLE).ask()
    
    use_tta = questionary.confirm("Enable TTA (Test-Time Augmentation, slower but better quality)?", 
                                  style=CUSTOM_STYLE).ask()
    
    use_custom_gpu = questionary.confirm("Custom GPU?", style=CUSTOM_STYLE).ask()
    gpu_id = None
    if use_custom_gpu:
        gpu_id = questionary.text("GPU ID (-1 for CPU, 0, 1, 2...):", style=CUSTOM_STYLE).ask()
        if gpu_id:
            gpu_id = int(gpu_id)
    
    use_custom_threads = questionary.confirm("Custom thread count?", style=CUSTOM_STYLE).ask()
    threads = None
    if use_custom_threads:
        threads = questionary.text("Thread count (load:proc:save, e.g. 2:2:2):", style=CUSTOM_STYLE).ask()
    
    images = get_image_files(input_path)
    
    print(f"\nFound {len(images)} image(s)")
    print(f"Model: {model}")
    print(f"Scale: {scale}x")
    print(f"TTA: {'Yes' if use_tta else 'No'}")
    
    for img in images:
        output = get_output_path(img, "realesrgan", f"{model}_s{scale}")
        print(f"\nProcessing: {img.name} -> {output.name}")
        process_realesrgan(img, output, scale, model, use_tta, gpu_id, threads)
    
    print("\nDone!")


if __name__ == "__main__":
    if not REALCUGAN_BIN.exists() or not REALESRGAN_BIN.exists():
        print("Error: Binary files not found in bin/")
        print(f"Expected: {REALCUGAN_BIN}")
        print(f"Expected: {REALESRGAN_BIN}")
        sys.exit(1)
    
    main_menu()
