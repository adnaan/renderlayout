## renderlayout

A wrapper over [goview](https://github.com/foolin/goview)

### Extras

- Functional options
- Opinionated view handler to render view data and user error

### Usage

See example directory for default directory structure.

```go
package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	rl "github.com/adnaan/renderlayout"
	"github.com/go-chi/chi"
)

func main() {
	indexLayout, err := rl.New(
		rl.Layout("index"),
		rl.DisableCache(true),
		rl.DefaultHandler(func(w http.ResponseWriter, r *http.Request) (rl.M, error) {
			return rl.M{
				"app_name": "renderlayout",
			}, nil
		}))

	if err != nil {
		log.Fatal(err)
	}

	appLayout, err := rl.New(
		rl.Layout("app"),
		rl.DisableCache(true),
		rl.DefaultHandler(func(w http.ResponseWriter, r *http.Request) (rl.M, error) {
			return rl.M{
				"app_name": "renderlayout",
			}, nil
		}))
	if err != nil {
		log.Fatal(err)
	}
	r := chi.NewRouter()
	r.Get("/", indexLayout.Handle("home",
		func(w http.ResponseWriter, r *http.Request) (rl.M, error) {
			return rl.M{
				"hello": "world",
			}, nil
		}))
	r.Get("/app", appLayout.Handle("dashboard",
		func(w http.ResponseWriter, r *http.Request) (rl.M, error) {
			err := fmt.Errorf("error in dashboard, %w",
				errors.New("a wrapped error which is shown to the user"))
			return rl.M{
				"dashboard": "dashboard",
			}, err
		}))
	err = http.ListenAndServe(":3000", r)
	if err != nil {
		log.Fatal(err)
	}
}

```