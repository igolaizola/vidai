package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/igopr/vidai/pkg/runway"
)

type Config struct {
	Token string
	Wait  time.Duration
	Debug bool
	Proxy string

	Output      string
	Model       string
	Image       string
	Text        string
	Extend      int
	Interpolate bool
	Upscale     bool
	Watermark   bool
	Width       int
	Height      int
	Explore     bool
}

// Run generates a video from an image and a text prompt.
func Run(ctx context.Context, cfg *Config) error {
	if cfg.Image == "" && cfg.Text == "" {
		return fmt.Errorf("vidai: image or text is required")
	}
	if cfg.Token == "" {
		return fmt.Errorf("token is required")
	}
	client, err := runway.New(&runway.Config{
		Token: cfg.Token,
		Wait:  cfg.Wait,
		Debug: cfg.Debug,
		Proxy: cfg.Proxy,
	})
	if err != nil {
		return fmt.Errorf("vidai: couldn't create client: %w", err)
	}

	var imageURL string
	if cfg.Image != "" {
		b, err := os.ReadFile(cfg.Image)
		if err != nil {
			return fmt.Errorf("vidai: couldn't read image: %w", err)
		}
		name := filepath.Base(cfg.Image)

		imageURL, err = client.Upload(ctx, name, b)
		if err != nil {
			return fmt.Errorf("vidai: couldn't upload image: %w", err)
		}
	}
	gen, err := client.Generate(ctx, &runway.GenerateRequest{
		Model:       cfg.Model,
		AssetURL:    imageURL,
		Prompt:      cfg.Text,
		Interpolate: cfg.Interpolate,
		Upscale:     cfg.Upscale,
		Watermark:   cfg.Watermark,
		Extend:      false,
		Width:       cfg.Width,
		Height:      cfg.Height,
		ExploreMode: cfg.Explore,
	})
	if err != nil {
		return fmt.Errorf("vidai: couldn't generate video: %w", err)
	}

	// Extend video
	for i := 0; i < cfg.Extend; i++ {
		gen, err = client.Generate(ctx, &runway.GenerateRequest{
			Model:       cfg.Model,
			AssetURL:    gen.URL,
			Prompt:      "",
			Interpolate: cfg.Interpolate,
			Upscale:     cfg.Upscale,
			Watermark:   cfg.Watermark,
			Extend:      true,
		})
		if err != nil {
			return fmt.Errorf("vidai: couldn't extend video: %w", err)
		}
	}

	// Use temp file if no output is set and we need to extend the video
	videoPath := cfg.Output
	if videoPath == "" && cfg.Extend > 0 {
		base := strings.TrimSuffix(filepath.Base(cfg.Image), filepath.Ext(cfg.Image))
		videoPath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.mp4", base))
	}

	// Download video
	if videoPath != "" {
		if err := client.Download(ctx, gen.URL, videoPath); err != nil {
			return fmt.Errorf("vidai: couldn't download video: %w", err)
		}
	}

	js, err := json.MarshalIndent(gen, "", "  ")
	if err != nil {
		return fmt.Errorf("vidai: couldn't marshal json: %w", err)
	}
	fmt.Println(string(js))
	return nil
}
