//go:build !unix

package plugin

func withEmbedFileLock(_ string, fn func() error) error {
	return fn()
}
