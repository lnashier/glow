package mapreduce

import (
	"context"
	"strings"
)

func Tokenize(ctx context.Context, in string, emit func(any)) {
	for _, s := range strings.FieldsFunc(in, func(r rune) bool {
		return !('A' <= r && r <= 'Z' || 'a' <= r && r <= 'z' || '0' <= r && r <= '9')
	}) {
		select {
		case <-ctx.Done():
		default:
			emit(s)
		}
	}
}
