package dns

import "time"

type Cacher interface {
	Set(k string, x interface{}, d time.Duration)
	Get(k string) (interface{}, bool)
	Delete(k string)
}
