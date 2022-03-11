package art

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openacid/testkeys"
)

func TestTreeTraversalPrefix(t *testing.T) {
	dataSet := []struct {
		keyPrefix string
		keys      []string
		expected  []string
	}{
		{
			"",
			[]string{},
			[]string{},
		},
		{
			"api",
			[]string{"api.foo.bar", "api.foo.baz", "api.foe.fum", "abc.123.456", "api.foo", "api"},
			[]string{"api.foo.bar", "api.foo.baz", "api.foe.fum", "api.foo", "api"},
		},
		{
			"a",
			[]string{"api.foo.bar", "api.foo.baz", "api.foe.fum", "abc.123.456", "api.foo", "api"},
			[]string{"api.foo.bar", "api.foo.baz", "api.foe.fum", "abc.123.456", "api.foo", "api"},
		}, {
			"b",
			[]string{"api.foo.bar", "api.foo.baz", "api.foe.fum", "abc.123.456", "api.foo", "api"},
			[]string{},
		},
		{
			"api.",
			[]string{"api.foo.bar", "api.foo.baz", "api.foe.fum", "abc.123.456", "api.foo", "api"},
			[]string{"api.foo.bar", "api.foo.baz", "api.foe.fum", "api.foo"},
		},
		{
			"api.foo.bar",
			[]string{"api.foo.bar", "api.foo.baz", "api.foe.fum", "abc.123.456", "api.foo", "api"},
			[]string{"api.foo.bar"},
		},
		{
			"api.end",
			[]string{"api.foo.bar", "api.foo.baz", "api.foe.fum", "abc.123.456", "api.foo", "api"},
			[]string{},
		}, {
			"",
			[]string{"api.foo.bar", "api.foo.baz", "api.foe.fum", "abc.123.456", "api.foo", "api"},
			[]string{"api.foo.bar", "api.foo.baz", "api.foe.fum", "abc.123.456", "api.foo", "api"},
		}, {
			"this:key:has",
			[]string{
				"this:key:has:a:long:prefix:3",
				"this:key:has:a:long:common:prefix:2",
				"this:key:has:a:long:common:prefix:1",
			},
			[]string{
				"this:key:has:a:long:prefix:3",
				"this:key:has:a:long:common:prefix:2",
				"this:key:has:a:long:common:prefix:1",
			},
		}, {
			"ele",
			[]string{"elector", "electibles", "elect", "electible"},
			[]string{"elector", "electibles", "elect", "electible"},
		},
		{
			"long.api.url.v1",
			[]string{"long.api.url.v1.foo", "long.api.url.v1.bar", "long.api.url.v2.foo"},
			[]string{"long.api.url.v1.foo", "long.api.url.v1.bar"},
		},
	}

	for _, d := range dataSet {
		tree := New()
		for _, k := range d.keys {
			tree.Insert(Key(k))
		}

		actual := tree.ForEachKeyPrefix(Key(d.keyPrefix))

		sort.Strings(d.expected)
		sort.Strings(actual)
		assert.Equal(t, d.expected, actual, d.keyPrefix)
	}

}

func TestTreeIterator(t *testing.T) {
	tree := New()
	tree.Insert(Key("2"))
	tree.Insert(Key("1"))

	it := tree.Iterator()
	assert.NotNil(t, it)
	assert.True(t, it.HasNext())
	n4, err := it.Next()
	assert.NoError(t, err)
	assert.Equal(t, Node4, n4.Type())

	assert.True(t, it.HasNext())
	v1, err := it.Next()
	assert.NoError(t, err)
	assert.Equal(t, v1.Key(), Key("1"))

	assert.True(t, it.HasNext())
	v2, err := it.Next()
	assert.NoError(t, err)
	assert.Equal(t, v2.Key(), Key("2"))

	assert.False(t, it.HasNext())
	bad, err := it.Next()
	assert.Nil(t, bad)
	assert.Equal(t, ErrNoMoreNodes, err)

}

func TestBigKeySetPrefixSearch(t *testing.T) {
	keys := getKeys("1mvl5_10")

	n := len(keys)
	fmt.Printf("key len %d\n", n)

	prefixs := make([]string, 0, n/10)
	tree := New()
	for _, k := range keys {
		if strings.HasPrefix(k, "z") {
			prefixs = append(prefixs, k)
		}
		tree.Insert(Key(k))
	}
	got := tree.ForEachKeyPrefix(Key("z"))
	assert.Equal(t, prefixs, got)
}

var cache map[string][]string = map[string][]string{}

func getKeys(fn string) []string {
	ss, ok := cache[fn]
	if ok {
		return ss
	}
	ks := testkeys.Load(fn)
	cache[fn] = ks
	return ks
}

func benchBigKeySet(b *testing.B, f func(b *testing.B, typ string, key []string)) {
	for _, fn := range testkeys.AssetNames() {
		keys := getKeys(fn)

		n := len(keys)
		if n < 1000 {
			continue
		}

		b.Run(fn, func(b *testing.B) {
			f(b, fn, keys)
		})
	}
}

func BenchmarkWordsTreeInsert(b *testing.B) {
	benchBigKeySet(b, func(b *testing.B, fn string, keys []string) {
		n := len(keys)
		b.ResetTimer()

		for i := 0; i < b.N/n; i++ {
			tree := New()

			for _, k := range keys {
				tree.Insert(Key(k))
			}
		}

	})
}

func BenchmarkWordsTreePrefixSearch(b *testing.B) {
	prefixs := []string{
		"abcdefghijklmnopqrstuvwxyz",
		"0123456789",
	}

	benchBigKeySet(b, func (b *testing.B, fn string, keys []string)  {
		n := len(keys)
		b.ResetTimer()

		for i := 0; i < b.N/n; i++ {
			tree := New()

			for _, k := range keys {
				tree.Insert(Key(k))
			}

			for _, prefix := range prefixs {
				for i := 0; i < len(prefix); i++ {
					tree.ForEachKeyPrefix(Key(prefix[i:i+1]))
				}
			}
		}
	})
}