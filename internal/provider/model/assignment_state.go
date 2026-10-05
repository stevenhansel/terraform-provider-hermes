package model

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

func validateModel(model *modelAssignmentModel) error {
	if model.Scope.IsUnknown() {
		return nil
	}
	scope := strings.ToLower(strings.TrimSpace(optionalString(model.Scope)))
	if scope == "" {
		scope = "main"
	}
	if scope != "main" && scope != "auxiliary" {
		return fmt.Errorf("scope must be main or auxiliary")
	}

	if model.Task.IsUnknown() {
		return nil
	}
	task := strings.TrimSpace(optionalString(model.Task))
	if scope == "auxiliary" && task == "" {
		return fmt.Errorf("task is required for an auxiliary assignment")
	}
	if scope == "main" && task != "" {
		return fmt.Errorf("task must be omitted for a main assignment")
	}

	if model.ModelProvider.IsUnknown() || model.Model.IsUnknown() {
		return nil
	}
	provider := optionalString(model.ModelProvider)
	modelName := optionalString(model.Model)
	if strings.TrimSpace(provider) == "" || strings.TrimSpace(modelName) == "" {
		return fmt.Errorf("model_provider and model are required")
	}

	model.Scope = types.StringValue(scope)
	if task == "" {
		model.Task = types.StringNull()
	} else {
		model.Task = types.StringValue(task)
	}
	return nil
}

func assignmentFromModel(model modelAssignmentModel) hermes.ModelAssignment {
	return hermes.ModelAssignment{
		Scope:                 optionalString(model.Scope),
		Provider:              optionalString(model.ModelProvider),
		Model:                 optionalString(model.Model),
		Task:                  optionalString(model.Task),
		BaseURL:               optionalString(model.BaseURL),
		APIKey:                optionalString(model.APIKey),
		ConfirmExpensiveModel: boolValue(model.ConfirmExpensiveModel),
		Profile:               optionalString(model.Profile),
	}
}

func setModelIdentity(model *modelAssignmentModel) {
	scope := optionalString(model.Scope)
	if scope == "" {
		scope = "main"
		model.Scope = types.StringValue(scope)
	}
	id := scope
	if task := optionalString(model.Task); task != "" {
		id += ":" + task
	}
	if profile := optionalString(model.Profile); profile != "" {
		id = profile + ":" + id
	}
	model.ID = types.StringValue(id)
}

func parseModelID(id string) (scope, task, profile string, err error) {
	parts := strings.Split(id, ":")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "", "", "", fmt.Errorf("ID must identify a main or auxiliary model assignment")
	}

	switch len(parts) {
	case 1:
		if parts[0] != "main" {
			return "", "", "", fmt.Errorf("main assignment ID must be main")
		}
		return "main", "", "", nil
	case 2:
		if parts[0] == "auxiliary" {
			if strings.TrimSpace(parts[1]) == "" {
				return "", "", "", fmt.Errorf("auxiliary assignment ID must include a task")
			}
			return "auxiliary", parts[1], "", nil
		}
		if parts[1] != "main" || strings.TrimSpace(parts[0]) == "" {
			return "", "", "", fmt.Errorf("profile-scoped main assignment ID must be <profile>:main")
		}
		return "main", "", parts[0], nil
	case 3:
		if parts[1] != "auxiliary" || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[2]) == "" {
			return "", "", "", fmt.Errorf("profile-scoped auxiliary assignment ID must be <profile>:auxiliary:<task>")
		}
		return "auxiliary", parts[2], parts[0], nil
	default:
		return "", "", "", fmt.Errorf("unsupported model assignment ID format")
	}
}

func optionalString(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return value.ValueString()
}

func boolValue(value types.Bool) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueBool()
}

func stringValueOrNull(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}
