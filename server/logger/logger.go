package logger

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	MaxLogDirSize = 10 * 1024 * 1024 // 10MB
	LogFileName   = "ecpay-server.log"
)

var (
	logFile     *os.File
	logDir      string
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

	// Open log file
	logPath := filepath.Join(logDir, LogFileName)
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	logFile = file

	// Set log output to file
	log.SetOutput(file)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	initialized = true

	// Check log directory size on startup
	go checkAndRotate()

	// Start periodic size check
	go periodicSizeCheck()

	Info("Logger initialized")
	return nil
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

// checkAndRotate checks directory size and rotates if necessary
func checkAndRotate() {
	mu.Lock()
	defer mu.Unlock()

	size, err := getDirSize(logDir)
	if err != nil {
		log.Printf("[LOGGER] Error checking directory size: %v", err)
		return
	}

	if size > MaxLogDirSize {
		rotateOldLogs()
	}
}

// getDirSize calculates total size of files in directory
func getDirSize(dir string) (int64, error) {
	var size int64
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	for _, file := range files {
		if !file.IsDir() {
			size += file.Size()
		}
	}
	return size, nil
}

// rotateOldLogs removes old log files when directory exceeds size limit
func rotateOldLogs() {
	currentLogPath := filepath.Join(logDir, LogFileName)

	// Read all files in log directory
	files, err := ioutil.ReadDir(logDir)
	if err != nil {
		log.Printf("[LOGGER] Error reading log directory: %v", err)
		return
	}

	// Remove old archived logs first (keep current log)
	for _, file := range files {
		if file.Name() != LogFileName && !file.IsDir() {
			filePath := filepath.Join(logDir, file.Name())
			if err := os.Remove(filePath); err != nil {
				log.Printf("[LOGGER] Error removing old log %s: %v", file.Name(), err)
			} else {
				log.Printf("[LOGGER] Removed old log: %s", file.Name())
			}
		}
	}

	// Check size again
	size, _ := getDirSize(logDir)
	if size > MaxLogDirSize {
		// Current log is still too big, archive and truncate
		archiveName := fmt.Sprintf("ecpay-server.%s.log", time.Now().Format("20060102-150405"))
		archivePath := filepath.Join(logDir, archiveName)

		// Close current log
		if logFile != nil {
			logFile.Close()
		}

		// Rename current to archive
		os.Rename(currentLogPath, archivePath)

		// Create new log file
		file, err := os.OpenFile(currentLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			log.Printf("[LOGGER] Error creating new log file: %v", err)
			return
		}
		logFile = file
		log.SetOutput(file)

		// Remove archive immediately if still over limit
		os.Remove(archivePath)

		log.Printf("[LOGGER] Log rotated and cleaned")
	}
}

// periodicSizeCheck checks log directory size every hour
func periodicSizeCheck() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		checkAndRotate()
	}
}
