package core

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
)

// Store manages the master list. Thread-safe.
type Store struct {
	path  string
	mu    sync.Mutex
	set   map[string]struct{}
	order []string // preserve insertion order for file output
}

// NewStore loads existing master list from disk (if any).
func NewStore(path string) (*Store, error) {
	s := &Store{
		path: path,
		set:  make(map[string]struct{}),
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		d := NormalizeDomain(scanner.Text())
		if d == "" {
			continue
		}
		if _, ok := s.set[d]; !ok {
			s.set[d] = struct{}{}
			s.order = append(s.order, d)
		}
	}
	return s, scanner.Err()
}

// Size returns current count of unique domains.
func (s *Store) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.order)
}

// Has checks membership.
func (s *Store) Has(domain string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.set[domain]
	return ok
}

// AddMany adds new domains, returns the slice that was actually new.
func (s *Store) AddMany(domains []string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var added []string
	for _, d := range domains {
		if d == "" {
			continue
		}
		if _, ok := s.set[d]; !ok {
			s.set[d] = struct{}{}
			s.order = append(s.order, d)
			added = append(added, d)
		}
	}
	return added
}

// Append writes new domains to the master file (open in append mode).
func (s *Store) Append(domains []string) error {
	if len(domains) == 0 {
		return nil
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	for _, d := range domains {
		if _, err := fmt.Fprintln(w, d); err != nil {
			return err
		}
	}
	return nil
}

// Sample returns up to n domains, randomly chosen.
func (s *Store) Sample(n int) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n <= 0 || len(s.order) == 0 {
		return nil
	}
	if n >= len(s.order) {
		out := make([]string, len(s.order))
		copy(out, s.order)
		return out
	}
	idx := rand.Perm(len(s.order))[:n]
	out := make([]string, n)
	for i, j := range idx {
		out[i] = s.order[j]
	}
	return out
}

// Filter returns domains matching substring (case-insensitive) and/or TLDs.
func (s *Store) Filter(substr string, tlds []string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	substr = strings.ToLower(strings.TrimSpace(substr))
	var out []string
	for _, d := range s.order {
		if substr != "" && !strings.Contains(d, substr) {
			continue
		}
		if !MatchesAnyTLD(d, tlds) {
			continue
		}
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// TLDHistogram returns a map of TLD -> count (top-level label only).
func (s *Store) TLDHistogram() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := make(map[string]int)
	for _, d := range s.order {
		labels := strings.Split(d, ".")
		if len(labels) == 0 {
			continue
		}
		h[labels[len(labels)-1]]++
	}
	return h
}