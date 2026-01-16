package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	MaxLogDirSize   = 10 * 1024 * 1024 // 10MB
	MinRetentionDay = 30               // Minimum 30 days retention
)

var (
	logFile     *os.File
	logDir      string
	currentDate string
	mu          sync.Mutex
	initialized bool
)

// Init initializes the logger with a log directory
func Init(dir string) error {
	mu.Lock()
	defer mu.Unlock()

	if initialized {
		return nil
	}

	logDir = dir

	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// Open today's log file
	if err := openDailyLogLocked(); err != nil {
		return err
	}

	initialized = true

	// Start daily rotation check
	go dailyRotationCheck()

	// Start periodic cleanup
	go periodicCleanup()

	log.Printf("[INFO] Logger initialized")
	return nil
}

// openDailyLogLocked opens or creates the log file for today (must hold lock)
func openDailyLogLocked() error {
	today := time.Now().Format("2006-01-02")
	logFileName := fmt.Sprintf("ecpay-server.%s.log", today)
	logPath := filepath.Join(logDir, logFileName)

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	// Close previous file if exists
	if logFile != nil {
		logFile.Close()
	}

	logFile = file
	currentDate = today

	log.SetOutput(file)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	return nil
}

// checkDateRotation checks if we need to rotate to a new day's log
func checkDateRotation() {
	mu.Lock()
	defer mu.Unlock()

	if !initialized {
		return
	}

	today := time.Now().Format("2006-01-02")
	if today != currentDate {
		if err := openDailyLogLocked(); err != nil {
			log.Printf("[LOGGER] Error rotating to new day: %v", err)
		} else {
			log.Printf("[LOGGER] Rotated to new day: %s", today)
		}
	}
}

// dailyRotationCheck checks for date change every minute
func dailyRotationCheck() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		checkDateRotation()
	}
}

// periodicCleanup runs cleanup every hour
func periodicCleanup() {
	// Wait a bit before first cleanup to let init complete
	time.Sleep(5 * time.Second)
	cleanupOldLogs()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		cleanupOldLogs()
	}
}

// cleanupOldLogs removes logs older than 30 days if total size exceeds limit
func cleanupOldLogs() {
	mu.Lock()
	defer mu.Unlock()

	if !initialized {
		return
	}

	// Get all log files
	files, err := os.ReadDir(logDir)
	if err != nil {
		log.Printf("[LOGGER] Error reading log directory: %v", err)
		return
	}

	// Filter and sort log files by date (oldest first)
	type logFileInfo struct {
		name string
		date string
		size int64
	}

	var logFiles []logFileInfo
	var totalSize int64

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		// Match pattern: ecpay-server.YYYY-MM-DD.log
		if !strings.HasPrefix(name, "ecpay-server.") || !strings.HasSuffix(name, ".log") {
			continue
		}

		// Extract date from filename
		datePart := strings.TrimPrefix(name, "ecpay-server.")
		datePart = strings.TrimSuffix(datePart, ".log")

		// Validate date format
		if _, err := time.Parse("2006-01-02", datePart); err != nil {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		logFiles = append(logFiles, logFileInfo{
			name: name,
			date: datePart,
			size: info.Size(),
		})
		totalSize += info.Size()
	}

	// Sort by date (oldest first)
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].date < logFiles[j].date
	})

	// Calculate cutoff date (30 days ago)
	cutoffDate := time.Now().AddDate(0, 0, -MinRetentionDay).Format("2006-01-02")

	// Only delete if:
	// 1. Total size exceeds limit
	// 2. Log is older than 30 days
	if totalSize <= MaxLogDirSize {
		return
	}

	for _, lf := range logFiles {
		// Stop if we're under the size limit
		if totalSize <= MaxLogDirSize {
			break
		}

		// Only delete logs older than retention period
		if lf.date >= cutoffDate {
			break
		}

		// Delete old log
		filePath := filepath.Join(logDir, lf.name)
		if err := os.Remove(filePath); err != nil {
			log.Printf("[LOGGER] Error removing old log %s: %v", lf.name, err)
		} else {
			log.Printf("[LOGGER] Removed old log: %s (size: %d bytes)", lf.name, lf.size)
			totalSize -= lf.size
		}
	}
}

// Close closes the log file
func Close() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	initialized = false
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[INFO] %s", msg)
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[ERROR] %s", msg)
}

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[DEBUG] %s", msg)
}

// Warn logs a warning message
func Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[WARN] %s", msg)
}

// Transaction logs a transaction event
func Transaction(transType, amount, orderNo, status string) {
	log.Printf("[TRANS] Type=%s Amount=%s OrderNo=%s Status=%s", transType, amount, orderNo, status)
}

// Protocol logs protocol-level events
func Protocol(direction, event string, data []byte) {
	if len(data) > 100 {
		log.Printf("[PROTO] %s %s data_len=%d first_100=%x...", direction, event, len(data), data[:100])
	} else {
		log.Printf("[PROTO] %s %s data=%x", direction, event, data)
	}
}
