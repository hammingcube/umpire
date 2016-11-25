package main

import (
	"flag"
	"fmt"
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
	serverdb = flag.String("serverdb", "", "server to get problems list (e.g. localhost:3013)")
)

var localAgent *umpire.Agent

func judge(c echo.Context) error {
	payload := &umpire.Payload{}
	if err := c.Bind(payload); err != nil {
		return err
	}
	c.Logger().Infof("judge: %#v", payload)
	done := make(chan interface{})
	go func() {
		done <- umpire.JudgeDefault(localAgent, payload)
	}()
	for {
		select {
		case out := <-done:
			return c.JSON(http.StatusCreated, out)
		case <-time.After(60 * time.Second):
			return c.JSON(http.StatusCreated, map[string]string{"status": "pending"})
		}
	}

}

func run(c echo.Context) error {
	payload := &umpire.Payload{}
	if err := c.Bind(payload); err != nil {
		return err
	}
	c.Logger().Infof("run: %#v", payload)
	out := umpire.RunDefault(localAgent, payload)
	return c.JSON(http.StatusCreated, out)
}

func initializeAgent(agent *umpire.Agent, problems, serverdb *string) error {
	if problems == nil || serverdb == nil {
		return fmt.Errorf("need to parse flags")
	}

	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}
	log.Info("Successfully initialized docker client")
	agent.Client = cli
	if *serverdb != "" {
		return nil
	}
	problemsDir, err := filepath.Abs(*problems)
	if err != nil {
		return err
	}
	agent.ProblemsDir = problemsDir
	log.Infof("Using `%s` as problems directory", problemsDir)
	return nil
}

func main() {
	flag.Parse()
	localAgent = &umpire.Agent{}
	if err := initializeAgent(localAgent, problems, serverdb); err != nil {
		log.Fatalf("failed to start: %v", err)
		return
	}

	e := echo.New()

	e.Logger.SetLevel(log.INFO)

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Routes
	e.POST("/judge", judge)
	e.POST("/run", run)

	// Start server
	if err := e.Start(":1323"); err != nil {
		e.Logger.Fatal(err.Error())
	}
}
