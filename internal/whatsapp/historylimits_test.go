package whatsapp

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	waWa6 "go.mau.fi/whatsmeow/proto/waWa6"
	"go.mau.fi/whatsmeow/store"
	"google.golang.org/protobuf/proto"
)

func registrationPayload(t *testing.T) *waWa6.ClientPayload {
	t.Helper()
	raw, err := proto.Marshal(store.DeviceProps)
	if err != nil {
		t.Fatal(err)
	}
	return &waWa6.ClientPayload{DevicePairingData: &waWa6.ClientPayload_DevicePairingRegistrationData{DeviceProps: raw}}
}

func decodeProps(t *testing.T, p *waWa6.ClientPayload) *waCompanionReg.DeviceProps {
	t.Helper()
	var props waCompanionReg.DeviceProps
	if err := proto.Unmarshal(p.GetDevicePairingData().GetDeviceProps(), &props); err != nil {
		t.Fatal(err)
	}
	return &props
}

func TestWithMinimalHistorySetsLimitsAndKeepsTheRest(t *testing.T) {
	props := decodeProps(t, withMinimalHistory(registrationPayload(t)))
	cfg := props.GetHistorySyncConfig()
	if cfg.GetFullSyncDaysLimit() != minimalHistoryDays || cfg.GetFullSyncSizeMbLimit() != minimalHistorySizeMB || cfg.GetRecentSyncDaysLimit() != minimalHistoryDays {
		t.Fatalf("limits not applied: %v", cfg)
	}
	// Everything else in DeviceProps must survive the rewrite.
	if props.GetOs() != store.DeviceProps.GetOs() || props.GetPlatformType() != store.DeviceProps.GetPlatformType() {
		t.Fatalf("device identity changed: %v", props)
	}
	if cfg.GetSupportGroupHistory() != store.DeviceProps.GetHistorySyncConfig().GetSupportGroupHistory() {
		t.Fatal("unrelated history flags changed")
	}
}

func TestWithMinimalHistoryLeavesLoginPayloadAlone(t *testing.T) {
	login := &waWa6.ClientPayload{Username: proto.Uint64(1)}
	if got := withMinimalHistory(login); got.GetDevicePairingData() != nil {
		t.Fatal("login payload gained pairing data")
	}
}

func TestDefaultPropsStayUnlimited(t *testing.T) {
	// The global default is what history-enabled instances keep sending.
	if store.DeviceProps.GetHistorySyncConfig().FullSyncDaysLimit != nil {
		t.Fatal("global DeviceProps was mutated")
	}
	_ = withMinimalHistory(registrationPayload(t))
	if store.DeviceProps.GetHistorySyncConfig().FullSyncDaysLimit != nil {
		t.Fatal("withMinimalHistory mutated the global DeviceProps")
	}
}
