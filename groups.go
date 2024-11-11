package relay29

import (
	"context"
	"sync"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip29"
)

type Group struct {
	nip29.Group
	mu sync.RWMutex
}

// NewGroup creates a new group from scratch (but doesn't store it in the groups map)
func (s *State) NewGroup(id string, creator string) *Group {
	group := &Group{
		Group: nip29.Group{
			Address: nip29.GroupAddress{
				ID:    id,
				Relay: "wss://" + s.Domain,
			},
			Roles:   s.defaultRoles,
			Members: make(map[string][]*nip29.Role, 12),
		},
	}

	group.Members[creator] = []*nip29.Role{s.groupCreatorDefaultRole}

	return group
}

// loadGroupsFromDB loads all the group metadata from all the past action messages.
func (s *State) loadGroupsFromDB(ctx context.Context) {
	groupMetadataEvents, _ := s.DB.QueryEvents(ctx, nostr.Filter{Kinds: []int{nostr.KindSimpleGroupCreateGroup}})
	for evt := range groupMetadataEvents {
		gtag := evt.Tags.GetFirst([]string{"h", ""})
		id := (*gtag)[1]

		group := s.NewGroup(id, evt.PubKey)
		f := nostr.Filter{
			Limit: 500, Kinds: nip29.ModerationEventKinds, Tags: nostr.TagMap{"h": []string{id}},
		}
		ch, _ := s.DB.QueryEvents(ctx, f)

		events := make([]*nostr.Event, 0, 5000)
		for event := range ch {
			events = append(events, event)
		}
		for i := len(events) - 1; i >= 0; i-- {
			evt := events[i]
			act, _ := PrepareModerationAction(evt)
			act.Apply(&group.Group)
		}

		// if the group was deleted there will be no actions after the delete
		if len(events) > 0 && events[0].Kind == nostr.KindSimpleGroupDeleteGroup {
			// we don't keep track of this if it was deleted
			continue
		}

		s.Groups.Store(group.Address.ID, group)
	}
}

func (s *State) GetGroupFromEvent(event *nostr.Event) *Group {
	group, _ := s.Groups.Load(GetGroupIDFromEvent(event))
	return group
}

func GetGroupIDFromEvent(event *nostr.Event) string {
	gtag := event.Tags.GetFirst([]string{"h", ""})
	groupId := (*gtag)[1]
	return groupId
}
