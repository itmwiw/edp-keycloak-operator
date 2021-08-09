package adapter

import (
	"context"
	"strings"
	"testing"

	"github.com/epam/edp-keycloak-operator/pkg/model"

	"github.com/go-resty/resty/v2"
	"github.com/jarcoal/httpmock"

	"github.com/Nerzal/gocloak/v8"
	"github.com/epam/edp-keycloak-operator/pkg/client/keycloak/mock"

	"github.com/pkg/errors"
)

func TestIsErrNotFound(t *testing.T) {
	err := errors.Wrap(ErrNotFound("not found"), "err")

	if errors.Cause(err).Error() != "not found" {
		t.Fatalf("wrong error message: %s", err.Error())
	}

	if !IsErrNotFound(err) {
		t.Fatal("error must be ErrNotFound")
	}

	if IsErrNotFound(errors.New("fake")) {
		t.Fatal("error is not ErrNotFound")
	}
}

func TestGoCloakAdapter_CreateClientScope(t *testing.T) {
	mockClient := MockGoCloakClient{}
	adapter := GoCloakAdapter{
		client:   &mockClient,
		token:    &gocloak.JWT{AccessToken: "token"},
		basePath: "",
		log:      &mock.Logger{},
	}

	restyClient := resty.New()
	httpmock.ActivateNonDefault(restyClient.GetClient())
	mockClient.On("RestyClient").Return(restyClient)

	rsp := httpmock.NewStringResponse(200, "")
	rsp.Header.Set("Location", "id/new-scope-id")

	httpmock.RegisterResponder("POST", strings.Replace(postClientScope, "{realm}", "realm1", 1),
		httpmock.ResponderFromResponse(rsp))

	defaultClientScopePath := strings.ReplaceAll(putDefaultClientScope, "{realm}", "realm1")
	defaultClientScopePath = strings.ReplaceAll(defaultClientScopePath, "{clientScopeID}", "new-scope-id")
	httpmock.RegisterResponder("PUT", defaultClientScopePath, httpmock.NewStringResponder(200, ""))

	id, err := adapter.CreateClientScope(context.Background(), "realm1",
		&ClientScope{Name: "demo", Default: true})
	if err != nil {
		t.Fatal(err)
	}

	if id == "" {
		t.Fatal("scope id is empty")
	}
}

func TestGoCloakAdapter_CreateClientScope_FailureSetDefault(t *testing.T) {
	mockClient := MockGoCloakClient{}
	adapter := GoCloakAdapter{
		client:   &mockClient,
		token:    &gocloak.JWT{AccessToken: "token"},
		basePath: "",
		log:      &mock.Logger{},
	}

	restyClient := resty.New()
	httpmock.ActivateNonDefault(restyClient.GetClient())
	mockClient.On("RestyClient").Return(restyClient)

	rsp := httpmock.NewStringResponse(200, "")
	rsp.Header.Set("Location", "id/new-scope-id")
	httpmock.Reset()
	httpmock.RegisterResponder("POST", strings.Replace(postClientScope, "{realm}", "realm1", 1),
		httpmock.ResponderFromResponse(rsp))

	_, err := adapter.CreateClientScope(context.Background(), "realm1",
		&ClientScope{Name: "demo", Default: true})
	if err == nil {
		t.Fatal("no error returned")
	}

	if !strings.Contains(err.Error(), "unable to set default client scope for realm") {
		t.Fatalf("wrong error returned: %s", err.Error())
	}
}

func TestGoCloakAdapter_CreateClientScope_FailureCreate(t *testing.T) {
	mockClient := MockGoCloakClient{}
	adapter := GoCloakAdapter{
		client: &mockClient,
		token:  &gocloak.JWT{AccessToken: "token"},
	}

	restyClient := resty.New()
	httpmock.Reset()
	httpmock.ActivateNonDefault(restyClient.GetClient())
	mockClient.On("RestyClient").Return(restyClient)

	_, err := adapter.CreateClientScope(context.Background(), "realm1",
		&ClientScope{Name: "demo", Default: true})

	if err == nil {
		t.Fatal("no error returned")
	}

	if !strings.Contains(err.Error(), "unable to create client scope") {
		t.Fatalf("wrong error returned: %s", err.Error())
	}
}

