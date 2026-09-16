package validate

import (
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var v *validator.Validate

var instanceIDRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// A number must start with its country code, and no country code begins with
// zero - a leading zero is a national trunk prefix, not part of the
// international number. Rejecting it here turns a slow, opaque failure from
// WhatsApp into an immediate, actionable error.
var phoneRegex = regexp.MustCompile(`^\+?[1-9]\d{6,14}$`)

// Phone validates that a string looks like an international phone number.
func Phone(s string) bool {
	return phoneRegex.MatchString(s)
}

// The local part of a recipient JID: plain digits for a person or a newsletter,
// digits-digits for a legacy group. Those are the only two shapes seen across a
// large sample of real chat JIDs.
var jidUserRegex = regexp.MustCompile(`^\d+(-\d+)?$`)

// Recipient validates a message recipient, accepting either a bare
// international phone number or a full JID such as "<digits>@s.whatsapp.net",
// "<digits>-<digits>@g.us" or "<digits>@newsletter".
//
// whatsmeow does not validate the local part: ParseJID accepts an address like
// "=<digits>@s.whatsapp.net", whatsmeow encrypts and sends to it, and WhatsApp
// never acknowledges a message addressed to a JID that cannot exist. The send
// burns the full 75-second ack budget before failing, with nothing naming the
// real problem. A template placeholder that leaks into the literal value is the
// usual source. Rejecting it here costs microseconds and names the bad field.
//
// The server part is deliberately not checked against an allowlist: a new
// server type is WhatsApp's to introduce, and rejecting one we haven't heard of
// would break valid sends. The local part is where the malformed input lands.
func Recipient(s string) bool {
	if s == "" {
		return false
	}

	user, server, hasServer := strings.Cut(s, "@")
	if !hasServer {
		// Bare number: must be a routable international phone number.
		return Phone(s)
	}
	if server == "" || strings.Contains(server, "@") {
		return false
	}

	// FormatRecipient strips one leading '+' before it looks for '@', so
	// "+<digits>@server" does reach its recipient and has to validate. Exactly
	// one: a second '+' survives into the local part, which is unroutable.
	return jidUserRegex.MatchString(strings.TrimPrefix(user, "+"))
}

// Init initializes the global validator with custom rules.
func Init() {
	v = validator.New(validator.WithRequiredStructEnabled())

	_ = v.RegisterValidation("instance_id", func(fl validator.FieldLevel) bool {
		return instanceIDRegex.MatchString(fl.Field().String())
	})

	_ = v.RegisterValidation("recipient", func(fl validator.FieldLevel) bool {
		return Recipient(fl.Field().String())
	})
}

// Struct validates a struct using the global validator.
func Struct(s any) error {
	if v == nil {
		Init()
	}
	return v.Struct(s)
}
