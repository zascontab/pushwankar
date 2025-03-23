package repository

// ErrNotificationNotFound define las operaciones para gestionar errores

var (
	ErrNotificationNotFound = NewError("notification not found")
	ErrDeviceNotFound       = NewError("device not found")
	ErrTokenNotFound        = NewError("token not found")
)

// NewError crea una nueva instancia de Error
func NewError(message string) error {
	return &Error{
		message: message,
	}
}

type Error struct {
	message string
}

func (e *Error) Error() string {
	return e.message
}
