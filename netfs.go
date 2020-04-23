package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"git.coinv.com/haolei/netfs/minwinsvc"
	"github.com/gin-gonic/gin"
	filedriver "github.com/goftp/file-driver"
	"github.com/goftp/server"
	"net"
	"net/http"
	"strconv"

	"github.com/winxxp/glog"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

const (
	VERSION = "1.2.0"
)

var (
	root        = flag.String("root", ".", "Root directory to serve, if not exist will create")
	httpAddress = flag.String("http-addr", ":8000", "server bind address")
	ftpAddress  = flag.String("ftp-addr", ":2121", "ftp bind address")

	user = flag.String("user", "admin", "Username for ftp server login")
	pass = flag.String("pass", "password", "Password for ftp server login")

	encodeURL = flag.Bool("encode-url", false, "encode get file url")

	version = flag.Bool("version", false, "print application version")
)

func init() {
	glog.PaddingColumns = 90
}

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("netfs %s\n", VERSION)
		os.Exit(0)
	}

	if *root == "" {
		*root, _ = os.Getwd()
	}
	RootDirectory, _ := filepath.Abs(*root)
	if _, err := os.Stat(RootDirectory); err != err {
		if os.IsNotExist(err) {
			err = os.MkdirAll(RootDirectory, os.ModePerm)
		}
		if err != nil {
			glog.WithError(err).Error("init root directory")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	RegisterSignal(cancel)
	if *encodeURL {
		NewHTTPFileServer(RootDirectory, *httpAddress).StartWithEncodeURL(ctx)
	} else {
		NewHTTPFileServer(RootDirectory, *httpAddress).StartRaw(ctx)
	}
	NewFTPServer(RootDirectory, *ftpAddress).Start(ctx)

	svc := make(chan bool)
	go minwinsvc.SetOnExit(func() {
		glog.Warning("receive service control signal")
		cancel()
		// wait other ctx done, because service call os.exit()
		time.Sleep(time.Second * 3)
		<-svc
	})

	<-ctx.Done()

	select {
	case <-time.NewTimer(time.Second * 2).C: // wait other ctx done
	case svc <- true:
	}

	glog.Warning("Monitor Quit")
}

//RegisterSignal 注册退出信号 CTRL＋C
func RegisterSignal(cancel context.CancelFunc) {
	go func() {
		sig := make(chan os.Signal)
		signal.Notify(sig, os.Interrupt, os.Kill)
		<-sig

		glog.WithField("signal", sig).Warning("receive interrupt signal")
		cancel()
	}()
}

type HTTPFileServer struct {
	*Engine

	Addr string

	// 文件初始路径
	RootDirectory string
}

func NewHTTPFileServer(root string, addr string) *HTTPFileServer {
	return &HTTPFileServer{
		Addr:          addr,
		RootDirectory: root,
	}
}

//StartWithEncodeURL 获取文件路径base64加密
func (s *HTTPFileServer) StartWithEncodeURL(ctx context.Context) {
	go func(ctx context.Context) {
		engine := NewGin()
		engine.GET("/:file", func(c *gin.Context) {
			file, err := base64.URLEncoding.DecodeString(c.Param("file"))
			if err != nil {
				c.AbortWithError(http.StatusBadRequest, err)
				return
			}
			c.File(filepath.Join(s.RootDirectory, string(file)))
		})

		if err := engine.Run(ctx, s.Addr); err != nil {
			glog.Fatal(err)
		}
	}(ctx)

	glog.WithFields(glog.Fields{
		"addr": s.Addr,
		"dir":  s.RootDirectory,
	}).Info("http server running with encode url mode")
}

//StartRaw 直接按目录访问，文件路径不加密
func (s *HTTPFileServer) StartRaw(ctx context.Context) {
	go func(ctx context.Context) {
		srv := &http.Server{
			Addr:    s.Addr,
			Handler: http.FileServer(http.Dir(s.RootDirectory)),
		}

		go func(ctx context.Context) {
			<-ctx.Done()
			srv.Close()
		}(ctx)

		if err := srv.ListenAndServe(); err != nil {
			glog.Fatal(err)
		}
	}(ctx)

	glog.WithField("addr", s.Addr).Info("start http in raw url mode")
}

type FTPServer struct {
	*server.Server
	opts *server.ServerOpts

	// 文件初始路径
	RootDirectory string
}

func NewFTPServer(root string, addr string) *FTPServer {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		glog.Fatal(err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		glog.Fatal(err)
	}

	ftpSrv := &FTPServer{
		opts: &server.ServerOpts{
			Factory: &filedriver.FileDriverFactory{
				RootPath: root,
				Perm:     server.NewSimplePerm("user", "group"),
			},
			Port:     port,
			Hostname: host,
			Auth:     &server.SimpleAuth{Name: *user, Password: *pass},
			Logger:   &FTPLog{},
		},
	}
	ftpSrv.Server = server.NewServer(ftpSrv.opts)

	return ftpSrv
}

func (s *FTPServer) Start(ctx context.Context) {
	go func(ctx context.Context) {
		<-ctx.Done()
		err := s.Shutdown()
		glog.WithResult(err).Log("ftp server close")
	}(ctx)

	glog.WithFields(glog.Fields{
		"addr":     net.JoinHostPort(s.Hostname, strconv.Itoa(s.Port)),
		"username": *user,
		"pass":     *pass,
	}).Info("ftp server start")

	go func() {
		err := s.ListenAndServe()
		glog.WithResult(err).Log("ftp server quit")
	}()
}

type FTPLog struct {
}

func (*FTPLog) Print(sessionId string, message interface{}) {
	glog.WithIDString(sessionId).Info(message)
}

func (*FTPLog) Printf(sessionId string, format string, v ...interface{}) {
	glog.WithIDString(sessionId).Infof(format, v...)
}

func (*FTPLog) PrintCommand(sessionId string, command string, params string) {
	glog.WithIDString(sessionId).Infof("> %s %s", command, params)
}

func (*FTPLog) PrintResponse(sessionId string, code int, message string) {
	glog.WithIDString(sessionId).Infof("< %d %s", code, message)
}
