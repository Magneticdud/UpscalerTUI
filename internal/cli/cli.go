package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Magneticdud/UpscalerGUI/internal/config"
	"github.com/Magneticdud/UpscalerGUI/internal/engine"
)

func Run() {
	fs := flag.NewFlagSet("upscaler", flag.ExitOnError)

	input   := fs.String("input",   "", "Input file or directory (required)")
	typeKey := fs.String("type",    "", "Guided mode image type: photo, illustration, anime, not_sure")
	eng     := fs.String("engine",  "", "Expert mode engine: realcugan, realesrgan")
	model   := fs.String("model",   "", "Expert mode model name")
	scale   := fs.String("scale",   "", "Scale factor: 2, 3, 4 (default from preset in guided mode)")
	noise   := fs.String("noise",   "", "Noise level for realcugan: -1, 0, 1, 2, 3")
	tta     := fs.Bool("tta",       false, "Enable TTA mode (slower, maximum quality)")
	gpu     := fs.String("gpu",     "", "GPU ID (-1 for CPU, 0, 1, …)")
	threads := fs.String("threads", "", "Thread count (e.g. 2:2:2)")
	output  := fs.String("output",  "", "Output directory (default: same as input)")
	tryAll  := fs.Bool("try-all",   false, "Run all model combinations and show a results summary")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  Guided:  upscaler --input <path> --type <photo|illustration|anime|not_sure> [--scale N] [--tta] [--output <dir>]")
		fmt.Fprintln(os.Stderr, "  Expert:  upscaler --input <path> --engine <realcugan|realesrgan> --model <model> --scale <N> [--noise <N>] [--tta] [--gpu <id>] [--threads <spec>] [--output <dir>]")
		fmt.Fprintln(os.Stderr, "  Try All: upscaler --input <path> --try-all [--output <dir>]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Flags:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Models:")
		fmt.Fprintln(os.Stderr, "  realcugan:  models-se, models-pro")
		fmt.Fprintln(os.Stderr, "  realesrgan: realesr-animevideov3, realesrgan-x4plus, realesrgan-x4plus-anime, realesrnet-x4plus")
	}

	fs.Parse(os.Args[1:])

	if *input == "" {
		fmt.Fprintln(os.Stderr, "error: --input is required")
		fs.Usage()
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Fprintln(os.Stderr, "\nAborted.")
		cancel()
	}()

	switch {
	case *tryAll:
		runTryAll(ctx, *input, *output)
	case *typeKey != "":
		runGuided(ctx, *input, *typeKey, *scale, *output, *tta)
	case *eng != "":
		runExpert(ctx, *input, *eng, *model, *scale, *noise, *output, *gpu, *threads, *tta)
	default:
		fmt.Fprintln(os.Stderr, "error: specify --type, --engine, or --try-all")
		fs.Usage()
		os.Exit(1)
	}
}

// drainToStdout prints every line from logCh to stdout.
func drainToStdout(logCh <-chan string) {
	for line := range logCh {
		fmt.Println(line)
	}
}

// runGuided runs the preset-based guided flow.
func runGuided(ctx context.Context, input, typeKey, scale, output string, tta bool) {
	preset, ok := config.ImageTypePresets[typeKey]
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unknown type %q — valid values: photo, illustration, anime, not_sure\n", typeKey)
		os.Exit(1)
	}

	if scale == "" {
		scale = preset.Scale
	} else {
		valid := config.GuidedValidScales[typeKey]
		if !contains(valid, scale) {
			fmt.Fprintf(os.Stderr, "error: scale %q not valid for type %q (valid: %v)\n", scale, typeKey, valid)
			os.Exit(1)
		}
	}

	images, err := engine.GetImageFiles(input)
	if err != nil || len(images) == 0 {
		fmt.Fprintf(os.Stderr, "error: no images found at %q\n", input)
		os.Exit(1)
	}

	fmt.Printf("Found %d image(s) — type: %s, scale: %sx, engine: %s/%s\n",
		len(images), config.ImageTypeLabels[typeKey], scale, preset.Engine, preset.Model)

	useJ2p := config.Jpeg2pngGuidedTypes[typeKey]
	allOK := true

	for _, img := range images {
		fmt.Printf("\nProcessing: %s\n", filepath.Base(img))

		var srcPath string
		var j2p engine.Jpeg2pngResult
		if useJ2p {
			j2p, _ = engine.RunJpeg2png(ctx, img)
		}
		if j2p.PngPath != "" {
			srcPath = j2p.PngPath
			fmt.Printf("  jpeg2png: %s → %s\n", filepath.Base(img), filepath.Base(srcPath))
		} else {
			srcPath = img
		}

		tag := typeKey + "_" + scale + "x"
		outPath := engine.GetOutputPath(img, "guided", tag, output)
		if j2p.PngPath != "" {
			outPath = replaceExt(outPath, ".png")
		}

		logCh := make(chan string, 256)
		go drainToStdout(logCh)

		var ok bool
		if preset.Engine == "realcugan" {
			ok = engine.ProcessRealcugan(ctx, logCh, engine.RealcuganOptions{
				InputPath:  srcPath,
				OutputPath: outPath,
				Scale:      scale,
				NoiseLevel: preset.Noise,
				ModelDir:   preset.Model,
				TTA:        tta,
			})
		} else {
			ok = engine.ProcessRealesrgan(ctx, logCh, engine.RealesrganOptions{
				InputPath:  srcPath,
				OutputPath: outPath,
				Scale:      scale,
				ModelName:  preset.Model,
				TTA:        tta,
			})
		}
		close(logCh)
		j2p.Cleanup()

		if ok {
			fmt.Printf("  → %s\n", filepath.Base(outPath))
		} else {
			fmt.Fprintf(os.Stderr, "  failed: %s\n", filepath.Base(img))
			allOK = false
		}
	}

	if allOK {
		fmt.Println("\nDone!")
	} else {
		fmt.Fprintln(os.Stderr, "\nCompleted with errors.")
		os.Exit(1)
	}
}

