package strategies

import (
	"github.com/pedropmedina/maximus/backtest"
	"github.com/pedropmedina/maximus/indicators"
)

// CloseOverSMA sells when the bar's close price is below the SMA and
// buys when it closes above SMA.
func CloseOverSMA(period int) func(s *backtest.Strategy) {
	return func(s *backtest.Strategy) {
		sma := indicators.SMA(period, s.Data.Prices(backtest.Close))
		bar := s.Data.LastBar()
		ma := sma[len(sma)-1]

		// Buy signal
		if bar.Open < ma && bar.Close > ma {
			s.Buy(backtest.TradeOpts{})
		}

		// Sell signal
		if bar.Open > ma && bar.Close < ma {
			s.Sell(backtest.TradeOpts{})
		}
	}
}
