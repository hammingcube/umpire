package main

import (
	"flag"
	"github.com/docker/docker/client"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/labstack/gommon/log"
	"github.com/maddyonline/umpire"
	"net/http"
	"path/filepath"
	"time"
)

var (
	problems = flag.String("problems", "../../", "directory containing problems")
)

var localAgent *umpire.Agent

func judge(c echo.Context) error {
	payload := &umpire.Payload{}
	if err := c.Bind(payload); err != nil {
		return err
	}
	log.Printf("payload: %v", payload)
	done := make(chan interface{})
	go func() {
		done <- umpire.JudgeDefault(localAgent, payload)
	}()
	for {
		select {
		case out := <-done:
			return c.JSON(http.StatusCreated, out)
		case <-time.After(5 * time.Second):
			return c.JSON(http.StatusCreated, map[string]string{"status": "pending"})
		}
	}

}

func run(c echo.Context) error {
	payload := &umpire.Payload{}
	if err := c.Bind(payload); err != nil {
		return err
	}
	log.Printf("payload: %v", payload)
	done := make(chan interface{})
	go func() {
		done <- umpire.RunDefault(localAgent, payload)
	}()
	for {
		select {
		case out := <-done:
			return c.JSON(http.StatusCreated, out)
		case <-time.After(5 * time.Second):
			return c.JSON(http.StatusCreated, map[string]string{"status": "pending"})
		}
	}

}

func main() {
	flag.Parse()
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Fatalf("%v", err)
		return
	}
	log.Info("Successfully initialized docker client")
	problemsDir, err := filepath.Abs(*problems)
	if err != nil {
		log.Fatalf("%v", err)
		return
	}
	log.Infof("Using `%s` as problems directory", problemsDir)
	localAgent = &umpire.Agent{cli, problemsDir}

	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.POST("/judge", judge)
	e.POST("/run", run)

	// Start server
	if err := e.Start(":1323"); err != nil {
		e.Logger.Fatal(err.Error())
	}
}
