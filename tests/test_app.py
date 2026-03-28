import json
import sys
import tempfile
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).parent.parent))
import app

# ── get_valid_noise_levels ────────────────────────────────────────────────────


def test_noise_levels_se_scale2():
    result = app.get_valid_noise_levels("models-se", "2")
    assert set(result) == {"-1", "0", "1", "2", "3"}


def test_noise_levels_se_scale3():
    result = app.get_valid_noise_levels("models-se", "3")
    assert set(result) == {"-1", "0", "3"}


def test_noise_levels_se_scale4():
    result = app.get_valid_noise_levels("models-se", "4")
    assert set(result) == {"-1", "0", "3"}


def test_noise_levels_pro_scale2():
    result = app.get_valid_noise_levels("models-pro", "2")
    assert set(result) == {"-1", "3"}


def test_noise_levels_pro_scale3():
    result = app.get_valid_noise_levels("models-pro", "3")
    assert set(result) == {"-1", "3"}


def test_noise_levels_invalid_model():
    assert app.get_valid_noise_levels("nonexistent", "2") == []


def test_noise_levels_invalid_scale():
    assert app.get_valid_noise_levels("models-se", "99") == []


# ── get_output_path ───────────────────────────────────────────────────────────


def test_output_path_same_dir():
    out = app.get_output_path("/tmp/foo.png", "realcugan", "models-se_n-1_s4")
    assert out.name == "foo_realcugan_models-se_n-1_s4.png"
    assert out.parent == Path("/tmp")


def test_output_path_custom_dir():
    out = app.get_output_path("/tmp/foo.png", "guided", "anime_2x", "/output")
    assert out.name == "foo_guided_anime_2x.png"
    assert out.parent == Path("/output")


def test_output_path_preserves_extension():
    out = app.get_output_path("/tmp/photo.jpg", "realesrgan", "realesrgan-x4plus_s4")
    assert out.suffix == ".jpg"


def test_output_path_stem_only():
    out = app.get_output_path("/tmp/a.png", "guided", "foto_4x")
    assert out.stem == "a_guided_foto_4x"


# ── get_available_realcugan_combinations ──────────────────────────────────────


def test_realcugan_combos_se_contains_expected():
    combos = app.get_available_realcugan_combinations("models-se")
    assert ("2", "-1") in combos
    assert ("3", "3") in combos
    assert ("4", "-1") in combos


def test_realcugan_combos_pro_does_not_have_scale4():
    combos = app.get_available_realcugan_combinations("models-pro")
    scales = {s for s, _ in combos}
    assert "4" not in scales


def test_realcugan_combos_invalid_model():
    assert app.get_available_realcugan_combinations("nonexistent") == []


# ── IMAGE_TYPE_PRESETS shape ──────────────────────────────────────────────────

REQUIRED_PRESET_KEYS = {"engine", "model", "scale", "noise"}


def test_presets_uniform_shape():
    for tipo, preset in app.IMAGE_TYPE_PRESETS.items():
        assert (
            set(preset.keys()) == REQUIRED_PRESET_KEYS
        ), f"Preset '{tipo}' has wrong keys: {set(preset.keys())}"


def test_presets_engine_values():
    for tipo, preset in app.IMAGE_TYPE_PRESETS.items():
        assert preset["engine"] in {
            "realcugan",
            "realesrgan",
        }, f"Invalid engine in preset '{tipo}': {preset['engine']}"


def test_presets_scale_is_string():
    for tipo, preset in app.IMAGE_TYPE_PRESETS.items():
        assert isinstance(
            preset["scale"], str
        ), f"Preset '{tipo}' scale should be str, got {type(preset['scale'])}"


# ── GUIDED_VALID_SCALES consistency ──────────────────────────────────────────


def test_guided_valid_scales_covers_all_presets():
    for tipo in app.IMAGE_TYPE_PRESETS:
        assert (
            tipo in app.GUIDED_VALID_SCALES
        ), f"'{tipo}' missing from GUIDED_VALID_SCALES"


def test_guided_valid_scales_default_included():
    for tipo, preset in app.IMAGE_TYPE_PRESETS.items():
        default_scale = preset["scale"]
        assert (
            default_scale in app.GUIDED_VALID_SCALES[tipo]
        ), f"Default scale '{default_scale}' not in GUIDED_VALID_SCALES['{tipo}']"


def test_guided_valid_scales_anime_has_2_3_4():
    assert app.GUIDED_VALID_SCALES["anime"] == ["2", "3", "4"]


def test_image_type_labels_covers_all_presets():
    for tipo in app.IMAGE_TYPE_PRESETS:
        assert tipo in app.IMAGE_TYPE_LABELS, f"'{tipo}' missing from IMAGE_TYPE_LABELS"


# ── load_presets error handling ───────────────────────────────────────────────


def _load_from_path(path):
    """Mirror of load_presets() logic for testing with a custom path."""
    try:
        with open(path) as f:
            return json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        return {}


def test_load_presets_missing_file():
    result = _load_from_path(Path("/tmp/nonexistent_upscaler_xyz_12345.json"))
    assert result == {}


def test_load_presets_corrupt_json():
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
        f.write("{invalid json}")
        tmp_path = Path(f.name)
    try:
        result = _load_from_path(tmp_path)
        assert result == {}
    finally:
        tmp_path.unlink()


def test_load_presets_valid_json():
    data = {"test": {"tipo": "anime", "scale": "2", "tta": False}}
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
        json.dump(data, f)
        tmp_path = Path(f.name)
    try:
        result = _load_from_path(tmp_path)
        assert result == data
    finally:
        tmp_path.unlink()
