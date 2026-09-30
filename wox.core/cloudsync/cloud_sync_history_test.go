package cloudsync

import (
	"encoding/json"
	"testing"
	"wox/database"
)

func TestCloudSyncHistoryRecordSize(t *testing.T) {
	for _, op := range []string{OpUpsert, OpDelete} {
		t.Run(op, func(t *testing.T) {
			change := CloudSyncChange{EntityType: EntityWoxSetting, Key: "测试", Op: op, ChangeID: "change-1"}
			if op == OpUpsert {
				change.Value = &CloudSyncEncryptedValue{KeyVersion: 1, Nonce: "nonce", Ciphertext: "encrypted-data"}
			}
			encoded, err := json.Marshal(change)
			if err != nil {
				t.Fatal(err)
			}
			push := cloudSyncPushHistoryDetails([]CloudSyncChange{change}, []uint{1}, []uint{1}, nil)
			if len(push) != 1 || push[0].SizeBytes != len(encoded) {
				t.Fatalf("push details = %+v, want size %d", push, len(encoded))
			}
			record := CloudSyncRecord{EntityType: change.EntityType, Key: change.Key, Op: op, Value: change.Value, ServerTs: 123}
			encoded, err = json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			pull := cloudSyncHistoryDetailFromRecord(record, CloudSyncHistoryStatusFailed, "apply failed")
			if pull.SizeBytes != len(encoded) {
				t.Fatalf("pull size = %d, want %d", pull.SizeBytes, len(encoded))
			}
			details, err := json.Marshal([]CloudSyncHistoryRecordDetail{pull})
			if err != nil {
				t.Fatal(err)
			}
			history, err := decodeCloudSyncHistoryRow(database.CloudSyncHistory{RecordKeysJSON: string(details)})
			if err != nil {
				t.Fatal(err)
			}
			if history.Details[0].SizeBytes != pull.SizeBytes {
				t.Fatal("size was lost when decoding persisted history")
			}
		})
	}
	legacy, err := decodeCloudSyncHistoryRow(database.CloudSyncHistory{RecordKeysJSON: `[{"entity_type":"wox_setting","key":"ThemeId","op":"upsert"}]`})
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Details[0].SizeBytes != 0 {
		t.Fatal("legacy history must keep its size unknown")
	}
}
