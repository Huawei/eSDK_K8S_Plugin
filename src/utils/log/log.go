package log

import (
	"os"
)

var logger *Logger

func Init(conf map[string]string) error {
	var err error
	logger, err = newLogger(conf)
	return err
}

func Close() {
	logger.close()
}

func Debugf(format string, args ...interface{}) {
	logger.formatWriteLog(debugLog, format, args...)
}

func Debugln(args ...interface{}) {
	logger.nonformatWriteLog(debugLog, args...)
}

func Infof(format string, args ...interface{}) {
	logger.formatWriteLog(infoLog, format, args...)
}

func Infoln(args ...interface{}) {
	logger.nonformatWriteLog(infoLog, args...)
}

func Warningf(format string, args ...interface{}) {
	logger.formatWriteLog(warningLog, format, args...)
}

func Warningln(args ...interface{}) {
	logger.nonformatWriteLog(warningLog, args...)
}

func Errorf(format string, args ...interface{}) {
	logger.formatWriteLog(errorLog, format, args...)
}

func Errorln(args ...interface{}) {
	logger.nonformatWriteLog(errorLog, args...)
}

func Fatalf(format string, args ...interface{}) {
	logger.formatWriteLog(fatalLog, format, args...)
	os.Exit(255)
}

func Fatalln(args ...interface{}) {
	logger.nonformatWriteLog(fatalLog, args...)
	os.Exit(255)
}

func Flush() {
	logger.lockAndFlushAll()
}
