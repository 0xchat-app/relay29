package main

import (
	"context"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

func checkLevelLimited(relayPubkey string) func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
	allowedMembers := map[int]int{
		0: 2,
		1: 10,
		2: 50,
		3: 250,
	}

	return func(ctx context.Context, event *nostr.Event) (bool, string) {
		group := state.GetGroupFromEvent(event)

		if event.PubKey == relayPubkey {
			return false, ""
		}

		// only relay owner can edit group level
		if event.Kind == nostr.KindSimpleGroupEditLevel && event.PubKey != relayPubkey {
			return true, "insufficient permissions"
		}

		if group.LevelUntil < nostr.Timestamp(time.Now().Unix()) {
			return true, "group level subscription expired"
		}

		maxMembers, ok := allowedMembers[group.Level]
		if !ok {
			return true, "unknown group level"
		}

		if len(group.Members) > maxMembers {
			return true, "group level limited"
		}

		return false, ""
	}
}
