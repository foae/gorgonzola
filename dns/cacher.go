package dns

import "time"

// Cacher defines the interface needed to interact
// with the caching layer of this package.
type Cacher interface {
	Set(k string, x interface{}, d time.Duration)
	Get(k string) (interface{}, bool)
	Delete(k string)
}
