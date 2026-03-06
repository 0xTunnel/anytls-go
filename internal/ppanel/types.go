package ppanel

import "encoding/json"

type ResponseEnvelope[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

type ServerBasic struct {
	PushInterval int64 `json:"push_interval"`
	PullInterval int64 `json:"pull_interval"`
}

type SecurityConfig struct {
	SNI           string `json:"sni"`
	AllowInsecure bool   `json:"allow_insecure"`
	Fingerprint   string `json:"fingerprint"`
}

type AnyTLSConfig struct {
	Port           int             `json:"port"`
	PaddingScheme  string          `json:"padding_scheme"`
	SecurityConfig *SecurityConfig `json:"security_config,omitempty"`
}

type ServerConfigResponse struct {
	Basic    ServerBasic     `json:"basic"`
	Protocol string          `json:"protocol"`
	Config   AnyTLSConfig    `json:"config"`
	RawData  json.RawMessage `json:"-"`
}

type ServerUser struct {
	ID          int64  `json:"id"`
	UUID        string `json:"uuid"`
	SpeedLimit  int64  `json:"speed_limit"`
	DeviceLimit int64  `json:"device_limit"`
}

type UserListResponse struct {
	Users []ServerUser `json:"users"`
}

type OnlineUser struct {
	UID int64  `json:"uid"`
	IP  string `json:"ip"`
}

type OnlineUsersRequest struct {
	Users []OnlineUser `json:"users"`
}

type UserTraffic struct {
	UID      int64 `json:"uid"`
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}

type PushTrafficRequest struct {
	Traffic []UserTraffic `json:"traffic"`
}

type ServerStatusRequest struct {
	CPU       float64 `json:"cpu"`
	Mem       float64 `json:"mem"`
	Disk      float64 `json:"disk"`
	UpdatedAt int64   `json:"updated_at"`
}
