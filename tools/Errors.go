package tools

type BufferTooSmall struct{}

func (BufferTooSmall) Error() (string) {
	return "Digest size is 256B"
}
