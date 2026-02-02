package order

type MiscCharge struct {
	MiscChargeName   string
	MiscChargeDesc   string
	MiscChargeAmount float64
}

type Tax struct {
	TaxName   string
	TaxAmount float64
}

type Customer struct {
	FirstName string
	LastName  string
	Company   string
	Phone     string
	Ext       string
	Email     string
}

type Delivery struct {
	Street       string
	CrossStreets string
	Suite        string
	Buz          string
	City         string
	State        string
	Zip          string
}

type Payment struct {
	Type          string
	Amount        float64
	CardNumber    string
	CardHolder    string
	AuthCode      string
	TransactionID string
	Token         string
}

type Item struct {
	Name      string
	SizeID    int
	SizeName  string
	Quantity  int
	Price     float64
	PLU       string
	Who       string
	GroupID   string
	Notes     string
	Modifiers []Modifier
}

type Modifier struct {
	Side     string
	Name     string
	Quantity int
	PLU      string
	Price    float64
	Action   string
}

type Coupon struct {
	Serial  string
	PLU     string
	Name    string
	Value   float64
	GroupID string
}
