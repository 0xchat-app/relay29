package nip42

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

// CreateUnsignedAuthEvent creates an event which should be sent via an "AUTH" command.
// If the authentication succeeds, the user will be authenticated as pubkey.
func CreateUnsignedAuthEvent(challenge, pubkey, relayURL string) nostr.Event {
	return nostr.Event{
		PubKey:    pubkey,
		CreatedAt: nostr.Now(),
		Kind:      nostr.KindClientAuthentication,
		Tags: nostr.Tags{
			nostr.Tag{"relay", relayURL},
			nostr.Tag{"challenge", challenge},
		},
		Content: "",
	}
}

// helper function for ValidateAuthEvent.
func parseURL(input string) (*url.URL, error) {
	return url.Parse(
		strings.ToLower(
			strings.TrimSuffix(input, "/"),
		),
	)
}

// ValidateAuthEvent checks whether event is a valid NIP-42 event for given challenge and relayURL.
// The result of the validation is encoded in the ok bool.
func ValidateAuthEvent(event *nostr.Event, challenge string, relayURL string) (pubkey string, ok bool) {
	fmt.Printf("DEBUG: ValidateAuthEvent - Event: %+v, Challenge: %s, RelayURL: %s\n", event, challenge, relayURL)

	if event.Kind != nostr.KindClientAuthentication {
		fmt.Printf("DEBUG: Kind validation failed - expected: %d, got: %d\n", nostr.KindClientAuthentication, event.Kind)
		return "", false
	}
	fmt.Printf("DEBUG: Kind validation passed\n")

	if event.Tags.FindWithValue("challenge", challenge) == nil {
		fmt.Printf("DEBUG: Challenge validation failed - expected: %s, event tags: %+v\n", challenge, event.Tags)
		return "", false
	}
	fmt.Printf("DEBUG: Challenge validation passed\n")

	expected, err := parseURL(relayURL)
	if err != nil {
		fmt.Printf("DEBUG: Expected URL parsing failed: %v\n", err)
		return "", false
	}
	fmt.Printf("DEBUG: Expected URL parsed: %+v\n", expected)

	tag := event.Tags.Find("relay")
	if tag == nil {
		fmt.Printf("DEBUG: Relay tag not found in event\n")
		return "", false
	}
	fmt.Printf("DEBUG: Relay tag found: %+v\n", tag)

	found, err := parseURL(tag[1])
	if err != nil {
		fmt.Printf("DEBUG: Found URL parsing failed: %v\n", err)
		return "", false
	}
	fmt.Printf("DEBUG: Found URL parsed: %+v\n", found)

	if expected.Scheme != found.Scheme ||
		expected.Host != found.Host ||
		expected.Path != found.Path {
		fmt.Printf("DEBUG: URL comparison failed - expected: %s://%s%s, found: %s://%s%s\n",
			expected.Scheme, expected.Host, expected.Path, found.Scheme, found.Host, found.Path)
		return "", false
	}
	fmt.Printf("DEBUG: URL validation passed\n")

	now := time.Now()
	eventTime := event.CreatedAt.Time()
	if eventTime.After(now.Add(10*time.Minute)) || eventTime.Before(now.Add(-10*time.Minute)) {
		fmt.Printf("DEBUG: Timestamp validation failed - now: %v, event: %v, diff: %v\n", now, eventTime, eventTime.Sub(now))
		return "", false
	}
	fmt.Printf("DEBUG: Timestamp validation passed\n")

	// save for last, as it is most expensive operation
	// no need to check returned error, since ok == true implies err == nil.
	if ok, _ := event.CheckSignature(); !ok {
		fmt.Printf("DEBUG: Signature validation failed\n")
		return "", false
	}
	fmt.Printf("DEBUG: Signature validation passed\n")

	fmt.Printf("DEBUG: All validations passed, returning pubkey: %s\n", event.PubKey)
	return event.PubKey, true
}
