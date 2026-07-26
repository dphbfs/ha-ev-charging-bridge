package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func setupLogging(path string) (io.Closer, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	file, err := newDailyLogWriter(path, time.Now)
	if err != nil {
		return nil, err
	}
	logger := slog.New(slog.NewTextHandler(io.MultiWriter(os.Stderr, file), &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	return file, nil
}

type dailyLogWriter struct {
	path string
	now  func() time.Time

	mu      sync.Mutex
	date    string
	current *os.File
}

func newDailyLogWriter(path string, now func() time.Time) (*dailyLogWriter, error) {
	path = strings.TrimSpace(path)
	if err := ensureParentDir(path); err != nil {
		return nil, err
	}
	writer := &dailyLogWriter{path: path, now: now}
	if err := writer.rotateLocked(); err != nil {
		return nil, err
	}
	return writer, nil
}

func (w *dailyLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateLocked(); err != nil {
		return 0, err
	}
	return w.current.Write(p)
}

func (w *dailyLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.current == nil {
		return nil
	}
	err := w.current.Close()
	w.current = nil
	return err
}

func (w *dailyLogWriter) rotateLocked() error {
	at := w.now()
	date := at.Format(time.DateOnly)
	if w.current != nil && w.date == date {
		return nil
	}
	if w.current != nil {
		if err := w.current.Close(); err != nil {
			return err
		}
	}

	file, err := os.OpenFile(datedLogPath(w.path, at), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	w.current = file
	w.date = date
	return nil
}

func datedLogPath(path string, at time.Time) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, name+"-"+at.Format(time.DateOnly)+ext)
}

func ensureParentDir(path string) error {
	dir := strings.TrimSpace(filepath.Dir(path))
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}
