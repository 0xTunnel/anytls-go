package session

import (
	"anytls/proxy/padding"
	"anytls/util"
	"encoding/binary"
	"io"
	"net"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/buf"
	"github.com/sirupsen/logrus"
)

func (s *Session) logger(event string, fields logrus.Fields) *logrus.Entry {
	entryFields := logrus.Fields{
		"component": "session",
		"event":     event,
	}
	if s != nil {
		if s.userID > 0 {
			entryFields["user_id"] = s.userID
		}
		if s.conn != nil && s.conn.RemoteAddr() != nil {
			entryFields["remote_addr"] = s.conn.RemoteAddr().String()
		}
	}
	for key, value := range fields {
		entryFields[key] = value
	}
	return logrus.WithFields(entryFields)
}

type Session struct {
	conn     net.Conn
	connLock sync.Mutex

	streams    map[uint32]*Stream
	streamLock sync.RWMutex

	dieOnce sync.Once
	die     chan struct{}
	dieHook func()

	padding *atomic.TypedValue[*padding.PaddingFactory]

	peerVersion byte
	userID      int64
	traffic     TrafficRecorder

	onNewStream func(stream *Stream)
}

type TrafficRecorder interface {
	AddUpload(userID int64, n int64)
	AddDownload(userID int64, n int64)
}

func NewServerSession(conn net.Conn, onNewStream func(stream *Stream), _padding *atomic.TypedValue[*padding.PaddingFactory]) *Session {
	s := &Session{
		conn:        conn,
		onNewStream: onNewStream,
		padding:     _padding,
		die:         make(chan struct{}),
		streams:     make(map[uint32]*Stream),
	}
	return s
}

func (s *Session) SetUserContext(userID int64, recorder TrafficRecorder) {
	s.userID = userID
	s.traffic = recorder
}

func (s *Session) SetCloseHook(hook func()) {
	if hook == nil {
		return
	}
	if s.dieHook == nil {
		s.dieHook = hook
		return
	}
	previous := s.dieHook
	s.dieHook = func() {
		previous()
		hook()
	}
}

func (s *Session) Run() {
	s.recvLoop()
}

func (s *Session) IsClosed() bool {
	select {
	case <-s.die:
		return true
	default:
		return false
	}
}

func (s *Session) Close() error {
	var once bool
	s.dieOnce.Do(func() {
		close(s.die)
		once = true
	})
	if !once {
		return io.ErrClosedPipe
	}
	if s.dieHook != nil {
		s.dieHook()
		s.dieHook = nil
	}
	func() {
		s.streamLock.Lock()
		defer s.streamLock.Unlock()
		for _, stream := range s.streams {
			stream.closeLocally()
		}
		s.streams = make(map[uint32]*Stream)
	}()
	return s.conn.Close()
}

