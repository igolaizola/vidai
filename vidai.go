package vidai

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/igolaizola/vidai/pkg/runway"
)

type Client struct {
	client     *runway.Client
	httpClient *http.Client
}

type Config struct {
	Token  string
	Wait   time.Duration
	Debug  bool
	Client *http.Client
}

func New(cfg *Config) *Client {
	httpClient := cfg.Client
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 2 * time.Minute,
		}
	}
	client := runway.New(&runway.Config{
		Token:  cfg.Token,
		Wait:   cfg.Wait,
		Debug:  cfg.Debug,
		Client: httpClient,
	})
	return &Client{
		client:     client,
		httpClient: httpClient,
	}
}

// Generate generates a video from an image and a text prompt.
func (c *Client) Generate(ctx context.Context, image, text, output string,
	extend int, interpolate, upscale, watermark bool) (string, error) {
	b, err := os.ReadFile(image)
	if err != nil {
		return "", fmt.Errorf("vidai: couldn't read image: %w", err)
	}
	name := filepath.Base(image)

	var imageURL string
	if image != "" {
		imageURL, err = c.client.Upload(ctx, name, b)
		if err != nil {
			return "", fmt.Errorf("vidai: couldn't upload image: %w", err)
		}
	}
	videoURL, err := c.client.Generate(ctx, imageURL, text, interpolate, upscale, watermark, false)
	if err != nil {
		return "", fmt.Errorf("vidai: couldn't generate video: %w", err)
	}

	// Extend video
	for i := 0; i < extend; i++ {
		videoURL, err = c.client.Generate(ctx, videoURL, "", interpolate, upscale, watermark, true)
		if err != nil {
			return "", fmt.Errorf("vidai: couldn't extend video: %w", err)
		}
	}

	// Use temp file if no output is set and we need to extend the video
	videoPath := output
	if videoPath == "" && extend > 0 {
		base := strings.TrimSuffix(filepath.Base(image), filepath.Ext(image))
		videoPath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.mp4", base))
	}

	// Download video
	if videoPath != "" {
		if err := c.download(ctx, videoURL, videoPath); err != nil {
			return "", fmt.Errorf("vidai: couldn't download video: %w", err)
		}
	}

	return videoURL, nil
}

// Extend extends a video using the previous video.
func (c *Client) Extend(ctx context.Context, input, output string, n int,
	interpolate, upscale, watermark bool) ([]string, error) {
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))

	// Copy input video to temp file
	vid := filepath.Join(os.TempDir(), fmt.Sprintf("%s-0.mp4", base))
	if err := copyFile(input, vid); err != nil {
		return nil, fmt.Errorf("vidai: couldn't copy input video: %w", err)
	}

	videos := []string{vid}
	var urls []string
	for i := 0; i < n; i++ {
		img := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d.jpg", base, i))

		// Extract last frame from video using the following command:
		// ffmpeg -sseof -1 -i input.mp4 -update 1 -q:v 1 output.jpg
		// This will seek to the last second of the input and output all frames.
		// But since -update 1 is set, each frame will be overwritten to the
		// same file, leaving only the last frame remaining.
		cmd := exec.CommandContext(ctx, "ffmpeg", "-sseof", "-1", "-i", vid, "-update", "1", "-q:v", "1", img)
		cmdOut, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("vidai: couldn't extract last frame (%s): %w", string(cmdOut), err)
		}

		// Read image
		b, err := os.ReadFile(img)
		if err != nil {
			return nil, fmt.Errorf("vidai: couldn't read image: %w", err)
		}
		name := filepath.Base(img)

		// Generate video
		imageURL, err := c.client.Upload(ctx, name, b)
		if err != nil {
			return nil, fmt.Errorf("vidai: couldn't upload image: %w", err)
		}
		videoURL, err := c.client.Generate(ctx, imageURL, "", interpolate, upscale, watermark, false)
		if err != nil {
			return nil, fmt.Errorf("vidai: couldn't generate video: %w", err)
		}

		// Remove temporary image
		if err := os.Remove(img); err != nil {
			log.Println(fmt.Errorf("vidai: couldn't remove image: %w", err))
		}

		// Download video to temp file
		vid = filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d.mp4", base, i+1))
		if err := c.download(ctx, videoURL, vid); err != nil {
			return nil, fmt.Errorf("vidai: couldn't download video: %w", err)
		}
		videos = append(videos, vid)
	}

	if output != "" {
		// Create list of videos
		var listData string
		for _, v := range videos {
			listData += fmt.Sprintf("file '%s'\n", filepath.Base(v))
		}
		list := filepath.Join(os.TempDir(), fmt.Sprintf("%s-list.txt", base))
		if err := os.WriteFile(list, []byte(listData), 0644); err != nil {
			return nil, fmt.Errorf("vidai: couldn't create list file: %w", err)
		}

		// Combine videos using the following command:
		// ffmpeg -f concat -safe 0 -i list.txt -c copy output.mp4
		cmd := exec.CommandContext(ctx, "ffmpeg", "-f", "concat", "-safe", "0", "-i", list, "-c", "copy", "-y", output)
		cmdOut, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("vidai: couldn't combine videos (%s): %w", string(cmdOut), err)
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

	return urls, nil
}

func (c *Client) download(ctx context.Context, url, output string) error {
	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("vidai: couldn't create request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vidai: couldn't download video: %w", err)
	}
	defer resp.Body.Close()

	// Write video to output
	f, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("vidai: couldn't create temp file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("vidai: couldn't write to temp file: %w", err)
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

func Loop(ctx context.Context, input, output string) error {
	// Reverse video using the following command:
	// ffmpeg -i input.mp4 -vf reverse temp.mp4
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("%s-reversed.mp4", filepath.Base(input)))
	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", input, "-vf", "reverse", tmp)
	cmdOut, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("vidai: couldn't reverse video (%s): %w", string(cmdOut), err)
	}

	// Obtain absolute path to input video
	absInput, err := filepath.Abs(input)
	if err != nil {
		return fmt.Errorf("vidai: couldn't get absolute path to input video: %w", err)
	}

	// Generate list of videos
	listData := fmt.Sprintf("file '%s'\nfile '%s'\n", absInput, filepath.Base(tmp))
	list := filepath.Join(os.TempDir(), fmt.Sprintf("%s-list.txt", filepath.Base(input)))
	if err := os.WriteFile(list, []byte(listData), 0644); err != nil {
		return fmt.Errorf("vidai: couldn't create list file: %w", err)
	}

	// Combine videos using the following command:
	// ffmpeg -f concat -safe 0 -i list.txt -c copy output.mp4
	cmd = exec.CommandContext(ctx, "ffmpeg", "-f", "concat", "-safe", "0", "-i", list, "-c", "copy", "-y", output)
	cmdOut, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("vidai: couldn't combine videos (%s): %w", string(cmdOut), err)
	}
	return nil
}
