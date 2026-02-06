package handler

import (
	"io"
	"net/http"
	"strconv"
	"time"

	orderApp "quicc/online/internal/app/order"
	"quicc/online/internal/domain/order"

	"github.com/labstack/echo/v4"
)

func (h *Handler) CreateOrder(c echo.Context) error {
	h.log.Info().Msg("Order request received")
	newOrder := orderApp.OrderRequest{}
	if err := c.Bind(&newOrder); err != nil {
		h.log.Error().Err(err).Msg("Unable to bind request")
		return c.String(http.StatusBadRequest, "Unable to bind request")
	}
	h.log.Debug().Msg("Order request bound")
	raw_payload, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.String(http.StatusBadRequest, "Unable to read request body")
	}
	h.log.Debug().Msg("Request body read")
	dateParsed, err := time.Parse(time.RFC3339, newOrder.SubmittedDate)
	if err != nil {
		return c.String(http.StatusBadRequest, "Unable to parse date")
	}
	h.log.Debug().Msg("Date parsed")
	dateCreated := dateParsed.Format("2006-01-02")
	h.orderSvc.Create(orderApp.CreateOrderCommand{
		OrderID:     strconv.Itoa(newOrder.OrderID),
		Payload:     string(raw_payload),
		DateCreated: dateCreated,
		CreatedAt:   newOrder.SubmittedDate,
	})
	h.log.Debug().Msg("Relaying to Publisher")
	err = h.orderSvc.RelayOrder(orderApp.RelayOrderCommand{
		OrderID: strconv.Itoa(newOrder.OrderID),
		Order: order.Order{
			ID:          strconv.Itoa(newOrder.OrderID),
			Payload:     string(raw_payload),
			DateCreated: dateCreated,
			CreatedAt:   newOrder.SubmittedDate,
		},
	})
	if err != nil {
		h.log.Error().Err(err).Msg("Error relaying order")
		return c.String(http.StatusInternalServerError, "Error relaying order")
	}
	h.log.Debug().Msg("Order relay successful")
	h.log.Info().Msg("Order request processed successfully")
	return c.String(http.StatusCreated, "Created order")
}

// TODO: Implement
func (h *Handler) GetTodaysOrders(c echo.Context) error {
	return c.String(http.StatusOK, "Todays orders")
}

// TODO: Implement
func (h *Handler) GetLatestOrder(c echo.Context) error {
	return c.String(http.StatusOK, "Latest order")
}
