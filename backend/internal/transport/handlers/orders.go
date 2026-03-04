package handler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"quicc/online/internal/domain/order"

	orderApp "quicc/online/internal/app/order"

	"github.com/labstack/echo/v4"
)

func (h *Handler) CreateOrder(c echo.Context) error {
	h.log.Info().Msg("Order request received")
	newOrder := orderApp.OrderRequest{}
	if err := c.Bind(&newOrder); err != nil {
		h.log.Error().Err(err).Msg("Unable to bind request")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Bad request", "message": "Unable to bind data, please check the order body"})
	}
	h.log.Debug().Msg("Order request bound")
	raw_payload, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Bad request", "message": "Unable to bind data, please check the order body"})
	}
	h.log.Debug().Msg("Request body read")
	// Log the submitted date to the console
	layout := "2006-01-02T15:04:05-0300"

	h.log.Debug().Str("submitted_date", newOrder.SubmittedDate).Msg("Submitted date")

	h.log.Debug().Str("valid_date", time.Now().Format(layout)).Msg("time.Now().Format(layout) result")

	// Parse the submitted date
	dateParsed, err := time.Parse(layout, newOrder.SubmittedDate)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Bad request", "message": "Unable to parse order submission date"})
	}
	h.log.Debug().Msg("Date parsed successfull")
	dateCreated := dateParsed.Format("2006-01-02")
	orderID := strconv.Itoa(newOrder.OrderID)
	out, err := h.orderSvc.Create(orderApp.CreateOrderCommand{
		OrderID:     orderID,
		Payload:     string(raw_payload),
		DateCreated: dateCreated,
		CreatedAt:   newOrder.SubmittedDate,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("Error creating order")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error creating order", "message": "Unable to create order"})
	}
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
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error relaying order", "message": "Unable to relay order"})
	}
	h.log.Debug().Msg("Order relay successful")
	h.log.Info().Msg("Order request processed successfully")
	h.notifier.Send(fmt.Sprintf("Order ID: %s, Date Created: %s", orderID, dateCreated))
	return c.JSON(http.StatusCreated, out)
}

// TODO: Implement
func (h *Handler) GetTodaysOrders(c echo.Context) error {
	return c.String(http.StatusOK, "Todays orders")
}

// TODO: Implement
func (h *Handler) GetLatestOrder(c echo.Context) error {
	return c.String(http.StatusOK, "Latest order")
}
