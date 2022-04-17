package log

import (
	"fmt"
	"log"
	"os"

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
		level: l,
	}
}

type StdLogger struct {
	level Level
}

func (l *StdLogger) Debug(msg string, fields ...Field) {
	if l.level <= Debug {
		cc := color.New(color.FgMagenta, color.Bold).SprintFunc()
		l.printStd(cc("[Debug]"), msg, fields...)
	}
}

func (l *StdLogger) Info(msg string, fields ...Field) {
	if l.level <= Info {
		cc := color.New(color.FgBlue, color.Bold).SprintFunc()
		l.printStd(cc("[Info]"), msg, fields...)
	}
}

func (l *StdLogger) Warn(msg string, fields ...Field) {
	if l.level <= Warn {
		cc := color.New(color.FgYellow, color.Bold).SprintFunc()
		l.printStd(cc("[Warn]"), msg, fields...)
	}
}

func (l *StdLogger) Error(msg string, fields ...Field) {
	cc := color.New(color.FgRed, color.Bold).SprintFunc()
	l.printStd(cc("[Error]"), msg, fields...)
}

func (l *StdLogger) Fatal(msg string, fields ...Field) {
	cc := color.New(color.FgHiRed, color.Bold).SprintFunc()
	l.printStd(cc("[FATAL]"), msg, fields...)
	os.Exit(1)
}
func (l *StdLogger) Panic(msg string, fields ...Field) {
	cc := color.New(color.FgHiRed, color.Bold).SprintFunc()
	l.printStd(cc("[PANIC]"), msg, fields...)
	panic(msg)
}

func (l *StdLogger) SetLevel(ll Level) {
	l.level = ll
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

	log.Println(v...)
}
