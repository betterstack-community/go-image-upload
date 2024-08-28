package main

import (
	"context"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var httpClient = &http.Client{
	Timeout:   2 * time.Minute,
	Transport: otelhttp.NewTransport(http.DefaultTransport),
}

// GitHubUserProfile represents the user profile data received
// from GitHub after successful authentication.
type GitHubUserProfile struct {
	Login           string `json:"login"`
	AvatarURL       string `json:"avatar_url"`
	GravatarID      string `json:"gravatar_id"`
	Type            string `json:"type"`
	Name            string `json:"name"`
	Company         string `json:"company"`
	Blog            string `json:"blog"`
	Location        string `json:"location"`
	Email           string `json:"email"`
	Bio             string `json:"bio"`
	TwitterUsername string `json:"twitter_username"`
}

type OauthResponse struct {
	AccessToken string `json:"access_token"`
}

// getGitHubUserProfile retrives the authenticated user's profile.
func getGitHubUserProfile(
	ctx context.Context,
	accessToken string,
) (*GitHubUserProfile, error) {
	endpoint := "https://api.github.com/user"

	var userProfile GitHubUserProfile

	client := resty.NewWithClient(httpClient)

	_, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "token "+accessToken).
		SetContext(ctx).
		SetResult(&userProfile).
		Get(endpoint)

	return &userProfile, err
}

// exchangeCodeForToken exchanges the received Oauth code for an access token.
func exchangeCodeForToken(
	ctx context.Context,
	endpoint string,
) (*OauthResponse, error) {
	var oauth OauthResponse

	client := resty.NewWithClient(httpClient)

	_, err := client.R().
		SetHeader("Accept", "application/json").
		SetContext(ctx).
		SetResult(&oauth).
		Post(endpoint)
	if err != nil {
		return nil, err
	}

	return &oauth, nil
}
