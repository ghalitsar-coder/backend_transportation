package logger

import (
	"log"
	"os"
)

var (
	InfoLog  *log.Logger
	ErrorLog *log.Logger
)

func init() {
	InfoLog = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLog = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func Info(msg string, args ...interface{}) {
	InfoLog.Printf(msg, args...)
}

func Error(msg string, args ...interface{}) {
	ErrorLog.Printf(msg, args...)
}

func Fatal(msg string, args ...interface{}) {
	ErrorLog.Fatalf(msg, args...)
}
