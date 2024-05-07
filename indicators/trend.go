package indicators

// SMA (Simple Moving Average) takes the closing prices of an asset over time period
// sums them up and divides them by the period/total e.g. (1 + 2 + 3 + 4) / 4 = 2.5
func SMA(period int, values []float64) []float64 {
	// Keep track of the sum up to i
	sum := float64(0)
	// Storage averages
	results := make([]float64, len(values))

	for i, v := range values {
		count := i + 1 // 1, 2, 3, 4, 5 -> 15
		sum += v

		if i >= period {
			// period = 2, values = []int{ 1, 2, 3, 4 }
			// []float64{ 1.0 } -> sum = 1
			// []float64{ 1.0,1.5 } -> sum = 3
			// []float64{ 1.0,1.5,2.5 } -> sum = ((1+2+3) - 1 = 5)
			// []float64{ 1.0,1.5,2.5,3.5 } -> sum = ((5+4) - 2 = 7)
			//
			// 10 - (1+2) -> 7
			sum -= values[i-period]
			count = period // 15
		}

		results[i] = sum / float64(count)
	}

	return results
}