// runExpert runs the fully manual expert flow.
func runExpert(ctx context.Context, input, eng, model, scale, noise, output, gpu, threads string, tta bool) {
	if model == "" {
		fmt.Fprintln(os.Stderr, "error: --model is required in expert mode")
		os.Exit(1)
	}
	if scale == "" {
		fmt.Fprintln(os.Stderr, "error: --scale is required in expert mode")
		os.Exit(1)
	}
	if eng != "realcugan" && eng != "realesrgan" {
		fmt.Fprintf(os.Stderr, "error: unknown engine %q — use realcugan or realesrgan\n", eng)
		os.Exit(1)
	}
	if eng == "realcugan" && noise == "" {
		fmt.Fprintln(os.Stderr, "error: --noise is required for realcugan (values: -1, 0, 1, 2, 3)")
		os.Exit(1)
	}
	if err := engine.ValidateGPUID(gpu); err != nil {
		fmt.Fprintf(os.Stderr, "error: --gpu: %v\n", err)
		os.Exit(1)
	}
	if err := engine.ValidateThreads(threads); err != nil {
		fmt.Fprintf(os.Stderr, "error: --threads: %v\n", err)
		os.Exit(1)
	}

	images, err := engine.GetImageFiles(input)
	if err != nil || len(images) == 0 {
		fmt.Fprintf(os.Stderr, "error: no images found at %q\n", input)
		os.Exit(1)
	}

	fmt.Printf("Found %d image(s) — engine: %s, model: %s, scale: %sx\n",
		len(images), eng, model, scale)

	useJ2p := config.Jpeg2pngModels[model]
	allOK := true

	for _, img := range images {
		fmt.Printf("\nProcessing: %s\n", filepath.Base(img))

		var srcPath string
		var j2p engine.Jpeg2pngResult
		if useJ2p {
			j2p, _ = engine.RunJpeg2png(ctx, img)
		}
		if j2p.PngPath != "" {
			srcPath = j2p.PngPath
			fmt.Printf("  jpeg2png: %s → %s\n", filepath.Base(img), filepath.Base(srcPath))
		} else {
			srcPath = img
		}

		logCh := make(chan string, 256)
		go drainToStdout(logCh)

		var outPath string
		var ok bool
		if eng == "realcugan" {
			tag := model + "_n" + noise + "_s" + scale
			outPath = engine.GetOutputPath(img, "realcugan", tag, output)
			if j2p.PngPath != "" {
				outPath = replaceExt(outPath, ".png")
			}
			ok = engine.ProcessRealcugan(ctx, logCh, engine.RealcuganOptions{
				InputPath:  srcPath,
				OutputPath: outPath,
				Scale:      scale,
				NoiseLevel: noise,
				ModelDir:   model,
				TTA:        tta,
				GPUID:      gpu,
				Threads:    threads,
			})
		} else {
			tag := model + "_s" + scale
			outPath = engine.GetOutputPath(img, "realesrgan", tag, output)
			if j2p.PngPath != "" {
				outPath = replaceExt(outPath, ".png")
			}
			ok = engine.ProcessRealesrgan(ctx, logCh, engine.RealesrganOptions{
				InputPath:  srcPath,
				OutputPath: outPath,
				Scale:      scale,
				ModelName:  model,
				TTA:        tta,
				GPUID:      gpu,
				Threads:    threads,
			})
		}
		close(logCh)
		j2p.Cleanup()

		if ok {
			fmt.Printf("  → %s\n", filepath.Base(outPath))
		} else {
			fmt.Fprintf(os.Stderr, "  failed: %s\n", filepath.Base(img))
			allOK = false
		}
	}

	if allOK {
		fmt.Println("\nDone!")
	} else {
		fmt.Fprintln(os.Stderr, "\nCompleted with errors.")
		os.Exit(1)
	}
}

