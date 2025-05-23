package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	middlewares "gateway_service/middleware"
	"github.com/gorilla/mux"
)

type ServiceConfig struct {
	URL   string
	Paths []string
	Auth  bool
}

var services = map[string]ServiceConfig{
	"blog": {
		URL: "http://blogs-service:8081",
		Paths: []string{
			"/blogs",
			"/comments",
		},
		Auth: true,
	},
	"auth": {
		URL: "http://auth-service:8082",
		Paths: []string{
			"/login",
			"/register",
			"/profile",
			"/update-user",
			"/validate-admin",
			"/validate-session",
		},
		Auth: false,
	},
	"events": {
		URL: "http://events-service:8083",
		Paths: []string{
			"/admin/events",
			"/events",
			"/uploads/events",
		},
		Auth: false,
	},
	"profiles": {
		URL: "http://profile-service:8084",
		Paths: []string{
			"/user/profiles",
			"/user/profiles/{user_id}",
			"/user/profiles/{user_id}/follow",
		},
		Auth: true,
	},

	"attractions": {
		URL: "http://attraction-service:8085",
		Paths: []string{
			"/admin/attractions",
			"/attractions",
			"/uploads",
		},
		Auth: false,
	},
	"review": {
		URL: "http://review-service:8086",
		Paths: []string{
			"/reviews",
		},
		Auth: true,
	},
	"uploads": {
		URL:   "http://profile-service:8084",
		Paths: []string{"/uploads"},
		Auth:  false,
	},
	"plans": {
		URL: "http://plan-service:8087",
		Paths: []string{
			"/api/plans",
			"/api/plans/",
			"/api/templates",
			"/api/templates/create-plan",
		},
		Auth: true,
	},
}

var pathAuthOverrides = map[string]bool{
	"/admin/events":      true,
	"/admin/attractions": true,
	"/attractions":       true,
	"/profiles":          true,
	"/review":            true,
	"/plans":             true,
}

func main() {
	r := mux.NewRouter()

	setupRoutes(r)

	handler := middlewares.CorsMiddleware(r)

	log.Println("Gateway service running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", handler))
}

func setupRoutes(r *mux.Router) {
	for _, config := range services {
		for _, path := range config.Paths {
			handler := createProxyHandler(config.URL)

			requiresAuth := config.Auth
			if override, exists := pathAuthOverrides[path]; exists {
				requiresAuth = override
			}

			if requiresAuth {
				handler = middlewares.AuthMiddleware(handler)
			}

			if path == "/profiles/{user_id}" ||
				path == "/profiles/{user_id}/follow" ||
				path == "/profiles/{user_id}/unfollow" ||
				path == "/profiles/{user_id}/followers" ||
				path == "/profiles/{user_id}/following" {
				r.Handle(path, handler).Methods("PATCH", "POST", "DELETE", "GET")
			} else {
				r.PathPrefix(path).Handler(handler)
			}
		}
	}
}

func createProxyHandler(targetServiceURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target, err := url.Parse(targetServiceURL)
		if err != nil {
			http.Error(w, "Invalid target URL", http.StatusInternalServerError)
			return
		}

		log.Printf("Proxying request to: %s%s", target.String(), r.URL.Path)

		proxy := httputil.NewSingleHostReverseProxy(target)

		proxy.ModifyResponse = func(response *http.Response) error {
			if response.StatusCode >= 300 && response.StatusCode < 400 {
				location := response.Header.Get("Location")
				if strings.Contains(location, target.Host) {
					response.Header.Set("Location", strings.Replace(location, target.Host, "localhost:8080", 1))
				}
			}
			return nil
		}

		r.Host = target.Host
		proxy.ServeHTTP(w, r)
	})
}
