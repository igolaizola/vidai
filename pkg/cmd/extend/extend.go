package extend

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
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

	Input       string
	Output      string
	N           int
	Model       string
	Folder      string
	Interpolate bool
	Upscale     bool
	Watermark   bool
	Explore     bool
}

// Run generates a video from an image and a text prompt.
func Run(ctx context.Context, cfg *Config) error {
	if cfg.Input == "" {
		return fmt.Errorf("input is required")
	}
	if cfg.N < 1 {
		return fmt.Errorf("n must be greater than 0")
	}
	if cfg.Token == "" {
		return fmt.Errorf("token is required")
	}
	client, err := runway.New(&runway.Config{
		Token:  cfg.Token,
		Wait:   cfg.Wait,
		Debug:  cfg.Debug,
		Proxy:  cfg.Proxy,
		Folder: cfg.Folder,
	})
	if err != nil {
		return fmt.Errorf("vidai: couldn't create client: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(cfg.Input), filepath.Ext(cfg.Input))

	// Copy input video to temp file
	vid := filepath.Join(os.TempDir(), fmt.Sprintf("%s-0.mp4", base))
	if err := copyFile(cfg.Input, vid); err != nil {
		return fmt.Errorf("vidai: couldn't copy input video: %w", err)
	}

	videos := []string{vid}
	var urls []string
	for i := 0; i < cfg.N; i++ {
		img := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d.jpg", base, i))

		// Extract last frame from video using the following command:
		// ffmpeg -sseof -1 -i input.mp4 -update 1 -q:v 1 output.jpg
		// This will seek to the last second of the input and output all frames.
		// But since -update 1 is set, each frame will be overwritten to the
		// same file, leaving only the last frame remaining.
		cmd := exec.CommandContext(ctx, "ffmpeg", "-sseof", "-1", "-i", vid, "-update", "1", "-q:v", "1", img)
		cmdOut, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("vidai: couldn't extract last frame (%s): %w", string(cmdOut), err)
		}

		// Read image
		b, err := os.ReadFile(img)
		if err != nil {
			return fmt.Errorf("vidai: couldn't read image: %w", err)
		}
		name := filepath.Base(img)

		// Generate video
		imageURL, assetID, err := client.Upload(ctx, name, b)
		if err != nil {
			return fmt.Errorf("vidai: couldn't upload image: %w", err)
		}
		defer func() {
			// Delete asset
			deleteCTX, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := client.Delete(deleteCTX, assetID); err != nil {
				log.Println(fmt.Errorf("vidai: couldn't delete asset: %w", err))
			}
		}()
		gen, err := client.Generate(ctx, &runway.GenerateRequest{
			Model:       cfg.Model,
			AssetURL:    imageURL,
			Prompt:      "",
			Interpolate: cfg.Interpolate,
			Upscale:     cfg.Upscale,
			Watermark:   cfg.Watermark,
			Extend:      false,
			ExploreMode: cfg.Explore,
		})
		if err != nil {
			return fmt.Errorf("vidai: couldn't generate video: %w", err)
		}
		urls = append(urls, gen.URL)

		// Remove temporary image
		if err := os.Remove(img); err != nil {
			log.Println(fmt.Errorf("vidai: couldn't remove image: %w", err))
		}

		// Download video to temp file
		vid = filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d.mp4", base, i+1))
		if err := client.Download(ctx, gen.URL, vid); err != nil {
			return fmt.Errorf("vidai: couldn't download video: %w", err)
		}
		videos = append(videos, vid)
	}

	if cfg.Output != "" {
		// Create list of videos
		var listData string
		for _, v := range videos {
			listData += fmt.Sprintf("file '%s'\n", filepath.Base(v))
		}
		list := filepath.Join(os.TempDir(), fmt.Sprintf("%s-list.txt", base))
		if err := os.WriteFile(list, []byte(listData), 0644); err != nil {
			return fmt.Errorf("vidai: couldn't create list file: %w", err)
		}

		// Combine videos using the following command:
		// ffmpeg -f concat -safe 0 -i list.txt -c copy output.mp4
		cmd := exec.CommandContext(ctx, "ffmpeg", "-f", "concat", "-safe", "0", "-i", list, "-c", "copy", "-y", cfg.Output)
		cmdOut, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("vidai: couldn't combine videos (%s): %w", string(cmdOut), err)
		}

		// Remove temporary list file
		if err := os.Remove(list); err != nil {
			log.Println(fmt.Errorf("vidai: couldn't remove list file: %w", err))
		}
	}

	// Remove temporary videos
	for _, v := range videos {
		if err := os.Remove(v); err != nil {
			log.Println(fmt.Errorf("vidai: couldn't remove video: %w", err))
		}
	}

	fmt.Println("URLs:")
	for _, u := range urls {
		fmt.Println(u)
	}
	return nil
}

func copyFile(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("vidai: couldn't open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("vidai: couldn't create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy source to destination
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("vidai: couldn't copy source to destination: %w", err)
	}
	return nil
}
