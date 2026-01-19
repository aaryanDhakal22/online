package main

import (
	"fmt"
	"net/http"

	"quicc/online/keys"

	"github.com/labstack/echo/v4"
)

type Order struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func main() {
	// KeyRing Setup
	keyStore := keys.KeyRing{}

	// Server Setup

	e := echo.New()

	api := e.Group("/api")

	api.GET("/v1/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	api.POST("/v1/orders", func(c echo.Context) error {
		// Test authorization with key
		if c.Request().Header.Get("Authorization") != "Bearer "+keyStore.Active.Key {
			return c.String(http.StatusUnauthorized, "Unauthorized")
		}
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
		newString := fmt.Sprintf("Key: %s\nGenerated at: %s", primedKey.Key, primedKey.GeneratedAt)
		fmt.Println(newString)
		keyStore.Status()
		return c.String(http.StatusOK, newString)
	})

	api.GET("/v1/use", func(c echo.Context) error {
		err := keyStore.Use()
		if err != nil {
			return c.String(http.StatusInternalServerError, "Unable to use key")
		}
		activeKey := keyStore.Active
		activeKey.Status = "Active"
		newString := fmt.Sprintf("Key: %s\nStatus: %s", activeKey.Key, activeKey.Status)
		fmt.Println(newString)
		keyStore.Status()
		return c.String(http.StatusOK, newString)
	})

	e.Logger.Fatal(e.Start(":1323"))
}
