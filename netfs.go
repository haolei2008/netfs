package main

import (
	"context"
	"flag"
	"git.coinv.com/haolei/netfs/minwinsvc"
	filedriver "github.com/goftp/file-driver"
	"github.com/goftp/server"
	"net"
	"strconv"

	"github.com/winxxp/glog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

var (
	root        = flag.String("root", ".", "Root directory to serve, if not exist will create")
	httpAddress = flag.String("http-addr", ":8000", "server bind address")
	ftpAddress  = flag.String("ftp-addr", ":2121", "ftp bind address")

	user = flag.String("user", "admin", "Username for ftp server login")
	pass = flag.String("pass", "password", "Password for ftp server login")
)

func init() {
	glog.PaddingColumns = 90
}

func main() {
	flag.Parse()

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
	NewHTTPFileServer(RootDirectory, *httpAddress).Start(ctx)
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

	glog.Warning("App Quit")
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
	http.Server

	// 文件初始路径
	RootDirectory string
}

func NewHTTPFileServer(root string, addr string) *HTTPFileServer {
	return &HTTPFileServer{
		Server: http.Server{
			Addr:    addr,
			Handler: http.FileServer(http.Dir(root)),
		},
		RootDirectory: root,
	}
}

func (s *HTTPFileServer) Start(ctx context.Context) {
	go func(ctx context.Context) {
		<-ctx.Done()
		err := s.Close()
		glog.WithResult(err).Log("http server close")
	}(ctx)

	glog.WithFields(glog.Fields{
		"addr": s.Addr,
		"dir":  s.RootDirectory,
	}).Info("http server running")

	go func() {
		err := s.ListenAndServe()
		glog.WithResult(err).Log("http server quit")
	}()
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
