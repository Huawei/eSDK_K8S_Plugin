package log

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	debugLog = iota
	infoLog
	warningLog
	errorLog
	fatalLog
	logSeverity = 5
)

type Logger struct {
	logDir        string
	logFilePrefix string
	logFilePath   string
	logDebug      bool
	logFileHandle *os.File
	logFileMaxCap int64
	logMutex      sync.Mutex
}

func newLogger(conf map[string]string) (*Logger, error) {
	var logDir = "/var/log/huawei"
	var maxSize int64 = 1024 * 1024 * 20
	var logDebug bool

	if "" != conf["logDir"] {
		logDir = conf["logDir"]
	}

	if "" != conf["logFileMaxCap"] {
		fileMaxCap, err := getNumInByte(conf["logFileMaxCap"])
		if err != nil {
			logrus.Errorf("Calc max log file size error: %v.", err)
			return nil, err
		}

		if fileMaxCap > 0 {
			maxSize = fileMaxCap
		}
	}

	if "" != conf["logDebug"] {
		logDebug, _ = strconv.ParseBool(conf["logDebug"])
	}

	logger := Logger{
		logDir:        logDir,
		logFileHandle: nil,
		logFilePrefix: conf["logFilePrefix"],
		logFileMaxCap: maxSize,
		logDebug:      logDebug,
	}

	err := logger.initLogFile()
	if err != nil {
		logrus.Errorf("Obtain log file error: %v.", err)
		return nil, err
	}

	return &logger, nil
}

func (l *Logger) initLogFile() error {
	destFilePath := l.logDir
	exist, err := isExist(destFilePath)
	if err != nil {
		return err
	}

	if !exist {
		err := os.MkdirAll(destFilePath, 0755)
		if err != nil {
			return err
		}
	}

	logFilePath := fmt.Sprintf("%s/%s", destFilePath, l.logFilePrefix)
	fileHandle, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return err
	}

	l.logFilePath = logFilePath
	l.logFileHandle = fileHandle

	return nil
}

func (l *Logger) checkLogFile() {
	exist, err := isExist(l.logFilePath)
	if err != nil || exist {
		return
	}

	fileHandle, err := os.OpenFile(l.logFilePath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return
	}

	if l.logFileHandle != nil {
		l.logFileHandle.Close()
	}

	l.logFileHandle = fileHandle
}

func (l *Logger) dumpLog() {
	l.checkLogFile()

	fileSize, err := getFileByteSize(l.logFileHandle)
	if err != nil || fileSize < l.logFileMaxCap {
		return
	}

	timeStr := time.Now().Format("20060102-150405")
	newFileName := fmt.Sprintf("%s/%s.%s", l.logDir, l.logFilePrefix, timeStr)
	err = copyFile(newFileName, l.logFilePath)
	// Only truncate log file in the case of dumping original log file success.
	if err == nil {
		l.logFileHandle.Truncate(0)
		l.logFileHandle.Seek(0, 0)
	}
}

func (l *Logger) formatWriteLog(level int, format string, args ...interface{}) {
	if level == debugLog && !l.logDebug {
		return
	}

	formatMsg := fmt.Sprintf(format, args...)
	l.writeLogMsg(level, formatMsg)
}

func (l *Logger) nonformatWriteLog(level int, args ...interface{}) {
	if level == debugLog && !l.logDebug {
		return
	}

	formatMsg := fmt.Sprint(args...)
	l.writeLogMsg(level, formatMsg)
}

func (l *Logger) writeLogMsg(level int, logMsg string) {
	l.logMutex.Lock()
	defer l.logMutex.Unlock()

	l.dumpLog()

	logLevel := getLogLevel(level)
	timeNow := time.Now().Format("2006-01-02 15:04:05.000000")
	logContent := fmt.Sprintf("%s %d %s%s\n", timeNow, os.Getpid(), logLevel, logMsg)

	err := write(l.logFileHandle, logContent)
	if err != nil {
		logrus.Errorf("Write log message %s to %s error.", logContent, l.logFilePath)
	}
}

func (l *Logger) lockAndFlushAll() {
	l.logMutex.Lock()
	defer l.logMutex.Unlock()
	l.logFileHandle.Sync()
}

func (l *Logger) close() {
	l.logFileHandle.Close()
}

func getLogLevel(level int) string {
	switch level {
	case debugLog:
		return "[DEBUG]: "
	case infoLog:
		return "[INFO]: "
	case warningLog:
		return "[WARNING]: "
	case errorLog:
		return "[ERROR]: "
	case fatalLog:
		return "[FATAL]: "
	default:
		return "[UNKNOWN]: "
	}
}
