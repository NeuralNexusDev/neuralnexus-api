package routes

import "net/http"

type Router func(*http.ServeMux, *http.ServeMux) (*http.ServeMux, *http.ServeMux)

func CreateStack(routers ...Router) Router {
	return func(mux, authedMux *http.ServeMux) (*http.ServeMux, *http.ServeMux) {
		for _, router := range routers {
			mux, authedMux = router(mux, authedMux)
		}
		return mux, authedMux
	}
}
