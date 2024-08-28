package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/betterstack-community/go-image-upload/db"
	"github.com/betterstack-community/go-image-upload/redisconn"
	"github.com/joho/godotenv"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var redisConn *redisconn.RedisConn

var dbConn *db.DBConn

var tracer trace.Tracer

//go:embed templates/*
var templates embed.FS

type Config struct {
	PostgresDB         string
	PostgresUser       string
	PostgresPassword   string
	PostgresHost       string
	PostgresURL        string
	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURI  string
	RedisAddr          string
	ServiceName        string
	CollectorURL       string
	InsecureMode       string
}

var conf Config

func init() {
	godotenv.Load()

	conf.PostgresDB = os.Getenv("POSTGRES_DB")
	conf.PostgresUser = os.Getenv("POSTGRES_USER")
	conf.PostgresPassword = os.Getenv("POSTGRES_PASSWORD")
	conf.PostgresHost = os.Getenv("POSTGRES_HOST")

	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=disable",
		conf.PostgresUser,
		conf.PostgresPassword,
		conf.PostgresHost,
		conf.PostgresDB,
	)

	conf.PostgresURL = connStr
	conf.RedisAddr = os.Getenv("REDIS_ADDR")
	conf.GitHubClientID = os.Getenv("GITHUB_CLIENT_ID")
	conf.GitHubClientSecret = os.Getenv("GITHUB_CLIENT_SECRET")
	conf.ServiceName = os.Getenv("OTEL_SERVICE_NAME")

	tracer = otel.Tracer(conf.ServiceName)
}

func main() {
	ctx := context.Background()

	otelShutdown, err := setupOTelSDK(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		err = errors.Join(err, otelShutdown(ctx))
		log.Println(err)
	}()

	redisConn, err = redisconn.NewRedisConn(ctx, conf.RedisAddr)
	if err != nil {
		log.Fatalf(
			"unable to connect to redis: %v",
			err,
		)
	}

	dbConn, err = db.NewDBConn(ctx, conf.PostgresDB, conf.PostgresURL)
	if err != nil {
		log.Fatalf(
			"unable to connect to db: %v",
			err,
		)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /auth/github/callback", completeGitHubAuth)

	mux.HandleFunc("GET /auth/github", redirectToGitHubLogin)

	mux.HandleFunc("GET /auth/logout", logout)

	mux.HandleFunc("GET /auth", renderAuth)

	mux.Handle("GET /", requireAuth(http.HandlerFunc(index)))

	mux.Handle("POST /upload", requireAuth(http.HandlerFunc(uploadImage)))

	httpSpanName := func(operation string, r *http.Request) string {
		return fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path)
	}

	handler := otelhttp.NewHandler(
		mux,
		"/",
		otelhttp.WithSpanNameFormatter(httpSpanName),
	)

	log.Println("Server started on port 8000")

	log.Fatal(http.ListenAndServe(":8000", handler))
}
