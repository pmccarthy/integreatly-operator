package user

import (
	"context"
	"fmt"
	configv1 "github.com/openshift/api/config/v1"
	"testing"

	keycloak "github.com/keycloak/keycloak-operator/pkg/apis/keycloak/v1alpha1"
	userv1 "github.com/openshift/api/user/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testIdentity = "test-identity"
	testEmail    = "test@email.com"
)

func TestGetUserEmailFromIdentity(t *testing.T) {

	scheme := runtime.NewScheme()
	err := userv1.AddToScheme(scheme)

	if err != nil {
		t.Fatalf("Error creating build scheme")
	}

	tests := []struct {
		Name          string
		FakeClient    k8sclient.Client
		User          userv1.User
		ExpectedEmail string
		ExpectedError bool
	}{
		{
			Name: "Test get email from identity",
			FakeClient: fake.NewFakeClientWithScheme(scheme, &userv1.Identity{
				ObjectMeta: v1.ObjectMeta{
					Name: testIdentity,
				},
				Extra: map[string]string{"email": testEmail},
			}),
			User: userv1.User{
				Identities: []string{testIdentity},
			},
			ExpectedEmail: testEmail,
			ExpectedError: false,
		},
		{
			Name:       "Test error getting identity",
			FakeClient: fake.NewFakeClientWithScheme(scheme),
			User: userv1.User{
				Identities: []string{testIdentity},
			},
			ExpectedEmail: "",
			ExpectedError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := GetUserEmailFromIdentity(context.TODO(), tt.FakeClient, tt.User)
			if (err != nil) != tt.ExpectedError {
				t.Errorf("GetUserEmailFromIdentity() error = %v, ExpectedErr %v", err, tt.ExpectedError)
				return
			}
			if got != tt.ExpectedEmail {
				t.Errorf("GetUserEmailFromIdentity() got = %v, want %v", got, tt.ExpectedEmail)
			}
		})
	}
}

func TestGetUsersInActiveIDPs(t *testing.T) {

	scheme := runtime.NewScheme()
	err := userv1.AddToScheme(scheme)
	err = configv1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("Error creating build scheme")
	}

	tests := []struct {
		Name          string
		FakeClient    k8sclient.Client
		ExpectedUsers *userv1.UserList
		ExpectError   bool
	}{
		{
			Name: "Test get email from identity",
			FakeClient: fake.NewFakeClientWithScheme(scheme, &userv1.Identity{
				ObjectMeta: v1.ObjectMeta{
					Name: "active-idp",
				},
				ProviderName: "exists",
			},
				&userv1.User{
					ObjectMeta: v1.ObjectMeta{
						Name: "exists",
					},
					Identities: []string{"active-idp"},
				},
				&userv1.Identity{
					ObjectMeta: v1.ObjectMeta{
						Name: "inactive-idp",
					},
					ProviderName: "non-existant",
				},
				&userv1.User{
					ObjectMeta: v1.ObjectMeta{
						Name: "non-existant",
					},
					Identities: []string{"inactive-idp"},
				},
				&configv1.OAuth{
					ObjectMeta: v1.ObjectMeta{
						Name: "cluster",
					},
					Spec: configv1.OAuthSpec{
						IdentityProviders: []configv1.IdentityProvider{
							{Name: "exists"},
						},
					},
				}),
			ExpectedUsers: &userv1.UserList{
				TypeMeta: v1.TypeMeta{},
				ListMeta: v1.ListMeta{},
				Items: []userv1.User{
					userv1.User{
						ObjectMeta: v1.ObjectMeta{
							Name: "exists",
						},
						Identities: []string{"active-idp"},
					},
				},
			},
			ExpectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := GetUsersInActiveIDPs(context.TODO(), tt.FakeClient)
			if (err != nil) != tt.ExpectError {
				t.Errorf("GetUsersInActiveIDPs() error = %v, ExpectedErr %v", err, tt.ExpectError)
				return
			}
			if len(tt.ExpectedUsers.Items) != len(got.Items) {
				t.Errorf("unexpected amount of found users, got %v expected %v", len(got.Items), len(tt.ExpectedUsers.Items))
			}
			for _, expectedUser := range tt.ExpectedUsers.Items {
				if !contains(got.Items, expectedUser) {
					t.Errorf("expected user: %v not found", expectedUser)
				}
			}
		})
	}
}

