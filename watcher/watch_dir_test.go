package watcher

import (
	"github.com/fsnotify/fsnotify"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestDirectoryWatcher(t *testing.T) {

	Convey("Test create new watcher", t, func() {
		watcher := NewWatcher("root", "drop")
		So(watcher, ShouldNotBeNil)
	})

	Convey("Test simple watcher watch", t, func() {
		watcher := NewWatcher("root", "drop")
		watcher.Watch(".")
		watcher.Close()
	})

	Convey("Handle events recognizes creates", t, func() {
		done := make(chan bool, 10)
		eventIn := make(chan fsnotify.Event, 10)
		adds := make(chan string, 10)
		files := make(chan string)
		removes := make(chan string, 10)

		go handleEventsForChans(done, eventIn, adds, removes, files)

		createDirEvent := fsnotify.Event{Name: "../watcher", Op: fsnotify.Create}

		eventIn <- createDirEvent

		add := <-adds

		So(add, ShouldEqual, "../watcher")

		createFileEvent := fsnotify.Event{Name: "watch_dir_test.go", Op: fsnotify.Create}

		eventIn <- createFileEvent

		file := <-files

		So(file, ShouldEqual, "watch_dir_test.go")

		done <- true
	})

	Convey("Test Build dirs", t, func() {
		dirs, err := determineStartDirs(".")
		So(err, ShouldBeNil)

		t.Log(dirs)
		So(len(dirs), ShouldEqual, 2) // the base directory, plus our test
	})
}
