package backtest

import (
	"log"
)

type OrderType string

const (
	Market OrderType = "market"
	Limit  OrderType = "limit"
	Stop   OrderType = "stop"
)

type Order struct {
	Id      string
	Size    float64
	Side    Side
	Stop    float64
	Limit   float64
	SL      float64
	TP      float64
	trade   *Trade
	broker  *broker
	hitAtOt OrderType
}

// indexOf returns index of order in queue else -1
func (o *Order) indexOf() int {
	for i, order := range o.broker.orders {
		if order.Id == o.Id {
			return i
		}
	}
	return -1
}

// remove deletes order from queue.
func (o *Order) remove() {
	i := o.indexOf()
	if i < 0 {
		log.Fatalf("No order found with id: %s\n", o.Id)
	}
	o.broker.orders = append(o.broker.orders[:i], o.broker.orders[i+1:]...)
}

// Cancel removes order from queue and itself from the parent trade's legs slice.
func (o *Order) Cancel() {
	if o.trade != nil {
		for _, leg := range o.trade.legs {
			if leg != nil && leg.Id == o.Id {
				leg = nil
			}
		}
	}
	o.remove()
}

// IsLong checks if order.Side is `Long`.
func (o *Order) IsLong() bool {
	return o.Side == Buy
}

// IsShort checks if order.Side is `Short`.
func (o *Order) IsShort() bool {
	return !o.IsLong()
}

// IsContingent checks whether order is part of a trade's legs slice(stop loss or take profit).
func (o *Order) IsContingent() bool {
	return o.trade != nil
}
