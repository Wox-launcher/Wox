package setting

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"wox/util"
)

func TestPinedQueryResultMatchesOnlyItsQuery(t *testing.T) {
	pinned := PinedQueryResult{}.withQuery("b r").withQuery(" b d ")
	if !pinned.hasQuery("b r") || !pinned.hasQuery("b d") {
		t.Fatalf("pinned queries = %#v", pinned.Queries)
	}
	if pinned.hasQuery("b") || pinned.hasQuery("b rd") {
		t.Fatalf("unrelated query matched: %#v", pinned.Queries)
	}
	if len(pinned.Queries) != 2 || pinned.Queries[0] != "b r" || pinned.Queries[1] != "b d" {
		t.Fatalf("queries = %#v, want [b r b d]", pinned.Queries)
	}

	remaining, keep := pinned.withoutQuery("b r")
	if !keep || remaining.hasQuery("b r") || !remaining.hasQuery("b d") {
		t.Fatalf("after unpin b r = %#v keep=%v", remaining.Queries, keep)
	}
	_, keep = remaining.withoutQuery("b d")
	if keep {
		t.Fatal("last query should remove the pin")
	}
}

func TestPinResultIsScopedToQuery(t *testing.T) {
	store := &memorySettingStore{values: map[string]string{}}
	manager := &Manager{woxSetting: &WoxSetting{
		PinedResults: &WoxSettingValue[*util.HashMap[ResultHash, PinedQueryResult]]{
			SettingValue: &SettingValue[*util.HashMap[ResultHash, PinedQueryResult]]{
				key:          "PinedResults",
				defaultValue: util.NewHashMap[ResultHash, PinedQueryResult](),
				settingStore: store,
			},
		},
	}}
	ctx := context.Background()

	reddit := NewResultHashFromParts("bookmark", "Reddit", "https://www.reddit.com/", "reddit")
	discord := NewResultHashFromParts("bookmark", "Discord", "https://discord.com/channels/1", "discord")
	manager.PinResult(ctx, reddit, "b r")
	manager.PinResult(ctx, discord, "b d")
	manager.PinResult(ctx, reddit, "b r ")

	if !manager.IsPinedResult(ctx, reddit, "b r") {
		t.Fatal("reddit should be pinned for b r")
	}
	if manager.IsPinedResult(ctx, discord, "b r") {
		t.Fatal("discord pin on b d should not apply to b r")
	}
	if !manager.IsPinedResult(ctx, discord, "b d") {
		t.Fatal("discord should stay pinned for b d")
	}

	manager.UnpinResult(ctx, reddit, "b r")
	if manager.IsPinedResult(ctx, reddit, "b r") {
		t.Fatal("reddit should be unpinned from b r")
	}
	if !manager.IsPinedResult(ctx, discord, "b d") {
		t.Fatal("unpinning reddit should keep the discord pin")
	}
}

type memorySettingStore struct {
	values map[string]string
}

func (s *memorySettingStore) Get(key string, target interface{}) error {
	raw, ok := s.values[key]
	if !ok {
		return errors.New("missing")
	}
	return json.Unmarshal([]byte(raw), target)
}

func (s *memorySettingStore) Set(key string, value interface{}) error {
	encoded, err := SerializeValue(value)
	if err != nil {
		return err
	}
	s.values[key] = encoded
	return nil
}

func (s *memorySettingStore) Delete(key string) error {
	delete(s.values, key)
	return nil
}
