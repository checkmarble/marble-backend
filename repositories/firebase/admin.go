package firebase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"firebase.google.com/go/v4/auth"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type Adminer interface {
	CreateUser(ctx context.Context, email, name string) error
}

type AdminClient struct {
	apiKey       string
	client       *auth.Client
	marbleAppUrl string
}

func NewAdminClient(apiKey string, client *auth.Client, marbleAppUrl string) *AdminClient {
	return &AdminClient{
		apiKey:       apiKey,
		client:       client,
		marbleAppUrl: marbleAppUrl,
	}
}

func (c AdminClient) CreateUser(ctx context.Context, email, name string) error {
	req := new(auth.UserToCreate).
		Email(email).
		EmailVerified(false).
		DisplayName(name)

	user, err := c.client.CreateUser(ctx, req)
	if err != nil {
		if auth.IsEmailAlreadyExists(err) {
			utils.LoggerFromContext(ctx).InfoContext(ctx, fmt.Sprintf("firebase user already exists for user %s, skipping creating it", email),
				"email", email)

			return nil
		}

		utils.LoggerFromContext(ctx).WarnContext(ctx, fmt.Sprintf("could not create firebase user %s, your administrator will need to create it manually", email),
			"error", err.Error(),
			"email", email)

		return err
	}

	utils.LoggerFromContext(ctx).InfoContext(ctx, fmt.Sprintf("firebase user created for user %s", user.Email),
		"uid", user.UID,
		"email", user.Email)

	if err := c.SendPasswordResetEmail(ctx, user); err != nil {
		utils.LoggerFromContext(ctx).WarnContext(ctx, fmt.Sprintf("could not send the password reset email to user %s: %s", user.Email, err.Error()),
			"uid", user.UID,
			"email", user.Email)
	}

	return nil
}

func (c AdminClient) SendPasswordResetEmail(ctx context.Context, user *auth.UserRecord) error {
	payload := struct {
		RequestType string `json:"requestType"` //nolint:tagliatelle
		Email       string `json:"email"`
		ContinueUrl string `json:"continueUrl"` //nolint:tagliatelle
	}{
		RequestType: "PASSWORD_RESET",
		Email:       user.Email,
		ContinueUrl: fmt.Sprintf("%s/sign-in-email?email=%s", c.marbleAppUrl, url.QueryEscape(user.Email)),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "could not create password reset request")
	}

	u := fmt.Sprintf("https://identitytoolkit.googleapis.com/v1/accounts:sendOobCode?key=%s", url.QueryEscape(c.apiKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return errors.Wrap(err, "could not create password reset request")
	}
	req.Header.Set("referer", c.marbleAppUrl)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "could not send password reset request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Newf("password reset request returned status %d", resp.StatusCode)
	}

	utils.LoggerFromContext(ctx).InfoContext(ctx, fmt.Sprintf("firebase user password reset email sent for user %s", user.Email),
		"uid", user.UID,
		"email", user.Email)

	return nil
}
