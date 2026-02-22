package testutil

// Ptr returns a pointer to the given value.
// This is useful in tests where struct literals require pointer fields.
//
//	testutil.Ptr(true)   // *bool
//	testutil.Ptr(42)     // *int
//	testutil.Ptr("foo")  // *string
//
// The explicit helper keeps call sites concise and avoids repeating the
// equivalent expansion:
//
//	v := <arg>
//	return &v
func Ptr[T any](v T) *T { return &v }
