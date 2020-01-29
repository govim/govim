// +build !race

package testsetup

import "time"

func RaceOrNot() string {
	return ""
}

func RaceSlowndown(v time.Duration) time.Duration {
	return v
}
