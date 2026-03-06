package main

import (
	"anytls/internal/node/state"
	"anytls/internal/ppanel"
	"anytls/proxy/padding"
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

func fetchNodeSnapshot(ctx context.Context, client *ppanel.Client) (*state.Snapshot, error) {
	config, err := client.FetchConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch server config: %w", err)
	}
	users, err := client.FetchUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch server users: %w", err)
	}
	snapshot, err := state.BuildSnapshot(config, users)
	if err != nil {
		return nil, fmt.Errorf("build runtime snapshot: %w", err)
	}
	if snapshot.PaddingScheme != "" && !padding.UpdatePaddingScheme([]byte(snapshot.PaddingScheme)) {
		return nil, fmt.Errorf("invalid padding scheme from panel")
	}
	return snapshot, nil
}

func resolveListenAddr(listen string, port int) string {
	if listen != "" {
		return listen
	}
	return fmt.Sprintf("0.0.0.0:%d", port)
}

func (s *myServer) runSyncLoop(ctx context.Context) {
	for {
		interval := time.Minute
		if snapshot := s.snapshotStore.Load(); snapshot != nil && snapshot.PullInterval > 0 {
			interval = snapshot.PullInterval
		}
		if !waitForNextTick(ctx, interval) {
			return
		}
		requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		snapshot, err := fetchNodeSnapshot(requestCtx, s.panelClient)
		cancel()
		if err != nil {
			eventLogger("node", logrus.Fields{"pull_interval": interval.String()}, "sync_snapshot_failed").WithError(err).Error("sync node snapshot failed")
			continue
		}
		if snapshot == nil {
			eventLogger("node", logrus.Fields{"pull_interval": interval.String()}, "sync_snapshot_nil").Error("sync node snapshot returned nil")
			continue
		}
		s.snapshotStore.Store(snapshot)
		eventLogger("node", logrus.Fields{
			"pull_interval": interval.String(),
			"push_interval": snapshot.PushInterval.String(),
			"user_count":    len(snapshot.UsersByID),
		}, "sync_snapshot").Info("node snapshot updated")
	}
}

func (s *myServer) runReportLoop(ctx context.Context) {
	for {
		interval := time.Minute
		if snapshot := s.snapshotStore.Load(); snapshot != nil && snapshot.PushInterval > 0 {
			interval = snapshot.PushInterval
		}
		if !waitForNextTick(ctx, interval) {
			return
		}
		requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		if err := s.reportOnce(requestCtx); err != nil {
			eventLogger("node", logrus.Fields{"push_interval": interval.String()}, "report_state_failed").WithError(err).Error("report node state failed")
		}
		cancel()
	}
}

func (s *myServer) reportOnce(ctx context.Context) error {
	if err := s.pushOnlineUsers(ctx); err != nil {
		return err
	}
	if err := s.pushTraffic(ctx); err != nil {
		return err
	}
	status, err := collectServerStatus(ctx)
	if err != nil {
		return err
	}
	if err := s.panelClient.PushStatus(ctx, status); err != nil {
		return fmt.Errorf("push server status: %w", err)
	}
	eventLogger("node", logrus.Fields{
		"online_user_count": len(s.deviceTracker.OnlineEntries()),
		"cpu":               status.CPU,
		"mem":               status.Mem,
		"disk":              status.Disk,
	}, "report_state").Debug("reported node state")
	return nil
}

func (s *myServer) pushOnlineUsers(ctx context.Context) error {
	entries := s.deviceTracker.OnlineEntries()
	users := make([]ppanel.OnlineUser, 0, len(entries))
	for _, entry := range entries {
		users = append(users, ppanel.OnlineUser{UID: entry.UserID, IP: entry.IP})
	}
	if err := s.panelClient.PushOnlineUsers(ctx, users); err != nil {
		return fmt.Errorf("push online users: %w", err)
	}
	eventLogger("node", logrus.Fields{"online_user_count": len(users)}, "push_online_users").Debug("reported online users")
	return nil
}

func (s *myServer) pushTraffic(ctx context.Context) error {
	pending := s.traffic.DrainAll()
	if len(pending) == 0 {
		return nil
	}
	traffic := make([]ppanel.UserTraffic, 0, len(pending))
	for userID, stats := range pending {
		traffic = append(traffic, ppanel.UserTraffic{
			UID:      userID,
			Upload:   stats.Upload,
			Download: stats.Download,
		})
	}
	if err := s.panelClient.PushUserTraffic(ctx, traffic); err != nil {
		s.traffic.Restore(pending)
		return fmt.Errorf("push user traffic: %w", err)
	}
	eventLogger("node", logrus.Fields{"traffic_user_count": len(traffic)}, "push_user_traffic").Debug("reported user traffic")
	return nil
}

func waitForNextTick(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
