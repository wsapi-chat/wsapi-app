package whatsapp

import "errors"

// ErrNotFound indicates the requested resource was not found.
var ErrNotFound = errors.New("not found")

// ErrUpstream indicates the WhatsApp server returned an error.
var ErrUpstream = errors.New("upstream error")
