package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/labstack/gommon/log"
	"github.com/maddyonline/umpire"
	"math/rand"
	"net/http"
	"path/filepath"
	"time"
)

var (
	problems = flag.String("problems", "../../", "directory containing problems")
	serverdb = flag.String("serverdb", "", "server to get problems list (e.g. localhost:3013)")
)

var localAgent *umpire.Agent

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	flag.Parse()
	localAgent = &umpire.Agent{}

	if err := initializeAgent(localAgent, problems, serverdb); err != nil {
		log.Fatalf("failed to start: %v", err)
		return
	}

	ticker := time.NewTicker(120 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				log.Info("Reinitializing umpire agent")
				initializeAgent(localAgent, problems, serverdb)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	e := echo.New()

	e.Logger.SetLevel(log.INFO)

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Routes
	e.POST("/judge", judge)
	e.POST("/run", run)
	e.POST("/validate", validate)

	// Start server
	if err := e.Start(":1323"); err != nil {
		e.Logger.Fatal(err.Error())
	}
}

func createSubmission(uid string, payload *umpire.Payload, response *umpire.Response) error {
	v := &struct {
		*umpire.Payload
		*umpire.Response
	}{payload, response}
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(v); err != nil {
		return err
	}
	url := fmt.Sprintf("%s/users/%s/submissions", *serverdb, uid)
	log.Infof("Sending request to %s", url)
	req, err := http.NewRequest("POST", url, bytes.NewReader(b.Bytes()))
	if err != nil {
		return err
	}
	req.Header.Add("content-type", "application/json")
	if _, err = http.DefaultClient.Do(req); err != nil {
		return err
	}
	return nil
}

func judge(c echo.Context) error {
	payload := &umpire.Payload{}
	if err := c.Bind(payload); err != nil {
		return err
	}
	c.Logger().Infof("judge: %#v", payload)
	done := make(chan *umpire.Response)
	go func() {
		done <- umpire.JudgeDefault(localAgent, payload)
	}()
	for {
		select {
		case out := <-done:
			go func() { createSubmission("anon", payload, out) }()
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

func validate(c echo.Context) error {
	jd := &umpire.JudgeData{}
	if err := c.Bind(jd); err != nil {
		return err
	}
	key, err := localAgent.UpdateProblemsCache(jd)
	if err != nil {
		return err
	}
	defer localAgent.RemoveFromProblemsCache(key)
	payload := &umpire.Payload{
		Problem:  &umpire.Problem{Id: key},
		Language: jd.Solution.Language,
		Files:    jd.Solution.Files,
	}
	out := umpire.JudgeDefault(localAgent, payload)
	return c.JSON(http.StatusOK, out)
}

func fetchProblems(apiUrl string) (map[string]*umpire.JudgeData, error) {
	url := fmt.Sprintf("%s/problems", apiUrl)
	log.Infof("Sending request to %s", url)
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("content-type", "application/json")
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	v := map[string]*umpire.JudgeData{}
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		return nil, err
	}
	log.Infof("fetchProblems: %#v", v)
	return v, nil
}

func initializeAgent(agent *umpire.Agent, problems, serverdb *string) error {
	if problems == nil || serverdb == nil {
		return fmt.Errorf("need to parse flags")
	}
	if agent.Client == nil {
		cli, err := client.NewEnvClient()
		if err != nil {
			return err
		}
		agent.Client = cli
		log.Info("Successfully initialized docker client")
	}

	if *serverdb != "" {
		log.Infof("Fetching problems from %s", *serverdb)
		data, err := fetchProblems(*serverdb)
		if err != nil {
			return err
		}
		agent.Data = data
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
