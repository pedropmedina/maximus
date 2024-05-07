package backtest

import (
	"log"
	"time"

	"github.com/google/uuid"
)

type Trade struct {
	Id         string
	Size       float64
	Side       Side
	EntryPrice float64
	ExitPrice  float64
	EntryBar   int
	ExitBar    int

	// legs keep track of trade's contingent orders i.e. stop and profit orders.
	legs [2]*Order

	// broker exposes broker functionality to the trade instance.
	broker *broker
}

// indexOf returns index of trade in slice else -1.
func (t *Trade) indexOf() int {
	for i, trade := range t.broker.trades {
		if t.Id == trade.Id {
			return i
		}
	}
	return -1
}

// setContingent sets `setStopLoss` and `setTakeProfit` lengs index(`i`).
func (t *Trade) setContingent(i int, price float64) {
	if price <= 0 {
		log.Fatalf("Price (%f) must be greater than 0\n", price)
	}
	o := t.legs[i]
	if o != nil {
		o.remove()
	}
	o = t.broker.newOrder(newOrderOpts{
		size:  t.Size,
		side:  reverseSide(t.Side),
		trade: t,
	})
	if i == 0 {
		o.Stop = price
	} else {
		o.Limit = price
	}
	t.legs[i] = o
}

// SetSL helps with setting trade's stop loss order.
func (t *Trade) SetSL(price float64) {
	t.setContingent(0, price)
}

// SetTP helps with setting trade's take profit order.
func (t *Trade) SetTP(price float64) {
	t.setContingent(1, price)
}

// isLong is true when side is `Buy`
func (t *Trade) IsLong() bool {
	return t.Side == Buy
}

// isShort is true when side is `Sell`
func (t *Trade) IsShort() bool {
	return t.Side == Sell
}

// EntryTime returns a `time.Time` when trade was entered.
func (t *Trade) EntryTime() time.Time {
	return t.broker.data.bars[t.EntryBar].Timestamp
}

// ExitTime returns a `time.Time` when trade was exited.
func (t *Trade) ExitTime() time.Time {
	return t.broker.data.bars[t.ExitBar].Timestamp
}

// Pnl calculates profits and losses per trade.
func (t *Trade) Pnl() float64 {
	price := t.broker.data.LastClose()
	if t.ExitPrice > 0 {
		price = t.ExitPrice
	}
	if t.IsLong() {
		return (price - t.EntryPrice) * t.Size
	}
	return (t.EntryPrice - price) * t.Size
}

// PnlPct is a percentage representation of `t.Pnl()`
func (t *Trade) PnlPct() float64 {
	price := t.broker.data.LastClose()
	if t.ExitPrice > 0 {
		price = t.ExitPrice
	}
	if t.IsLong() {
		return (price/t.EntryPrice - 1) * t.Size
	}
	return (t.EntryPrice/price - 1) * t.Size
}

// Value returns trade total value in cash (volume × price).
func (t *Trade) Value() float64 {
	price := t.broker.data.LastClose()
	if t.ExitPrice > 0 {
		price = t.ExitPrice
	}
	return t.Size * price
}

// Close places a new market order in the opposite direction to handle the closure of the trade.
func (t *Trade) Close() {
	o := &Order{
		Id:     uuid.NewString(),
		Side:   reverseSide(t.Side),
		Size:   t.Size, // I'm not 100% sure about this?
		trade:  t,
		broker: t.broker,
	}
	// Add new order to the front of the queue
	t.broker.orders = append([]*Order{o}, t.broker.orders...)
}
