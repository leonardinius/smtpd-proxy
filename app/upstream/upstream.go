package upstream

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/big"
	"sync"

	"github.com/jordan-wright/email"
)

var errEmptyRegistry = errors.New("empty sender registry")

// Email wrapper for package specific (github.com/jordan-wright/email) email.
type Email = email.Email

// NewEmailFromReader reads email from DATA stream.
func NewEmailFromReader(r io.Reader) (*Email, error) {
	var envelope *email.Email
	var err error
	if envelope, err = email.NewEmailFromReader(r); err != nil {
		return nil, err
	}

	return envelope, nil
}

// Server inits the connecion pool to the server.
type Server interface {
	Configure(ctx context.Context, config map[string]any) (Forwarder, error)
}

// Forwarder queues individual email.
type Forwarder interface {
	Forward(ctx context.Context, mail *Email) error
}

// Registry envelope holde of multiple upstream servers.
type Registry interface {
	Forwarder
	AddForwarder(forwarder Forwarder, weight int)
	Len() int
}

var (
	_ Registry  = (*RegistryMap)(nil)
	_ Forwarder = (*RegistryMap)(nil)
)

// context.Context metadata key.
type entryKey string

var entryContextKey entryKey

type EntryMeta struct {
	UID string
}

// registryEntry entry witin registry.
type registryEntry struct {
	originalWeight int
	threshold      int
	sender         Forwarder
	meta           EntryMeta
}

type RegistryMap struct {
	mu            sync.Mutex
	entriesSorted []registryEntry
	totalWeight   int
	rnd           randInt
	logger        *slog.Logger
}

// randInt interface for random values.
// extracted to interface to be to be used in tests.
type randInt interface {
	Intn(n int) int
	Int() int
}

type randIntStruc struct{}

var (
	_       randInt = (*randIntStruc)(nil)
	_maxInt         = big.NewInt(math.MaxInt)
)

func newRandIntStruc() *randIntStruc {
	return &randIntStruc{}
}

func (s *randIntStruc) Intn(n int) int {
	maxN := big.NewInt(int64(n))
	val, err := rand.Int(rand.Reader, maxN)
	if err != nil {
		panic(err) // out of randomness, should never happen.
	}
	return int(val.Int64())
}

func (s *randIntStruc) Int() int {
	val, err := rand.Int(rand.Reader, _maxInt)
	if err != nil {
		panic(err) // out of randomness, should never happen.
	}
	return int(val.Int64())
}

// NewEmptyRegistry creates empty new registry.
func NewEmptyRegistry(logger *slog.Logger) *RegistryMap {
	return &RegistryMap{
		rnd:    newRandIntStruc(),
		logger: logger,
	}
}

func (r *RegistryMap) AddForwarder(forwarder Forwarder, weight int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	newTotal := r.totalWeight + weight
	uid := fmt.Sprintf("uid:%04x", r.rnd.Int())
	newEntry := registryEntry{originalWeight: weight, threshold: newTotal, sender: forwarder, meta: EntryMeta{UID: uid}}
	r.entriesSorted = append(r.entriesSorted, newEntry)
	r.totalWeight = newTotal
}

func (r *RegistryMap) pick() (Forwarder, *registryEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.entriesSorted) == 0 {
		return nil, nil, errEmptyRegistry
	}

	chance := r.rnd.Intn(r.totalWeight + 1)
	acc := 0
	for _, entry := range r.entriesSorted {
		if chance >= acc && chance <= entry.threshold {
			return entry.sender, &entry, nil
		}
		acc = entry.threshold
	}
	panic(fmt.Sprintf("unexpected, chance=%d entries:%v", chance, r.entriesSorted))
}

func (r *RegistryMap) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entriesSorted)
}

func (r *RegistryMap) Forward(ctx context.Context, mail *Email) error {
	sender, entry, err := r.pick()
	if err != nil {
		return err
	}

	uid := entry.meta.UID
	err = sender.Forward(context.WithValue(ctx, entryContextKey, &entry.meta), mail)
	if err != nil {
		r.logger.WarnContext(ctx, "forward error", "uid", uid, "err", err)
	}
	return err
}

// FromContext returns the entryMeta value stored in ctx, if any.
func FromContext(ctx context.Context) (meta *EntryMeta, ok bool) {
	u, ok := ctx.Value(entryContextKey).(*EntryMeta)
	return u, ok
}