func contains(s []userv1.User, e userv1.User) bool {
	for _, a := range s {
		if a.Name == e.Name {
			return true
		}
	}
	return false
}

func TestAppendUpdateProfileActionForUserWithoutEmail(t *testing.T) {

	tests := []struct {
		Name                string
		KeyCloakUser        keycloak.KeycloakAPIUser
		AddedRequiredAction bool
	}{
		{
			Name: "Test Update Profile action is added for user with empty email",
			KeyCloakUser: keycloak.KeycloakAPIUser{
				Email:           "",
				RequiredActions: []string{},
			},
			AddedRequiredAction: true,
		},
		{
			Name: "Test Update Profile action is not added for user with email",
			KeyCloakUser: keycloak.KeycloakAPIUser{
				Email:           testEmail,
				RequiredActions: []string{},
			},
			AddedRequiredAction: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			AppendUpdateProfileActionForUserWithoutEmail(&tt.KeyCloakUser)
			if tt.AddedRequiredAction && len(tt.KeyCloakUser.RequiredActions) != 1 {
				t.Fatal("Expected user to be updated with required action but wasn't")
			}

			if !tt.AddedRequiredAction && len(tt.KeyCloakUser.RequiredActions) != 0 {
				t.Fatal("Expected user to not be updated with required action but was")
			}
		})
	}
}

func TestGetValidGeneratedUserName(t *testing.T) {

	tests := []struct {
		Name                  string
		KeyCloakUser          keycloak.KeycloakAPIUser
		ExpectedGeneratedName string
	}{
		{
			Name: "Test - Username is lower cased",
			KeyCloakUser: keycloak.KeycloakAPIUser{
				UserName: "TEST",
			},
			ExpectedGeneratedName: fmt.Sprintf("%s%s", GeneratedNamePrefix, "test"),
		},
		{
			Name: "Test - Username is lower cased and invalid characters replaced",
			KeyCloakUser: keycloak.KeycloakAPIUser{
				UserName: "TEST_USER@Example.com",
			},
			ExpectedGeneratedName: fmt.Sprintf("%s%s%s%s%s%s%s%s", GeneratedNamePrefix, "test", invalidCharacterReplacement, "user", invalidCharacterReplacement, "example", invalidCharacterReplacement, "com"),
		},
		{
			Name: "Test - Username replacement character is not added to the end of generated name",
			KeyCloakUser: keycloak.KeycloakAPIUser{
				UserName: "Tester01#",
			},
			ExpectedGeneratedName: fmt.Sprintf("%s%s", GeneratedNamePrefix, "tester01"),
		},
		{
			Name: "Test - UserId is added to generated name",
			KeyCloakUser: keycloak.KeycloakAPIUser{
				UserName: "Tester.01#",
				FederatedIdentities: []keycloak.FederatedIdentity{
					{
						UserID: "54d19771-aab6-49bb-913f-ce94e0ae5600",
					},
				},
			},
			ExpectedGeneratedName: fmt.Sprintf("%s%s%s%s", GeneratedNamePrefix, "tester", invalidCharacterReplacement, "01-54d19771-aab6-49bb-913f-ce94e0ae5600"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if got := GetValidGeneratedUserName(tt.KeyCloakUser); got != tt.ExpectedGeneratedName {
				t.Errorf("GetValidGeneratedUserName() = %v, want %v", got, tt.ExpectedGeneratedName)
			}
		})
	}
}
