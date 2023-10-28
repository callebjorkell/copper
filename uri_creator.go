package copper

import (
	"fmt"
	"strings"
)

type UriCreator struct {
	Host     string
	BasePath string
}

func (d UriCreator) Absolute(path string) string {
	return fmt.Sprintf("%v%v", d.Host, d.Relative(path))
}

func (d UriCreator) Relative(path string) string {
	return fmt.Sprintf("/%v/%v", d.BasePath, strings.TrimLeft(path, "/"))
}
