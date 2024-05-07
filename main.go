// Design:
// bt := NewBt(BtOptions{ Symbol: "APPL", Cash: 10000.00, Comission: ...})
// bt.Run(Strategy(StrategyOptions{}))
package main

import (
	"log"
	"os"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/joho/godotenv"
	"github.com/pedropmedina/maximus/backtest"
	"github.com/pedropmedina/maximus/strategies"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	// Alpaca marketdata client
	c := marketdata.NewClient(marketdata.ClientOpts{
		BaseURL:   os.Getenv("APCA_API_DATA_URL"),
		APIKey:    os.Getenv("APCA_API_KEY_ID"),
		APISecret: os.Getenv("APCA_API_SECRET_KEY"),
	})

	// 5 min marketdata for SPY
	bars, err := c.GetBars("SPY", marketdata.GetBarsRequest{
		TimeFrame: marketdata.NewTimeFrame(5, marketdata.Min),
		Start:     time.Date(2024, 2, 10, 9, 30, 0, 0, time.UTC),
		End:       time.Date(2024, 2, 21, 16, 30, 0, 0, time.UTC),
	})
	if err != nil {
		panic(err)
	}

	// maximus.New(bars, { Symbol: "APPL", ...}).Strategy().Run().Summary()
	backtest := backtest.New(bars, backtest.Opts{}).Strategy(strategies.CloseOverSMA(14))
	backtest.Run().Summary()
}
