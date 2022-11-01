package jlog

//Handler writes logs to somewhere
type Handler interface {
	//Rotate()
	Write(p []byte) (n int, err error)
	Close() error
}
