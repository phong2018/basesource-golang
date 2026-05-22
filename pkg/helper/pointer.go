package helper

func Deref[T any](v *T) T {
	if v == nil {
		var zero T
		return zero
	}
	return *v
}

func Ptr[T any](v T) *T { return &v }
