// +build race

package testsetup

import (
	"fmt"
	"math/big"
	"os"
	"time"
)

func RaceOrNot() string {
	return "-race"
}

func RaceSlowndown(v time.Duration) time.Duration {
	r := big.NewRat(1, 1)
	if v := os.Getenv(EnvTestRaceSlowdown); v != "" {
		if _, ok := r.SetString(v); !ok {
			// Because r is undefined if SetString fails
			panic(fmt.Errorf("%v=%q is not a valid floating point value", EnvTestRaceSlowdown, v))
		}
	}
	r.Mul(r, big.NewRat(int64(v), 1))
	return time.Duration(r.Num().Int64() / r.Denom().Int64())
}
