package upstream

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitAddForward(t *testing.T) {
	r := newRegistry( /*uids*/ 1, 2, 3, 4 /*pic percentages*/)
	r.AddForwarder(nil, 10)
	r.AddForwarder(nil, 20)
	r.AddForwarder(nil, 50)
	r.AddForwarder(nil, 30)
	assert.Equal(t, 4, len(r.entriesSorted))
	assert.ElementsMatch(t, []int{10, 30, 80, 110},
		[]int{
			r.entriesSorted[0].threshold,
			r.entriesSorted[1].threshold,
			r.entriesSorted[2].threshold,
			r.entriesSorted[3].threshold,
		})
}

func TestRegistryRandomForwardPickRandomThresholds(t *testing.T) {
	extractor := func(f Forwarder, e *registryEntry, err error) string {
		if err != nil {
			return err.Error()
		}
		return e.meta.UID
	}

	var tests = []struct {
		rnd       int
		forwarder string
	}{
		{0, "uid:0001"},
		{10, "uid:0001"},
		{11, "uid:0002"},
		{20, "uid:0002"},
		{30, "uid:0002"},
		{31, "uid:0003"},
		{80, "uid:0003"},
		{81, "uid:0004"},
		{110, "uid:0004"},
	}
	for _, test := range tests {
		r := newRegistry( /*uids*/ 1, 2, 3, 4 /*pic percentages*/, test.rnd)
		r.AddForwarder(nil, 10)
		r.AddForwarder(nil, 20)
		r.AddForwarder(nil, 50)
		r.AddForwarder(nil, 30)
		assert.Equal(t, test.forwarder, extractor(r.pick()))
	}
}

type MockRandom struct {
	Values []int
}

func (m *MockRandom) Intn(n int) int {
	i := m.Values[0]
	m.Values = m.Values[1:]
	return i
}

func (m *MockRandom) Int() int {
	i := m.Values[0]
	m.Values = m.Values[1:]
	return i
}

func newRegistry(randomValues ...int) *RegistryMap {
	r := NewEmptyRegistry()
	r.rnd = &MockRandom{Values: randomValues}
	return r
}
