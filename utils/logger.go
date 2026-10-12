/*
 Copyright (c) 2024-2025 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package utils

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"io"
	"os"
	"strings"
	"time"
)

func GetLogger(loglevel, logPath, logfmt string) log.Logger {
	// Empty path or '-' explicitly selects stdout-only logging.
	var writer io.Writer = os.Stdout
	if logPath != "" && logPath != "-" {
		out, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot open log file; using stdout: %v\n", err)
		} else {
			writer = io.MultiWriter(os.Stdout, out)
		}
	}
	var logCreator func(io.Writer) log.Logger
	switch strings.ToLower(logfmt) {
	case "json":
		logCreator = log.NewJSONLogger
	case "logfmt":
		logCreator = log.NewLogfmtLogger
	default:
		logCreator = log.NewLogfmtLogger
	}

	nw := writer
	// create a logger
	logger := logCreator(log.NewSyncWriter(nw))

	// set loglevel
	var loglevelFilterOpt level.Option
	switch strings.ToLower(loglevel) {
	case "debug":
		loglevelFilterOpt = level.AllowDebug()
	case "info":
		loglevelFilterOpt = level.AllowInfo()
	case "warn":
		loglevelFilterOpt = level.AllowWarn()
	case "error":
		loglevelFilterOpt = level.AllowError()
	default:
		loglevelFilterOpt = level.AllowInfo()
	}
	logger = level.NewFilter(logger, loglevelFilterOpt)
	logger = log.With(logger,
		"ts", log.TimestampFormat(time.Now, time.RFC3339),
		"caller", log.DefaultCaller,
	)
	return logger
}

func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}
