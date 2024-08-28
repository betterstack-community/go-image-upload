package main

import (
	"context"
	"fmt"
	"html/template"
	"image"
	"net/http"
	"path/filepath"
	"time"

	_ "image/jpeg"
	_ "image/png"

	"github.com/Kagami/go-avif"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/betterstack-community/go-image-upload/models"
)

const MAX_UPLOAD_SIZE = 10 * 1024 * 1024 // 10MB

var sessionCookieKey = "session_key"

// redirectToGitHubLogin creates an authentication token and redirects to the
// GitHub Oauth URL.
func redirectToGitHubLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stateToken, err := redisConn.CreateAuthToken(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	endpoint := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=user&state=%s",
		conf.GitHubClientID,
		conf.GitHubRedirectURI,
		stateToken,
	)

	http.Redirect(w, r, endpoint, http.StatusSeeOther)
}

func completeGitHubAuth(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")

	state := r.URL.Query().Get("state")

	ctx := r.Context()

	err := redisConn.VerifyAndDelToken(ctx, state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	endpoint := fmt.Sprintf(
		"https://github.com/login/oauth/access_token?client_id=%s&client_secret=%s&code=%s&redirect_uri=%s",
		conf.GitHubClientID,
		conf.GitHubClientSecret,
		code,
		conf.GitHubRedirectURI,
	)

	g, err := exchangeCodeForToken(ctx, endpoint)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	userProfile, err := getGitHubUserProfile(ctx, g.AccessToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user := &models.User{
		FullName: userProfile.Name,
		Email:    userProfile.Email,
	}

	err = dbConn.FindOrCreateUser(ctx, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	token, err := redisConn.CreateSessionToken(ctx, user.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieKey,
		Value:    token,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieKey)
	if err != nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	sessionCookie := http.Cookie{
		Name:     sessionCookieKey,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
	}

	http.SetCookie(w, &sessionCookie)

	_ = redisConn.DeleteSessionToken(r.Context(), cookie.Value)

	http.Redirect(w, r, "/auth", http.StatusSeeOther)
}

func requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(
			r.Context(),
			"requireAuth",
			trace.WithSpanKind(trace.SpanKindServer),
		)

		cookie, err := r.Cookie(sessionCookieKey)
		if err != nil {
			http.Redirect(w, r, "/auth", http.StatusSeeOther)
			span.AddEvent(
				"redirecting to /auth",
				trace.WithAttributes(
					attribute.String("reason", "missing session cookie"),
				),
			)
			span.End()
			return
		}

		span.SetAttributes(
			attribute.String("app.cookie.value", cookie.Value),
		)

		email, err := redisConn.GetSessionToken(ctx, cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/auth", http.StatusSeeOther)
			span.AddEvent(
				"redirecting to /auth",
				trace.WithAttributes(
					attribute.String("reason", err.Error()),
				))
			span.End()
			return
		}

		ctx = context.WithValue(r.Context(), "email", email)

		req := r.WithContext(ctx)

		span.SetStatus(codes.Ok, "authenticated successfully")

		span.End()

		next.ServeHTTP(w, req)
	})
}

func index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	email, ok := ctx.Value("email").(string)
	if !ok {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	user := &models.User{
		Email: email,
	}

	err := dbConn.GetUser(ctx, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(template.ParseFS(
		templates,
		filepath.Join(
			"templates",
			"default.html",
		),
		filepath.Join("templates", "index.html"),
	))

	tmpl.ExecuteTemplate(w, "default", user)
}

func renderAuth(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFS(
		templates,
		filepath.Join(
			"templates",
			"default.html",
		),
		filepath.Join("templates", "auth.html"),
	))

	tmpl.ExecuteTemplate(w, "default", nil)
}

func uploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MAX_UPLOAD_SIZE)
	if err := r.ParseMultipartForm(MAX_UPLOAD_SIZE); err != nil {
		http.Error(
			w,
			"The uploaded file should be no more than 10MB",
			http.StatusBadRequest,
		)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		fmt.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/avif")

	err = avif.Encode(w, img, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
