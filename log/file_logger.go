package log

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

/*
 NO OP
*/

func NewFileLogger(path string) (Logger, error) {
	i, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if i.IsDir() {
		// TODO: append file?
		return nil, fmt.Errorf("log file cannot be directory")
	}

	// try to delete file
	_ = os.Remove(path)
	// might not exist, so ignore error

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}

	// Create a buffered writer
	writer := bufio.NewWriter(f)

	globalLogger := &FileLogger{
		Path: path,
		f:    f,
		w:    writer,
	}

	return globalLogger, nil
}

type FileLogger struct {
	Path   string
	f      *os.File
	w      *bufio.Writer
	fields []Field
}

func (l *FileLogger) Debug(msg string, f ...Field) {
	// msg = fmt.Sprintf("[DEBUG] %s", msg)
	// l.printFile(msg, f...)
}
func (l *FileLogger) Info(msg string, f ...Field) {
	msg = fmt.Sprintf("[INFO] %s", msg)
	l.printFile(msg, f...)
}
func (l *FileLogger) Warn(msg string, f ...Field) {
	msg = fmt.Sprintf("[WARN] %s", msg)
	l.printFile(msg, f...)
}
func (l *FileLogger) Error(msg string, f ...Field) {
	msg = fmt.Sprintf("[ERROR] %s", msg)
	l.printFile(msg, f...)
}
func (l *FileLogger) Fatal(msg string, f ...Field) {
	defer l.f.Close()
	msg = fmt.Sprintf("[FATAL] %s", msg)
	l.printFile(msg, f...)
	os.Exit(1)
}
func (l *FileLogger) Panic(msg string, f ...Field) {
	defer l.f.Close()
	msg = fmt.Sprintf("[FATAL] %s", msg)
	l.printFile(msg, f...)
	panic(msg)
}
func (l *FileLogger) SetLevel(Level) {}

func (l *FileLogger) With(f ...Field) Logger {
	l.fields = append(l.fields, f...)
	return l
}

func (l *FileLogger) Close() {
	defer l.f.Close()
}

func (l *FileLogger) printFile(msg string, fields ...Field) {
	now := time.Now()
	fmt.Fprintf(l.w, "%s - %s\n", now.Format("2006-01-02 15:04:05"), msg)
	for _, fv := range fields {
		fmt.Fprintf(l.w, "\t%s - %+v\n", fv.Key, fv.Value)
	}

	_ = l.w.Flush()
}
