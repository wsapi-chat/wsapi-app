package whatsapp

import (
	"fmt"
	"log/slog"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// SlogAdapter adapts slog.Logger to whatsmeow's waLog.Logger interface.
type SlogAdapter struct {
	logger *slog.Logger
}

func NewSlogAdapter(logger *slog.Logger) *SlogAdapter {
	return &SlogAdapter{logger: logger}
}

func (a *SlogAdapter) Debugf(msg string, args ...interface{}) {
	a.logger.Debug(fmt.Sprintf(msg, args...))
}

func (a *SlogAdapter) Infof(msg string, args ...interface{}) {
	a.logger.Info(fmt.Sprintf(msg, args...))
}

func (a *SlogAdapter) Warnf(msg string, args ...interface{}) {
	a.logger.Warn(fmt.Sprintf(msg, args...))
}

func (a *SlogAdapter) Errorf(msg string, args ...interface{}) {
	a.logger.Error(fmt.Sprintf(msg, args...))
}

func (a *SlogAdapter) Sub(module string) waLog.Logger {
	return &SlogAdapter{logger: a.logger.With("module", module)}
}
