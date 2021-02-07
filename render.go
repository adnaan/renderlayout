package renderlayout

import (
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

type M map[string]interface{}

type ViewHandlerFunc func(w http.ResponseWriter, r *http.Request) (M, error)

type LayoutRendererOption func(renderer *LayoutRenderer)

// DefaultHandler is the function called everytime before a template is rendered. Default is nil
// This can be used to set template variables needed in every template.
func DefaultHandler(handlerFunc ViewHandlerFunc) LayoutRendererOption {
	return func(renderer *LayoutRenderer) {
		renderer.defaultHandler = handlerFunc
	}
}

// ErrorKey changes the template variable name containing view errors. Default value is "error"
func ErrorKey(key string) LayoutRendererOption {
	return func(renderer *LayoutRenderer) {
		renderer.errorKey = key
	}
}

// TemplatesPath is the path to root directory for the templates. Default value is "templates"
func TemplatesPath(templatesPath string) LayoutRendererOption {
	return func(renderer *LayoutRenderer) {
		renderer.root = templatesPath
	}
}

// LayoutsPath sets name of the main template to be used. Default value is "layouts"
// The path is searched within the templates layouts path. e.g. "templates/layouts"
func LayoutsPath(layouts string) LayoutRendererOption {
	return func(renderer *LayoutRenderer) {
		renderer.layouts = layouts
	}
}

// PartialsPath sets the path to main template to be used. Default value is "partials"
// The path is searched within the templates path. e.g. "templates/partials"
func PartialsPath(partials string) LayoutRendererOption {
	return func(renderer *LayoutRenderer) {
		renderer.partials = partials
	}
}

// Layout sets name of the main template to be used. Default value is "index"
// The path is searched within the templates layouts path. e.g. "templates/layouts/index.html"
func Layout(layout string) LayoutRendererOption {
	return func(renderer *LayoutRenderer) {
		renderer.layout = layout
	}
}

// Extension sets the file extension for templates and partials. Default value is html
func Extension(extension string) LayoutRendererOption {
	return func(renderer *LayoutRenderer) {
		renderer.extension = fmt.Sprintf(".%s", extension)
	}
}

// DisableCache disables the cache. Default value is false
func DisableCache(disableCache bool) LayoutRendererOption {
	return func(renderer *LayoutRenderer) {
		renderer.disableCache = disableCache
	}
}

// Delimiters sets the template delimiters. Default value is, left: {{ , right: }}
func Delimiters(left, right string) LayoutRendererOption {
	return func(renderer *LayoutRenderer) {
		renderer.delims = goview.Delims{
			Left:  left,
			Right: right,
		}
	}
}

// AddFuncs adds additional templates funcs. Default is nil
// github.com/Masterminds/sprig is already configured.
func AddFuncs(funcMap template.FuncMap) LayoutRendererOption {
	return func(renderer *LayoutRenderer) {
		renderer.funcs = funcMap
	}
}

// DefaultError sets the default value set in template variable "error". Default value is "Internal Error"
func DefaultError(defaultError string) LayoutRendererOption {
	return func(renderer *LayoutRenderer) {
		renderer.defaultError = defaultError
	}
}

// RenderError sets the error shown to the user when rendering fails completely. Default value is "Something went wrong."
func RenderError(error string) LayoutRendererOption {
	return func(renderer *LayoutRenderer) {
		renderer.renderError = error
	}
}

func StaticView(w http.ResponseWriter, r *http.Request) (M, error) {
	return M{}, nil
}

func New(opts ...LayoutRendererOption) (*LayoutRenderer, error) {

	d := &LayoutRenderer{
		root:         "templates",
		partials:     "partials",
		errorKey:     "error",
		layout:       "index",
		layouts:      "layouts",
		extension:    ".html",
		defaultError: "Internal Error",
		renderError:  "Something went wrong.",
		disableCache: false,
		delims: goview.Delims{
			Left:  "{{",
			Right: "}}",
		},
	}

	for _, opt := range opts {
		opt(d)
	}

	allFuncs := make(template.FuncMap)
	for k, v := range d.funcs {
		allFuncs[k] = v
	}

	for k, v := range sprig.FuncMap() {
		allFuncs[k] = v
	}

	d.funcs = allFuncs

	fileInfo, err := ioutil.ReadDir(fmt.Sprintf("%s/%s", d.root, d.partials))
	if err != nil {
		return nil, err
	}
	var partials []string
	for _, file := range fileInfo {
		if !strings.HasSuffix(file.Name(), d.extension) {
			continue
		}
		partials = append(partials, fmt.Sprintf("%s/%s",
			d.partials,
			strings.TrimSuffix(file.Name(), d.extension)))
	}

	viewEngine := goview.New(goview.Config{
		Root:         d.root,
		Extension:    d.extension,
		Master:       fmt.Sprintf("%s/%s", d.layouts, d.layout),
		Partials:     partials,
		DisableCache: d.disableCache,
		Funcs:        sprig.FuncMap(), // http://masterminds.github.io/sprig/
	})

	d.viewEngine = viewEngine
	return d, nil
}

type LayoutRenderer struct {
	errorKey     string
	root         string
	layout       string
	layouts      string
	partials     string
	extension    string
	disableCache bool
	defaultError string
	renderError  string
	delims       goview.Delims
	funcs        template.FuncMap

	goviewConfig   *goview.Config
	viewEngine     *goview.ViewEngine
	defaultHandler ViewHandlerFunc
}

func first(str string) string {
	if len(str) == 0 {
		return ""
	}
	tmp := []rune(str)
	tmp[0] = unicode.ToUpper(tmp[0])
	return string(tmp)
}

func (lr *LayoutRenderer) Handle(view string, viewHandlerFunc ViewHandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		viewData := make(map[string]interface{})
		if lr.defaultHandler != nil {
			defaultData, err := lr.defaultHandler(w, r)
			if err != nil {
				log.Println("renderlayout:defaultHandler func => ", err)
				viewError := errors.Unwrap(err)
				if viewError != nil {
					viewData[lr.errorKey] = first(strings.ToLower(viewError.Error()))
				} else {
					viewData[lr.errorKey] = lr.defaultError
				}
			}

			for k, v := range defaultData {
				viewData[k] = v
			}
		}

		data, err := viewHandlerFunc(w, r)
		if err != nil {
			log.Println("renderlayout:viewHandlerFunc => ", err)
			viewError := errors.Unwrap(err)
			if viewError != nil {
				if viewData[lr.errorKey] != nil {
					viewData[lr.errorKey] = fmt.Sprintf("%s %s",
						viewData[lr.errorKey],
						first(strings.ToLower(viewError.Error())))
				} else {
					viewData[lr.errorKey] = first(strings.ToLower(viewError.Error()))
				}
			} else {
				viewData[lr.errorKey] = lr.defaultError
			}
		}

		for k, v := range viewData {
			data[k] = v
		}

		err = lr.viewEngine.Render(w, http.StatusOK, view, data)
		if err != nil {
			fmt.Printf("renderlayout:render error: %v with data %v \n", err, data)
			fmt.Fprintf(w, lr.renderError)
			return
		}

	})
}

func (lr *LayoutRenderer) HandleStatic(view string) http.HandlerFunc {
	return lr.HandleStaticWithData(view, nil)
}

func (lr *LayoutRenderer) HandleStaticWithData(view string, data M) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		viewData := make(map[string]interface{})
		if lr.defaultHandler != nil {
			defaultData, err := lr.defaultHandler(w, r)
			if err != nil {
				log.Println("renderlayout:defaultHandler func => ", err)
				viewError := errors.Unwrap(err)
				if viewError != nil {
					viewData[lr.errorKey] = first(strings.ToLower(viewError.Error()))
				} else {
					viewData[lr.errorKey] = lr.defaultError
				}
			}

			for k, v := range defaultData {
				viewData[k] = v
			}
		}

		for k, v := range data {
			viewData[k] = v
		}

		err := lr.viewEngine.Render(w, http.StatusOK, view, viewData)
		if err != nil {
			fmt.Printf("renderlayout:render error: %v with data %v \n", err, viewData)
			fmt.Fprintf(w, lr.renderError)
			return
		}
	})
}
