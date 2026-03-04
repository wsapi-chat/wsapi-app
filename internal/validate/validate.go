package validate

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

var v *validator.Validate

var instanceIDRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)
var phoneRegex = regexp.MustCompile(`^\+?\d{7,15}$`)

// Phone validates that a string looks like an international phone number.
func Phone(s string) bool {
	return phoneRegex.MatchString(s)
}

// Init initializes the global validator with custom rules.
func Init() {
	v = validator.New(validator.WithRequiredStructEnabled())

	_ = v.RegisterValidation("instance_id", func(fl validator.FieldLevel) bool {
		return instanceIDRegex.MatchString(fl.Field().String())
	})
}

// Struct validates a struct using the global validator.
func Struct(s any) error {
	if v == nil {
		Init()
	}
	return v.Struct(s)
}
