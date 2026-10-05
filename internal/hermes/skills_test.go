package hermes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestClientManagesSkillsAndHubActions(t *testing.T) {
	var actionStatusCalls int
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/auth/password-login" && request.URL.Query().Get("profile") != "assistant" && request.URL.Path != "/api/actions/skills-install-morning-brief-a1b2c3d4/status" {
			t.Errorf("profile query = %q for %s, want assistant", request.URL.Query().Get("profile"), request.URL.Path)
		}
		switch {
		case request.URL.Path == "/auth/password-login":
			response.WriteHeader(http.StatusOK)
		case request.Method == http.MethodGet && request.URL.Path == "/api/skills":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`[{
                "name":"morning-brief","description":"Morning summary","category":"productivity",
                "enabled":true,"usage":3,"provenance":"hub"
            }]`))
		case request.Method == http.MethodPut && request.URL.Path == "/api/skills/toggle":
			var body skillToggleRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode skill toggle: %v", err)
			}
			if body.Name != "morning-brief" || !body.Enabled || body.Profile != "assistant" {
				t.Errorf("skill toggle = %#v, want enabled assistant skill", body)
			}
			response.WriteHeader(http.StatusOK)
		case request.Method == http.MethodGet && request.URL.Path == "/api/skills/hub/sources":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{
                "sources":[{"id":"hermes-index","label":"Hermes Index","available":true,"searchable":true}],
                "index_available":true,
                "featured":[],
                "installed":{"official/productivity/morning-brief":{"identifier":"official/productivity/morning-brief","name":"morning-brief","trust_level":"official","scan_verdict":"safe"}}
            }`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/skills/hub/install":
			var body skillInstallRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode skill install: %v", err)
			}
			if body.Identifier != "official/productivity/morning-brief" || body.Profile != "assistant" {
				t.Errorf("skill install = %#v, want assistant hub identifier", body)
			}
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"ok":true,"pid":42,"name":"skills-install-morning-brief-a1b2c3d4"}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/skills/hub/uninstall":
			var body skillUninstallRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode skill uninstall: %v", err)
			}
			if body.Name != "morning-brief" || body.Profile != "assistant" {
				t.Errorf("skill uninstall = %#v, want assistant skill", body)
			}
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"ok":true,"pid":43,"name":"skills-uninstall-morning-brief-a1b2c3d4"}`))
		case request.Method == http.MethodGet && request.URL.Path == "/api/actions/skills-install-morning-brief-a1b2c3d4/status":
			if request.URL.Query().Get("lines") != "1" {
				t.Errorf("action lines query = %q, want 1", request.URL.Query().Get("lines"))
			}
			actionStatusCalls++
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"name":"skills-install-morning-brief-a1b2c3d4","running":false,"exit_code":0,"pid":42,"lines":["secret must not be retained"]}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "iac", "secret")
	if err != nil {
		t.Fatal(err)
	}

	skills, err := client.ListSkills(context.Background(), "assistant")
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 1 || skills[0].Name != "morning-brief" || skills[0].Usage != 3 || skills[0].Provenance != "hub" {
		t.Fatalf("skills = %#v, want decoded skill", skills)
	}
	if err := client.SetSkillEnabled(context.Background(), "assistant", "morning-brief", true); err != nil {
		t.Fatalf("SetSkillEnabled: %v", err)
	}

	sources, err := client.ListSkillHubSources(context.Background(), "assistant")
	if err != nil {
		t.Fatalf("ListSkillHubSources: %v", err)
	}
	wantInstalled := SkillHubInstallation{Identifier: "official/productivity/morning-brief", Name: "morning-brief", TrustLevel: "official", ScanVerdict: "safe"}
	if !reflect.DeepEqual(sources.Installed["official/productivity/morning-brief"], wantInstalled) {
		t.Fatalf("installed = %#v, want %#v", sources.Installed, wantInstalled)
	}

	action, err := client.StartSkillInstall(context.Background(), "assistant", "official/productivity/morning-brief")
	if err != nil || action != "skills-install-morning-brief-a1b2c3d4" {
		t.Fatalf("StartSkillInstall = %q, %v", action, err)
	}
	if err := client.WaitForAction(context.Background(), action, 2*time.Second); err != nil {
		t.Fatalf("WaitForAction: %v", err)
	}
	if actionStatusCalls != 1 {
		t.Fatalf("action status calls = %d, want one immediate terminal poll", actionStatusCalls)
	}

	if _, err := client.StartSkillUninstall(context.Background(), "assistant", "morning-brief"); err != nil {
		t.Fatalf("StartSkillUninstall: %v", err)
	}
}

func TestWaitForActionDoesNotExposeLogLines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/auth/password-login" {
			response.WriteHeader(http.StatusOK)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"name":"failed","running":false,"exit_code":2,"lines":["token=do-not-leak"]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "iac", "secret")
	if err != nil {
		t.Fatal(err)
	}
	err = client.WaitForAction(context.Background(), "failed", 2*time.Second)
	if err == nil || err.Error() == "" {
		t.Fatal("WaitForAction succeeded, want failure")
	}
	if strings.Contains(err.Error(), "do-not-leak") {
		t.Fatalf("WaitForAction leaked action output: %v", err)
	}
}

func TestActionPathEscaping(t *testing.T) {
	if got := url.PathEscape("skills/install/foo"); got != "skills%2Finstall%2Ffoo" {
		t.Fatalf("PathEscape = %q", got)
	}
}