func TestGoCloakAdapter_CreateClientScope_FailureGetID(t *testing.T) {
	mockClient := MockGoCloakClient{}
	adapter := GoCloakAdapter{
		client: &mockClient,
		token:  &gocloak.JWT{AccessToken: "token"},
	}

	restyClient := resty.New()
	httpmock.Reset()
	httpmock.ActivateNonDefault(restyClient.GetClient())
	mockClient.On("RestyClient").Return(restyClient)

	rsp := httpmock.NewStringResponse(200, "")
	httpmock.RegisterResponder("POST", strings.Replace(postClientScope, "{realm}", "realm1", 1),
		httpmock.ResponderFromResponse(rsp))

	_, err := adapter.CreateClientScope(context.Background(), "realm1",
		&ClientScope{Name: "demo", Default: true})

	err = errors.Cause(err)
	if err == nil {
		t.Fatal("no error returned")
	}

	if !strings.Contains(err.Error(), "location header is not set or empty") {
		t.Fatalf("wrong error returned: %s", err.Error())
	}
}

func TestGoCloakAdapter_UpdateClientScope(t *testing.T) {
	mockClient := MockGoCloakClient{}
	adapter := GoCloakAdapter{
		client:   &mockClient,
		token:    &gocloak.JWT{AccessToken: "token"},
		basePath: "",
		log:      &mock.Logger{},
	}

	var (
		realmName = "realm1"
		scopeID   = "scope1"
	)

	restyClient := resty.New()
	httpmock.ActivateNonDefault(restyClient.GetClient())
	mockClient.On("RestyClient").Return(restyClient)
	mockClient.On("GetClientScope", realmName, scopeID).Return(&gocloak.ClientScope{
		ID: gocloak.StringP("scope1"),
		ProtocolMappers: &[]gocloak.ProtocolMappers{
			{
				Name: gocloak.StringP("mp1"),
				ID:   gocloak.StringP("mp_id1"),
			},
		},
	}, nil)

	putClientScope := strings.ReplaceAll(putClientScope, "{realm}", realmName)
	putClientScope = strings.ReplaceAll(putClientScope, "{id}", scopeID)
	httpmock.RegisterResponder("PUT", putClientScope, httpmock.NewStringResponder(200, ""))

	deleteDefaultClientScope := strings.ReplaceAll(deleteDefaultClientScope, "{realm}", realmName)
	deleteDefaultClientScope = strings.ReplaceAll(deleteDefaultClientScope, "{clientScopeID}", scopeID)
	httpmock.RegisterResponder("DELETE", deleteDefaultClientScope, httpmock.NewStringResponder(200, ""))

	deleteClientScopeProtocolMapper := strings.ReplaceAll(deleteClientScopeProtocolMapper, "{realm}", realmName)
	deleteClientScopeProtocolMapper = strings.ReplaceAll(deleteClientScopeProtocolMapper, "{clientScopeID}", scopeID)
	deleteClientScopeProtocolMapper = strings.ReplaceAll(deleteClientScopeProtocolMapper, "{protocolMapperID}", "mp_id1")
	httpmock.RegisterResponder("DELETE", deleteClientScopeProtocolMapper, httpmock.NewStringResponder(200, ""))

	createClientScopeProtocolMapper := strings.ReplaceAll(createClientScopeProtocolMapper, "{realm}", realmName)
	createClientScopeProtocolMapper = strings.ReplaceAll(createClientScopeProtocolMapper, "{clientScopeID}", scopeID)
	httpmock.RegisterResponder("POST", createClientScopeProtocolMapper, httpmock.NewStringResponder(200, ""))

	putDefaultClientScope := strings.ReplaceAll(putDefaultClientScope, "{realm}", realmName)
	putDefaultClientScope = strings.ReplaceAll(putDefaultClientScope, "{clientScopeID}", scopeID)
	httpmock.RegisterResponder("PUT", putDefaultClientScope, httpmock.NewStringResponder(200, ""))

	if err := adapter.UpdateClientScope(context.Background(), realmName, scopeID, &ClientScope{
		Name: "scope1",
		ProtocolMappers: []ProtocolMapper{
			{
				Name: "mp2",
			},
		},
	}); err != nil {
		t.Fatalf("%+v", err)
	}

	if err := adapter.UpdateClientScope(context.Background(), realmName, scopeID, &ClientScope{
		Name: "scope1",
		ProtocolMappers: []ProtocolMapper{
			{
				Name: "mp2",
			},
		},
		Default: true,
	}); err != nil {
		t.Fatalf("%+v", err)
	}
}

