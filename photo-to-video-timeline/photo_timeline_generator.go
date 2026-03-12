package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type Config struct {
	BaseDir            string
	StartDate          string
	FPS                int
	StartTime          string
	CacheDir           string
	HistoricDir        string
	FontPath           string
	FontSize           float64
	Workers            int
	JPEGQuality        int
	KeepCacheOnSuccess bool
	FFmpegPath         string
	Verbose            bool
}

type SelectedFile struct {
	Path     string
	BaseName string
	Label    string
	Size     int64
	DatePart string
	TimePart string
}

type Job struct {
	Index int
	File  SelectedFile
}

type Result struct {
	Index   int
	OutPath string
	Err     error
}

var timeRe = regexp.MustCompile(`_(\d{6})\.`)

func main() {
	cfg, err := parseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func parseFlags() (Config, error) {
	var cfg Config

	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")

	flag.StringVar(&cfg.BaseDir, "base-dir", "/home/resager/rudb/mount/2tb-ext-part/rudb/dev/android/api/storage/photo", "base directory with YYYY-MM-DD folders")
	flag.StringVar(&cfg.StartDate, "date", yesterday, "start folder date in YYYY-MM-DD")
	flag.IntVar(&cfg.FPS, "fps", 10, "frames per second in output video")
	flag.StringVar(&cfg.StartTime, "start-time", "06:00:00", "time boundary in HH:MM:SS")
	flag.StringVar(&cfg.CacheDir, "cache-dir", "./cache", "directory for temporary files")
	flag.StringVar(&cfg.HistoricDir, "historic-dir", "./historic", "directory for output videos")
	flag.StringVar(&cfg.FontPath, "font", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", "ttf font path; empty uses embedded Go font")
	flag.Float64Var(&cfg.FontSize, "font-size", 24, "font size for frame labels")
	flag.IntVar(&cfg.Workers, "workers", max(runtime.NumCPU(), 1), "number of image processing workers")
	flag.IntVar(&cfg.JPEGQuality, "jpeg-quality", 92, "jpeg quality for temporary frames")
	flag.BoolVar(&cfg.KeepCacheOnSuccess, "keep-cache", false, "keep temporary frames after successful build")
	flag.StringVar(&cfg.FFmpegPath, "ffmpeg", "ffmpeg", "ffmpeg executable path")
	flag.BoolVar(&cfg.Verbose, "v", true, "verbose logging")
	flag.Parse()

	if cfg.FPS <= 0 {
		return cfg, errors.New("fps must be > 0")
	}
	if cfg.Workers <= 0 {
		return cfg, errors.New("workers must be > 0")
	}
	if cfg.JPEGQuality < 1 || cfg.JPEGQuality > 100 {
		return cfg, errors.New("jpeg-quality must be in range 1..100")
	}
	if _, err := time.Parse("2006-01-02", cfg.StartDate); err != nil {
		return cfg, fmt.Errorf("invalid date: %w", err)
	}
	if _, err := time.Parse("15:04:05", cfg.StartTime); err != nil {
		return cfg, fmt.Errorf("invalid start-time: %w", err)
	}

	return cfg, nil
}

func run(cfg Config) error {
	if err := os.MkdirAll(cfg.CacheDir, 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	if err := os.MkdirAll(cfg.HistoricDir, 0o755); err != nil {
		return fmt.Errorf("create historic dir: %w", err)
	}

	cacheAbs, err := filepath.Abs(cfg.CacheDir)
	if err != nil {
		return err
	}
	historicAbs, err := filepath.Abs(cfg.HistoricDir)
	if err != nil {
		return err
	}
	baseAbs, err := filepath.Abs(cfg.BaseDir)
	if err != nil {
		return err
	}

	runTS := time.Now().Format("20060102_150405")
	logPath := filepath.Join(cacheAbs, fmt.Sprintf("make_timeline_%s_%s.log", cfg.StartDate, runTS))
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()

	logger := log.New(io.MultiWriter(os.Stdout, logFile), "", log.LstdFlags)
	logger.Printf("Start")

	startDate, _ := time.Parse("2006-01-02", cfg.StartDate)
	nextDate := startDate.Add(24 * time.Hour)
	startDir := filepath.Join(baseAbs, cfg.StartDate)
	nextDir := filepath.Join(baseAbs, nextDate.Format("2006-01-02"))

	logger.Printf("Log file          : %s", logPath)
	logger.Printf("Start date        : %s", cfg.StartDate)
	logger.Printf("Next date         : %s", nextDate.Format("2006-01-02"))
	logger.Printf("FPS               : %d", cfg.FPS)
	logger.Printf("Start time        : %s", cfg.StartTime)
	logger.Printf("Base dir          : %s", baseAbs)
	logger.Printf("Start dir         : %s", startDir)
	logger.Printf("Next dir          : %s", nextDir)
	logger.Printf("Workers           : %d", cfg.Workers)

	if st, err := os.Stat(startDir); err != nil || !st.IsDir() {
		return fmt.Errorf("start dir not found: %s", startDir)
	}
	if st, err := os.Stat(nextDir); err != nil || !st.IsDir() {
		return fmt.Errorf("next dir not found: %s", nextDir)
	}

	boundary, err := hhmmss(cfg.StartTime)
	if err != nil {
		return err
	}

	selected, err := collectSelectedFiles(startDir, nextDir, boundary)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return errors.New("no files selected")
	}

	var selectedBytes int64
	for _, f := range selected {
		selectedBytes += f.Size
	}
	logger.Printf("Selected files    : %d", len(selected))
	logger.Printf("Selected size     : %s", humanSize(selectedBytes))

	tmpImgDir := filepath.Join(cacheAbs, fmt.Sprintf("frames_%s_%s", cfg.StartDate, runTS))
	if err := os.MkdirAll(tmpImgDir, 0o755); err != nil {
		return fmt.Errorf("create temp image dir: %w", err)
	}
	logger.Printf("Temp image dir    : %s", tmpImgDir)

	var cleanupOK bool
	defer func() {
		if cleanupOK && !cfg.KeepCacheOnSuccess {
			_ = os.RemoveAll(tmpImgDir)
		}
	}()

	fontData, err := loadFontData(cfg)
	if err != nil {
		return err
	}

	processed, tmpBytes, err := renderFrames(selected, tmpImgDir, fontData, cfg, logger)
	if err != nil {
		return err
	}
	logger.Printf("Temporary size    : %s", humanSize(tmpBytes))

	inputPath := filepath.Join(cacheAbs, fmt.Sprintf("input_%s_%s.txt", cfg.StartDate, runTS))
	if err := writeConcatInput(inputPath, processed, cfg.FPS); err != nil {
		return err
	}
	defer func() {
		if cleanupOK && !cfg.KeepCacheOnSuccess {
			_ = os.Remove(inputPath)
		}
	}()

	outputPath := filepath.Join(historicAbs, fmt.Sprintf(
		"timeline_%s_%dfps_from_%s.mp4",
		nextDate.Format("2006-01-02"),
		cfg.FPS,
		strings.ReplaceAll(cfg.StartTime, ":", "-"),
	))

	logger.Printf("Building video    : %s", outputPath)
	if err := runFFmpeg(cfg.FFmpegPath, inputPath, outputPath, logger); err != nil {
		return err
	}

	st, err := os.Stat(outputPath)
	if err != nil {
		return fmt.Errorf("stat output video: %w", err)
	}
	logger.Printf("Video size        : %s", humanSize(st.Size()))
	logger.Printf("Done              : %s", outputPath)

	cleanupOK = true
	return nil
}

func collectSelectedFiles(startDir, nextDir string, boundary int) ([]SelectedFile, error) {
	first, err := collectFromDir(startDir, func(t int) bool { return t >= boundary })
	if err != nil {
		return nil, err
	}
	second, err := collectFromDir(nextDir, func(t int) bool { return t < boundary })
	if err != nil {
		return nil, err
	}
	selected := append(first, second...)
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Path < selected[j].Path
	})
	return selected, nil
}

