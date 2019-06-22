package main

import (
	"context"
	"flag"
	"git.coinv.com/haolei/httpfs/minwinsvc"
	"github.com/winxxp/glog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

var (
	address   = flag.String("addr", ":8000", "server bind address")
	directory = flag.String("dir", "", "http file directory")
)

func main() {
	flag.Parse()

	if len(*directory) == 0 {
		*directory, _ = os.Getwd()
	}
	dir, _ := filepath.Abs(*directory)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sig := make(chan os.Signal)
		signal.Notify(sig, os.Interrupt, os.Kill)
		<-sig

		glog.Warning("receive interrupt signal")
		cancel()
	}()

	glog.Info("run on: ", *address)
	glog.Info("diretory: ", *directory)

	server := http.Server{
		Addr:    *address,
		Handler: http.FileServer(http.Dir(dir)),
	}

	go func(ctx context.Context) {
		<-ctx.Done()
		err := server.Close()
		glog.WithResult(err).Log("server close")
	}(ctx)

	svc := make(chan bool)
	go minwinsvc.SetOnExit(func() {
		glog.Warning("receive service control signal")
		cancel()
		// wait other ctx done, because service call os.exit()
		time.Sleep(time.Second * 3)
		<-svc
	})

	err := server.ListenAndServe()
	glog.WithResult(err).Log("server quit")

	select {
	case <-time.NewTimer(time.Second * 2).C: // wait other ctx done
	case svc <- true:
	}

	glog.Warning("App Quit")
}
