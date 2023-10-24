package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

var logLevels = []logrus.Level{
	logrus.DebugLevel,
	logrus.InfoLevel,
	logrus.WarnLevel,
	logrus.ErrorLevel,
}

var logDescriptions = []string{
	"Just keep logging, just keep logging",
	"A coral reef event occurred",
	"A jellyfish stung, an error was encountered",
	"Information swimming along",
	"Warning: Shark ahead!",
	"Finding a fatal error",
	"Help! A panic occurred",
	"Nemo and Dory went on an adventure",
	"Marlin faced his fears",
	"Squirt showed off his turtle skills",
	"Bruce practiced fish-friendly behavior",
	"Seagulls said 'Mine, mine, mine!'",
	"Mr. Ray gave a lesson in oceanography",
	"P. Sherman, 42 Wallaby Way, Sydney",
	"Gill and the gang plotted an escape",
	"Jacques cleaned the tank with élan",
	"Peach was the ultimate fish nanny",
	"Nigel offered his 'Medical Miracle'",
	"Coral and her friends made a colorful reef",
	"Crush and Squirt rode the EAC waves",
}

// Generate a log file randomly
func generateLog(logDirectory string, fileName string, numEntries int) {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{ForceColors: false})

	logFileName := fmt.Sprintf("%s.log", fileName)
	filePath := filepath.Join(logDirectory, logFileName)

	logFile, err := os.Create(filePath)
	if err != nil {
		logger.WithField("error", err).Error("Failed to create log file")
		return
	}
	defer logFile.Close()

	logger.SetOutput(logFile)

	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)

	for i := 0; i < numEntries; i++ {
		level := logLevels[random.Intn(len(logLevels))]
		description := logDescriptions[random.Intn(len(logDescriptions))]

		entry := logger.WithFields(logrus.Fields{})

		switch level {
		case logrus.DebugLevel:
			entry.Debug(description)
		case logrus.InfoLevel:
			entry.Info(description)
		case logrus.WarnLevel:
			entry.Warn(description)
		case logrus.ErrorLevel:
			entry.Error(description)
		}
	}
	logger.Info("Log entries generated successfully")
}

// Execute the input string as a command
func executeCommand(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stderr.String(), nil
	}

	return stdout.String(), nil
}

func clearTerminal() {
	// Detect the operating system and use the appropriate clear command
	clearCommand := "clear" // Default for Unix-like systems

	if runtime.GOOS == "windows" {
		clearCommand = "cls" // Use "cls" for Windows
	}

	// Execute the clear command
	cmd := exec.Command(clearCommand)
	cmd.Stdout = os.Stdout
	cmd.Run()
}
