// Package validators provides centralized validation functions for resource names
// across the AgentRegistry CLI and services.
package validators

import (
	"fmt"
	"strings"

	"github.com/agentregistry-dev/agentregistry/pkg/api/v1alpha1"
)

// Python keywords that cannot be used as agent names — agent names become
// Python identifiers in generated code, so the CLI layer rejects them in
// addition to the DNS-1123 form.
var pythonKeywords = map[string]struct{}{
	"False": {}, "None": {}, "True": {}, "and": {}, "as": {}, "assert": {},
	"async": {}, "await": {}, "break": {}, "class": {}, "continue": {}, "def": {},
	"del": {}, "elif": {}, "else": {}, "except": {}, "finally": {}, "for": {},
	"from": {}, "global": {}, "if": {}, "import": {}, "in": {}, "is": {},
	"lambda": {}, "nonlocal": {}, "not": {}, "or": {}, "pass": {}, "raise": {},
	"return": {}, "try": {}, "while": {}, "with": {}, "yield": {},
}

// ValidateProjectName checks if the provided project name is valid for use as a directory name.
// This is a permissive check for filesystem safety, not a resource-name check.
func ValidateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if strings.ContainsAny(name, " \t\n\r/\\:*?\"<>|") {
		return fmt.Errorf("project name contains invalid characters")
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("project name cannot start with a dot")
	}
	return nil
}

// validateName applies the v1alpha1 DNS-1123 subdomain rule with a
// kind-aware error message so CLI users see "skill name must be..." rather
// than the generic backend error.
func validateName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("%s name cannot be empty", kind)
	}
	if len(name) > v1alpha1.DNSSubdomainMaxLen {
		return fmt.Errorf("%s name %q is too long (max %d chars, got %d)", kind, name, v1alpha1.DNSSubdomainMaxLen, len(name))
	}
	if !v1alpha1.DNSSubdomainRegex.MatchString(name) {
		return fmt.Errorf("%s name %q must be DNS-1123 subdomain: lowercase alphanumeric, hyphens, and dots; start/end with alphanumeric; each dot-separated segment 1-63 chars", kind, name)
	}
	return nil
}

// ValidateAgentName checks if the agent name is valid. DNS-1123 subdomain,
// must start with a lowercase letter (agent names back Python package
// directories in generated code), and the [-.]-to-_ form must not be a
// Python keyword.
func ValidateAgentName(name string) error {
	if err := validateName("agent", name); err != nil {
		return err
	}
	// https://docs.python.org/3/reference/lexical_analysis.html#identifiers
	// name_start: "a"..."z" | "A"..."Z" | "_" | <non-ASCII character>
	if name[0] < 'a' || name[0] > 'z' {
		return fmt.Errorf("agent name %q must start with a lowercase letter", name)
	}
	sanitized := strings.NewReplacer("-", "_", ".", "_").Replace(name)
	if _, isKeyword := pythonKeywords[sanitized]; isKeyword {
		return fmt.Errorf("agent name %q is a Python keyword and cannot be used", name)
	}
	return nil
}

// ValidateSkillName enforces DNS-1123 subdomain form.
func ValidateSkillName(name string) error {
	return validateName("skill", name)
}

// ValidatePromptName enforces DNS-1123 subdomain form.
func ValidatePromptName(name string) error {
	return validateName("prompt", name)
}

// ValidateMCPServerName enforces DNS-1123 subdomain form.
func ValidateMCPServerName(name string) error {
	return validateName("MCP server", name)
}
