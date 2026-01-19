package main

import (
	"fmt"
	"net/http"

	"quicc/online/keys"
	custom_middlewares "quicc/online/middlewares"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
)

type Order struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}
	// KeyRing Setup
	keyStore := keys.KeyRing{}

	// Server Setup

	e := echo.New()

	api := e.Group("/api")

	admin := api.Group("")

	admin.Use(custom_middlewares.AdminPasscodeMiddleware())

	protected := api.Group("")

	protected.Use(custom_middlewares.RequireAuth(&keyStore))

	api.GET("/v1/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	protected.POST("/v1/orders", func(c echo.Context) error {
		newOrder := new(Order)
		if err := c.Bind(newOrder); err != nil {
			fmt.Printf("Error: %v\n", err)
			return c.String(http.StatusBadRequest, err.Error())
		}
		fmt.Printf("New order was created: %v\n", newOrder)
		return c.JSON(http.StatusCreated, newOrder)
	})

	api.GET("/v1/generate", func(c echo.Context) error {
		err := keyStore.Generate()
		if err != nil {
			return c.String(http.StatusInternalServerError, "Unable to generate key")
		}
		primedKey := keyStore.Primed
		return c.JSON(http.StatusOK, primedKey)
	})

	admin.GET("/v1/use", func(c echo.Context) error {
		err := keyStore.Use()
		if err != nil {
			return c.String(http.StatusInternalServerError, "Unable to use key")
		}
		activeKey := keyStore.Active
		activeKey.Status = "Active"
		return c.JSON(http.StatusOK, activeKey)
	})

	protected.GET("/v1/verify", func(c echo.Context) error {
		return c.String(http.StatusOK, "Verified")
	})

	e.Logger.Fatal(e.Start(":1323"))
}
