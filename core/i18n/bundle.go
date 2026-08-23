package i18n

import (
	"sort"
	"sync"
	"time"
)

type Options struct {
	Default   string
	Supported []string
	Fallbacks map[string]string
	Debug     bool
	Zone      *time.Location
	Formatter Formatter
	OnMissing func(tag, msgid string)
}

type Bundle struct {
	def       string
	supported []string
	fallbacks map[string]string
	catalogs  map[string]Catalog
	chains    map[string][]string
	debug     bool
	zone      *time.Location
	formatter Formatter
	onMissing func(tag, msgid string)
	reported  sync.Map
	mu        sync.RWMutex
}

func NewBundle(opts Options) *Bundle {
	b := &Bundle{
		def:       Normalise(opts.Default),
		fallbacks: map[string]string{},
		catalogs:  map[string]Catalog{},
		chains:    map[string][]string{},
		debug:     opts.Debug,
		zone:      opts.Zone,
		formatter: opts.Formatter,
		onMissing: opts.OnMissing,
	}
	if b.zone == nil {
		b.zone = time.UTC
	}
	if b.def == "" {
		b.def = DefaultTag
	}
	for from, to := range opts.Fallbacks {
		b.fallbacks[Normalise(from)] = Normalise(to)
	}

	seen := map[string]bool{}
	for _, tag := range append([]string{b.def}, opts.Supported...) {
		normalised := Normalise(tag)
		if normalised == "" || seen[normalised] {
			continue
		}
		seen[normalised] = true
		b.supported = append(b.supported, normalised)
	}
	return b
}

func (b *Bundle) Default() string { return b.def }

func (b *Bundle) Supported() []string {
	return append([]string{}, b.supported...)
}

func (b *Bundle) Multilingual() bool { return len(b.supported) > 1 }

func (b *Bundle) Add(catalog Catalog) {
	if catalog == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.catalogs[Normalise(catalog.Tag())] = catalog
	b.chains = map[string][]string{}
}

func (b *Bundle) Has(tag string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, found := b.catalogs[Normalise(tag)]
	return found
}

func (b *Bundle) Catalog(tag string) (Catalog, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	catalog, found := b.catalogs[Normalise(tag)]
	return catalog, found
}

func (b *Bundle) Supports(tag string) bool {
	normalised := Normalise(tag)
	for _, candidate := range b.supported {
		if candidate == normalised {
			return true
		}
	}
	return false
}

func (b *Bundle) Chain(tag string) []string {
	normalised := Normalise(tag)

	b.mu.RLock()
	cached, found := b.chains[normalised]
	b.mu.RUnlock()
	if found {
		return cached
	}

	chain := b.buildChain(normalised)

	b.mu.Lock()
	b.chains[normalised] = chain
	b.mu.Unlock()
	return chain
}

func (b *Bundle) buildChain(tag string) []string {
	chain := []string{}
	seen := map[string]bool{}

	add := func(candidate string) {
		if candidate == "" || seen[candidate] {
			return
		}
		seen[candidate] = true
		chain = append(chain, candidate)
	}

	add(tag)
	if next, found := b.fallbacks[tag]; found {
		add(next)
		if second, found := b.fallbacks[next]; found {
			add(second)
		}
	}
	if base := BaseOf(tag); base != tag {
		add(base)
		if next, found := b.fallbacks[base]; found {
			add(next)
		}
	}
	add(b.def)
	return chain
}

func (b *Bundle) Locale(tag string) *Locale {
	normalised := Normalise(tag)
	if normalised == "" || !b.Supports(normalised) {
		normalised = b.def
	}
	return &Locale{
		bundle:    b,
		tag:       normalised,
		chain:     b.Chain(normalised),
		zone:      b.zone,
		formatter: b.formatter,
	}
}

func (b *Bundle) lookup(chain []string, context, msgid string, n int, plural bool) (string, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, tag := range chain {
		catalog, found := b.catalogs[tag]
		if !found {
			continue
		}
		if plural {
			if value, ok := catalog.LookupPlural(context, msgid, n); ok {
				return value, true
			}
			continue
		}
		if value, ok := catalog.Lookup(context, msgid); ok {
			return value, true
		}
	}
	return "", false
}

func (b *Bundle) reportMissing(tag, msgid string) {
	if _, already := b.reported.LoadOrStore(tag+contextSeparator+msgid, true); already {
		return
	}
	if b.onMissing != nil {
		b.onMissing(tag, msgid)
	}
}

func (b *Bundle) Missing() []string {
	out := []string{}
	b.reported.Range(func(key, _ any) bool {
		if text, ok := key.(string); ok {
			out = append(out, text)
		}
		return true
	})
	sort.Strings(out)
	return out
}
