package main

import (
	"fmt"
	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	"github.com/winxxp/glog"
	"strconv"
	"time"
)

var (
	IDGen *snowflake.Node
)

var (
	dunno     = []byte("???")
	centerDot = []byte("·")
	dot       = []byte(".")
	slash     = []byte("/")
)

func init() {
	var err error
	IDGen, err = snowflake.NewNode(time.Now().Unix() % 1024)
	if err != nil {
		panic(err)
	}
}

func Logger(notlogged ...string) gin.HandlerFunc {
	var skip map[string]struct{}

	if length := len(notlogged); length > 0 {
		skip = make(map[string]struct{}, length)

		for _, path := range notlogged {
			skip[path] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		start := time.Now()
		rid := IDGen.Generate().Base58()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Set("rid", rid)
		c.Next()

		if _, ok := skip[path]; !ok {
			end := time.Now()
			latency := end.Sub(start)

			clientIP := c.ClientIP()
			method := c.Request.Method
			statusCode := c.Writer.Status()
			comment := c.Errors.ByType(gin.ErrorTypePrivate).String()

			var entry = glog.WithID(rid).Depth(1).WithFields(glog.Fields{
				"latency": latency,
				"client":  clientIP,
				"comment": comment,
				"raw":     raw,
			})

			var logPad func(s string, rs string, pad byte)

			switch statusCode / 100 {
			case 1, 2:
				logPad = entry.PadInfo
			case 3:
				logPad = entry.PadWarning
			case 4, 5:
				logPad = entry.PadError
			default:
				logPad = entry.PadError
			}

			logPad(fmt.Sprintf("%-7s%s", method, path), strconv.Itoa(statusCode), '-')
		}
	}
}
