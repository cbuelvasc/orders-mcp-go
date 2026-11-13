// Package domain is the core of the hexagon: pure Go, standard library only.
package domain

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	orderIDFormat    = regexp.MustCompile(`^ORD-\d{1,10}$`)
	customerIDFormat = regexp.MustCompile(`^CUS-\d{1,10}$`)
)

// OrderID is a value object: it can only hold a valid id like ORD-1042.
type OrderID struct{ value string }

// ParseOrderID normalizes and validates an order id.
func ParseOrderID(raw string) (OrderID, error) {
	v := strings.ToUpper(strings.TrimSpace(raw))
	if !orderIDFormat.MatchString(v) {
		return OrderID{}, fmt.Errorf("%w: invalid order id %q, expected format ORD-<digits>", ErrInvalidInput, v)
	}
	return OrderID{v}, nil
}

func (id OrderID) String() string { return id.value }

// CustomerID is a value object: it can only hold a valid id like CUS-7.
type CustomerID struct{ value string }

// ParseCustomerID normalizes and validates a customer id.
func ParseCustomerID(raw string) (CustomerID, error) {
	v := strings.ToUpper(strings.TrimSpace(raw))
	if !customerIDFormat.MatchString(v) {
		return CustomerID{}, fmt.Errorf("%w: invalid customer id %q, expected format CUS-<digits>", ErrInvalidInput, v)
	}
	return CustomerID{v}, nil
}

func (id CustomerID) String() string { return id.value }
