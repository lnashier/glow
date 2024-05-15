package flow

import "fmt"

func keygen(prefix string) func() string {
	count := 0
	return func() string {
		count++
		return fmt.Sprintf("%s%d", prefix, count)
	}
}
