package client

import (
	"fmt"
	"net/netip"
	"slices"
	"sync"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/AdguardTeam/golibs/container"
	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/log"
)

// Storage contains information about persistent and runtime clients.
type Storage struct {
	// allTags is a set of all client tags.
	allTags *container.MapSet[string]

	// mu protects index of persistent clients.
	mu *sync.Mutex

	// index contains information about persistent clients.
	index *Index

	// runtimeIndex contains information about runtime clients.
	runtimeIndex map[netip.Addr]*Runtime
}

// NewStorage returns initialized client storage.
func NewStorage(clientTags []string) (s *Storage) {
	allTags := container.NewMapSet(clientTags...)

	return &Storage{
		allTags:      allTags,
		mu:           &sync.Mutex{},
		index:        NewIndex(),
		runtimeIndex: map[netip.Addr]*Runtime{},
	}
}

// Add stores persistent client information or returns an error.  p must be
// valid persistent client.
func (s *Storage) Add(p *Persistent) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	err = s.check(p)
	if err != nil {
		return fmt.Errorf("adding client: %w", err)
	}

	s.index.Add(p)

	return nil
}

// check returns an error if persistent client information contains errors.
//
// TODO(s.chzhen):  Remove persistent client information validation.
func (s *Storage) check(p *Persistent) (err error) {
	switch {
	case p == nil:
		return errors.Error("client is nil")
	case p.Name == "":
		return errors.Error("empty name")
	case p.IDsLen() == 0:
		return errors.Error("id required")
	case p.UID == UID{}:
		return errors.Error("uid required")
	}

	err = s.index.ClashesUID(p)
	if err != nil {
		// Don't wrap the error since there is already an annotation deferred.
		return err
	}

	conf, err := proxy.ParseUpstreamsConfig(p.Upstreams, &upstream.Options{})
	if err != nil {
		return fmt.Errorf("invalid upstream servers: %w", err)
	}

	err = conf.Close()
	if err != nil {
		log.Error("client: closing upstream config: %s", err)
	}

	for _, t := range p.Tags {
		if !s.allTags.Has(t) {
			return fmt.Errorf("invalid tag: %q", t)
		}
	}

	// TODO(s.chzhen):  Move to the constructor.
	slices.Sort(p.Tags)

	return nil
}

// RemoveByName removes persistent client information.  ok is false if no such
// client exists by that name.
func (s *Storage) RemoveByName(name string) (ok bool) {
	p, ok := s.index.FindByName(name)
	if !ok {
		return false
	}

	s.index.Delete(p)

	return true
}

// Update updates stored persistent client information p with new information n
// or returns an error.  p and n must have the same UID.
func (s *Storage) Update(p, n *Persistent) (err error) {
	defer func() { err = errors.Annotate(err, "updating client: %w") }()

	err = s.check(n)
	if err != nil {
		// Don't wrap the error since there is already an annotation deferred.
		return err
	}

	err = s.index.Clashes(n)
	if err != nil {
		// Don't wrap the error since there is already an annotation deferred.
		return err
	}

	s.index.Delete(p)
	s.index.Add(n)

	return nil
}

// ClientRuntime returns the saved runtime client by ip.  If no such client
// exists, returns nil.
func (s *Storage) ClientRuntime(ip netip.Addr) (rc *Runtime) {
	return s.runtimeIndex[ip]
}

// AddRuntime saves the runtime client information in the storage.  IP address
// of a client must be unique.  rc must not be nil.
func (s *Storage) AddRuntime(rc *Runtime) {
	ip := rc.Addr()
	s.runtimeIndex[ip] = rc
}

// SizeRuntime returns the number of the runtime clients.
func (s *Storage) SizeRuntime() (n int) {
	return len(s.runtimeIndex)
}

// RangeRuntime calls f for each runtime client in an undefined order.
func (s *Storage) RangeRuntime(f func(rc *Runtime) (cont bool)) {
	for _, rc := range s.runtimeIndex {
		if !f(rc) {
			return
		}
	}
}

// DeleteRuntime removes the runtime client by ip.
func (s *Storage) DeleteRuntime(ip netip.Addr) {
	delete(s.runtimeIndex, ip)
}

// DeleteBySource removes all runtime clients that have information only from
// the specified source and returns the number of removed clients.
func (s *Storage) DeleteBySource(src Source) (n int) {
	for ip, rc := range s.runtimeIndex {
		rc.unset(src)

		if rc.isEmpty() {
			delete(s.runtimeIndex, ip)
			n++
		}
	}

	return n
}
