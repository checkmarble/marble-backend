package models

type ModelResult[M any] struct {
	Model M
	Error error
}

// This helper struct is used in repositories where we want to stream models from a SQL query to a channel, and read them
// in a loop. Since the models are sent from a goroutine, we need to stop this goroutine and make it close the SQL query
// before the transaction is committed or rolled back. Any code consuming a ChannelOfModels in a transaction should call
// CloseAndWaitUntilDone() before returning and closing the transaction.
// Callers outside of a transaction context need not call CloseAndWaitUntilDone(), as the channel will be closed when the
// goroutine finishes.
type ChannelOfModels[M any] struct {
	Models chan ModelResult[M]
	Stop   chan struct{}
	done   chan struct{}
}

func NewChannelOfModels[M any](modelsChannel chan ModelResult[M]) ChannelOfModels[M] {
	return ChannelOfModels[M]{
		Models: modelsChannel,
		Stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

func (c *ChannelOfModels[M]) CloseAndWaitUntilDone() {
	select {
	case c.Stop <- struct{}{}:
		<-c.done
	case <-c.done:
	}
}

func (c *ChannelOfModels[M]) CloseChannels() {
	// don't close Stop, as it creates a risk of race condition when calling CloseAndWaitUntilDone()
	close(c.Models)
	close(c.done)
}
