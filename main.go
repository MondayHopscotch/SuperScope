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

	flag.Parse()

	log.Println("root: ", *rootDir, "   drop: ", *dropDir)
	if *rootDir == "" || *dropDir == "" {
		log.Println("Both root and drop directories must be specified")
		flag.Usage()
		os.Exit(1)
	}

	watcher := watcher.NewWatcher(*rootDir, *dropDir)

	watcher.Watch()

	death := death.NewDeath(syscall.SIGINT, syscall.SIGTERM)
	death.WaitForDeath(watcher)
}
