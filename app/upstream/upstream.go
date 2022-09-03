package upstream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/jordan-wright/email"
	"github.com/leonardinius/smtpd-proxy/app/zlog"
)

var errorEmptyRegistry = errors.New("empty sender registry")

// Email wrapper for package specific (github.com/jordan-wright/email) email
type Email = email.Email

// NewEmailFromReader reads email from DATA stream
func NewEmailFromReader(r io.Reader) (*Email, error) {
	var envelope *email.Email
	var err error
	if envelope, err = email.NewEmailFromReader(r); err != nil {
		return nil, err
	}

	return envelope, nil
}

// Server inits the connecion pool to the server
type Server interface {
	Configure(config map[string]any) (Forwarder, error)
}

// Forwarder queues individual email
type Forwarder interface {
	Forward(ctx context.Context, mail *Email) error
}

// Registry envelope holde of multiple upstream servers
type Registry interface {
	Forwarder
	AddForwarder(forwarder Forwarder, weight int)
	Len() int
}

var _ Registry = (*RegistryMap)(nil)
var _ Forwarder = (*RegistryMap)(nil)

// context.Context metadata key
type entryKey string

var entryContextKey entryKey

type EntryMeta struct {
	UID string
}

// registryEntry entry witin registry
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
}

// randInt interface for random values.
// extracted to interface to be to be used in tests
type randInt interface {
	Intn(n int) int
	Int() int
}

type randIntStruc struct {
	gorand *rand.Rand
}

var _ randInt = (*randIntStruc)(nil)

func newRandIntStruc() *randIntStruc {
	return &randIntStruc{rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (s *randIntStruc) Intn(n int) int {
	return s.gorand.Intn(n)
}

func (s *randIntStruc) Int() int {
	return s.gorand.Int()
}

// NewEmptyRegistry creates empty new registry
func NewEmptyRegistry() *RegistryMap {
	r := new(RegistryMap)
	r.rnd = newRandIntStruc()
	return r
}

func (r *RegistryMap) AddForwarder(forwarder Forwarder, weight int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	newTotal := r.totalWeight + weight
	var uid = fmt.Sprintf("uid:%04x", r.rnd.Int())
	newEntry := registryEntry{originalWeight: weight, threshold: newTotal, sender: forwarder, meta: EntryMeta{UID: uid}}
	r.entriesSorted = append(r.entriesSorted, newEntry)
	r.totalWeight = newTotal
}

func (r *RegistryMap) pick() (Forwarder, *registryEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.entriesSorted) == 0 {
		return nil, nil, errorEmptyRegistry
	}

	// nolint:gosec // can't avoid it in this place
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
		zlog.Warnf("%v err [%v]", uid, err)
	}
	return err
}

// FromContext returns the entryMeta value stored in ctx, if any.
func FromContext(ctx context.Context) (meta *EntryMeta, ok bool) {
	u, ok := ctx.Value(entryContextKey).(*EntryMeta)
	return u, ok
}
