package usecases

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var testSubnets = []string{"10.10.1.2/16", "127.0.0.0/8"}

func ipWhitelistTestHarness(t *testing.T, use AllowedNetworksUse, cidrs []string) (context.Context, *gin.Engine, *httptest.ResponseRecorder) {
	t.Helper()

	gin.SetMode(gin.ReleaseMode)

	creds := models.Credentials{OrganizationId: "orgid"}

	subnets := make([]net.IPNet, 0)

	for _, cidr := range cidrs {
		_, subnet, _ := net.ParseCIDR(cidr)

		subnets = append(subnets, *subnet)
	}

	repo := new(mocks.OrganizationRepository)

	if cidrs != nil {
		repo.On("GetOrganizationAllowedNetworks", mock.Anything, mock.Anything, "orgid").
			Return(subnets, nil)
	} else {
		repo.On("GetOrganizationAllowedNetworks", mock.Anything, mock.Anything, "orgid").
			Return(nil, errors.New("could not retrieve whitelist"))
	}

	uc := AllowedNetworksUsecase{
		executorFactory: executor_factory.NewExecutorFactoryStub(),
		repository:      repo,
	}

	w := httptest.NewRecorder()
	_, e := gin.CreateTestContext(w)

	e.Use(uc.Guard(use))

	ctx := context.WithValue(t.Context(), utils.ContextKeyCredentials, creds)

	return ctx, e, w
}

func requestFromIpAddress(t *testing.T, ctx context.Context, ip string) *http.Request {
	t.Helper()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)

	if ip != "" {
		req.Header.Set("x-real-ip", ip)
	}

	return req
}

func TestIpWhitelistForLoginAllowed(t *testing.T) {
	called := false
	ctx, e, wr := ipWhitelistTestHarness(t, AllowedNetworksLogin, testSubnets)
	req := requestFromIpAddress(t, ctx, "127.0.0.1")

	e.GET("/", func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "OK")
	})

	e.ServeHTTP(wr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, wr.Code)
	assert.Equal(t, "OK", wr.Body.String())
}

func TestIpWhitelistForLoginRestricted(t *testing.T) {
	called := false
	ctx, e, wr := ipWhitelistTestHarness(t, AllowedNetworksLogin, testSubnets)
	req := requestFromIpAddress(t, ctx, "192.168.1.2")

	e.GET("/", func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "OK")
	})

	e.ServeHTTP(wr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusForbidden, wr.Code)
	assert.NotContains(t, wr.Body.String(), "OK")
}

func TestIpWhitelistForLoginNoClientIp(t *testing.T) {
	called := false
	ctx, e, wr := ipWhitelistTestHarness(t, AllowedNetworksLogin, testSubnets)
	req := requestFromIpAddress(t, ctx, "127.0.0.1")

	e.GET("/", func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "OK")
	})

	e.ServeHTTP(wr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, wr.Code)
	assert.Equal(t, "OK", wr.Body.String())
}

func TestIpWhitelistForLoginNoWhitelist(t *testing.T) {
	called := false
	ctx, e, wr := ipWhitelistTestHarness(t, AllowedNetworksLogin, []string{})
	req := requestFromIpAddress(t, ctx, "")

	e.GET("/", func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "OK")
	})

	e.ServeHTTP(wr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, wr.Code)
	assert.Equal(t, "OK", wr.Body.String())
}

// TODO: this might change depending on ip_whitelist_usecase.go:73.
func TestIpWhitelistForLoginError(t *testing.T) {
	called := false
	ctx, e, wr := ipWhitelistTestHarness(t, AllowedNetworksLogin, nil)
	req := requestFromIpAddress(t, ctx, "127.0.0.1")

	e.GET("/", func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "OK")
	})

	e.ServeHTTP(wr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, wr.Code)
	assert.Equal(t, "OK", wr.Body.String())
}

func TestIpWhitelistForEndpointAllowed(t *testing.T) {
	called := false
	ctx, e, wr := ipWhitelistTestHarness(t, AllowedNetworksOther, testSubnets)
	req := requestFromIpAddress(t, ctx, "10.10.255.200")

	e.GET("/", func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "OK")
	})

	e.ServeHTTP(wr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, wr.Code)
	assert.Equal(t, "OK", wr.Body.String())
}

func TestIpWhitelistForEndpointRestricted(t *testing.T) {
	called := false
	ctx, e, wr := ipWhitelistTestHarness(t, AllowedNetworksOther, testSubnets)
	req := requestFromIpAddress(t, ctx, "172.16.10.10")

	e.GET("/", func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "OK")
	})

	e.ServeHTTP(wr, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusForbidden, wr.Code)
	assert.NotContains(t, wr.Body.String(), "OK")
}
