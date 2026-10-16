package orderApp

// RelayOrderCommand carries an inbound order from the HTTP handler to the
// service. Body is the raw bytes the vendor POSTed; OrderID is sha256(body).
// We never inspect Body, never persist it, and never log it — it goes
// straight to SQS and out of our hands.
type RelayOrderCommand struct {
	OrderID string
	Body    []byte
}

type RelayOrderResult struct {
	Status     string `json:"status"`
	ExtOrderID string `json:"ext_order_id"`
	BodyBytes  int    `json:"body_bytes"`
}
