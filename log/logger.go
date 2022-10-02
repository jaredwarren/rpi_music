package log

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strings"

	"github.com/fatih/color"
)

type Field struct {
	Key   string
	Value any
}

type Logger interface {
	Debug(string, ...Field)
	Info(string, ...Field)
	Warn(string, ...Field)
	Error(string, ...Field)
	Fatal(string, ...Field)
	Panic(string, ...Field)
	SetLevel(Level)
}

// std "log" interface
type ILog interface {
	SetOutput(w io.Writer)
	Output(calldepth int, s string) error
	Printf(format string, v ...any)
	Print(v ...any)
	Println(v ...any)
	Fatal(v ...any)
	Fatalf(format string, v ...any)
	Fatalln(v ...any)
	Panic(v ...any)
	Panicf(format string, v ...any)
	Panicln(v ...any)
	Flags() int
	Prefix() string
	SetPrefix(prefix string)
	Writer() io.Writer
}

type Level int64

const (
	Debug Level = iota
	Info
	Warn
)

func Any(k string, v any) Field {
	return Field{
		Key:   k,
		Value: v,
	}
}
func Error(v error) Field {
	return Field{
		Key:   "error",
		Value: v,
	}
}

func NewStdLogger(l Level) Logger {
	return &StdLogger{
		Level: l,
		Log:   log.New(os.Stdout, "", 5),
	}
}

type StdLogger struct {
	Level Level
	Log   ILog
}

func (l *StdLogger) Debug(msg string, fields ...Field) {
	if l.Level <= Debug {
		f := getLastFile()
		if f != "" {
			fields = append(fields, Any("caller", f))
		}
		cc := color.New(color.FgMagenta, color.Bold).SprintFunc()
		l.printStd(cc("[Debug]"), msg, fields...)
	}
}

func (l *StdLogger) Info(msg string, fields ...Field) {
	if l.Level <= Info {
		f := getLastFile()
		if f != "" {
			fields = append(fields, Any("caller", f))
		}
		cc := color.New(color.FgBlue, color.Bold).SprintFunc()
		l.printStd(cc("[Info]"), msg, fields...)
	}
}

func (l *StdLogger) Warn(msg string, fields ...Field) {
	if l.Level <= Warn {
		f := getLastFile()
		if f != "" {
			fields = append(fields, Any("caller", f))
		}
		cc := color.New(color.FgYellow, color.Bold).SprintFunc()
		l.printStd(cc("[Warn]"), msg, fields...)
	}
}

func (l *StdLogger) Error(msg string, fields ...Field) {
	f := getLastFile()
	if f != "" {
		fields = append(fields, Any("caller", f))
	}
	cc := color.New(color.FgRed, color.Bold).SprintFunc()
	l.printStd(cc("[Error]"), msg, fields...)
}

func getLastFile() string {
	s := debug.Stack()
	r := bytes.NewReader(s)
	scanner := bufio.NewScanner(r)

	files := []string{}
	for scanner.Scan() {
		str := scanner.Text()
		if strings.Contains(str, ".go:") {
			files = append(files, str)
		}
	}
	for i := len(files) - 1; i >= 0; i-- {
		if strings.Contains(files[i], "/logger.go:") {
			return files[i+1]
		}
	}

	return ""
}

func (l *StdLogger) Fatal(msg string, fields ...Field) {
	f := getLastFile()
	if f != "" {
		fields = append(fields, Any("caller", f))
	}

	cc := color.New(color.FgHiRed, color.Bold).SprintFunc()
	l.printStd(cc("[FATAL]"), msg, fields...)
	os.Exit(1)
}
func (l *StdLogger) Panic(msg string, fields ...Field) {
	f := getLastFile()
	if f != "" {
		fields = append(fields, Any("caller", f))
	}

	cc := color.New(color.FgHiRed, color.Bold).SprintFunc()
	l.printStd(cc("[PANIC]"), msg, fields...)
	panic(msg)
}

func (l *StdLogger) SetLevel(ll Level) {
	l.Level = ll
}

func (l *StdLogger) printStd(ll, msg string, fields ...Field) {
	v := []any{
		ll,
		msg,
	}
	cc := color.New(color.FgHiWhite, color.Bold).SprintFunc()
	for _, fv := range fields {
		v = append(v, fmt.Sprintf("\n\t%s: %+v", cc(fv.Key), fv.Value))
	}

	l.Log.Println(v...)
}

/*
 NO OP
*/

func NewNoOpLogger() Logger {
	return &NoOpLogger{}
}

type NoOpLogger struct{}

func (l *NoOpLogger) Debug(string, ...Field) {}
func (l *NoOpLogger) Info(string, ...Field)  {}
func (l *NoOpLogger) Warn(string, ...Field)  {}
func (l *NoOpLogger) Error(string, ...Field) {}
func (l *NoOpLogger) Fatal(string, ...Field) {}
func (l *NoOpLogger) Panic(string, ...Field) {}
func (l *NoOpLogger) SetLevel(Level)         {}
