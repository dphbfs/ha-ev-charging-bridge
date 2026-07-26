package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDatedLogPath(t *testing.T) {
	at := time.Date(2026, 7, 24, 12, 0, 0, 0, time.Local)
	got := datedLogPath(filepath.Join("log", "app.log"), at)
	want := filepath.Join("log", "app-2026-07-24.log")
	if got != want {
		t.Fatalf("datedLogPath() = %q, want %q", got, want)
	}
}

func TestDailyLogWriterRotatesWhenDayChanges(t *testing.T) {
	dir := t.TempDir()
	current := time.Date(2026, 7, 24, 23, 59, 0, 0, time.Local)
	writer, err := newDailyLogWriter(filepath.Join(dir, "app.log"), func() time.Time {
		return current
	})
	if err != nil {
		t.Fatalf("newDailyLogWriter() error = %v", err)
	}
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	if _, err := writer.Write([]byte("first\n")); err != nil {
		t.Fatalf("Write() first error = %v", err)
	}
	current = current.Add(2 * time.Minute)
	if _, err := writer.Write([]byte("second\n")); err != nil {
		t.Fatalf("Write() second error = %v", err)
	}

	firstPath := filepath.Join(dir, "app-2026-07-24.log")
	secondPath := filepath.Join(dir, "app-2026-07-25.log")
	first, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("read first log: %v", err)
	}
	if string(first) != "first\n" {
		t.Fatalf("first log = %q, want first line", first)
	}
	second, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("read second log: %v", err)
	}
	if string(second) != "second\n" {
		t.Fatalf("second log = %q, want second line", second)
	}
}
