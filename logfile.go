package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	logDayFormat     = "20060102"
	logRetentionDays = 30
)

// dailyLogWriter appends to the log file and rotates it to <path>.YYYYMMDD when the day changes
type dailyLogWriter struct {
	mu   sync.Mutex
	path string
	file *os.File
	day  string
}

func openDailyLog(path string) (*dailyLogWriter, error) {
	w := &dailyLogWriter{path: path}
	if err := w.open(time.Now()); err != nil {
		return nil, err
	}
	return w, nil
}

// open rotates the existing file if it was last written on another day, then opens the log file
func (w *dailyLogWriter) open(now time.Time) error {
	today := now.Format(logDayFormat)
	if info, err := os.Stat(w.path); err == nil {
		if day := info.ModTime().Format(logDayFormat); day != today {
			rotateLogFile(w.path, day)
		}
	}
	removeOldLogs(w.path, now)

	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	w.file = file
	w.day = today
	return nil
}

func (w *dailyLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	if now.Format(logDayFormat) != w.day {
		w.file.Close()
		if err := w.open(now); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

func (w *dailyLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

// rotateLogFile moves the log file to <path>.<day>, appending if that file already exists
func rotateLogFile(path, day string) {
	target := path + "." + day
	if _, err := os.Stat(target); os.IsNotExist(err) {
		os.Rename(path, target)
		return
	}

	src, err := os.Open(path)
	if err != nil {
		return
	}
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		src.Close()
		return
	}
	_, copyErr := io.Copy(dst, src)
	src.Close()
	dst.Close()
	if copyErr == nil {
		os.Remove(path)
	}
}

// removeOldLogs deletes rotated log files older than logRetentionDays
func removeOldLogs(path string, now time.Time) {
	matches, err := filepath.Glob(path + ".*")
	if err != nil {
		return
	}
	cutoff := now.AddDate(0, 0, -logRetentionDays).Format(logDayFormat)
	for _, match := range matches {
		day := strings.TrimPrefix(match, path+".")
		if _, err := time.Parse(logDayFormat, day); err != nil {
			continue
		}
		if day < cutoff {
			os.Remove(match)
		}
	}
}
