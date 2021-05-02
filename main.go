package main

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/portier/portier-go"
)

//go:embed *.html
var fs embed.FS
var tmpl = template.Must(template.ParseFS(fs, "*.html"))

const defaultRemember = 14 * 24 * 60 * 60 // 2 weeks

type authPage struct{}

func (p authPage) render(w io.Writer) {
	err := tmpl.ExecuteTemplate(w, "auth.html", p)
	if err != nil {
		log.Fatal("auth.html render error:", err)
	}
}

type errorPage struct {
	Message string
}

func (p errorPage) render(w io.Writer) {
	err := tmpl.ExecuteTemplate(w, "error.html", p)
	if err != nil {
		log.Fatal("error.html render error:", err)
	}
}

func main() {
	listen := flag.String("listen", ":8081", "listen address")
	authURL := flag.String("url", "", "base URL proxied here")
	cookieName := flag.String("cookie", "AuthSession", "cookie name")
	secret := flag.String("secret", "", "cookie signing secret")
	remember := flag.Int("remember", defaultRemember, "session duration in seconds")
	secure := flag.Bool("secure", false, "set the cookie secure flag")
	broker := flag.String("broker", portier.DefaultBroker, "Portier broker to use")
	optional := flag.Bool("optional", false, "make login optional")
	allowListFile := flag.String("allowlist", "", "file of emails (one per line) to limit access to")
	flag.Parse()

	if *authURL == "" {
		log.Fatal("The -url flag is required")
	}
	if *secret == "" {
		log.Fatal("The -secret flag is required")
	}

	parsedURL, err := url.Parse(*authURL)
	if err != nil {
		log.Fatal("invalid -url:", err)
	}
	if !parsedURL.IsAbs() {
		log.Fatal("invalid -url: not an absolute URL")
	}

	client, err := portier.NewClient(&portier.Config{
		Broker:      *broker,
		RedirectURI: fmt.Sprintf("%s/verify", *authURL),
	})
	if err != nil {
		log.Fatal("portier.NewClient error:", err)
	}

	var allowList map[string]struct{}
	if *allowListFile != "" {
		file, err := os.Open(*allowListFile)
		if err != nil {
			log.Fatal("invalid -allowlist:", err)
		}
		defer file.Close()

		allowList = make(map[string]struct{})
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				allowList[line] = struct{}{}
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}

	authPath := parsedURL.Path
	verifyPath := fmt.Sprintf("%s/verify", parsedURL.Path)
	sigName := fmt.Sprintf("%s.sig", *cookieName)
	signValue := func(val string) string {
		hasher := hmac.New(sha256.New, []byte(*secret))
		hasher.Write([]byte(val))
		return base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	}

	http.HandleFunc("/check", func(rw http.ResponseWriter, req *http.Request) {
		var email string

		cookie, _ := req.Cookie(*cookieName)
		sig, _ := req.Cookie(sigName)
		if cookie != nil && sig != nil && cookie.Value != "" && sig.Value != "" {
			expected := signValue(cookie.Value)
			if subtle.ConstantTimeCompare([]byte(sig.Value), []byte(expected)) == 1 {
				email = cookie.Value
			}
		}

		rw.Header().Set("X-Portier-Email", email)

		if email != "" && allowList != nil {
			if _, ok := allowList[email]; !ok {
				rw.WriteHeader(403)
				return
			}
		}

		if email != "" || *optional {
			rw.WriteHeader(204)
		} else {
			rw.WriteHeader(403)
		}
	})

	http.HandleFunc(authPath, func(rw http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case "GET":
			authPage{}.render(rw)

		case "POST":
			email := req.PostFormValue("email")
			if email == "" {
				authPage{}.render(rw)
				return
			}

			authURL, err := client.StartAuth(email)
			if err != nil {
				log.Print("portier.Client.StartAuth error:", err)
				rw.WriteHeader(500)
				errorPage{Message: err.Error()}.render(rw)
				return
			}

			rw.Header().Set("Location", authURL)
			rw.WriteHeader(303)

		case "DELETE":
			http.SetCookie(rw, &http.Cookie{
				Name:     *cookieName,
				Path:     "/",
				MaxAge:   -1,
				Secure:   *secure,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			http.SetCookie(rw, &http.Cookie{
				Name:     sigName,
				Path:     "/",
				MaxAge:   -1,
				Secure:   *secure,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			rw.WriteHeader(204)

		default:
			rw.WriteHeader(405)
			errorPage{Message: "invalid request method"}.render(rw)
		}
	})

	http.HandleFunc(verifyPath, func(rw http.ResponseWriter, req *http.Request) {
		if req.Method != "POST" {
			rw.WriteHeader(405)
			errorPage{Message: "invalid request method"}.render(rw)
			return
		}

		token := req.PostFormValue("id_token")
		if token == "" {
			msg := req.PostFormValue("error_description")
			if msg == "" {
				msg = "missing id_token"
			}
			rw.WriteHeader(400)
			errorPage{Message: msg}.render(rw)
			return
		}

		email, err := client.Verify(token)
		if err != nil {
			rw.WriteHeader(400)
			errorPage{Message: err.Error()}.render(rw)
			return
		}

		if allowList != nil {
			if _, ok := allowList[email]; !ok {
				rw.WriteHeader(403)
				errorPage{Message: "your email address is not allowed to access this site"}.render(rw)
				return
			}
		}

		http.SetCookie(rw, &http.Cookie{
			Name:     *cookieName,
			Value:    email,
			Path:     "/",
			MaxAge:   *remember,
			Secure:   *secure,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		http.SetCookie(rw, &http.Cookie{
			Name:     sigName,
			Value:    signValue(email),
			Path:     "/",
			MaxAge:   *remember,
			Secure:   *secure,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		rw.Header().Set("Location", "/")
		rw.WriteHeader(303)
	})

	log.Fatal(http.ListenAndServe(*listen, nil))
}
