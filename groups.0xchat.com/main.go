package main

import (
	"context"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/fiatjaf/eventstore/lmdb"
	"github.com/fiatjaf/khatru"
	"github.com/fiatjaf/khatru/policies"
	"github.com/fiatjaf/relay29"
	"github.com/fiatjaf/relay29/khatru29"
	"github.com/kelseyhightower/envconfig"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip29"
	"github.com/rs/zerolog"
)

type Settings struct {
	Port             string `envconfig:"PORT" default:"5577"`
	Domain           string `envconfig:"DOMAIN" default:"groups.0xchat.com"`
	RelayName        string `envconfig:"RELAY_NAME" default:"0xchat groups relay"`
	RelayPrivkey     string `envconfig:"RELAY_PRIVKEY" required:"true"`
	RelayDescription string `envconfig:"RELAY_DESCRIPTION"`
	RelayContact     string `envconfig:"RELAY_CONTACT"`
	RelayIcon        string `envconfig:"RELAY_ICON"`
	DatabasePath     string `envconfig:"DATABASE_PATH" default:"./db"`

	RelayPubkey string `envconfig:"-"`
}

var (
	s     Settings
	db    = &lmdb.LMDBBackend{}
	log   = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	relay *khatru.Relay
	state *relay29.State
)

var (
	kingRole   = &nip29.Role{Name: "king", Description: "the group's max top admin"}
	bishopRole = &nip29.Role{Name: "bishop", Description: "the group's noble servant"}
)

func main() {
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't process envconfig")
		return
	}
	s.RelayPubkey, _ = nostr.GetPublicKey(s.RelayPrivkey)

	// load db
	db.Path = s.DatabasePath
	db.MaxLimit = 40000
	if err := db.Init(); err != nil {
		log.Fatal().Err(err).Msg("failed to initialize database")
		return
	}
	log.Debug().Str("path", db.Path).Msg("initialized database")

	// init relay29 stuff
	relay, state = khatru29.Init(relay29.Options{
		Domain:                  s.Domain,
		DB:                      db,
		SecretKey:               s.RelayPrivkey,
		DefaultRoles:            []*nip29.Role{kingRole, bishopRole},
		GroupCreatorDefaultRole: kingRole,
	})

	// setup group-related restrictions
	state.AllowAction = func(ctx context.Context, group nip29.Group, role *nip29.Role, action relay29.Action) bool {
		if role == kingRole {
			// owners can do everything
			return true
		}
		if role == bishopRole {
			// admins can invite new users, delete people and messages
			switch action.(type) {
			case relay29.RemoveUser:
				return true
			case relay29.DeleteEvent:
				return true
			case relay29.PutUser:
				return true
			}
		}
		// no one else can do anything else
		return false
	}

	// init relay
	relay.Info.Name = s.RelayName
	relay.Info.Description = s.RelayDescription
	relay.Info.Contact = s.RelayContact
	relay.Info.Icon = s.RelayIcon

	relay.OverwriteDeletionOutcome = append(relay.OverwriteDeletionOutcome,
		blockDeletesOfOldMessages,
	)
	relay.RejectEvent = slices.Insert(relay.RejectEvent, 2,
		policies.PreventLargeTags(640),
		policies.RestrictToSpecifiedKinds(
			7, 9, 10, 11, 12, 16, 20, 1018, 1068, 1111,
			30023, 31922, 31923, 9802,
			9000, 9001, 9002, 9003, 9004, 9005, 9006, 9007, 9008, 9009,
			9021, 9022, 9321, 9735, 34235, 34236,
		),
		policies.PreventTimestampsInThePast(60*time.Second),
		policies.PreventTimestampsInTheFuture(30*time.Second),
	)

	log.Info().Str("relay-pubkey", s.RelayPubkey).Msg("running on http://0.0.0.0:" + s.Port)
	if err := http.ListenAndServe(":"+s.Port, relay); err != nil {
		log.Fatal().Err(err).Msg("failed to serve")
	}
}