func (s *Session) recvLoop() error {
	defer func() {
		if r := recover(); r != nil {
			s.logger("panic", logrus.Fields{"panic": r}).Errorln(string(debug.Stack()))
		}
	}()
	defer s.Close()

	var receivedSettingsFromClient bool
	var hdr rawHeader

	for {
		if s.IsClosed() {
			return io.ErrClosedPipe
		}
		if _, err := io.ReadFull(s.conn, hdr[:]); err != nil {
			return err
		}

		sid := hdr.StreamID()
		switch hdr.Cmd() {
		case cmdPSH:
			if hdr.Length() == 0 {
				continue
			}
			buffer := buf.Get(int(hdr.Length()))
			if _, err := io.ReadFull(s.conn, buffer); err != nil {
				buf.Put(buffer)
				return err
			}
			if s.userID > 0 && s.traffic != nil {
				s.traffic.AddUpload(s.userID, int64(hdr.Length()))
			}
			s.streamLock.RLock()
			stream, ok := s.streams[sid]
			s.streamLock.RUnlock()
			if ok {
				stream.pipeW.Write(buffer)
			}
			buf.Put(buffer)
		case cmdSYN:
			if !receivedSettingsFromClient {
				s.logger("protocol_violation", logrus.Fields{"reason": "missing_settings", "stream_id": sid}).Warn("client opened stream before sending settings")
				f := newFrame(cmdAlert, 0)
				f.data = []byte("client did not send its settings")
				s.writeControlFrame(f)
				return nil
			}
			var stream *Stream
			var shouldStart bool
			func() {
				s.streamLock.Lock()
				defer s.streamLock.Unlock()
				if _, ok := s.streams[sid]; ok {
					return
				}
				stream = newStream(sid, s)
				s.streams[sid] = stream
				shouldStart = true
			}()
			if shouldStart {
				go func() {
					if s.onNewStream != nil {
						s.onNewStream(stream)
					} else {
						stream.Close()
					}
				}()
			}
		case cmdFIN:
			var stream *Stream
			var ok bool
			func() {
				s.streamLock.Lock()
				defer s.streamLock.Unlock()
				stream, ok = s.streams[sid]
				delete(s.streams, sid)
			}()
			if ok {
				stream.closeLocally()
			}
			continue
		case cmdWaste:
			if err := s.discardPayload(hdr.Length()); err != nil {
				return err
			}
		case cmdSettings:
			buffer := buf.Get(int(hdr.Length()))
			if _, err := io.ReadFull(s.conn, buffer); err != nil {
				buf.Put(buffer)
				return err
			}
			receivedSettingsFromClient = true
			m := util.StringMapFromBytes(buffer)
			paddingF := s.padding.Load()
			if m["padding-md5"] != paddingF.Md5 {
				s.logger("update_padding_scheme", nil).Debug("client padding scheme differs, sending update")
				f := newFrame(cmdUpdatePaddingScheme, 0)
				f.data = paddingF.RawScheme
				if _, err := s.writeControlFrame(f); err != nil {
					buf.Put(buffer)
					return err
				}
			}
			if v, err := strconv.Atoi(m["v"]); err == nil && v >= 2 {
				s.peerVersion = byte(v)
				s.logger("peer_version_negotiated", logrus.Fields{"peer_version": v}).Debug("negotiated peer protocol version")
				f := newFrame(cmdServerSettings, 0)
				f.data = util.StringMap{"v": "2"}.ToBytes()
				if _, err := s.writeControlFrame(f); err != nil {
					buf.Put(buffer)
					return err
				}
			}
			buf.Put(buffer)
		case cmdAlert:
			buffer := buf.Get(int(hdr.Length()))
			if _, err := io.ReadFull(s.conn, buffer); err != nil {
				buf.Put(buffer)
				return err
			}
			s.logger("protocol_alert", logrus.Fields{"message": string(buffer)}).Warn("received alert from peer")
			buf.Put(buffer)
			return nil
		case cmdUpdatePaddingScheme, cmdSYNACK, cmdServerSettings:
			if err := s.discardPayload(hdr.Length()); err != nil {
				return err
			}
		case cmdHeartRequest:
			if _, err := s.writeControlFrame(newFrame(cmdHeartResponse, sid)); err != nil {
				return err
			}
		case cmdHeartResponse:
			continue
		default:
			continue
		}
	}
}

func (s *Session) discardPayload(length uint16) error {
	if length == 0 {
		return nil
	}
	_, err := io.CopyN(io.Discard, s.conn, int64(length))
	return err
}

func (s *Session) streamClosed(sid uint32) error {
	if s.IsClosed() {
		return io.ErrClosedPipe
	}
	_, err := s.writeControlFrame(newFrame(cmdFIN, sid))
	s.streamLock.Lock()
	defer s.streamLock.Unlock()
	delete(s.streams, sid)
	return err
}

func (s *Session) writeDataFrame(sid uint32, data []byte) (int, error) {
	dataLen := len(data)

	buffer := buf.NewSize(dataLen + headerOverHeadSize)
	buffer.WriteByte(cmdPSH)
	binary.BigEndian.PutUint32(buffer.Extend(4), sid)
	binary.BigEndian.PutUint16(buffer.Extend(2), uint16(dataLen))
	buffer.Write(data)
	_, err := s.writeConn(buffer.Bytes())
	buffer.Release()
	if err != nil {
		return 0, err
	}
	if s.userID > 0 && s.traffic != nil {
		s.traffic.AddDownload(s.userID, int64(dataLen))
	}

	return dataLen, nil
}

func (s *Session) writeControlFrame(frame frame) (int, error) {
	dataLen := len(frame.data)

	buffer := buf.NewSize(dataLen + headerOverHeadSize)
	buffer.WriteByte(frame.cmd)
	binary.BigEndian.PutUint32(buffer.Extend(4), frame.sid)
	binary.BigEndian.PutUint16(buffer.Extend(2), uint16(dataLen))
	buffer.Write(frame.data)

	s.conn.SetWriteDeadline(time.Now().Add(time.Second * 5))

	_, err := s.writeConn(buffer.Bytes())
	buffer.Release()
	if err != nil {
		s.Close()
		return 0, err
	}

	s.conn.SetWriteDeadline(time.Time{})

	return dataLen, nil
}

func (s *Session) writeConn(b []byte) (int, error) {
	s.connLock.Lock()
	defer s.connLock.Unlock()
	return s.conn.Write(b)
}