// runTryAll runs every model/scale/noise combination on the input.
func runTryAll(ctx context.Context, input, output string) {
	images, err := engine.GetImageFiles(input)
	if err != nil || len(images) == 0 {
		fmt.Fprintf(os.Stderr, "error: no images found at %q\n", input)
		os.Exit(1)
	}

	type rcCombo struct{ modelDir, scale, noise string }
	type esrCombo struct{ modelName, scale string }

	var rcCombos []rcCombo
	for _, modelDir := range config.RealcuganModels {
		for _, s := range config.GetRealcuganScales(modelDir) {
			for _, n := range config.GetValidNoiseLevels(modelDir, s) {
				rcCombos = append(rcCombos, rcCombo{modelDir, s, n})
			}
		}
	}
	var esrCombos []esrCombo
	for _, modelName := range config.RealesrganModels {
		for _, s := range config.RealesrganModelScales[modelName] {
			esrCombos = append(esrCombos, esrCombo{modelName, s})
		}
	}

	total := len(images) * (len(rcCombos) + len(esrCombos))
	fmt.Printf("Found %d image(s) × %d combinations = %d outputs\n\n",
		len(images), len(rcCombos)+len(esrCombos), total)

	type result struct {
		filename   string
		modelLabel string
		scale      string
		ok         bool
	}
	var results []result
	count := 0

	for _, img := range images {
		fmt.Printf("\n### %s\n", filepath.Base(img))

		j2p, _ := engine.RunJpeg2png(ctx, img)
		if j2p.PngPath != "" {
			fmt.Printf("  jpeg2png: %s → %s\n", filepath.Base(img), filepath.Base(j2p.PngPath))
		}

		for _, rc := range rcCombos {
			count++
			src := img
			if j2p.PngPath != "" {
				src = j2p.PngPath
			}
			tag := rc.modelDir + "_n" + rc.noise + "_s" + rc.scale
			outPath := engine.GetOutputPath(img, "realcugan", tag, output)
			if j2p.PngPath != "" {
				outPath = replaceExt(outPath, ".png")
			}
			fmt.Printf("[%d] Real-CUGAN | %s | noise=%s | scale=%sx\n", count, rc.modelDir, rc.noise, rc.scale)

			logCh := make(chan string, 256)
			go drainToStdout(logCh)
			ok := engine.ProcessRealcugan(ctx, logCh, engine.RealcuganOptions{
				InputPath:  src,
				OutputPath: outPath,
				Scale:      rc.scale,
				NoiseLevel: rc.noise,
				ModelDir:   rc.modelDir,
			})
			close(logCh)

			label := config.RealcuganModelLabels[rc.modelDir] + ", " + config.NoiseLabels[rc.noise]
			results = append(results, result{filepath.Base(outPath), label, rc.scale + "x", ok})
		}

		for _, esr := range esrCombos {
			count++
			src := img
			useJ2p := j2p.PngPath != "" && config.Jpeg2pngModels[esr.modelName]
			if useJ2p {
				src = j2p.PngPath
			}
			tag := esr.modelName + "_s" + esr.scale
			outPath := engine.GetOutputPath(img, "realesrgan", tag, output)
			if useJ2p {
				outPath = replaceExt(outPath, ".png")
			}
			fmt.Printf("[%d] Real-ESRGAN | %s | scale=%sx\n", count, esr.modelName, esr.scale)

			logCh := make(chan string, 256)
			go drainToStdout(logCh)
			ok := engine.ProcessRealesrgan(ctx, logCh, engine.RealesrganOptions{
				InputPath:  src,
				OutputPath: outPath,
				Scale:      esr.scale,
				ModelName:  esr.modelName,
			})
			close(logCh)

			label := config.RealesrganModelLabels[esr.modelName]
			results = append(results, result{filepath.Base(outPath), label, esr.scale + "x", ok})
		}

		j2p.Cleanup()
	}

	// Summary table
	ok := 0
	for _, r := range results {
		if r.ok {
			ok++
		}
	}
	fmt.Printf("\nResults: %d/%d completed\n", ok, len(results))
	fmt.Printf("%-50s  %-40s  %-6s  %s\n", "Output file", "Model", "Scale", "Status")
	fmt.Printf("%-50s  %-40s  %-6s  %s\n", "─────────────────────────────────────────────────", "───────────────────────────────────────", "──────", "──────")
	for _, r := range results {
		status := "OK"
		if !r.ok {
			status = "FAIL"
		}
		fmt.Printf("%-50s  %-40s  %-6s  %s\n", r.filename, r.modelLabel, r.scale, status)
	}

	if ok < len(results) {
		os.Exit(1)
	}
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func replaceExt(path, ext string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[:i] + ext
		}
	}
	return path + ext
}
