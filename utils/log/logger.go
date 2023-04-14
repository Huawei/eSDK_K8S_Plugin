/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package log

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var (
	logger         LoggingInterface
	testInitLogger sync.Once
)

type key string

const (
	timestampFormat = "2006-01-02 15:04:05.000000"

	csiRequestID key = "csi.requestid"
	requestID        = "requestID"
)

// LoggingInterface is an interface exposes logging functionality
type LoggingInterface interface {
	Logger

	flushable

	closable

	AddContext(ctx context.Context) Logger
}

// Closable is an interface for closing logging streams.
// The interface should be implemented by hooks.
type closable interface {
	close()
}

// Flushable is an interface to commit current content of logging stream
type flushable interface {
	flush()
}

// Logger exposes logging functionality
type Logger interface {
	Debugf(format string, args ...interface{})

	Debugln(args ...interface{})

	Infof(format string, args ...interface{})

	Infoln(args ...interface{})

	Warningf(format string, args ...interface{})

	Warningln(args ...interface{})

	Errorf(format string, args ...interface{})

	Errorln(args ...interface{})

	Fatalf(format string, args ...interface{})

	Fatalln(args ...interface{})
}

type loggerImpl struct {
	*logrus.Logger
	hooks     []logrus.Hook
	formatter logrus.Formatter
}

var _ LoggingInterface = &loggerImpl{}

func parseLogLevel(logLevel string) (logrus.Level, error) {
	switch logLevel {
	case "debug":
		return logrus.DebugLevel, nil
	case "info":
		return logrus.InfoLevel, nil
	case "warning":
		return logrus.WarnLevel, nil
	case "error":
		return logrus.ErrorLevel, nil
	case "fatal":
		return logrus.FatalLevel, nil
	default:
		return logrus.FatalLevel, fmt.Errorf("invalid logging level [%v]", logLevel)
	}
}

// LoggingRequest use to init the logging service
type LoggingRequest struct {
	LogName       string
	LogFileSize   string
	LoggingModule string
	LogLevel      string
	LogFileDir    string
	MaxBackups    uint
}

var maxBackups uint

// InitLogging configures logging. Logs are written to a log file or stdout/stderr.
// Since logrus doesn't support multiple writers, each log stream is implemented as a hook.
func InitLogging(req *LoggingRequest) error {
	var tmpLogger loggerImpl

	// initialize logrus in wrapper
	tmpLogger.Logger = logrus.New()

	// No output except for the hooks
	tmpLogger.Logger.SetOutput(ioutil.Discard)

	// set logging level
	level, err := parseLogLevel(req.LogLevel)
	if err != nil {
		return err
	}
	tmpLogger.Logger.SetLevel(level)

	// initialize log formatter
	formatter := &PlainTextFormatter{TimestampFormat: timestampFormat, pid: os.Getpid()}

	hooks := make([]logrus.Hook, 0)
	switch req.LoggingModule {
	case "file":
		maxBackups = req.MaxBackups
		logFilePath := fmt.Sprintf("%s/%s", req.LogFileDir, req.LogName)
		// Write to the log file
		logFileHook, err := newFileHook(logFilePath, req.LogFileSize, formatter)
		if err != nil {
			return fmt.Errorf("could not initialize logging to file: %v", err)
		}
		hooks = append(hooks, logFileHook)
	case "console":
		// Write to stdout/stderr
		logConsoleHook, err := newConsoleHook(formatter)
		if err != nil {
			return fmt.Errorf("could not initialize logging to console: %v", err)
		}
		hooks = append(hooks, logConsoleHook)
	default:
		return fmt.Errorf("invalid logging module [%v]. Support only 'file' or 'console'",
			req.LoggingModule)
	}

	tmpLogger.hooks = hooks
	for _, hook := range tmpLogger.hooks {
		// initialize logrus with hooks
		tmpLogger.Logger.AddHook(hook)
	}

	logger = &tmpLogger
	return nil
}

// PlainTextFormatter is a formatter to ensure formatted logging output
type PlainTextFormatter struct {
	// TimestampFormat to use for display when a full timestamp is printed
	TimestampFormat string

	// process identity number
	pid int
}

var _ logrus.Formatter = &PlainTextFormatter{}

