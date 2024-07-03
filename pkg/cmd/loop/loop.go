package loop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func Run(ctx context.Context, input, output string) error {
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
