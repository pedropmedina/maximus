package backtest

import (
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/google/uuid"
)

type broker struct {
	opts         Opts
	data         *Data
	position     *Position
	orders       []*Order
	trades       []*Trade
	closedTrades []*Trade
	equities     []float64
	cash         float64
}

type newOrderOpts struct {
	size   float64
	side   Side
	stop   float64
	limit  float64
	sl     float64
	tp     float64
	trade  *Trade
	broker *broker
}

// newOrder adds a new order to the queue, but before doing so, it validates prices
// and checks for `opts.exclusiveOrder`.
func (b *broker) newOrder(opts newOrderOpts) *Order {
	if !b.opts.fractionable && opts.size > 1 && !isWhole(opts.size) {
		log.Fatalf("Can't evaluate fractional size (%f > 1.00) unless `opts.fractionable` is set to `true`\n", opts.size)
	}

	var price float64
	if opts.limit > 0 {
		price = opts.limit
	} else if opts.stop > 0 {
		price = opts.stop
	} else {
		price = b.data.LastClose()
	}
	// Assert stop loss < entry < target profit for long position
	if opts.side == Buy {
		var msg strings.Builder
		msg.WriteString("Long orders require: \n")
		if opts.sl > 0 && opts.tp > 0 &&
			!(opts.sl < price && opts.tp > price) {
			fmt.Fprintf(&msg, "Stop Loss (%f) < ", opts.sl)
			fmt.Fprintf(&msg, "Price (%f) < ", price)
			fmt.Fprintf(&msg, "Profit Target (%f)", opts.tp)
			log.Fatalln(msg)
		} else if opts.sl > 0 && opts.sl > price {
			fmt.Fprintf(&msg, "Stop Loss (%f) < ", opts.sl)
			fmt.Fprintf(&msg, "Price (%f) ", price)
			log.Fatalln(msg)
		} else if opts.tp > 0 && opts.tp < price {
			fmt.Fprintf(&msg, "Profit Target (%f) > ", opts.tp)
			fmt.Fprintf(&msg, "Price (%f) ", price)
			log.Fatalln(msg)
		}
	}
	// Assert stop loss > entry > target profit for short position
	if opts.side == Sell {
		var msg strings.Builder
		msg.WriteString("Short orders require: \n")
		if opts.sl > 0 && opts.tp > 0 &&
			!(opts.sl > price && opts.tp < price) {
			fmt.Fprintf(&msg, "Stop Loss (%f) > ", opts.sl)
			fmt.Fprintf(&msg, "Price (%f) > ", price)
			fmt.Fprintf(&msg, "Profit Target (%f)", opts.tp)
			log.Fatalln(msg)
		} else if opts.sl > 0 && opts.sl < price {
			fmt.Fprintf(&msg, "Stop Loss (%f) > ", opts.sl)
			fmt.Fprintf(&msg, "Price (%f) ", price)
			log.Fatalln(msg)
		} else if opts.tp > 0 && opts.tp > price {
			fmt.Fprintf(&msg, "Profit Target (%f) < ", opts.tp)
			fmt.Fprintf(&msg, "Price (%f) ", price)
			log.Fatalln(msg)
		}
	}
	// Include broker instance if not preset in opts
	if opts.broker == nil {
		opts.broker = b
	}
	// New order
	order := &Order{
		Id:     uuid.NewString(),
		Size:   opts.size,
		Side:   opts.side,
		Stop:   opts.stop,
		Limit:  opts.limit,
		SL:     opts.sl,
		TP:     opts.tp,
		trade:  opts.trade,
		broker: opts.broker,
	}
	// Prioritize order related to open trades by putting them to top of the queue
	if opts.trade != nil && opts.trade.indexOf() >= 0 {
		b.orders = append([]*Order{order}, b.orders...)
	} else {
		if b.opts.exclusiveOrder {
			for _, o := range b.orders {
				if o.trade == nil {
					o.Cancel()
				}
			}
			for _, t := range b.trades {
				t.Close()
			}
		}
		b.orders = append(b.orders, order)
	}

	return order
}