// Format ensure unified and formatted logging output
func (f *PlainTextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := entry.Buffer
	if entry.Buffer == nil {
		b = &bytes.Buffer{}
	}

	_, _ = fmt.Fprintf(b, "%s %d %s", entry.Time.Format(f.TimestampFormat), f.pid, getLogLevel(entry.Level))

	if len(entry.Data) != 0 {
		for key, value := range entry.Data {
			_, _ = fmt.Fprintf(b, "[%s:%v] ", key, value)
		}
	}

	_, _ = fmt.Fprintf(b, "%s\n", entry.Message)

	return b.Bytes(), nil
}

func getLogLevel(level logrus.Level) string {
	switch level {
	case logrus.DebugLevel:
		return "[DEBUG]: "
	case logrus.InfoLevel:
		return "[INFO]: "
	case logrus.WarnLevel:
		return "[WARNING]: "
	case logrus.ErrorLevel:
		return "[ERROR]: "
	case logrus.FatalLevel:
		return "[FATAL]: "
	default:
		return "[UNKNOWN]: "
	}
}

// Debugf ensures output of formatted debug logs
func Debugf(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

// Debugln ensures output of Debug logs
func Debugln(args ...interface{}) {
	logger.Debugln(args...)
}

// Infof ensures output of formatted info logs
func Infof(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

// Infoln ensures output of info logs
func Infoln(args ...interface{}) {
	logger.Infoln(args...)
}

// Warningf ensures output of formatted warning logs
func Warningf(format string, args ...interface{}) {
	logger.Warningf(format, args...)
}

// Warningln ensures output of warning logs
func Warningln(args ...interface{}) {
	logger.Warningln(args...)
}

// Errorf ensures output of formatted error logs
func Errorf(format string, args ...interface{}) {
	logger.Errorf(format, args...)
}

// Errorln ensures output of error logs
func Errorln(args ...interface{}) {
	logger.Errorln(args...)
}

// Fatalf ensures output of formatted fatal logs
func Fatalf(format string, args ...interface{}) {
	logger.Fatalf(format, args...)
}

// Fatalln ensures output of fatal logs
func Fatalln(args ...interface{}) {
	logger.Fatalln(args...)
}

// AddContext ensures appending context info in log
func AddContext(ctx context.Context) Logger {
	return logger.AddContext(ctx)
}

func (logger *loggerImpl) flush() {
	for _, hook := range logger.hooks {
		flushable, ok := hook.(flushable)
		if ok {
			flushable.flush()
		}
	}
}

func (logger *loggerImpl) close() {
	for _, hook := range logger.hooks {
		flushable, ok := hook.(closable)
		if ok {
			flushable.close()
		}
	}
}

// AddContext ensures appending context info in log
func (logger *loggerImpl) AddContext(ctx context.Context) Logger {
	if ctx.Value(csiRequestID) == nil {
		return logger
	}
	return logger.WithFields(logrus.Fields{
		requestID: ctx.Value(csiRequestID),
	})
}

// EnsureGRPCContext ensures adding request id in incoming context
func EnsureGRPCContext(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {
	var requestID string
	md, ok := metadata.FromIncomingContext(ctx)
	// if no metadata, generate one
	if !ok {
		md = metadata.Pairs()
		ctx = metadata.NewIncomingContext(ctx, md)
	}

	if reqIDs, ok := md[string(csiRequestID)]; ok && len(reqIDs) > 0 {
		requestID = reqIDs[0]
	}

	if requestID == "" {
		randomID, err := rand.Prime(rand.Reader, 32)
		if err != nil {
			Errorf("Failed in random ID generation for GRPC request ID logging: %v", err)
			return handler(ctx, req)
		}
		requestID = randomID.String()
	}

	return handler(context.WithValue(ctx, csiRequestID, requestID), req)
}

// Flush ensures to commit current content of logging stream
func Flush() {
	logger.flush()
}

// Close ensures closing output stream
func Close() {
	logger.close()
}

// FilteredLog will not print the logs that need to be filtered, and the log level will be as required.
func FilteredLog(ctx context.Context, isSkip, isDebug bool, msg string) {
	if isSkip {
		return
	}

	if isDebug {
		AddContext(ctx).Debugln(msg)
	} else {
		AddContext(ctx).Infoln(msg)
	}
}
