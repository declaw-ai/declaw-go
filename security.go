package declaw

// SecurityPolicy is the top-level security configuration for a Declaw sandbox.
// It composes PII detection, injection defense, traffic transformations,
// network policy, audit logging, and environment variable security.
type SecurityPolicy struct {
	PII              *PIIConfig
	InjectionDefense *InjectionDefenseConfig
	Transformations  []TransformationRule
	Network          *NetworkPolicy
	Audit            *AuditConfig
	EnvSecurity      *EnvSecurityConfig
	Toxicity         *ToxicityConfig
	CodeSecurity     *CodeSecurityConfig
	InvisibleText    *InvisibleTextConfig
}

// PIIType identifies a category of personally identifiable information.
type PIIType string

const (
	PIIEmail      PIIType = "email"
	PIIPhone      PIIType = "phone"
	PIISSN        PIIType = "ssn"
	PIICreditCard PIIType = "credit_card"
	PIIPersonName PIIType = "person_name"
	PIIAPIKey     PIIType = "api_key"
	PIIAddress    PIIType = "address"
	PIIIPAddress  PIIType = "ip_address"
)

// RedactionAction specifies what to do when PII is detected.
type RedactionAction string

const (
	// RedactionActionRedact replaces detected PII with a redacted placeholder.
	RedactionActionRedact RedactionAction = "redact"

	// RedactionActionBlock blocks the entire request containing PII.
	RedactionActionBlock RedactionAction = "block"

	// RedactionActionLogOnly logs the detection without modifying the content.
	RedactionActionLogOnly RedactionAction = "log_only"
)

// PIIConfig configures PII (personally identifiable information) detection and redaction.
type PIIConfig struct {
	Enabled bool
	Types   []PIIType
	Action  RedactionAction
	Model   string
}

// InjectionSensitivity controls how aggressively prompt injection is detected.
type InjectionSensitivity string

const (
	InjectionSensitivityLow    InjectionSensitivity = "low"
	InjectionSensitivityMedium InjectionSensitivity = "medium"
	InjectionSensitivityHigh   InjectionSensitivity = "high"
)

// InjectionAction specifies what to do when prompt injection is detected.
type InjectionAction string

const (
	// InjectionActionBlock blocks the request containing the detected injection.
	InjectionActionBlock InjectionAction = "block"

	// InjectionActionLogOnly logs the detection without blocking.
	InjectionActionLogOnly InjectionAction = "log_only"
)

// InjectionDefenseConfig configures prompt injection detection.
type InjectionDefenseConfig struct {
	Enabled     bool
	Sensitivity InjectionSensitivity
	Action      InjectionAction
}

// TransformDirection specifies which direction a transformation applies to.
type TransformDirection string

const (
	// TransformIn applies the transformation to inbound traffic only.
	TransformIn TransformDirection = "in"

	// TransformOut applies the transformation to outbound traffic only.
	TransformOut TransformDirection = "out"

	// TransformBoth applies the transformation to both inbound and outbound traffic.
	TransformBoth TransformDirection = "both"
)

// TransformationRule defines a find-and-replace transformation on network traffic.
type TransformationRule struct {
	Match     string
	Replace   string
	Direction TransformDirection
}

// NetworkPolicy controls allowed and denied outbound network destinations.
type NetworkPolicy struct {
	AllowOut []string
	DenyOut  []string
}

// AuditConfig configures audit logging for sandbox activity.
type AuditConfig struct {
	Enabled             bool
	RedactSensitiveData bool
}

// EnvSecurityConfig configures environment variable security.
type EnvSecurityConfig struct {
	// MaskPatterns are glob patterns for variable names to mask in logs.
	MaskPatterns []string

	// SensitiveVars are environment variables injected securely.
	SensitiveVars []SecureEnvVar
}

// SecureEnvVar is an environment variable that is handled securely
// (masked in logs and audit trails).
type SecureEnvVar struct {
	Name  string
	Value string
}

