package backtest

type Position struct {
	broker *broker
}

// Position size in units of asset. Negative if position is short.
func (p *Position) Size() float64 {
	var sum float64
	for _, t := range p.broker.trades {
		sum += t.Size
	}
	return sum
}

// Profit (positive) or loss (negative) of the current position in cash units.
func (p *Position) Pnl() float64 {
	var sum float64
	for _, t := range p.broker.trades {
		sum += t.Pnl()
	}
	return sum
}

// Close portion of position by closing `portion` of each active trade. See `Trade.close`.
func (p *Position) Close(portion float64) {
	for _, t := range p.broker.trades {
		t.Close()
	}

}

// NO SURE ABOUT THESE!
//         //Profit (positive) or loss (negative) of the current position in percent.
//     func (p *Position) pnlPct()  float {
//         weights = np.abs([trade.size for trade in self.__broker.trades])
//         weights = weights / weights.sum()
//         pl_pcts = np.array([trade.pl_pct for trade in self.__broker.trades])
//         return (pl_pcts * weights).sum()
// }
//
//
//         //True if the position is long (position size is positive).
//     func (p *Position) isLong()  bool {
//         return self.size > 0
// }
//
//
//         //True if the position is short (position size is negative).
//     func (p *Position) isShort()  bool {
//         return self.size < 0
// }
