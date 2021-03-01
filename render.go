package renderlayout

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"unicode"

	"github.com/Masterminds/sprig"
	"github.com/foolin/goview"
)

type D map[string]interface{}

type Data func(w http.ResponseWriter, r *http.Request) (D, error)

type Render func(view string, dataFuncs ...Data) http.HandlerFunc

func StaticData(d D) Data {
	return func(_ http.ResponseWriter, _ *http.Request) (D, error) {
		return d, nil
	}
}

type Option func(renderer *renderer)

// Debug enables verbose logging. Prints the data being rendered in the template. Default is false
func Debug(enable bool) Option {
	return func(renderer *renderer) {
		renderer.debug = enable
	}
}

// DefaultData is the function called everytime before a template is rendered. Default is nil
// This can be used to set template variables needed in every template.
func DefaultData(data Data) Option {
	return func(renderer *renderer) {
		renderer.defaultData = data
	}
}

// ErrorKey changes the template variable name containing view errors. Default value is "errors"
// errors.Unwrap is used to find the error to be shown to the user, otherwise it's only logged.
func ErrorKey(key string) Option {
	return func(renderer *renderer) {
		renderer.errorKey = key
	}
}

// TemplatesPath is the path to root directory for the templates. Default value is "templates"
func TemplatesPath(templatesPath string) Option {
	return func(renderer *renderer) {
		renderer.root = templatesPath
	}
}

// LayoutsPath sets name of the main template to be used. Default value is "layouts"
// The path is searched within the templates layouts path. e.g. "templates/layouts"
func LayoutsPath(layouts string) Option {
	return func(renderer *renderer) {
		renderer.layouts = layouts
	}
}

// PartialsPath sets the path to main template to be used. Default value is "partials"
// The path is searched within the templates path. e.g. "templates/partials"
func PartialsPath(partials string) Option {
	return func(renderer *renderer) {
		renderer.partials = partials
	}
}

// Layout sets name of the main template to be used. Default value is "index"
// The path is searched within the templates layouts path. e.g. "templates/layouts/index.html"
func Layout(layout string) Option {
	return func(renderer *renderer) {
		renderer.layout = layout
	}
}

// Extension sets the file extension for templates and partials. Default value is html
func Extension(extension string) Option {
	return func(renderer *renderer) {
		renderer.extension = fmt.Sprintf(".%s", extension)
	}
}

// DisableCache disables the cache. Default value is false
func DisableCache(disableCache bool) Option {
	return func(renderer *renderer) {
		renderer.disableCache = disableCache
	}
}

// Delimiters sets the template delimiters. Default value is, left: {{ , right: }}
func Delimiters(left, right string) Option {
	return func(renderer *renderer) {
		renderer.delims = goview.Delims{
			Left:  left,
			Right: right,
		}
	}
}

// AddFuncs adds additional templates funcs. Default is nil
// github.com/Masterminds/sprig is already configured.
func AddFuncs(funcMap template.FuncMap) Option {
	return func(renderer *renderer) {
		renderer.funcs = funcMap
	}
}

// RenderError sets the error shown to the user when rendering fails completely. Default value is "Something went wrong."
func RenderError(error string) Option {
	return func(renderer *renderer) {
		renderer.renderError = error
	}
}

func New(opts ...Option) (Render, error) {

	lr := &renderer{
		root:         "templates",
		partials:     "partials",
		errorKey:     "errors",
		layout:       "index",
		layouts:      "layouts",
		extension:    ".html",
		renderError:  "Something went wrong.",
		disableCache: false,
		debug:        false,
		delims: goview.Delims{
			Left:  "{{",
			Right: "}}",
		},
	}

	for _, opt := range opts {
		opt(lr)
	}

	allFuncs := make(template.FuncMap)
	for k, v := range lr.funcs {
		allFuncs[k] = v
	}

	for k, v := range sprig.FuncMap() {
		allFuncs[k] = v
	}

	lr.funcs = allFuncs

	fileInfo, err := ioutil.ReadDir(fmt.Sprintf("%s/%s", lr.root, lr.partials))
	if err != nil {
		return nil, err
	}
	var partials []string
	for _, file := range fileInfo {
		if !strings.HasSuffix(file.Name(), lr.extension) {
			continue
		}
		partials = append(partials, fmt.Sprintf("%s/%s",
			lr.partials,
			strings.TrimSuffix(file.Name(), lr.extension)))
	}

	viewEngine := goview.New(goview.Config{
		Root:         lr.root,
		Extension:    lr.extension,
		Master:       fmt.Sprintf("%s/%s", lr.layouts, lr.layout),
		Partials:     partials,
		DisableCache: lr.disableCache,
		Funcs:        sprig.FuncMap(), // http://masterminds.github.io/sprig/
	})

	lr.viewEngine = viewEngine
	return func(view string, dataFuncs ...Data) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			viewData := make(map[string]interface{})
			var errStrings []string
			if lr.defaultData != nil {
				defaultData, err := lr.defaultData(w, r)
				if err != nil {
					// a wrapped error is shown to the user.
					viewError := errors.Unwrap(err)
					if viewError != nil {
						errStrings = append(errStrings, first(strings.ToLower(viewError.Error())))
						log.Printf("user:renderlayout:defaultData => %v \n ", err)
					} else {
						log.Printf("internal: renderlayout:defaultData => %v \n ", err)
					}
				}

				for k, v := range defaultData {
					viewData[k] = v
				}
			}

			// `errorkey` errors are merged. everything else is overwritten
			for _, dataFunc := range dataFuncs {
				data, err := dataFunc(w, r)
				if err != nil {
					// a wrapped error is shown to the user.
					viewError := errors.Unwrap(err)
					if viewError != nil {
						errStrings = append(errStrings, first(strings.ToLower(viewError.Error())))
						log.Printf("user error => renderlayout:defaultData => %v \n ", err)
					} else {
						log.Printf("internal error => renderlayout:defaultData => %v \n ", err)
					}
				}

				for k, v := range data {
					viewData[k] = v
				}
			}
			if len(errStrings) > 0 {
				viewData[lr.errorKey] = errStrings
			}

			err = lr.viewEngine.Render(w, http.StatusOK, view, viewData)
			if err != nil {
				log.Printf("renderlayout:render view [%s.%s],  error: %v, with data => \n %s \n",
					view, lr.extension, err, pretty(viewData))
				fmt.Fprintf(w, lr.renderError)
				return
			} else {
				if lr.debug {
					log.Printf("renderlayout:render view: [%s.%s], with data => \n %s \n",
						view, lr.extension, pretty(viewData))
				}
			}
		}
	}, nil
}

func pretty(data map[string]interface{}) string {
	var viewDataStr string
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("error marshalling:%v\n", err)
		viewDataStr = fmt.Sprintf("%v", data)
	} else {
		viewDataStr = string(b)
	}
	return viewDataStr

}

type renderer struct {
	errorKey     string
	root         string
	layout       string
	layouts      string
	partials     string
	extension    string
	disableCache bool
	renderError  string
	delims       goview.Delims
	funcs        template.FuncMap

	goviewConfig *goview.Config
	viewEngine   *goview.ViewEngine
	defaultData  Data
	debug        bool
}

func first(str string) string {
	if len(str) == 0 {
		return ""
	}
	tmp := []rune(str)
	tmp[0] = unicode.ToUpper(tmp[0])
	return string(tmp)
}
