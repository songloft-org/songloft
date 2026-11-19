package parentwatch

import (
	"os"
	"testing"
	"time"
)

func TestParentPipeRemainsLiveUntilWriterCloses(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	done := Done(reader)
	if _, err := writer.Write([]byte("keep alive")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
		t.Fatal("parent was still alive")
	case <-time.After(20 * time.Millisecond):
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("parent exit did not close lifetime signal")
	}
}
