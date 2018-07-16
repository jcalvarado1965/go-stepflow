package inprocess

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	stepflow "github.com/jcalvarado1965/go-stepflow"
)

type consoleLogger struct {
	Mutex *sync.Mutex
}

// NewConsoleLogger creates a console logger
func NewConsoleLogger() stepflow.Logger {
	return &consoleLogger{
		Mutex: &sync.Mutex{},
	}
}

func (l *consoleLogger) Debugf(ctx context.Context, format string, params ...interface{}) {
	l.logf(ctx, color.FgBlue, "DEBUG", format, params)
}

func (l *consoleLogger) Infof(ctx context.Context, format string, params ...interface{}) {
	l.logf(ctx, color.FgWhite, "INFO ", format, params)
}

func (l *consoleLogger) Warnf(ctx context.Context, format string, params ...interface{}) {
	l.logf(ctx, color.FgYellow, "WARN ", format, params)
}

func (l *consoleLogger) Errorf(ctx context.Context, format string, params ...interface{}) {
	l.logf(ctx, color.FgRed, "ERROR", format, params)
}

func trimID(id string) string {
	return id[0:strings.IndexAny(id, "-")]
}

func (l *consoleLogger) logf(ctx context.Context, attr color.Attribute, level string, format string, params []interface{}) {
	l.Mutex.Lock()
	defer l.Mutex.Unlock()

	color.Set(attr)
	defer color.Unset()

	now := time.Now()
	fmt.Printf("%s %s ", level, now.Format(time.Stamp))

	if wfrunid := ctx.Value(stepflow.DataflowRunContextKey); wfrunid != nil {
		fmt.Printf("[%s] ", trimID(string(wfrunid.(stepflow.DataflowRunID))))
		if flowid := ctx.Value(stepflow.FlowContextKey); flowid != nil {
			fmt.Printf("[%s] ", trimID(string(flowid.(stepflow.FlowID))))
			if stepid := ctx.Value(stepflow.StepContextKey); stepid != nil {
				fmt.Printf("[%s] ", stepid.(string))
			}
		}
	}

	fmt.Printf(format, params...)
	fmt.Print("\n")
}
