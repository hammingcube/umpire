package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/labstack/gommon/log"
	"github.com/maddyonline/umpire"
	"math/rand"
	"net/http"
	"time"
)

const REFRESH_INTERVAL = 120 * time.Second

var judgeDataSource []map[string]*umpire.JudgeData
var currentSrc int

func init() {
	rand.Seed(time.Now().UnixNano())
}

var (
	problemsdir = flag.String("problemsdir", "../../problems", "directory containing problems")
	serverdb    = flag.String("serverdb", "", "server to get problems list (e.g. http://localhost:3033)")
)

func main() {
	judgeDataSource = make([]map[string]*umpire.JudgeData, 2)
	flag.Parse()
	agent, err := umpire.NewAgent(nil, nil)
	if err != nil {
		log.Fatalf("failed to start: %v", err)
		return
	}
	updateJudgeData(agent, problemsdir, serverdb)
	go refreshJudgeData(agent, problemsdir, serverdb)
	server := NewUmpireServer(agent)
	e := server.e
	if err := e.Start(":1323"); err != nil {
		e.Logger.Fatal(err.Error())
	}
}

func refreshJudgeData(agent *umpire.Agent, problemsdir, serverdb *string) {
	ticker := time.NewTicker(REFRESH_INTERVAL)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				log.Info("Refreshing umpire data")
				updateJudgeData(agent, problemsdir, serverdb)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
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

type UmpireServer struct {
	localAgent *umpire.Agent
	e          *echo.Echo
}

func NewUmpireServer(localAgent *umpire.Agent) *UmpireServer {
	if localAgent == nil {
		return nil
	}
	e := echo.New()
	server := &UmpireServer{
		localAgent,
		e,
	}
	e.Logger.SetLevel(log.INFO)

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Routes
	e.POST("/judge", server.judge)
	e.POST("/run", server.run)
	e.POST("/validate", server.validate)
	e.POST("/execute", server.execute)

	return server
}

func (us *UmpireServer) judge(c echo.Context) error {
	localAgent := us.localAgent
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

func (us *UmpireServer) run(c echo.Context) error {
	localAgent := us.localAgent
	payload := &umpire.Payload{}
	if err := c.Bind(payload); err != nil {
		return err
	}
	c.Logger().Infof("run: %#v", payload)
	out := umpire.RunDefault(localAgent, payload)
	return c.JSON(http.StatusOK, out)
}

func (us *UmpireServer) execute(c echo.Context) error {
	localAgent := us.localAgent
	payload := &umpire.Payload{}
	if err := c.Bind(payload); err != nil {
		return err
	}
	c.Logger().Infof("execute: %#v", payload)
	out := umpire.ExecuteDefault(localAgent, payload)
	return c.JSON(http.StatusOK, out)
}

func (us *UmpireServer) validate(c echo.Context) error {
	localAgent := us.localAgent
	jd := &umpire.JudgeData{}
	if err := c.Bind(jd); err != nil {
		return err
	}
	err, out := umpire.Validate(localAgent, jd)
	if err != nil {
		return err
	}
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
	log.Infof("fetchProblems: number of problems=%d", len(v))
	return v, nil
}

func updateJudgeData(agent *umpire.Agent, problemsdir, serverdb *string) {
	m := map[string]*umpire.JudgeData{}
	if serverdb != nil && *serverdb != "" {
		log.Infof("Fetching problems from %s", *serverdb)
		if data, err := fetchProblems(*serverdb); err == nil && data != nil {
			for k, v := range data {
				m[k] = v
			}
		} else {
			log.Infof("data=%+v, err=%v in fetching problems", data, err)
		}
	}
	if problemsdir != nil && *problemsdir != "" {
		data := map[string]*umpire.JudgeData{}
		log.Infof("Using %s directory as source of problems", *problemsdir)
		if err := umpire.ReadAllProblems(data, *problemsdir); err == nil {
			log.Infof("number of problems read from directory=%d", len(data))
			for k, v := range data {
				m[k] = v
			}
		} else {
			log.Infof("data=%+v, err=%v in reading problemsdir", data, err)
		}
	}
	judgeDataSource[(currentSrc+1)%2] = m
	currentSrc = (currentSrc + 1) % 2
	// Make the following step safe for concurrent use
	agent.Data = judgeDataSource[currentSrc]
}
