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
		rl.DefaultData(func(w http.ResponseWriter, r *http.Request) (rl.D, error) {
			return rl.D{
				"app_name": "renderlayout",
			}, nil
		}))

	if err != nil {
		log.Fatal(err)
	}

	appLayout, err := rl.New(
		rl.Layout("app"),
		rl.DisableCache(true),
		rl.DefaultData(func(w http.ResponseWriter, r *http.Request) (rl.D, error) {
			return rl.D{
				"app_name": "renderlayout",
			}, nil
		}))
	if err != nil {
		log.Fatal(err)
	}
	r := chi.NewRouter()
	r.Get("/", indexLayout("home",
		func(w http.ResponseWriter, r *http.Request) (rl.D, error) {
			return rl.D{
				"hello": "world",
			}, nil
		}))
	r.Get("/app", appLayout("dashboard",
		func(w http.ResponseWriter, r *http.Request) (rl.D, error) {
			err := fmt.Errorf("error in dashboard, %w",
				errors.New("a wrapped error which is shown to the user"))
			return rl.D{
				"dashboard": "dashboard",
			}, err
		}))
	err = http.ListenAndServe(":3000", r)
	if err != nil {
		log.Fatal(err)
	}
}