// ToxicityConfig configures toxicity detection in sandbox I/O.
type ToxicityConfig struct {
	Enabled   bool
	Threshold float64
}

// CodeSecurityConfig configures code security scanning.
type CodeSecurityConfig struct {
	Enabled                 bool
	DetectSuspiciousImports bool
}

// InvisibleTextConfig configures detection of invisible/zero-width characters.
type InvisibleTextConfig struct {
	Enabled         bool
	DetectZeroWidth bool
}

// RequiresTLSInterception returns true if any active scanner in the policy
// requires TLS interception to inspect traffic content. Scanners that require
// interception include PII detection, injection defense, transformations,
// toxicity, code security, and invisible text detection.
func (sp *SecurityPolicy) RequiresTLSInterception() bool {
	if sp == nil {
		return false
	}
	if sp.PII != nil && sp.PII.Enabled {
		return true
	}
	if sp.InjectionDefense != nil && sp.InjectionDefense.Enabled {
		return true
	}
	if len(sp.Transformations) > 0 {
		return true
	}
	if sp.Toxicity != nil && sp.Toxicity.Enabled {
		return true
	}
	if sp.CodeSecurity != nil && sp.CodeSecurity.Enabled {
		return true
	}
	if sp.InvisibleText != nil && sp.InvisibleText.Enabled {
		return true
	}
	return false
}

// ToJSON serializes the security policy to a map with snake_case keys
// suitable for sending to the Declaw API.
func (sp *SecurityPolicy) ToJSON() map[string]interface{} {
	if sp == nil {
		return nil
	}
	m := make(map[string]interface{})

	if sp.PII != nil {
		pii := map[string]interface{}{
			"enabled": sp.PII.Enabled,
		}
		if len(sp.PII.Types) > 0 {
			types := make([]interface{}, len(sp.PII.Types))
			for i, t := range sp.PII.Types {
				types[i] = string(t)
			}
			pii["types"] = types
		}
		if sp.PII.Action != "" {
			pii["action"] = string(sp.PII.Action)
		}
		if sp.PII.Model != "" {
			pii["model"] = sp.PII.Model
		}
		m["pii"] = pii
	}

	if sp.InjectionDefense != nil {
		inj := map[string]interface{}{
			"enabled": sp.InjectionDefense.Enabled,
		}
		if sp.InjectionDefense.Sensitivity != "" {
			inj["sensitivity"] = string(sp.InjectionDefense.Sensitivity)
		}
		if sp.InjectionDefense.Action != "" {
			inj["action"] = string(sp.InjectionDefense.Action)
		}
		m["injection_defense"] = inj
	}

	if len(sp.Transformations) > 0 {
		transforms := make([]interface{}, len(sp.Transformations))
		for i, t := range sp.Transformations {
			transforms[i] = map[string]interface{}{
				"match":     t.Match,
				"replace":   t.Replace,
				"direction": string(t.Direction),
			}
		}
		m["transformations"] = transforms
	}

	if sp.Network != nil {
		net := map[string]interface{}{}
		if len(sp.Network.AllowOut) > 0 {
			net["allow_out"] = sp.Network.AllowOut
		}
		if len(sp.Network.DenyOut) > 0 {
			net["deny_out"] = sp.Network.DenyOut
		}
		m["network"] = net
	}

	if sp.Audit != nil {
		m["audit"] = map[string]interface{}{
			"enabled":               sp.Audit.Enabled,
			"redact_sensitive_data": sp.Audit.RedactSensitiveData,
		}
	}

	if sp.EnvSecurity != nil {
		env := map[string]interface{}{}
		if len(sp.EnvSecurity.MaskPatterns) > 0 {
			env["mask_patterns"] = sp.EnvSecurity.MaskPatterns
		}
		if len(sp.EnvSecurity.SensitiveVars) > 0 {
			vars := make([]interface{}, len(sp.EnvSecurity.SensitiveVars))
			for i, v := range sp.EnvSecurity.SensitiveVars {
				vars[i] = map[string]interface{}{
					"name":  v.Name,
					"value": v.Value,
				}
			}
			env["sensitive_vars"] = vars
		}
		m["env_security"] = env
	}

	if sp.Toxicity != nil {
		m["toxicity"] = map[string]interface{}{
			"enabled":   sp.Toxicity.Enabled,
			"threshold": sp.Toxicity.Threshold,
		}
	}

	if sp.CodeSecurity != nil {
		m["code_security"] = map[string]interface{}{
			"enabled":                    sp.CodeSecurity.Enabled,
			"detect_suspicious_imports": sp.CodeSecurity.DetectSuspiciousImports,
		}
	}

	if sp.InvisibleText != nil {
		m["invisible_text"] = map[string]interface{}{
			"enabled":            sp.InvisibleText.Enabled,
			"detect_zero_width": sp.InvisibleText.DetectZeroWidth,
		}
	}

	return m
}

