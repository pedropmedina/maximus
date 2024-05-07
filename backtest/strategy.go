package backtest

type Strategy struct {
	broker       *broker
	Data         *Data
	Position     *Position
	Orders       []*Order
	Trades       []*Trade
	ClosedTrades []*Trade
}

type TradeOpts struct {
	Size  float64
	Stop  float64
	Limit float64
	SL    float64
	TP    float64
	Trade *Trade
}

func (s Strategy) Buy(opts TradeOpts) {
	s.broker.newOrder(newOrderOpts{
		side:  Buy,
		size:  opts.Size,
		stop:  opts.Stop,
		limit: opts.Limit,
		sl:    opts.SL,
		tp:    opts.TP,
		trade: opts.Trade,
	})
}

func (s Strategy) Sell(opts TradeOpts) {
	s.broker.newOrder(newOrderOpts{
		side:  Sell,
		size:  opts.Size,
		stop:  opts.Stop,
		limit: opts.Limit,
		sl:    opts.SL,
		tp:    opts.TP,
		trade: opts.Trade,
	})
}
