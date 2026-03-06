package state

import (
	"anytls/internal/ppanel"
	"crypto/sha256"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

const (
	defaultPullInterval = time.Minute
	defaultPushInterval = time.Minute
)

type User struct {
	ID          int64
	UUID        string
	AuthHash    [32]byte
	SpeedLimit  int64
	DeviceLimit int64
}

type Snapshot struct {
	Protocol               string
	Port                   int
	PullInterval           time.Duration
	PushInterval           time.Duration
	TrafficReportThreshold int64
	PaddingScheme          string
	UsersByHash            map[[32]byte]User
	UsersByID              map[int64]User
}

func BuildSnapshot(config *ppanel.ServerConfigResponse, users []ppanel.ServerUser) (*Snapshot, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	protocol := strings.TrimSpace(config.Protocol)
	if protocol == "" {
		protocol = "anytls"
	}
	if protocol != "anytls" {
		return nil, fmt.Errorf("unsupported protocol %q", protocol)
	}
	if config.Config.Port <= 0 {
		return nil, fmt.Errorf("invalid anytls port %d", config.Config.Port)
	}

	snapshot := &Snapshot{
		Protocol:      protocol,
		Port:          config.Config.Port,
		PullInterval:  durationFromSeconds(config.Basic.PullInterval, defaultPullInterval),
		PushInterval:  durationFromSeconds(config.Basic.PushInterval, defaultPushInterval),
		PaddingScheme: strings.TrimSpace(config.Config.PaddingScheme),
		UsersByHash:   make(map[[32]byte]User, len(users)),
		UsersByID:     make(map[int64]User, len(users)),
	}
	for _, user := range users {
		if user.ID <= 0 {
			return nil, fmt.Errorf("invalid user id %d", user.ID)
		}
		uuid := strings.TrimSpace(user.UUID)
		if uuid == "" {
			return nil, fmt.Errorf("user %d uuid is empty", user.ID)
		}
		entry := User{
			ID:          user.ID,
			UUID:        uuid,
			AuthHash:    sha256.Sum256([]byte(uuid)),
			SpeedLimit:  user.SpeedLimit,
			DeviceLimit: user.DeviceLimit,
		}
		if _, exists := snapshot.UsersByHash[entry.AuthHash]; exists {
			return nil, fmt.Errorf("duplicate auth hash for user %d", user.ID)
		}
		snapshot.UsersByHash[entry.AuthHash] = entry
		snapshot.UsersByID[entry.ID] = entry
	}
	return snapshot, nil
}

func (s *Snapshot) LookupAuthHash(hash []byte) (User, bool) {
	if s == nil || len(hash) != sha256.Size {
		return User{}, false
	}
	var key [sha256.Size]byte
	copy(key[:], hash)
	user, ok := s.UsersByHash[key]
	return user, ok
}

type Store struct {
	current atomic.Pointer[Snapshot]
}

func NewStore(snapshot *Snapshot) *Store {
	store := &Store{}
	if snapshot != nil {
		store.current.Store(snapshot)
	}
	return store
}

func (s *Store) Load() *Snapshot {
	if s == nil {
		return nil
	}
	return s.current.Load()
}

func (s *Store) Store(snapshot *Snapshot) {
	if s == nil || snapshot == nil {
		return
	}
	s.current.Store(snapshot)
}

func durationFromSeconds(seconds int64, fallback time.Duration) time.Duration {
	if seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}
