package configuration

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/sirupsen/logrus"

	"github.com/authelia/authelia/v4/internal/logging"
	"github.com/authelia/authelia/v4/internal/templates"
)

// FilteredFile implements a koanf.Provider.
type FilteredFile struct {
	path    string
	filters []BytesFilter
}

// FilteredFileProvider returns a koanf.Provider which provides filtered file output.
func FilteredFileProvider(path string, filters ...BytesFilter) *FilteredFile {
	return &FilteredFile{
		path:    filepath.Clean(path),
		filters: filters,
	}
}

// ReadBytes reads the contents of a file on disk, passes it through any configured filters, and returns the bytes.
func (f *FilteredFile) ReadBytes() (data []byte, err error) {
	if data, err = os.ReadFile(f.path); err != nil {
		return nil, err
	}

	if len(data) == 0 || len(f.filters) == 0 {
		return data, nil
	}

	for _, filter := range f.filters {
		if data, err = filter(data); err != nil {
			return nil, err
		}
	}

	return data, nil
}

// Read is not supported by the filtered file koanf.Provider.
func (f *FilteredFile) Read() (map[string]any, error) {
	return nil, errors.New("filtered file provider does not support this method")
}

// BytesFilter describes a func used to filter files.
type BytesFilter func(in []byte) (out []byte, err error)

// NewFileFiltersDefault returns the default list of BytesFilter.
func NewFileFiltersDefault() []BytesFilter {
	return []BytesFilter{
		NewExpandEnvFileFilter(),
		NewTemplateFileFilter(),
	}
}

// NewFileFilters returns a list of BytesFilter provided they are valid.
func NewFileFilters(names []string) (filters []BytesFilter, err error) {
	filters = make([]BytesFilter, len(names))

	filterMap := map[string]int{}

	for i, name := range names {
		name = strings.ToLower(name)

		switch name {
		case "template":
			filters[i] = NewTemplateFileFilter()
		case "expand-env":
			filters[i] = NewExpandEnvFileFilter()
		default:
			return nil, fmt.Errorf("invalid filter named '%s'", name)
		}

		if _, ok := filterMap[name]; ok {
			return nil, fmt.Errorf("duplicate filter named '%s'", name)
		} else {
			filterMap[name] = 1
		}
	}

	return filters, nil
}

// NewExpandEnvFileFilter is a BytesFilter which passes the bytes through os.ExpandEnv.
func NewExpandEnvFileFilter() BytesFilter {
	log := logging.Logger()

	return func(in []byte) (out []byte, err error) {
		out = []byte(os.Expand(string(in), templates.FuncGetEnv))

		if log.Level >= logrus.TraceLevel {
			log.
				WithField("content", base64.RawStdEncoding.EncodeToString(out)).
				Trace("Expanded Env File Filter completed successfully")
		}

		return out, nil
	}
}

// NewTemplateFileFilter is a BytesFilter which passes the bytes through text/template.
func NewTemplateFileFilter() BytesFilter {
	t := template.New("config.template").Funcs(templates.FuncMap())

	log := logging.Logger()

	return func(in []byte) (out []byte, err error) {
		if t, err = t.Parse(string(in)); err != nil {
			return nil, err
		}

		buf := &bytes.Buffer{}

		if err = t.Execute(buf, nil); err != nil {
			return nil, err
		}

		out = buf.Bytes()

		if log.Level >= logrus.TraceLevel {
			log.
				WithField("content", base64.RawStdEncoding.EncodeToString(out)).
				Trace("Templated File Filter completed successfully")
		}

		return out, nil
	}
}