// processOrders evaluates and processes orders in the queue one at a time.
func (b *broker) processOrders() {
	reprocess := false
	barI := len(b.data.bars) - 1
	bar := b.data.bars[barI]
	prevBar := b.data.bars[max(0, barI-1)]
	for _, o := range b.orders {
		// Stop orders are handle down below in the `else` clause of the limit order
		// as they become `market` orders once hit. There are instances where an order
		// can include both `stop` and `limit` prices and be reached/hit within the
		// same bar. In such instances we prioritize the stop/market order as there's
		// no predictive way of determining which got hit first `stop/market` or `limit`
		// unless we take a look at the tape or something more in detail which is outside
		// the scope of this project for now.
		if o.Stop > 0 && o.hitAtOt == "" {
			isStopHit := (o.IsLong() && bar.High >= o.Stop) ||
				(o.IsShort() && bar.Low <= o.Stop)
			if !isStopHit {
				continue
			}

			o.hitAtOt = Stop
		}
		// Check for limit order and handle price accordingly or else deal with
		// market/stop(become market orders) orders taking into account `tradeOnClose`
		// option which sets our price to the previous bar's close instead of the current
		// bar's open.
		var price float64
		if o.Limit > 0 && o.hitAtOt == "" {
			isLimitHit := (o.IsLong() && bar.Low < o.Limit) ||
				(o.IsShort() && bar.High > o.Limit)
			if !isLimitHit {
				continue
			}

			if o.IsLong() {
				price = min(bar.Open, o.Limit)
			} else {
				price = max(bar.Open, o.Limit)
			}

			o.hitAtOt = Limit
		} else {
			if b.opts.tradeOnClose {
				price = prevBar.Close
			} else {
				price = bar.Open
			}

			if o.Stop > 0 {
				if o.IsLong() {
					price = max(price, o.Stop)
				} else {
					price = min(price, o.Stop)
				}
			}

			o.hitAtOt = Market
		}
		// `processedAtBarI` is key in referencing the bar we process the order at.
		var processedAtBarI int
		if o.hitAtOt == Market && b.opts.tradeOnClose {
			processedAtBarI = barI - 1
		} else {
			processedAtBarI = barI
		}
		// This ensures we handle fractional and non-fractional orders in accordance with
		// the user's params and default opts. The main difference it's that fractional
		// orders are treated as is as opposed to non-fractional orders meaning we don't do
		// any kind of rounding of the size.
		var size float64
		if b.opts.fractionable {
			if o.Size == 0 {
				size = max((b.opts.orderSize*b.cash)/price, 0)
			} else {
				size = max(o.Size, 0)
			}
		} else {
			if o.Size == 0 {
				size = max(math.Floor((b.opts.orderSize*b.cash)/price), 0)
			} else if o.Size < 1 {
				size = max(math.Floor((o.Size*b.cash)/price), 0)
			} else {
				size = max(math.Floor(o.Size), 0)
			}
		}
		// TODO: This is not a good solution (IMPROVE ME). Look into context and signals to handle!
		if size == 0 {
			o.Cancel()
			fmt.Printf("size: %f, orderSize: %f, cash: %f, price: %f\n", size, b.opts.orderSize, b.cash, price)
			continue
		}

		// Given an order in the opposite side of existing trade(s) we'll want to reduce
		// and/or close trade(s) given our computed size above ^. Notice we prioritize
		// order's parent trade before iterating open trades.
		if o.trade != nil && o.trade.Side != o.Side {
			if o.trade.indexOf() >= 0 {
				b.reduceTrade(o.trade, size, price, processedAtBarI)
			} else {
				o.Cancel()
			}
			continue
		}
		for _, trade := range b.trades {
			if trade.Side == o.Side {
				continue
			}
			if size >= trade.Size {
				b.closeTrade(trade, price, processedAtBarI)
				size -= trade.Size
			} else {
				b.reduceTrade(trade, size, price, processedAtBarI)
				size = 0
			}
			if size == 0 {
				break
			}
		}
		// Create new trade with size left following closing of open trades. Notice we're
		// reprocessing this order right away via recursion if the order itself it's market
		// order and it includes a stop loss and/or target profit in order to address
		// legs hit within same bar
		if size > 0 {
			b.openTrade(
				o.Side,
				size,
				price,
				o.SL,
				o.TP,
				processedAtBarI,
			)
			if o.hitAtOt == Market && (o.SL > 0 || o.TP > 0) {
				reprocess = true
			}
		}

		// Order could have been closed by its parent trade.
		if o != nil {
			o.remove()
		}
	}
	// Recursively go all over queue of orders with the same bar's data in order to
	// handle stop losses and/or take profit orders that might get hit within the same
	// bar its entry order.
	//
	//       height @ 20.00 ->
	//                        ▐
	//                        ▐▔ <- take profit @ 18.50
	//       entry @ 17.50 -> ▐
	//                        ▐
	//                       ▔▐
	//                          <- low @ 15.00
	//
	if reprocess {
		b.processOrders()
	}
}

