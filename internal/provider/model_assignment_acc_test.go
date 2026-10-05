package provider

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccModelAssignment(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests require TF_ACC=1")
	}

	endpoint := strings.TrimSpace(os.Getenv("HERMES_ACC_ENDPOINT"))
	username := os.Getenv("HERMES_ACC_USERNAME")
	password := os.Getenv("HERMES_ACC_PASSWORD")
	profile := strings.TrimSpace(os.Getenv("HERMES_ACC_PROFILE"))
	baseURL := strings.TrimSpace(os.Getenv("HERMES_ACC_BASE_URL"))
	model := strings.TrimSpace(os.Getenv("HERMES_ACC_MODEL"))
	if endpoint == "" || username == "" || password == "" || profile == "" || baseURL == "" || model == "" {
		t.Fatal("HERMES_ACC_ENDPOINT, HERMES_ACC_USERNAME, HERMES_ACC_PASSWORD, HERMES_ACC_PROFILE, HERMES_ACC_BASE_URL, and HERMES_ACC_MODEL are required")
	}

	// The provider intentionally reads its machine credentials from the same
	// environment variables used in production. The acceptance-specific names
	// keep operators from accidentally pointing tests at an existing credential.
	t.Setenv(envEndpoint, endpoint)
	t.Setenv(envUsername, username)
	t.Setenv(envPassword, password)

	config := testAccModelAssignmentConfig(profile, model, baseURL)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hermes": providerserver.NewProtocol6WithError(New("acceptance")()),
		},
		Steps: []resource.TestStep{
			{
				ResourceName: "hermes_model_assignment.llama",
				Config:       config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hermes_model_assignment.llama", "id", profile+":main"),
					resource.TestCheckResourceAttr("hermes_model_assignment.llama", "scope", "main"),
					resource.TestCheckResourceAttr("hermes_model_assignment.llama", "model_provider", "custom"),
					resource.TestCheckResourceAttr("hermes_model_assignment.llama", "model", model),
				),
			},
			{
				ResourceName: "hermes_model_assignment.llama",
				RefreshState: true,
			},
			{
				ResourceName:      "hermes_model_assignment.llama",
				ImportState:       true,
				ImportStateId:     profile + ":main",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccModelAssignmentConfig(profile, model, baseURL string) string {
	return fmt.Sprintf(`provider "hermes" {}

resource "hermes_model_assignment" "llama" {
  scope          = "main"
  profile        = %s
  model_provider = "custom"
  model          = %s
  base_url       = %s
}
`, strconv.Quote(profile), strconv.Quote(model), strconv.Quote(baseURL))
}
