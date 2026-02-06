package handler

import (
	"io"
	"net/http"
	"strconv"
	"time"

	orderApp "quicc/online/internal/app/order"

	"github.com/labstack/echo/v4"
)

func (h *Handler) CreateOrder(c echo.Context) error {
	newOrder := orderApp.OrderRequest{}
	if err := c.Bind(&newOrder); err != nil {
		return c.String(http.StatusBadRequest, "Unable to bind request")
	}
	raw_payload, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.String(http.StatusBadRequest, "Unable to read request body")
	}
	dateParsed, err := time.Parse(time.RFC3339, newOrder.SubmittedDate)
	if err != nil {
		return c.String(http.StatusBadRequest, "Unable to parse date")
	}
	dateCreated := dateParsed.Format("2006-01-02")
	h.orderSvc.Create(orderApp.CreateOrderCommand{
		OrderID:     strconv.Itoa(newOrder.OrderID),
		Payload:     string(raw_payload),
		DateCreated: dateCreated,
		CreatedAt:   newOrder.SubmittedDate,
	})
	h.log.Info().Msg("Order request received")
	return c.String(http.StatusOK, "Created order")
}

func (h *Handler) GetTodaysOrders(c echo.Context) error {
	return c.String(http.StatusOK, "Todays orders")
}

func (h *Handler) GetLatestOrder(c echo.Context) error {
	return c.String(http.StatusOK, "Latest order")
}
