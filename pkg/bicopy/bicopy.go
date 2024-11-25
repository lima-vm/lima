package bicopy

import (
	"io"
	"sync"

	"github.com/sirupsen/logrus"
)

type NamedReadWriter struct {
	ReadWriter io.ReadWriter
	Name       string
}

func Bicopy(context string, x, y NamedReadWriter) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(x.ReadWriter, y.ReadWriter); err != nil {
			logrus.WithError(err).Errorf("%s: io.Copy(%s, %s)", context, x.Name, y.Name)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := io.Copy(y.ReadWriter, x.ReadWriter); err != nil {
			logrus.WithError(err).Errorf("%s: io.Copy(%s, %s)", context, y.Name, x.Name)
		}
	}()
	wg.Wait()
}
