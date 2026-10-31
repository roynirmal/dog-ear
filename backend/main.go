package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
)

func init() {
	// Go does not know the web app manifest extension and would serve text/plain.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

func readBody(e *core.RequestEvent, v any, limit int64) error {
	e.Request.Body = http.MaxBytesReader(e.Response, e.Request.Body, limit)
	d := json.NewDecoder(e.Request.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return errors.New("invalid JSON or request too large")
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return errors.New("unexpected request content")
	}
	return nil
}

func apiError(err error) error {
	if errors.Is(err, ErrAccess) {
		return apis.NewForbiddenError("Access denied", nil)
	}
	if errors.Is(err, ErrNotFound) {
		return apis.NewNotFoundError("Not found", nil)
	}
	return apis.NewBadRequestError(err.Error(), nil)
}

func register(app core.App) {
	app.OnRecordAuthWithOAuth2Request("users").BindFunc(func(e *core.RecordAuthWithOAuth2RequestEvent) error {
		if e.ProviderName != "google" || e.OAuth2User == nil || e.OAuth2User.Email == "" {
			return apis.NewForbiddenError("A verified Google email is required", nil)
		}
		if e.Record != nil && !strings.EqualFold(e.Record.Email(), e.OAuth2User.Email) {
			return apis.NewForbiddenError("Google email must match this account", nil)
		}
		// Never allow caller-supplied createData to choose a different identity.
		name := []rune(e.OAuth2User.Name)
		if len(name) > 160 {
			name = name[:160]
		}
		e.CreateData = map[string]any{"email": e.OAuth2User.Email, "name": string(name)}
		return e.Next()
	})
	app.OnRecordRequestEmailChangeRequest("users").BindFunc(func(e *core.RecordRequestEmailChangeRequestEvent) error {
		return apis.NewForbiddenError("Email changes are not supported", nil)
	})
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/api/dogear/config", func(e *core.RequestEvent) error {
			c, err := app.FindCollectionByNameOrId("users")
			if err != nil {
				return err
			}
			p, ok := c.OAuth2.GetProviderConfig("google")
			return e.JSON(200, map[string]any{"google": c.OAuth2.Enabled && ok && p.ClientId != "" && p.ClientSecret != ""})
		})
		se.Router.GET("/api/dogear/search", func(e *core.RequestEvent) error {
			books, err := searchBooks(e.Request.Context(), e.Request.URL.Query().Get("q"))
			if err != nil {
				return apiError(err)
			}
			return e.JSON(200, books)
		}).Bind(apis.RequireAuth("users"))
		se.Router.POST("/api/dogear/{action}", func(e *core.RequestEvent) error {
			var in Input
			if err := readBody(e, &in, 64*1024); err != nil {
				return apiError(err)
			}
			out, err := execute(app, e.Auth, e.Request.PathValue("action"), in)
			if err != nil {
				return apiError(err)
			}
			return e.JSON(200, out)
		}).Bind(apis.RequireAuth("users"))
		// The statically exported frontend, when built (make up). Unknown paths
		// fall back to index.html; PocketBase's own /api and /_/ routes win.
		if info, err := os.Stat("pb_public"); err == nil && info.IsDir() {
			se.Router.GET("/{path...}", apis.Static(os.DirFS("pb_public").(fs.FS), true))
		}
		return se.Next()
	})
}

func main() {
	app := pocketbase.New()
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{Automigrate: false})
	register(app)
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := configure(app); err != nil {
			return err
		}
		return e.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