func TestGoCloakAdapter_GetClientScope(t *testing.T) {
	mockClient := MockGoCloakClient{}
	adapter := GoCloakAdapter{
		client:   &mockClient,
		token:    &gocloak.JWT{AccessToken: "token"},
		basePath: "",
		log:      &mock.Logger{},
	}

	restyClient := resty.New()
	httpmock.ActivateNonDefault(restyClient.GetClient())
	mockClient.On("RestyClient").Return(restyClient)

	result := []*model.ClientScope{{Name: gocloak.StringP("name1")}}

	getOneClientScope := strings.ReplaceAll(getOneClientScope, "{realm}", "realm1")
	httpmock.RegisterResponder("GET", getOneClientScope,
		httpmock.NewJsonResponderOrPanic(200, &result))

	if _, err := adapter.GetClientScope("name1", "realm1"); err != nil {
		t.Fatal(err)
	}
}

func TestGoCloakAdapter_DeleteClientScope(t *testing.T) {
	mockClient := MockGoCloakClient{}
	adapter := GoCloakAdapter{
		client:   &mockClient,
		token:    &gocloak.JWT{AccessToken: "token"},
		basePath: "",
		log:      &mock.Logger{},
	}

	restyClient := resty.New()
	httpmock.ActivateNonDefault(restyClient.GetClient())
	mockClient.On("RestyClient").Return(restyClient)

	deleteDefaultClientScope := strings.ReplaceAll(deleteDefaultClientScope, "{realm}", "realm1")
	deleteDefaultClientScope = strings.ReplaceAll(deleteDefaultClientScope, "{clientScopeID}", "scope1")

	httpmock.RegisterResponder("DELETE", deleteDefaultClientScope, httpmock.NewStringResponder(200, ""))
	mockClient.On("DeleteClientScope", "realm1", "scope1").Return(nil)

	if err := adapter.DeleteClientScope(context.Background(), "realm1", "scope1"); err != nil {
		t.Fatal(err)
	}
}

func TestGetClientScope(t *testing.T) {
	_, err := getClientScope("scope1", []*model.ClientScope{})
	if !IsErrNotFound(err) {
		t.Fatalf("wrong error returned: %s", err.Error())
	}
}

func TestGoCloakAdapter_DeleteClientScope_Failure(t *testing.T) {
	mockClient := MockGoCloakClient{}
	adapter := GoCloakAdapter{
		client:   &mockClient,
		token:    &gocloak.JWT{AccessToken: "token"},
		basePath: "",
		log:      &mock.Logger{},
	}

	restyClient := resty.New()
	httpmock.ActivateNonDefault(restyClient.GetClient())
	mockClient.On("RestyClient").Return(restyClient)
	httpmock.Reset()

	err := adapter.DeleteClientScope(context.Background(), "realm1", "scope1")
	if err == nil {
		t.Fatal("no error returned")
	}

	if !strings.Contains(err.Error(), "unable to unset default client scope for realm") {
		t.Fatalf("wrong error returned: %s", err.Error())
	}

	deleteDefaultClientScope := strings.ReplaceAll(deleteDefaultClientScope, "{realm}", "realm1")
	deleteDefaultClientScope = strings.ReplaceAll(deleteDefaultClientScope, "{clientScopeID}", "scope1")

	httpmock.RegisterResponder("DELETE", deleteDefaultClientScope, httpmock.NewStringResponder(200, ""))
	mockClient.On("DeleteClientScope", "realm1", "scope1").Return(errors.New("mock fatal"))

	err = adapter.DeleteClientScope(context.Background(), "realm1", "scope1")
	if err == nil {
		t.Fatal("no error returned")
	}

	if !strings.Contains(err.Error(), "unable to delete client scope") {
		t.Fatalf("wrong error returned: %s", err.Error())
	}
}
