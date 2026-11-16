package sessions

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// processSalt is mixed into password hashes so that the in-memory key (and
// anything derived from it) cannot be used for an offline dictionary attack
// against a well-known unsalted sha256.
var processSalt = func() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("sessions: crypto/rand: " + err.Error())
	}
	return b
}()

// Key identifies one cached session: host + user + sha256(salt||pass).
// It never contains the password itself. String and LogValue omit the hash.
type Key struct {
	Host     string // bmc.Target.Key() ("host:port")
	User     string
	passHash string
}

// KeyFor builds the session key for t and c.
func KeyFor(t bmc.Target, c bmc.Creds) Key {
	h := sha256.New()
	h.Write(processSalt)
	h.Write([]byte(c.Pass))
	return Key{Host: t.Key(), User: c.User, passHash: hex.EncodeToString(h.Sum(nil))}
}

func (k Key) mapKey() string { return k.Host + "\x00" + k.User + "\x00" + k.passHash }

// ID is a short opaque identifier for the key, safe for logs, debug output and
// cache keys. It distinguishes different passwords for the same user without
// revealing anything about them.
func (k Key) ID() string {
	s := sha256.Sum256([]byte("id\x00" + k.mapKey()))
	return hex.EncodeToString(s[:6])
}

// String renders host and user only.
func (k Key) String() string { return k.User + "@" + k.Host }

// LogValue implements slog.LogValuer (never the hash).
func (k Key) LogValue() slog.Value {
	return slog.GroupValue(slog.String("host", k.Host), slog.String("user", k.User), slog.String("id", k.ID()))
}

// normHost canonicalises a host for the target cache (accepts "host",
// "host:port" or a bmc.Target.Key()).
func normHost(h string) string {
	if t, err := bmc.ParseTarget(h); err == nil {
		return t.Key()
	}
	return strings.ToLower(h)
}
