package backtest

import (
	"cmp"
	"fmt"
	"log"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Data wraps multipe data types e.g. bars, tape, ... and exposes
// several methods for easy access.
type Data struct {
	bars []marketdata.Bar
}

type Price string

const (
	Open  Price = "open"
	High  Price = "high"
	Low   Price = "low"
	Close Price = "close"
)

// Prices returns list of prices type e.g. open, close ...
func (d *Data) Prices(p Price) []float64 {
	var prices []float64
	for _, b := range d.Bars() {
		switch p {
		case Open:
			prices = append(prices, b.Open)
		case High:
			prices = append(prices, b.High)
		case Low:
			prices = append(prices, b.Low)
		default:
			prices = append(prices, b.Close)
		}

	}
	return prices
}

// Bars returns list of bars.
func (d *Data) Bars() []marketdata.Bar {
	return d.bars
}

// FirstBar returns first bar in bars' list.
func (d *Data) FirstBar() marketdata.Bar {
	return d.bars[0]
}

// LastBar returns last bar in bars' list.
func (d *Data) LastBar() marketdata.Bar {
	return d.bars[len(d.bars)-1]
}

func getBarPrice(bar Bar, p Price) float64 {
	return map[Price]float64{
		Open:  bar.Open,
		High:  bar.High,
		Low:   bar.Low,
		Close: bar.Close,
	}[p]
}

// FirstPrice returns first bar's price e.g. close, close ...
func (d *Data) FirstPrice(p Price) float64 {
	return getBarPrice(d.FirstBar(), p)
}

// LastPrice returns last bar's price e.g. close, close ...
func (d *Data) LastPrice(p Price) float64 {
	return getBarPrice(d.LastBar(), p)
}

// FirstClose returns first bar's close price.
func (d *Data) FirstClose() float64 {
	return d.FirstBar().Close
}

// FirstClose returns last bar's close price.
func (d *Data) LastClose() float64 {
	return d.LastBar().Close
}

// BarAt returns bar at given index allowing easy backward access.
//
//	BarAt(0) // get first bar
//	BarAt(-1) // get last bar
func (d *Data) BarAt(i int) marketdata.Bar {
	if i < 0 {
		return d.bars[len(d.bars)-(-i)]
	}
	return d.bars[i]
}

type Side string

const (
	Buy  Side = "buy"
	Sell Side = "sell"
)

// No ideal if we're planning on supporting other maket data providers
// I guess we'll have to map the data nd contruct the struct ourselves
type Bar = marketdata.Bar

type Backtest struct {
	broker   *broker
	data     *Data
	strategy func(s *Strategy) // make this an []StrategyCb instead
}

type Opts struct {
	// leverage       float64 -> no sure about this?
	// This is our starting capital. Defaults to 10,000.00.
	cash float64
	// Value between 0 and 1 sets default order size based on perc of capital. Defaults to 3%.
	orderSize float64
	// Value between 0 and 1 sets margin
	margin float64
	// Value between 0 and 1 sets commision charged per trade
	commision float64
	// Enter trade on bar close else default to next bar's open
	tradeOnClose bool
	// When true it'll keep only one trade open at a time
	exclusiveOrder bool
	// When set to true order size will be treated as `fractional` instead of
	// `notional` trade. E.g. 0.50 of a shared priced at $200 will create a trade of $100.
	fractionable bool
}

// New is our starting point. This is where we define our config for the backtest.
// TODO: I'm thinking of going the same direction as VectorBt where data fetching
// is built-in. All the user needs to provide a connection keys to a service e.g. Alpaca, Yahoo...
func New(bars []Bar, opts Opts) *Backtest {
	if opts.cash == 0 {
		opts.cash = 100000.00
	}
	if opts.orderSize == 0 {
		opts.orderSize = 0.03
	}

	broker := &broker{
		opts:     opts,
		cash:     opts.cash,
		data:     &Data{},
		equities: make([]float64, len(bars)),
	}
	broker.position = &Position{broker: broker}

	return &Backtest{
		broker: broker,
		data:   &Data{bars: bars},
	}
}

// Strategies is the user's main way of interacting with the lib. It takes
// a list of user defined callbacks we iterate and pass in an instance of `Strategy`
// to allow order placement, trade manipulation, data access, ...
func (bt *Backtest) Strategy(cb func(s *Strategy)) *Backtest {
	bt.strategy = cb
	return bt
}

// Run runs all strategies on each bar.
// TODO: This needs to change as we provide support for other data types (tape)
func (bt *Backtest) Run() *Backtest {
	// There's nothing we can do without data
	if len(bt.data.bars) == 0 {
		log.Fatalln("There isn't data available for this period.")
	}

	s := &Strategy{
		broker:       bt.broker,
		Data:         bt.broker.data,
		Position:     bt.broker.position,
		Orders:       bt.broker.orders,
		Trades:       bt.broker.trades,
		ClosedTrades: bt.broker.closedTrades,
	}

	for i := range bt.data.bars {
		// Data represents bars up until current index
		bt.broker.data.bars = bt.data.bars[:int(float64(i+1))]

		// TODO: Support more than one strategy
		bt.strategy(s)

		bt.broker.next() // I might rename this from `next` -> `process`
	}

	return bt
}

// Summary outputs the stats for strategies.
func (bt *Backtest) Summary() {
	equities := bt.broker.equities

	var drawdowns = make([]float64, len(equities))
	var peak = equities[0]
	for i, equity := range equities {
		if peak > equity {
			drawdowns[i] = equity/peak - 1
		} else {
			peak = equity
		}
	}

	var drawdownDurations []time.Duration
	var since time.Time
	for i, dd := range drawdowns {
		if dd < 0 && since.IsZero() {
			since = bt.data.Bars()[i].Timestamp
		} else if dd == 0 && !since.IsZero() {
			drawdownDurations = append(drawdownDurations, bt.data.Bars()[i].Timestamp.Sub(since))
			since = time.Time{}
		}
	}

	var returnsPct []float64
	var returns []float64
	var durations []time.Duration
	var exposure = make([]float64, len(bt.data.bars))
	for _, t := range bt.broker.closedTrades {
		returnsPct = append(returnsPct, t.PnlPct())
		returns = append(returns, t.Pnl())
		durations = append(durations, t.ExitTime().Sub(t.EntryTime()))
		for i := t.EntryBar; i <= t.ExitBar; i++ {
			exposure[i] = 1
		}
	}

	wrPct := (float64(count(returns, func(e float64) bool { return e > 0 })) /
		float64(len(bt.broker.closedTrades))) * 100

	retPct := (equities[len(equities)-1] - equities[0]) / equities[0] * 100

	bhRetPct := (bt.broker.data.LastClose() - bt.broker.data.FirstClose()) /
		bt.broker.data.FirstClose() * 100

	pos := sumFunc(returns, func(e float64) bool { return e > 0 })
	neg := math.Abs(sumFunc(returns, func(e float64) bool { return e < 0 }))
	pf := pos
	if neg != 0 {
		pf = pos / neg
	}

	data := [][2]string{
		{"start", bt.data.FirstBar().Timestamp.String()},
		{"end", bt.data.LastBar().Timestamp.String()},
		{"duration", bt.data.LastBar().Timestamp.Sub(bt.data.FirstBar().Timestamp).String()},
		{"exposure time", fmt.Sprintf("%f%%", mean(exposure)*100)},
		{"equity final", fmt.Sprintf("$%f", equities[len(equities)-1])},
		{"equity peak", fmt.Sprintf("$%f", slices.Max(equities))},
		{"return", fmt.Sprintf("%f%%", retPct)},
		{"buy & hold return", fmt.Sprintf("%f%%", bhRetPct)},
		{"max drawdown", fmt.Sprintf("$%f", slices.Min(equities))},
		{"max drawdown pct", fmt.Sprintf("%f%%", slices.Min(drawdowns)*100)},
		{"avg. drawdown", fmt.Sprintf("%f%%", mean(drawdowns)*100)},
		{"max. drawdown duration", slices.Max(drawdownDurations).String()},
		{"avg. drawdown duration", mean(drawdownDurations).String()},
		{"# trades", strconv.Itoa(len(bt.broker.closedTrades))},
		{"win rate", fmt.Sprintf("%f%%", wrPct)},
		{"best trade", fmt.Sprintf("%f%%", slices.Max(returnsPct)*100)},
		{"worst trade", fmt.Sprintf("%f%%", slices.Min(returnsPct)*100)},
		{"avg. trade", fmt.Sprintf("%f%%", mean(returnsPct)*100)},
		{"max. trade duration", slices.Max(durations).String()},
		{"avg. trade duration", mean(durations).String()},
		{"profit factor", fmt.Sprintf("%f", pf)},
		{"expectancy", fmt.Sprintf("%f%%", mean(returns)*100)},
	}

	row := slices.MaxFunc(data, func(a, b [2]string) int {
		return cmp.Compare(len(a[0]), len(b[0]))
	})
	span := len(row[0]) + 1

	var s strings.Builder
	for _, d := range data {
		var padding string
		for i := 0; i < span-(len(d[0])+1); i++ {
			padding += " "
		}
		s.WriteString(fmt.Sprintf("%s %s: %s\n", padding, cases.Title(language.English).String(d[0]), d[1]))
	}

	fmt.Println(s.String())
}