func collectFromDir(dir string, keep func(int) bool) ([]SelectedFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}
	var out []SelectedFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".jpg") && !strings.HasSuffix(strings.ToLower(name), ".jpeg") {
			continue
		}
		m := timeRe.FindStringSubmatch(name)
		if len(m) != 2 {
			continue
		}
		t, _ := strconv.Atoi(m[1])
		if !keep(t) {
			continue
		}
		full := filepath.Join(dir, name)
		st, err := os.Stat(full)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", full, err)
		}
		out = append(out, SelectedFile{
			Path:     full,
			BaseName: name,
			Label:    stripAllExtensions(name),
			Size:     st.Size(),
			TimePart: m[1],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func renderFrames(files []SelectedFile, tmpDir string, fontData []byte, cfg Config, logger *log.Logger) ([]string, int64, error) {
	jobs := make(chan Job)
	results := make(chan Result, cfg.Workers)

	var totalBytes int64
	var counter atomic.Int64

	var wg sync.WaitGroup
	for w := 0; w < cfg.Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			face, err := newFontFaceFromData(fontData, cfg.FontSize)
			if err != nil {
				results <- Result{Err: err}
				return
			}
			defer face.Close()

			for job := range jobs {
				outPath := filepath.Join(tmpDir, fmt.Sprintf("%06d_%s", job.Index+1, job.File.BaseName))
				err := renderSingleFrame(job.File.Path, outPath, job.File.Label, face, cfg.JPEGQuality)
				if err == nil {
					if st, statErr := os.Stat(outPath); statErr == nil {
						atomic.AddInt64(&totalBytes, st.Size())
					}
				}
				results <- Result{Index: job.Index, OutPath: outPath, Err: err}
				_ = counter.Add(1)
			}
		}()
	}

	go func() {
		for i, f := range files {
			jobs <- Job{Index: i, File: f}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	processed := make([]string, len(files))
	completed := 0
	for res := range results {
		if res.Err != nil {
			return nil, 0, res.Err
		}
		completed++
		if cfg.Verbose {
			logger.Printf("[%d/%d] %s", completed, len(files), filepath.Base(res.OutPath))
		}
		processed[res.Index] = res.OutPath
	}

	return processed, totalBytes, nil
}

func renderSingleFrame(inPath, outPath, label string, face font.Face, quality int) error {
	f, err := os.Open(inPath)
	if err != nil {
		return fmt.Errorf("open input %s: %w", inPath, err)
	}
	defer f.Close()

	src, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("decode image %s: %w", inPath, err)
	}

	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)

	// background box near bottom center
	paddingX := 14
	paddingY := 10
	metrics := face.Metrics()
	textHeight := (metrics.Ascent + metrics.Descent).Ceil()
	textWidth := font.MeasureString(face, label).Ceil()

	rectW := textWidth + paddingX*2
	rectH := textHeight + paddingY*2

	x0 := bounds.Min.X + (bounds.Dx()-rectW)/2
	if x0 < bounds.Min.X {
		x0 = bounds.Min.X
	}
	y0 := bounds.Max.Y - rectH - 12
	if y0 < bounds.Min.Y {
		y0 = bounds.Min.Y
	}
	rect := image.Rect(x0, y0, x0+rectW, y0+rectH)
	draw.Draw(dst, rect, &image.Uniform{C: color.RGBA{0, 0, 0, 160}}, image.Point{}, draw.Over)

	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.White),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(x0 + paddingX),
			Y: fixed.I(y0 + paddingY + metrics.Ascent.Ceil()),
		},
	}
	d.DrawString(label)

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output %s: %w", outPath, err)
	}
	defer out.Close()

	if err := jpeg.Encode(out, dst, &jpeg.Options{Quality: quality}); err != nil {
		return fmt.Errorf("encode jpeg %s: %w", outPath, err)
	}

	return nil
}

