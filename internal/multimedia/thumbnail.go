// internal/multimedia/thumbnail.go
// FFmpeg wrapper: captura un fotograma JPEG del segundo 5 de un video.
// Si el archivo es audio, genera una imagen de waveform.
package multimedia

import (
	"context"
	"fmt"
	"log"
	"os/exec"
)

// Thumbnail extrae un JPEG del segundo 5 de inputPath.
// Para archivos de audio genera un waveform PNG.
// El callback recibe 100 al completar.
func Thumbnail(ctx context.Context, inputPath string, cb progressFn) (string, error) {
	out := outputPath(inputPath, ".jpg")
	log.Printf("[thumbnail] %s → %s", inputPath, out)

	// Intenta capturar un fotograma de video.
	args := []string{
		"-y",
		"-ss", "00:00:05",
		"-i", inputPath,
		"-frames:v", "1",
		"-q:v", "2",
		"-vf", "scale=320:-1",
		out,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out2, err := cmd.CombinedOutput(); err != nil {
		// Falló el modo video — intenta waveform para archivos de audio.
		log.Printf("[thumbnail] video frame failed (%v), trying waveform", err)
		log.Printf("[thumbnail] ffmpeg output: %s", string(out2))

		waveArgs := []string{
			"-y",
			"-i", inputPath,
			"-filter_complex", "showwavespic=s=320x100:colors=#1DB954",
			"-frames:v", "1",
			out,
		}
		waveCmd := exec.CommandContext(ctx, "ffmpeg", waveArgs...)
		if waveOut, waveErr := waveCmd.CombinedOutput(); waveErr != nil {
			return "", fmt.Errorf("thumbnail fallido (video y waveform): %w — %s", waveErr, string(waveOut))
		}
	}

	if cb != nil {
		cb(100)
	}
	return out, nil
}