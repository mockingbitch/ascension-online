package breakthrough

import (
	crand "crypto/rand"
	"math/rand/v2"
	"sync"
)

type lockedRNG struct {
	mu sync.Mutex
	r  *rand.Rand
}

func (l *lockedRNG) IntN(n int) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.IntN(n)
}

// NewRNG returns a goroutine-safe ChaCha8 RNG seeded from crypto/rand.
func NewRNG() RNG {
	var seed [32]byte
	if _, err := crand.Read(seed[:]); err != nil {
		panic(err)
	}
	return &lockedRNG{r: rand.New(rand.NewChaCha8(seed))}
}