// ParseSecurityPolicy deserializes a SecurityPolicy from a map returned by the API.
func ParseSecurityPolicy(data map[string]interface{}) *SecurityPolicy {
	if data == nil {
		return &SecurityPolicy{}
	}

	sp := &SecurityPolicy{}

	if piiRaw, ok := data["pii"]; ok && piiRaw != nil {
		if piiMap, ok := piiRaw.(map[string]interface{}); ok {
			pii := &PIIConfig{}
			if v, ok := piiMap["enabled"].(bool); ok {
				pii.Enabled = v
			}
			if v, ok := piiMap["action"].(string); ok {
				pii.Action = RedactionAction(v)
			}
			if v, ok := piiMap["model"].(string); ok {
				pii.Model = v
			}
			if typesRaw, ok := piiMap["types"].([]interface{}); ok {
				pii.Types = make([]PIIType, len(typesRaw))
				for i, t := range typesRaw {
					if s, ok := t.(string); ok {
						pii.Types[i] = PIIType(s)
					}
				}
			}
			// Handle []string from ToJSON round-trip
			if typesRaw, ok := piiMap["types"].([]string); ok {
				pii.Types = make([]PIIType, len(typesRaw))
				for i, t := range typesRaw {
					pii.Types[i] = PIIType(t)
				}
			}
			sp.PII = pii
		}
	}

	if idRaw, ok := data["injection_defense"]; ok && idRaw != nil {
		if idMap, ok := idRaw.(map[string]interface{}); ok {
			id := &InjectionDefenseConfig{}
			if v, ok := idMap["enabled"].(bool); ok {
				id.Enabled = v
			}
			if v, ok := idMap["sensitivity"].(string); ok {
				id.Sensitivity = InjectionSensitivity(v)
			}
			if v, ok := idMap["action"].(string); ok {
				id.Action = InjectionAction(v)
			}
			sp.InjectionDefense = id
		}
	}

	if tRaw, ok := data["transformations"]; ok && tRaw != nil {
		switch tSlice := tRaw.(type) {
		case []interface{}:
			sp.Transformations = make([]TransformationRule, len(tSlice))
			for i, item := range tSlice {
				if m, ok := item.(map[string]interface{}); ok {
					rule := TransformationRule{}
					if v, ok := m["match"].(string); ok {
						rule.Match = v
					}
					if v, ok := m["replace"].(string); ok {
						rule.Replace = v
					}
					if v, ok := m["direction"].(string); ok {
						rule.Direction = TransformDirection(v)
					}
					sp.Transformations[i] = rule
				}
			}
		case []map[string]interface{}:
			sp.Transformations = make([]TransformationRule, len(tSlice))
			for i, m := range tSlice {
				rule := TransformationRule{}
				if v, ok := m["match"].(string); ok {
					rule.Match = v
				}
				if v, ok := m["replace"].(string); ok {
					rule.Replace = v
				}
				if v, ok := m["direction"].(string); ok {
					rule.Direction = TransformDirection(v)
				}
				sp.Transformations[i] = rule
			}
		}
	}

	if nRaw, ok := data["network"]; ok && nRaw != nil {
		if nMap, ok := nRaw.(map[string]interface{}); ok {
			net := &NetworkPolicy{}
			if aRaw, ok := nMap["allow_out"]; ok {
				net.AllowOut = parseStringSlice(aRaw)
			}
			if dRaw, ok := nMap["deny_out"]; ok {
				net.DenyOut = parseStringSlice(dRaw)
			}
			sp.Network = net
		}
	}

	if aRaw, ok := data["audit"]; ok && aRaw != nil {
		if aMap, ok := aRaw.(map[string]interface{}); ok {
			audit := &AuditConfig{}
			if v, ok := aMap["enabled"].(bool); ok {
				audit.Enabled = v
			}
			if v, ok := aMap["redact_sensitive_data"].(bool); ok {
				audit.RedactSensitiveData = v
			}
			sp.Audit = audit
		}
	}

	if eRaw, ok := data["env_security"]; ok && eRaw != nil {
		if eMap, ok := eRaw.(map[string]interface{}); ok {
			env := &EnvSecurityConfig{}
			if pRaw, ok := eMap["mask_patterns"]; ok {
				env.MaskPatterns = parseStringSlice(pRaw)
			}
			if vRaw, ok := eMap["sensitive_vars"]; ok && vRaw != nil {
				switch svSlice := vRaw.(type) {
				case []interface{}:
					env.SensitiveVars = make([]SecureEnvVar, len(svSlice))
					for i, item := range svSlice {
						if m, ok := item.(map[string]interface{}); ok {
							sv := SecureEnvVar{}
							if v, ok := m["name"].(string); ok {
								sv.Name = v
							}
							if v, ok := m["value"].(string); ok {
								sv.Value = v
							}
							env.SensitiveVars[i] = sv
						}
					}
				case []map[string]interface{}:
					env.SensitiveVars = make([]SecureEnvVar, len(svSlice))
					for i, m := range svSlice {
						sv := SecureEnvVar{}
						if v, ok := m["name"].(string); ok {
							sv.Name = v
						}
						if v, ok := m["value"].(string); ok {
							sv.Value = v
						}
						env.SensitiveVars[i] = sv
					}
				}
			}
			sp.EnvSecurity = env
		}
	}

	if tRaw, ok := data["toxicity"]; ok && tRaw != nil {
		if tMap, ok := tRaw.(map[string]interface{}); ok {
			tox := &ToxicityConfig{}
			if v, ok := tMap["enabled"].(bool); ok {
				tox.Enabled = v
			}
			if v, ok := tMap["threshold"].(float64); ok {
				tox.Threshold = v
			}
			sp.Toxicity = tox
		}
	}

	if cRaw, ok := data["code_security"]; ok && cRaw != nil {
		if cMap, ok := cRaw.(map[string]interface{}); ok {
			cs := &CodeSecurityConfig{}
			if v, ok := cMap["enabled"].(bool); ok {
				cs.Enabled = v
			}
			if v, ok := cMap["detect_suspicious_imports"].(bool); ok {
				cs.DetectSuspiciousImports = v
			}
			sp.CodeSecurity = cs
		}
	}

	if iRaw, ok := data["invisible_text"]; ok && iRaw != nil {
		if iMap, ok := iRaw.(map[string]interface{}); ok {
			it := &InvisibleTextConfig{}
			if v, ok := iMap["enabled"].(bool); ok {
				it.Enabled = v
			}
			if v, ok := iMap["detect_zero_width"].(bool); ok {
				it.DetectZeroWidth = v
			}
			sp.InvisibleText = it
		}
	}

	return sp
}

// parseStringSlice converts an interface{} that may be []interface{} or []string
// into a []string.
func parseStringSlice(v interface{}) []string {
	switch s := v.(type) {
	case []interface{}:
		result := make([]string, len(s))
		for i, item := range s {
			if str, ok := item.(string); ok {
				result[i] = str
			}
		}
		return result
	case []string:
		return s
	}
	return nil
}
