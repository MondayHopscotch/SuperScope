package main

import (
	"flag"
	"github.com/MondayHopscotch/SuperScope/watcher"
	"github.com/vrecan/death"
	"syscall"
)

func main() {
	rootDir := flag.String("root", "~/Media", "Root file to watch for new files")
	//dropDir := flag.String("drop", "~/TrackerDropoff", "Dropoff for tracker files")

	flag.Parse()

	watcher := watcher.NewWatcher()

	watcher.Watch(*rootDir)

	death := death.NewDeath(syscall.SIGINT, syscall.SIGTERM)
	death.WaitForDeath(watcher)
}
