package watcher

import (
	"github.com/fsnotify/fsnotify"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"testing"
)

func TestDirectoryWatcher(t *testing.T) {

	Convey("Test create new watcher", t, func() {
		watcher := NewWatcher("root", "drop", "complete", "media")
		So(watcher, ShouldNotBeNil)
	})

	Convey("Test simple watcher watch", t, func() {
		resetTestDir()
		watcher := NewSimpleWatcher("test/watch", "test/drop", "test/complete", "test/media")
		watcher.Watch()
		watcher.Close()

	})

	Convey("Handle events recognizes creates", t, func() {
		resetTestDir()
		testFile := "test/watch/movies/test.torrent"
		_, err := os.Create(testFile)
		So(err, ShouldBeNil)

		done := make(chan bool, 10)
		eventIn := make(chan fsnotify.Event, 10)
		adds := make(chan string, 10)
		files := make(chan string)
		removes := make(chan string, 10)

		go handleEventsForChans(done, eventIn, adds, removes, files)

		createDirEvent := fsnotify.Event{Name: "test", Op: fsnotify.Create}

		eventIn <- createDirEvent

		add := <-adds

		So(add, ShouldEqual, "test")

		createFileEvent := fsnotify.Event{Name: testFile, Op: fsnotify.Create}

		eventIn <- createFileEvent

		file := <-files

		So(file, ShouldEqual, testFile)

		done <- true
	})

	Convey("Test Build dirs", t, func() {
		dirs, err := determineStartDirs("test/watch")
		So(err, ShouldBeNil)

		t.Log(dirs)
		So(len(dirs), ShouldEqual, 4) // the base directory, plus our test
	})

	Convey("Test file found", t, func() {
		resetTestDir()
		watcher := NewSimpleWatcher("test/watch", "test/drop", "test/complete", "test/media")

		go watcher.handleFilesFound()

		testFile := "test/watch/movies/test.torrent"
		_, err := os.Create(testFile)
		So(err, ShouldBeNil)

		watcher.Files <- testFile

		_, err = os.Stat("test/drop/test.torrent")
		So(err, ShouldNotBeNil)
	})
}

func resetTestDir() {
	os.RemoveAll("test")
	os.Mkdir("test", os.ModePerm)
	os.Mkdir("test/watch", os.ModePerm)
	os.Mkdir("test/watch/tv", os.ModePerm)
	os.Mkdir("test/watch/movies", os.ModePerm)
	os.Mkdir("test/watch/music", os.ModePerm)

	os.Mkdir("test/drop", os.ModePerm)
}
