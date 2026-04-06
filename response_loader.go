package mocka

// ResponseLoader loads response entries from a backing store.
type ResponseLoader interface {
	Load() (entries []Entry, err error)
}
