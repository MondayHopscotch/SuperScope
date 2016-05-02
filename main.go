package main

import (
	"flag"
	"github.com/MondayHopscotch/SuperScope/watcher"
	"github.com/vrecan/death"
	"log"
	"os"
	"syscall"
)

func main() {
	rootDir := flag.String("root", "", "Root file to watch for new files")
	dropDir := flag.String("drop", "", "Dropoff for tracker files")
	completedDir := flag.String("complete", "", "Where the completed files will be found")
	mediaDir := flag.String("media", "", "Final resting place for finished files")

	flag.Parse()

	log.Println("root: ", *rootDir, "   drop: ", *dropDir)
	if *rootDir == "" || *dropDir == "" || *completedDir == "" || *mediaDir == "" {
		log.Println("All flags must be provided")
		flag.Usage()
		os.Exit(1)
	}

	watcher := watcher.NewWatcher(*rootDir, *dropDir, *completedDir, *mediaDir)

	watcher.Watch()

	death := death.NewDeath(syscall.SIGINT, syscall.SIGTERM)
	death.WaitForDeath(watcher)
}
