package main

import (
	"anytls/internal/config"
	"anytls/internal/node/state"
	"anytls/internal/ppanel"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"path/filepath"
	"sort"
	"time"

	"github.com/sirupsen/logrus"
)

func fetchNodeSnapshot(ctx context.Context, client *ppanel.Client, userSnapshotPath string) (*state.Snapshot, error) {
	config, err := client.FetchConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch server config: %w", err)
	}
	users, err := client.FetchUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch server users: %w", err)
	}
	if userSnapshotPath != "" {
		if err := ppanel.WriteUserList(userSnapshotPath, users); err != nil {
			eventLogger("node", logrus.Fields{"user_snapshot_path": userSnapshotPath}, "persist_user_snapshot_failed").WithError(err).Warn("persist user snapshot failed")
		}
	}
	adapter, err := selectProtocolAdapter(config.Protocol)
	if err != nil {
		return nil, fmt.Errorf("select protocol adapter: %w", err)
	}
	snapshot, err := adapter.BuildSnapshot(config, users)
	if err != nil {
		return nil, fmt.Errorf("build runtime snapshot: %w", err)
	}
	return snapshot, nil
}

func (s *myServer) handleInboundConnection(ctx context.Context, conn net.Conn) {
	adapter, err := s.currentProtocolAdapter()
	if err != nil {
		eventLogger("inbound", nil, "protocol_adapter_unavailable").WithError(err).Error("failed to resolve protocol adapter")
		if conn != nil {
			_ = conn.Close()
		}
		return
	}
	adapter.HandleConn(ctx, conn, s)
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
		snapshot, err := fetchNodeSnapshot(requestCtx, s.panelClient, s.userSnapshotPath)
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
		fields := logrus.Fields{
			"port":          snapshot.Port,
			"pull_interval": interval.String(),
			"push_interval": snapshot.PushInterval.String(),
			"user_count":    len(snapshot.UsersByID),
		}
		if s.rememberSnapshotSignature(snapshotSignature(snapshot)) {
			eventLogger("node", fields, "sync_snapshot").Info("node snapshot updated")
			continue
		}
		logDiagnostic("node", fields, "sync_snapshot_unchanged", "node snapshot unchanged")
	}
}

func resolveUserSnapshotPath(nodeConfig *config.NodeConfig) string {
	if nodeConfig == nil {
		return ""
	}
	baseDir := ""
	if nodeConfig.Path != "" {
		baseDir = filepath.Dir(nodeConfig.Path)
	}
	if baseDir == "" {
		return ""
	}
	return filepath.Join(baseDir, "users.json")
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
	logDiagnostic("node", logrus.Fields{
		"online_user_count": len(s.deviceTracker.OnlineEntries()),
		"cpu":               status.CPU,
		"mem":               status.Mem,
		"disk":              status.Disk,
	}, "report_state", "reported node state")
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
	if len(users) > 0 {
		logDiagnostic("node", logrus.Fields{"online_user_count": len(users)}, "push_online_users", "reported online users")
	}
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
	logDiagnostic("node", logrus.Fields{"traffic_user_count": len(traffic)}, "push_user_traffic", "reported user traffic")
	return nil
}

func snapshotSignature(snapshot *state.Snapshot) string {
	if snapshot == nil {
		return ""
	}
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s|%d|%s|%s|%d|%s|%d", snapshot.Protocol, snapshot.Port, snapshot.PullInterval, snapshot.PushInterval, snapshot.TrafficReportThreshold, snapshot.PaddingScheme, len(snapshot.UsersByID))
	userIDs := make([]int64, 0, len(snapshot.UsersByID))
	for userID := range snapshot.UsersByID {
		userIDs = append(userIDs, userID)
	}
	sort.Slice(userIDs, func(i, j int) bool {
		return userIDs[i] < userIDs[j]
	})
	for _, userID := range userIDs {
		user := snapshot.UsersByID[userID]
		_, _ = fmt.Fprintf(hash, "|%d|%s|%d|%d", user.ID, user.UUID, user.SpeedLimit, user.DeviceLimit)
	}
	return hex.EncodeToString(hash.Sum(nil))
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
