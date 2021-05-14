package captcha

import "time"

var(
	GCLimitNumber = 1024
	Expiration = 5 * time.Minute
	DefaultMemStore = NewMemoryStore(GCLimitNumber, Expiration)
)
