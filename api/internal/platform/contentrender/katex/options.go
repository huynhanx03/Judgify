package katex

import "runtime"

const maximumPoolSize = 16

type config struct {
	poolSize int
}

// Option configures an Engine before its runtime pool is created.
type Option func(*config) error

// WithPoolSize sets the maximum number of concurrent JavaScript renders.
//
// A size above 16 is rejected deliberately: every pooled runtime owns an
// initialized KaTeX heap, so an unbounded pool would turn concurrency into an
// unbounded memory multiplier.
func WithPoolSize(size int) Option {
	return func(configuration *config) error {
		if configuration == nil || size < 1 || size > maximumPoolSize {
			return ErrInvalidConfiguration
		}
		configuration.poolSize = size
		return nil
	}
}

func defaultConfig() config {
	size := min(runtime.GOMAXPROCS(0), maximumPoolSize)
	if size < 1 {
		size = 1
	}
	return config{poolSize: size}
}
