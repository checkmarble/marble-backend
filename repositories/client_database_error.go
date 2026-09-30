package repositories

// ClientDatabaseError labels infrastructure failures at the ClientDB boundary.
// Domain permissions and Marble metadata errors remain unclassified.
type ClientDatabaseError struct{ Err error }

func (e ClientDatabaseError) Error() string { return e.Err.Error() }
func (e ClientDatabaseError) Unwrap() error { return e.Err }
