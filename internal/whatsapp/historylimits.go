package whatsapp

import (
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	waWa6 "go.mau.fi/whatsmeow/proto/waWa6"
	"google.golang.org/protobuf/proto"
)

// Requested from the phone when the instance does not use history sync. The
// phone still sends what whatsmeow needs to work (bootstrap, keys, contacts),
// but not years of messages: inflating a large account's full history can
// exhaust the process's memory.
const (
	minimalHistoryDays   = 1
	minimalHistorySizeMB = 10
)

// LimitHistoryUnless makes pairing request minimal history from the phone
// whenever historyEnabled reports false at pairing time. It only affects the
// registration payload, so already paired devices are unchanged.
func (s *Service) LimitHistoryUnless(historyEnabled func() bool) {
	s.client.GetClientPayload = func() *waWa6.ClientPayload {
		payload := s.client.Store.GetClientPayload()
		if historyEnabled() {
			return payload
		}
		return withMinimalHistory(payload)
	}
}

// withMinimalHistory rewrites the history sync limits inside a registration
// payload. Login payloads (no pairing data) are returned as is.
func withMinimalHistory(payload *waWa6.ClientPayload) *waWa6.ClientPayload {
	pairing := payload.GetDevicePairingData()
	if pairing == nil {
		return payload
	}
	var props waCompanionReg.DeviceProps
	if err := proto.Unmarshal(pairing.GetDeviceProps(), &props); err != nil {
		return payload
	}
	if props.HistorySyncConfig == nil {
		props.HistorySyncConfig = &waCompanionReg.DeviceProps_HistorySyncConfig{}
	}
	cfg := props.HistorySyncConfig
	cfg.FullSyncDaysLimit = proto.Uint32(minimalHistoryDays)
	cfg.FullSyncSizeMbLimit = proto.Uint32(minimalHistorySizeMB)
	cfg.RecentSyncDaysLimit = proto.Uint32(minimalHistoryDays)
	props.RequireFullSync = proto.Bool(false)

	raw, err := proto.Marshal(&props)
	if err != nil {
		return payload
	}
	pairing.DeviceProps = raw
	return payload
}
