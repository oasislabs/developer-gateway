package insecure

import (
	"errors"
	"net/http"

	"github.com/oasislabs/developer-gateway/stats"
)

const HeaderKey string = "X-OASIS-INSECURE-AUTH"

var ErrDataTooShort = errors.New("Payload data is too short")

// InsecureAuth is an insecure authentication mechanism that may be
// useful for debugging and testing. It should not be used in
// setups with real users.
type InsecureAuth struct{}

func (a InsecureAuth) Name() string {
	return "auth.insecure.InsecureAuth"
}

func (a InsecureAuth) Stats() stats.Metrics {
	return nil
}

func (a InsecureAuth) Authenticate(req *http.Request) (string, error) {
	value := req.Header.Get(HeaderKey)
	if len(value) == 0 {
		return "", ErrDataTooShort
	}

	return value, nil
}

func (InsecureAuth) Verify(data string, expectedAAD string) error {
	if len(data) == 0 {
		return ErrDataTooShort
	}

	return nil
}
