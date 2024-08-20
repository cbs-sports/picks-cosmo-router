package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"go.uber.org/zap"
)

type sapi struct {
	baseUrl string
	authKey string
	logger  *zap.Logger
}

type sapiResponse struct {
	Details *sapiUser `json:"details"`
}

type sapiUser struct {
	CustId       int64  `json:"custId"`
	UserLogin    string `json:"userLogin"`
	Email        string `json:"emailAddress"`
	FirstName    string `json:"firstName"`
	LastName     string `json:"lastName"`
	EncryptedPid string `json:"encryptedPid"`
}

func newSapi(logger *zap.Logger) (*sapi, error) {
	baseUrl, authKey := "https://sapi-qa.cbssports.com", ""

	if value, exists := os.LookupEnv("SAPI_URL"); exists {
		baseUrl = value
	} else {
		return nil, fmt.Errorf("SAPI_URL env variable not found")
	}

	if value, exists := os.LookupEnv("SAPI_AUTH_KEY"); exists {
		authKey = value
	} else {
		return nil, fmt.Errorf("SAPI_AUTH_KEY env variable not found")

	}

	return &sapi{
		baseUrl: baseUrl,
		authKey: authKey,
		logger:  logger,
	}, nil
}

func (s sapi) userDetails(ctx context.Context, userLogin string) (*sapiUser, error) {
	s.logger.Debug("getting sapi user details", zap.String("userLogin", userLogin))

	url := fmt.Sprintf("%s/sporty-api/user/details?edition=us&view=json&returnType=99&authKey=%s&userLogin=%s", s.baseUrl, s.authKey, userLogin)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("can not build request: %v", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error while requesting sapi user details: %v", err)
	}
	if http.StatusOK != response.StatusCode {
		return nil, fmt.Errorf("error status code received for sapi user details: %v", response.StatusCode)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error while reading sapi user response: %v", err)
	}

	var sapiResponse sapiResponse
	err = json.Unmarshal(responseBody, &sapiResponse)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshaling user response: %v", err)
	}

	return sapiResponse.Details, nil
}
