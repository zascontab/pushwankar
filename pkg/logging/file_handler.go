package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileHandler handles writing logs to files with rotation
type FileHandler struct {
	directory    string
	filenameBase string
	file         *os.File
	maxSizeMB    int64
	maxAgeDays   int
	maxBackups   int
	currentSize  int64
	mu           sync.Mutex
}

// NewFileHandler creates a new file handler
func NewFileHandler(directory, filenameBase string, options ...FileHandlerOption) (*FileHandler, error) {
	handler := &FileHandler{
		directory:    directory,
		filenameBase: filenameBase,
		maxSizeMB:    100, // Default 100MB
		maxAgeDays:   7,   // Default 7 days
		maxBackups:   5,   // Default 5 backups
	}

	// Apply options
	for _, option := range options {
		option(handler)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Clean old log files
	if err := handler.cleanOldLogFiles(); err != nil {
		return nil, fmt.Errorf("failed to clean old log files: %w", err)
	}

	// Open the log file
	if err := handler.openLogFile(); err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return handler, nil
}

// FileHandlerOption is a function that configures a FileHandler
type FileHandlerOption func(*FileHandler)

// WithMaxSizeMB sets the maximum size of log files in megabytes
func WithMaxSizeMB(maxSizeMB int64) FileHandlerOption {
	return func(h *FileHandler) {
		h.maxSizeMB = maxSizeMB
	}
}

// WithMaxAgeDays sets the maximum age of log files in days
func WithMaxAgeDays(maxAgeDays int) FileHandlerOption {
	return func(h *FileHandler) {
		h.maxAgeDays = maxAgeDays
	}
}

// WithMaxBackups sets the maximum number of backup log files
func WithMaxBackups(maxBackups int) FileHandlerOption {
	return func(h *FileHandler) {
		h.maxBackups = maxBackups
	}
}

// Write implements the io.Writer interface
func (h *FileHandler) Write(p []byte) (n int, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if rotation is needed
	h.currentSize += int64(len(p))
	if h.currentSize > h.maxSizeMB*1024*1024 {
		if err := h.rotate(); err != nil {
			return 0, fmt.Errorf("failed to rotate log file: %w", err)
		}
	}

	return h.file.Write(p)
}

// Close closes the file handler
func (h *FileHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.file != nil {
		return h.file.Close()
	}
	return nil
}

// openLogFile opens the current log file
func (h *FileHandler) openLogFile() error {
	filename := filepath.Join(h.directory, fmt.Sprintf("%s.log", h.filenameBase))

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	// Get file info to track size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}

	h.file = file
	h.currentSize = info.Size()

	return nil
}

// rotate rotates the log file
func (h *FileHandler) rotate() error {
	// Close the current file
	if h.file != nil {
		if err := h.file.Close(); err != nil {
			return err
		}
		h.file = nil
	}

	// Current log file
	currentFilename := filepath.Join(h.directory, fmt.Sprintf("%s.log", h.filenameBase))

	// Backup file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupFilename := filepath.Join(h.directory, fmt.Sprintf("%s-%s.log", h.filenameBase, timestamp))

	// Rename the current file to backup
	if err := os.Rename(currentFilename, backupFilename); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Open a new log file
	if err := h.openLogFile(); err != nil {
		return err
	}

	// Reset the size counter
	h.currentSize = 0

	// Clean up old log files
	if err := h.cleanOldLogFiles(); err != nil {
		// Just log the error and continue
		fmt.Fprintf(os.Stderr, "Failed to clean old log files: %v\n", err)
	}

	return nil
}

// cleanOldLogFiles cleans up old log files based on age and count limits
func (h *FileHandler) cleanOldLogFiles() error {
	// Get list of backup files
	pattern := filepath.Join(h.directory, fmt.Sprintf("%s-*.log", h.filenameBase))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	// Filter out files that are too old
	var validFiles []string
	cutoffTime := time.Now().Add(-time.Duration(h.maxAgeDays) * 24 * time.Hour)

	for _, filePath := range matches {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		if fileInfo.ModTime().After(cutoffTime) {
			validFiles = append(validFiles, filePath)
		} else {
			// Remove files that are too old
			os.Remove(filePath)
		}
	}

	// Sort by modification time (newest first)
	type fileAge struct {
		path    string
		modTime time.Time
	}

	var files []fileAge
	for _, filePath := range validFiles {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			continue
		}
		files = append(files, fileAge{filePath, fileInfo.ModTime()})
	}

	// Sort files by modification time (newest first)
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].modTime.Before(files[j].modTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	// Remove excess files
	if len(files) > h.maxBackups {
		for i := h.maxBackups; i < len(files); i++ {
			os.Remove(files[i].path)
		}
	}

	return nil
}
