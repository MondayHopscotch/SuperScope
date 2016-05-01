package watcher

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Watcher interface {
	Watch()
	Close() error
}

type SimpleWatcher struct {
	root        string
	dropOff     string
	watcher     *fsnotify.Watcher
	WatchedDirs map[string]bool
	ActiveFiles map[string]string
	EventsDone  chan bool
	WatcherDone chan bool
	FilesDone   chan bool
	Adds        chan string
	Removes     chan string
	Files       chan string
}

func NewWatcher(root string, dropOff string) Watcher {
	return NewSimpleWatcher(root, dropOff)
}

func NewSimpleWatcher(root string, dropOff string) *SimpleWatcher {
	return &SimpleWatcher{
		root:        root,
		dropOff:     dropOff,
		EventsDone:  make(chan bool),
		WatcherDone: make(chan bool),
		Adds:        make(chan string, 10),
		Removes:     make(chan string, 10),
		Files:       make(chan string, 10),
		WatchedDirs: make(map[string]bool, 0),
		ActiveFiles: make(map[string]string, 0),
	}
}

func (w *SimpleWatcher) Watch() {
	log.Println("Watcher being started in ", w.root)

	newWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	w.watcher = newWatcher

	log.Println("Scanning file system")

	startingDirs, err := determineStartDirs(w.root)
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

	if err != nil {
		log.Fatal("Failed to add root dir: ", err)
	}

	log.Println("Directories added. Starting watcher")

	go w.handleEvents()

	go w.handleFSWatcher()

	go w.handleFilesFound()
}

func (w *SimpleWatcher) Close() error {
	w.watcher.Close()
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
	done <- true

	log.Println(fmt.Sprint("%v+", dirs))

	return dirs, nil
}

func buildDirs(dirChan chan<- string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Print(err)
			return err
		}
		if info.IsDir() {
			dirChan <- path
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
			log.Println("\tevent:", event)
			if event.Op&fsnotify.Create == fsnotify.Create {
				if isNewFile(event.Name) {
					continue
				}
				stat, err := os.Stat(event.Name)
				if err != nil {
					log.Println("Error stat'ing ", event.Name, ": ", err)
					continue
				}

				if stat.IsDir() {
					log.Println("Need new watcher for ", event.Name)
					adds <- event.Name
				} else {
					ext := strings.ToLower(path.Ext(event.Name))
					log.Println("extension: ", ext)
					if strings.Compare(ext, ".torrent") == 0 {
						log.Println("New file for consumption ", event.Name)
						files <- event.Name
					}
				}
			}
		case <-done:
			return
		}
	}
}

func isNewFile(name string) bool {
	fileBaseName := strings.ToLower(filepath.Base(name))
	log.Println(fileBaseName)
	return strings.HasPrefix(fileBaseName, "new ")
}

func (w *SimpleWatcher) handleFilesFound() {
	for {
		select {
		case file := <-w.Files:
			// save path
			// move file to deluge drop point
			base := filepath.Base(file)
			w.ActiveFiles[base] = file
			os.Rename(file, path.Join(w.dropOff, base))
			log.Println(w.ActiveFiles)
		case <-w.FilesDone:
			return
		}
	}
}
