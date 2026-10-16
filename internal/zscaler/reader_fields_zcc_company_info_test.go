package zscaler

import (
	"fmt"
	"testing"

	zcccompany "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/company"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// TestCompanyInfoDevicePostureFrequencyModes pins the per-mode contract of
// devicePostureFrequency: ids and per-OS frequencies in every mode, the
// posture name in standard and share only. webAppConfig never renders.
func TestCompanyInfoDevicePostureFrequencyModes(t *testing.T) {
	t.Parallel()

	const nameCanary = "company-info-posture-name-canary"
	const webAppCanary = "company-info-webapp-canary"
	info := zcccompany.CompanyInfo{
		DevicePostureFrequency: []zcccompany.DevicePostureFrequency{{
			PostureID:    7,
			PostureName:  nameCanary,
			IosValue:     11,
			AndroidValue: 12,
			WindowsValue: 13,
			MacValue:     14,
			LinuxValue:   15,
			DefaultValue: 16,
		}},
		WebAppConfig: &zcccompany.WebAppConfig{EnableFipsMode: webAppCanary},
	}
	records := []resources.SourceRecord{structSourceRecord(info)}

	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
		got := projectOneRecordInMode(t, resources.ProductZCC, resourceZCCCompanyInfo, mode, records)
		assertFieldsAbsent(t, "company-info", got, "webAppConfig")
		assertNoCanaries(t, "company-info", got, webAppCanary)
		entries := mustProjectedList(t, got, "devicePostureFrequency")
		if len(entries) != 1 {
			t.Fatalf("%s devicePostureFrequency = %v, want one entry", mode, entries)
		}
		entry, ok := entries[0].(map[string]any)
		if !ok {
			t.Fatalf("%s devicePostureFrequency[0] = %T, want object", mode, entries[0])
		}
		for key, want := range map[string]int{
			"postureId": 7, "iosValue": 11, "androidValue": 12, "windowsValue": 13,
			"macValue": 14, "linuxValue": 15, "defaultValue": 16,
		} {
			if fmt.Sprint(entry[key]) != fmt.Sprint(want) {
				t.Errorf("%s devicePostureFrequency[0].%s = %v, want %d", mode, key, entry[key], want)
			}
		}
		_, hasName := entry["postureName"]
		if wantName := mode != redact.ModeParanoid; hasName != wantName {
			t.Errorf("%s devicePostureFrequency[0] has postureName = %v, want %v", mode, hasName, wantName)
		}
		if mode == redact.ModeParanoid {
			assertNoCanaries(t, "company-info", got, nameCanary)
		}
	}
}
