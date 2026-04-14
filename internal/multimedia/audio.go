// internal/multimedia/audio.go
// FFmpeg wrapper: extrae el audio de un video y lo guarda como MP3.
package multimedia

import (
	"context"
	"fmt"
	"log"
	"os/exec"
)

// ExtractAudio extrae el track de audio de inputPath y lo guarda como MP3.
// El callback recibe 0-100 conforme FFmpeg avanza.
func ExtractAudio(ctx context.Context, inputPath string, cb progressFn) (string, error) {
	out := outputPath(inputPath, ".mp3")
	log.Printf("[extract_audio] %s → %s", inputPath, out)

	args := []string{
		"-y",
		"-i", inputPath,
		"-vn",
		"-acodec", "libmp3lame",
		"-ab", "192k",
		"-ar", "44100",
		"-progress", "pipe:2",
		"-nostats",
		out,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("ffmpeg start: %w", err)
	}

	go streamProgress(stderr, cb)

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("ffmpeg extract_audio: %w", err)
	}
	return out, nil
}