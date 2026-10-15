package scan

import "math"

func extensionEntropy(counts map[string]int64) float64 {
	var total int64
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		return 0
	}

	ent := 0.0
	for _, c := range counts {
		p := float64(c) / float64(total)
		if p > 0 {
			ent -= p * math.Log2(p)
		}
	}
	return ent
}
