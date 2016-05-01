package watcher

import (
	"log"
	"github.com/fsnotify/fsnotify"
	"os"
	"fmt"
	"path/filepath"
)

type Watcher interface {
	Watch(root string)
	Close() error
}


type SimpleWatcher struct {
	watcher     *fsnotify.Watcher
	WatchedDirs map[string]bool
	EventsDone chan bool
	WatcherDone chan bool
	Adds chan string
	Removes chan string
	Files chan string
}

func NewWatcher() Watcher {
	return &SimpleWatcher{
		EventsDone: make(chan bool),
		WatcherDone: make(chan bool),
		Adds: make(chan string, 10),
		Removes: make(chan string, 10),
		Files: make(chan string, 10),
	}
}

func (w *SimpleWatcher) Watch(root string) {
	w.WatchedDirs = make(map[string]bool, 0)

	newWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer newWatcher.Close()

	w.watcher = newWatcher

	log.Println("Scanning file system")

	startingDirs, err := determineStartDirs(root)
	if err != nil {
		log.Fatal("Unable to read root dir: ", err)
		return
	}

	log.Println("Finished scan. Adding directories to watcher")

	for _, dir := range startingDirs {
		log.Println("Adding ", dir, " as root directory to watch")
		err = w.watcher.Add(dir)
		if err != nil {
			log.Fatal("Failed to add root dir: ", err)
		}
		w.WatchedDirs[dir] = true
	}

	log.Println("Directories added. Starting watcher")

	go w.handleEvents()

	go w.handleFSWatcher()
}

func (w *SimpleWatcher) Close() error {
	w.EventsDone <- true
	w.WatcherDone <- true
	return nil
}

func determineStartDirs(root string) ([]string, error) {
	dirs := make([]string, 0)
	dirChan := make(chan string, 1)
	done := make(chan bool)

	go func() {
		for {
			select {
			case dir := <-dirChan:
				dirs = append(dirs, dir)
			case <-done:
				return
			}
		}
	}()

	filepath.Walk(root, buildDirs(dirChan))
	done<- true

	log.Println(fmt.Sprint("%v+", dirs))

	return dirs, nil
}

func buildDirs(dirChan chan<- string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Print(err)
			return nil
		}
		if info.IsDir() {
			dirChan<- path
		}
		return nil
	}
}

func (w *SimpleWatcher) handleFSWatcher() {
	for {
		select {
		case newWatch := <-w.Adds:
			err := w.watcher.Add(newWatch)
			if err != nil {
				log.Println("Error adding watch dir ", newWatch, err)
				continue
			}
		case oldWatch := <-w.Removes:
			err := w.watcher.Remove(oldWatch)
			if err != nil {
				log.Println("Error removing watch dir ", oldWatch, err)
				continue
			}
		case <-w.WatcherDone:
			return
		}
	}
}

func (w *SimpleWatcher) handleEvents() {
	handleEventsForChans(w.EventsDone, w.watcher.Events, w.Adds, w.Removes, w.Files)
}

func handleEventsForChans(done chan bool, eventIn <-chan fsnotify.Event, adds chan<- string, removes chan<- string, files chan<- string) {
	for {
		select {
		case event := <-eventIn:
			log.Println("event:", event)
			if event.Op & fsnotify.Create == fsnotify.Create {
				stat, err := os.Stat(event.Name)
				if err != nil {
					log.Println("Error stat'ing ", event.Name, ": ", err)
					continue
				}

				if stat.IsDir() {
					log.Println("Need new watcher for ", event.Name)
					adds <- event.Name
				} else {
					log.Println("New file for consumption ", event.Name)
					files <- event.Name
				}
			}

			//if event.Op & fsnotify.Rename == fsnotify.Rename {
			//	// validate existence of all of our directories?
			//	// TODO: clean up the watcher pointed at the old name
			//
			//}
			//
			//if event.Op & fsnotify.Write == fsnotify.Write {
			//	log.Println("modified file:", event.Name)
			//}
		case <-done:
			return
		}
	}
}