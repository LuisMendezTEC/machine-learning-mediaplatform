package main

// We need to use a .env variable for the
import (
	"log"
	"os"

	"github.com/LuisMendezTEC/mediaplatform.git/internal/queue"
)

func main() {
	log.Println("[worker] starting, id:", os.Getenv("WORKER_ID"))

	q := queue.New(os.Getenv("REDIS_ADDR"), os.Getenv("REDIS_PASSWORD"))
	_ = q

	// TODO (Person 2): implementar goroutine pool + FFmpeg
	log.Println("[worker] ready — task execution not yet implemented")
	select {}
}
