package retry

import (
	"math"
	"math/rand"
	"time"
)

func ExponentialBackoff(
	attempt int,
) time.Duration {

	base := math.Pow(
		2,
		float64(attempt),
	)

	jitter := rand.Intn(1000)

	return time.Duration(
		base,
	)*time.Second + time.Duration(
		jitter,
	)*time.Millisecond
}