// equity returns sum of open trade's PnL plus available cash.
func (b *broker) equity() float64 {
	var sum float64
	for _, t := range b.trades {
		sum += t.Pnl()
	}
	return sum + b.cash
}

// openTrade creates a new trade and adds stop loss and take profit target when requested.
func (b *broker) openTrade(side Side, size, price, slPrice, tpPrice float64, processedAtBarI int) {
	trade := &Trade{
		Id:         uuid.NewString(),
		Size:       size,
		Side:       side,
		EntryPrice: price,
		EntryBar:   processedAtBarI,
		broker:     b,
	}
	b.trades = append(b.trades, trade)
	// create a S/L order
	if slPrice > 0 {
		trade.SetSL(slPrice)
	}
	// create T/P order
	if tpPrice > 0 {
		trade.SetTP(tpPrice)
	}
}

// closeTrade moves trade to closedTrades list and removes any pending leg order.
func (b *broker) closeTrade(trade *Trade, price float64, processedAtBarI int) {
	trade.ExitPrice = price
	trade.ExitBar = processedAtBarI

	i := trade.indexOf()
	b.closedTrades = append(b.closedTrades, trade)
	b.trades = append(b.trades[:i], b.trades[i+1:]...)

	for _, o := range trade.legs {
		if o != nil {
			o.remove()
		}
	}

	// Update cash
	b.cash += trade.Pnl()
}

// reduceTrade reduces the size of the trade given the size param. This is the case
// when an order it's hit in the opposite side, yet its size isn't big enough to close
// the trade. When reducing the trade, we're essentially creating a new trade(entry)
// with the remaining size and closing the previous one with the reduced size at the given price.
func (b *broker) reduceTrade(trade *Trade, size, price float64, processedAtBarI int) {
	sizeLeft := trade.Size - size

	// Something it's wrong with the strategy where allocated order's size in the opposite
	// way is greater than the existing trade.
	if sizeLeft < 0 {
		log.Fatalf("Size provided of %f can't be greater than trade size %f\n", size, trade.Size)
	}

	// Is size left is 0, then we just have to close the trade as we're reducing its entirety
	if sizeLeft == 0 {
		b.closeTrade(trade, price, processedAtBarI)
		return
	}

	// Reduce size for trade and legs
	trade.Size = sizeLeft
	for _, o := range trade.legs {
		if o != nil {
			o.Size = trade.Size
		}
	}

	// New transacted trade reflects that new trade taken with the given size and price
	closedTrade := *trade
	closedTrade.Id = uuid.NewString()
	closedTrade.Size = size
	closedTrade.legs = [2]*Order{}
	b.trades = append(b.trades, &closedTrade)
	b.closeTrade(&closedTrade, price, processedAtBarI)
}

// next processes orders and update cash and equity
func (b *broker) next() {
	// Process orders on each bar
	b.processOrders()

	// Last bar index
	i := len(b.data.bars) - 1

	// Calculate equity on each bar
	equity := b.equity()

	// Track equity curve on each bar iteration
	b.equities[i] = equity

	// Close all trades once account is blown-up
	if equity <= 0 {
		for _, t := range b.trades {
			b.closeTrade(t, b.data.LastBar().Close, i)
		}

		// Update account
		b.cash = 0

		// Exit program
		log.Fatalf("You blew up your account!\n")
	}
}
