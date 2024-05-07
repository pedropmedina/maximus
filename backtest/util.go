package backtest

import (
	"math"

	"golang.org/x/exp/constraints"
)

func reverseSide(side Side) Side {
	if side == Buy {
		return Sell
	}
	return Buy
}

func isWhole(n float64) bool {
	return math.Ceil(n) == n
}

func sum[S []E, E constraints.Integer | constraints.Float](s S) E {
	var sum E
	for _, e := range s {
		sum += e
	}
	return sum
}

func sumFunc[S []E, E constraints.Integer | constraints.Float](s S, pred func(x E) bool) E {
	var sum E
	for _, e := range s {
		if pred(e) {
			sum += e
		}
	}
	return sum
}

func count[S []E, E constraints.Integer | constraints.Float](s S, pred func(x E) bool) int {
	var count int
	for _, e := range s {
		if pred(e) {
			count++
		}
	}
	return count
}

func mean[S []E, E constraints.Integer | constraints.Float](s S) E {
	return sum(s) / E(len(s))
}
