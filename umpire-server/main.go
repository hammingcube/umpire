package main

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/standard"
	"github.com/labstack/echo/middleware"
	"github.com/maddyonline/umpire"
	"net/http"
	"strconv"
	"time"
)

type (
	user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
)

var (
	users = map[int]*user{}
	seq   = 1
)

//----------
// Handlers
//----------

func createUser(c echo.Context) error {
	payload := &umpire.Payload{}
	if err := c.Bind(payload); err != nil {
		return err
	}
	done := make(chan interface{})
	go func() {
		done <- umpire.Judge(payload)
	}()
	for {
		select {
		case out := <-done:
			return c.JSON(http.StatusCreated, out)
		case <-time.After(5 * time.Second):
			return c.JSON(http.StatusCreated, map[string]string{"status": "pending"})
		}
	}

	// users[u.ID] = u
	// seq++
	//return c.JSON(http.StatusCreated, payload)
}

func getUser(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	return c.JSON(http.StatusOK, users[id])
}

func updateUser(c echo.Context) error {
	u := new(user)
	if err := c.Bind(u); err != nil {
		return err
	}
	id, _ := strconv.Atoi(c.Param("id"))
	users[id].Name = u.Name
	return c.JSON(http.StatusOK, users[id])
}

func deleteUser(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	delete(users, id)
	return c.NoContent(http.StatusNoContent)
}

func main() {
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.POST("/users", createUser)
	e.GET("/users/:id", getUser)
	e.PUT("/users/:id", updateUser)
	e.DELETE("/users/:id", deleteUser)

	// Start server
	e.Run(standard.New(":1323"))
}
