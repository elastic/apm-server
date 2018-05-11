//+build !linux

package elasticapm

func currentProcessTitle() (string, error) {
	// TODO(axw)
	return "", nil
}
