package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/winxxp/glog"
	"net/http"
)

type Engine struct {
	*gin.Engine
}

func NewGin() *Engine {
	engine := &Engine{
		Engine: gin.New(),
	}

	engine.Use(Logger("/", "/menu"), Recovery())

	return engine
}

func (e *Engine) Run(ctx context.Context, addr string) error {
	server := &http.Server{
		Addr:    addr,
		Handler: e.Engine,
	}

	go func() {
		<-ctx.Done()
		err := server.Close()
		glog.WithResult(err).Error("server close")
	}()

	return server.ListenAndServe()
}
