package aigateway

import (
	"regexp"
	"strconv"
	"strings"

	"clawreef/internal/models"
)

// templatePlaceholderPattern matches `{{ NAME }}` placeholders.
var templatePlaceholderPattern = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// buildInstanceTemplateVars returns the supported placeholder map for an instance.
// Placeholders are case-insensitive on lookup; values are stringified.
// Available variables when the request comes from an OpenClaw instance:
//   INSTANCE_ID, INSTANCE_NAME, INSTANCE_TYPE, INSTANCE_STATUS, INSTANCE_USER_ID,
//   INSTANCE_POD_NAME, INSTANCE_POD_NAMESPACE, INSTANCE_POD_IP, INSTANCE_ACCESS_URL,
//   INSTANCE_ACCESS_TOKEN, INSTANCE_OS_TYPE, INSTANCE_OS_VERSION
func buildInstanceTemplateVars(instance *models.Instance) map[string]string {
	if instance == nil {
		return map[string]string{}
	}

	vars := map[string]string{
		"INSTANCE_ID":      strconv.Itoa(instance.ID),
		"INSTANCE_NAME":    instance.Name,
		"INSTANCE_TYPE":    instance.Type,
		"INSTANCE_STATUS":  instance.Status,
		"INSTANCE_USER_ID": strconv.Itoa(instance.UserID),
		"INSTANCE_OS_TYPE": instance.OSType,
	}
	if instance.OSVersion != "" {
		vars["INSTANCE_OS_VERSION"] = instance.OSVersion
	}
	if instance.PodName != nil {
		vars["INSTANCE_POD_NAME"] = *instance.PodName
	}
	if instance.PodNamespace != nil {
		vars["INSTANCE_POD_NAMESPACE"] = *instance.PodNamespace
	}
	if instance.PodIP != nil {
		vars["INSTANCE_POD_IP"] = *instance.PodIP
	}
	if instance.AccessURL != nil {
		vars["INSTANCE_ACCESS_URL"] = *instance.AccessURL
	}
	if instance.AccessToken != nil {
		vars["INSTANCE_ACCESS_TOKEN"] = *instance.AccessToken
	}
	return vars
}

// resolveTemplateString substitutes `{{VAR}}` placeholders in the input with values
// from vars. Unknown placeholders are left unchanged.
func resolveTemplateString(input string, vars map[string]string) string {
	if input == "" || len(vars) == 0 {
		return input
	}
	if !strings.Contains(input, "{{") {
		return input
	}
	return templatePlaceholderPattern.ReplaceAllStringFunc(input, func(match string) string {
		submatches := templatePlaceholderPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		name := strings.ToUpper(strings.TrimSpace(submatches[1]))
		if value, ok := vars[name]; ok {
			return value
		}
		return match
	})
}