func writeConcatInput(path string, files []string, fps int) error {
	frameDuration := 1.0 / float64(fps)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create concat input: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, p := range files {
		esc := strings.ReplaceAll(p, "'", "'\\''")
		if _, err := fmt.Fprintf(w, "file '%s'\n", esc); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "duration %.10f\n", frameDuration); err != nil {
			return err
		}
	}
	if len(files) > 0 {
		esc := strings.ReplaceAll(files[len(files)-1], "'", "'\\''")
		if _, err := fmt.Fprintf(w, "file '%s'\n", esc); err != nil {
			return err
		}
	}
	return w.Flush()
}

func runFFmpeg(ffmpegPath, inputPath, outputPath string, logger *log.Logger) error {
	cmd := exec.Command(
		ffmpegPath,
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", inputPath,
		"-vf", "format=yuv420p",
		outputPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Printf("ffmpeg output:\n%s", string(out))
		return fmt.Errorf("ffmpeg failed: %w", err)
	}
	if len(out) > 0 {
		logger.Printf("ffmpeg output:\n%s", string(out))
	}
	return nil
}

func loadFontData(cfg Config) ([]byte, error) {
	if cfg.FontPath != "" {
		b, err := os.ReadFile(cfg.FontPath)
		if err == nil && len(b) > 0 {
			return b, nil
		}
	}
	return goregular.TTF, nil
}

func newFontFaceFromData(data []byte, size float64) (font.Face, error) {
	ft, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	face, err := opentype.NewFace(ft, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("create font face: %w", err)
	}
	return face, nil
}

func hhmmss(s string) (int, error) {
	t, err := time.Parse("15:04:05", s)
	if err != nil {
		return 0, fmt.Errorf("parse time %q: %w", s, err)
	}
	return t.Hour()*10000 + t.Minute()*100 + t.Second(), nil
}

func humanSize(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	v := float64(b)
	i := -1
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	return fmt.Sprintf("%.2f %s", v, units[max(i, 0)])
}

func stripAllExtensions(name string) string {
	for {
		ext := filepath.Ext(name)
		if ext == "" {
			return name
		}
		name = strings.TrimSuffix(name, ext)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func round(f float64) int {
	return int(math.Round(f))
}
