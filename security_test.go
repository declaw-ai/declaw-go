package declaw

import (
	"testing"
)

// --- PIIType constants ---

func TestPIIType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		piiType  PIIType
		expected string
	}{
		{"PIIEmail", PIIEmail, "email"},
		{"PIIPhone", PIIPhone, "phone"},
		{"PIISSN", PIISSN, "ssn"},
		{"PIICreditCard", PIICreditCard, "credit_card"},
		{"PIIPersonName", PIIPersonName, "person_name"},
		{"PIIAPIKey", PIIAPIKey, "api_key"},
		{"PIIAddress", PIIAddress, "address"},
		{"PIIIPAddress", PIIIPAddress, "ip_address"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.piiType) != tc.expected {
				t.Errorf("%s = %q, want %q", tc.name, tc.piiType, tc.expected)
			}
		})
	}
}

func TestPIIType_AllConstants_Count(t *testing.T) {
	// Verify we have all 8 PII types
	allTypes := []PIIType{
		PIIEmail, PIIPhone, PIISSN, PIICreditCard,
		PIIPersonName, PIIAPIKey, PIIAddress, PIIIPAddress,
	}
	if len(allTypes) != 8 {
		t.Errorf("expected 8 PII types, got %d", len(allTypes))
	}
}

// --- RedactionAction constants ---

func TestRedactionAction_Constants(t *testing.T) {
	tests := []struct {
		name     string
		action   RedactionAction
		expected string
	}{
		{"RedactionActionRedact", RedactionActionRedact, "redact"},
		{"RedactionActionBlock", RedactionActionBlock, "block"},
		{"RedactionActionLogOnly", RedactionActionLogOnly, "log_only"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.action) != tc.expected {
				t.Errorf("%s = %q, want %q", tc.name, tc.action, tc.expected)
			}
		})
	}
}

// --- PIIConfig ---

func TestPIIConfig_AllFields(t *testing.T) {
	cfg := PIIConfig{
		Enabled: true,
		Types:   []PIIType{PIIEmail, PIISSN, PIICreditCard},
		Action:  RedactionActionRedact,
		Model:   "presidio",
	}

	if !cfg.Enabled {
		t.Error("expected Enabled = true")
	}
	if len(cfg.Types) != 3 {
		t.Errorf("expected 3 types, got %d", len(cfg.Types))
	}
	if cfg.Types[0] != PIIEmail {
		t.Errorf("Types[0] = %q, want %q", cfg.Types[0], PIIEmail)
	}
	if cfg.Action != RedactionActionRedact {
		t.Errorf("Action = %q, want %q", cfg.Action, RedactionActionRedact)
	}
	if cfg.Model != "presidio" {
		t.Errorf("Model = %q, want %q", cfg.Model, "presidio")
	}
}

func TestPIIConfig_ZeroValue(t *testing.T) {
	var cfg PIIConfig

	if cfg.Enabled {
		t.Error("zero Enabled should be false")
	}
	if cfg.Types != nil {
		t.Error("zero Types should be nil")
	}
	if cfg.Action != "" {
		t.Errorf("zero Action = %q, want empty", cfg.Action)
	}
	if cfg.Model != "" {
		t.Errorf("zero Model = %q, want empty", cfg.Model)
	}
}

func TestPIIConfig_DisabledWithTypes(t *testing.T) {
	cfg := PIIConfig{
		Enabled: false,
		Types:   []PIIType{PIIEmail},
		Action:  RedactionActionBlock,
	}

	if cfg.Enabled {
		t.Error("should be disabled")
	}
	// Types and Action should still be set even if disabled
	if len(cfg.Types) != 1 {
		t.Errorf("Types length = %d, want 1", len(cfg.Types))
	}
}

// --- InjectionSensitivity constants ---

func TestInjectionSensitivity_Constants(t *testing.T) {
	tests := []struct {
		name        string
		sensitivity InjectionSensitivity
		expected    string
	}{
		{"Low", InjectionSensitivityLow, "low"},
		{"Medium", InjectionSensitivityMedium, "medium"},
		{"High", InjectionSensitivityHigh, "high"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.sensitivity) != tc.expected {
				t.Errorf("%s = %q, want %q", tc.name, tc.sensitivity, tc.expected)
			}
		})
	}
}

// --- InjectionAction constants ---

func TestInjectionAction_Constants(t *testing.T) {
	tests := []struct {
		name     string
		action   InjectionAction
		expected string
	}{
		{"Block", InjectionActionBlock, "block"},
		{"LogOnly", InjectionActionLogOnly, "log_only"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.action) != tc.expected {
				t.Errorf("%s = %q, want %q", tc.name, tc.action, tc.expected)
			}
		})
	}
}

// --- InjectionDefenseConfig ---

func TestInjectionDefenseConfig_AllFields(t *testing.T) {
	cfg := InjectionDefenseConfig{
		Enabled:     true,
		Sensitivity: InjectionSensitivityHigh,
		Action:      InjectionActionBlock,
	}

	if !cfg.Enabled {
		t.Error("expected Enabled = true")
	}
	if cfg.Sensitivity != InjectionSensitivityHigh {
		t.Errorf("Sensitivity = %q, want %q", cfg.Sensitivity, InjectionSensitivityHigh)
	}
	if cfg.Action != InjectionActionBlock {
		t.Errorf("Action = %q, want %q", cfg.Action, InjectionActionBlock)
	}
}

func TestInjectionDefenseConfig_ZeroValue(t *testing.T) {
	var cfg InjectionDefenseConfig

	if cfg.Enabled {
		t.Error("zero Enabled should be false")
	}
	if cfg.Sensitivity != "" {
		t.Errorf("zero Sensitivity = %q, want empty", cfg.Sensitivity)
	}
	if cfg.Action != "" {
		t.Errorf("zero Action = %q, want empty", cfg.Action)
	}
}

// --- TransformDirection constants ---

func TestTransformDirection_Constants(t *testing.T) {
	tests := []struct {
		name      string
		direction TransformDirection
		expected  string
	}{
		{"In", TransformIn, "in"},
		{"Out", TransformOut, "out"},
		{"Both", TransformBoth, "both"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.direction) != tc.expected {
				t.Errorf("%s = %q, want %q", tc.name, tc.direction, tc.expected)
			}
		})
	}
}

// --- TransformationRule ---

func TestTransformationRule_AllFields(t *testing.T) {
	rule := TransformationRule{
		Match:     `sk-\w+`,
		Replace:   "[REDACTED]",
		Direction: TransformOut,
	}

	if rule.Match != `sk-\w+` {
		t.Errorf("Match = %q, want expected", rule.Match)
	}
	if rule.Replace != "[REDACTED]" {
		t.Errorf("Replace = %q, want expected", rule.Replace)
	}
	if rule.Direction != TransformOut {
		t.Errorf("Direction = %q, want %q", rule.Direction, TransformOut)
	}
}

func TestTransformationRule_BothDirection(t *testing.T) {
	rule := TransformationRule{
		Match:     `Bearer \w+`,
		Replace:   "Bearer [TOKEN]",
		Direction: TransformBoth,
	}

	if rule.Direction != TransformBoth {
		t.Errorf("Direction = %q, want %q", rule.Direction, TransformBoth)
	}
}

func TestTransformationRule_ZeroValue(t *testing.T) {
	var rule TransformationRule

	if rule.Match != "" {
		t.Errorf("zero Match = %q, want empty", rule.Match)
	}
	if rule.Replace != "" {
		t.Errorf("zero Replace = %q, want empty", rule.Replace)
	}
	if rule.Direction != "" {
		t.Errorf("zero Direction = %q, want empty", rule.Direction)
	}
}

// --- NetworkPolicy ---

func TestNetworkPolicy_AllFields(t *testing.T) {
	np := NetworkPolicy{
		AllowOut: []string{"*.openai.com", "pypi.org"},
		DenyOut:  []string{"*"},
	}

	if len(np.AllowOut) != 2 {
		t.Errorf("AllowOut length = %d, want 2", len(np.AllowOut))
	}
	if np.AllowOut[0] != "*.openai.com" {
		t.Errorf("AllowOut[0] = %q, want expected", np.AllowOut[0])
	}
	if len(np.DenyOut) != 1 {
		t.Errorf("DenyOut length = %d, want 1", len(np.DenyOut))
	}
}

func TestNetworkPolicy_ZeroValue(t *testing.T) {
	var np NetworkPolicy

	if np.AllowOut != nil {
		t.Errorf("zero AllowOut should be nil")
	}
	if np.DenyOut != nil {
		t.Errorf("zero DenyOut should be nil")
	}
}

// --- AuditConfig ---

func TestAuditConfig_Enabled(t *testing.T) {
	cfg := AuditConfig{
		Enabled:             true,
		RedactSensitiveData: true,
	}

	if !cfg.Enabled {
		t.Error("expected Enabled = true")
	}
	if !cfg.RedactSensitiveData {
		t.Error("expected RedactSensitiveData = true")
	}
}

func TestAuditConfig_Disabled(t *testing.T) {
	cfg := AuditConfig{Enabled: false}

	if cfg.Enabled {
		t.Error("expected Enabled = false")
	}
}

func TestAuditConfig_ZeroValue(t *testing.T) {
	var cfg AuditConfig

	if cfg.Enabled {
		t.Error("zero Enabled should be false")
	}
	if cfg.RedactSensitiveData {
		t.Error("zero RedactSensitiveData should be false")
	}
}

// --- ToxicityConfig ---

func TestToxicityConfig_AllFields(t *testing.T) {
	cfg := ToxicityConfig{
		Enabled:   true,
		Threshold: 0.8,
	}

	if !cfg.Enabled {
		t.Error("expected Enabled = true")
	}
	if cfg.Threshold != 0.8 {
		t.Errorf("Threshold = %f, want 0.8", cfg.Threshold)
	}
}

func TestToxicityConfig_ZeroValue(t *testing.T) {
	var cfg ToxicityConfig

	if cfg.Enabled {
		t.Error("zero Enabled should be false")
	}
	if cfg.Threshold != 0 {
		t.Errorf("zero Threshold = %f, want 0", cfg.Threshold)
	}
}

// --- CodeSecurityConfig ---

func TestCodeSecurityConfig_AllFields(t *testing.T) {
	cfg := CodeSecurityConfig{
		Enabled:                 true,
		DetectSuspiciousImports: true,
	}

	if !cfg.Enabled {
		t.Error("expected Enabled = true")
	}
	if !cfg.DetectSuspiciousImports {
		t.Error("expected DetectSuspiciousImports = true")
	}
}

func TestCodeSecurityConfig_ZeroValue(t *testing.T) {
	var cfg CodeSecurityConfig

	if cfg.Enabled {
		t.Error("zero Enabled should be false")
	}
	if cfg.DetectSuspiciousImports {
		t.Error("zero DetectSuspiciousImports should be false")
	}
}

// --- InvisibleTextConfig ---

func TestInvisibleTextConfig_AllFields(t *testing.T) {
	cfg := InvisibleTextConfig{
		Enabled:         true,
		DetectZeroWidth: true,
	}

	if !cfg.Enabled {
		t.Error("expected Enabled = true")
	}
	if !cfg.DetectZeroWidth {
		t.Error("expected DetectZeroWidth = true")
	}
}

func TestInvisibleTextConfig_ZeroValue(t *testing.T) {
	var cfg InvisibleTextConfig

	if cfg.Enabled {
		t.Error("zero Enabled should be false")
	}
	if cfg.DetectZeroWidth {
		t.Error("zero DetectZeroWidth should be false")
	}
}

// --- EnvSecurityConfig ---

func TestEnvSecurityConfig_AllFields(t *testing.T) {
	cfg := EnvSecurityConfig{
		MaskPatterns: []string{"*_KEY", "*_SECRET", "*_PASSWORD"},
		SensitiveVars: []SecureEnvVar{
			{Name: "OPENAI_API_KEY", Value: "sk-secret"},
		},
	}

	if len(cfg.MaskPatterns) != 3 {
		t.Errorf("MaskPatterns length = %d, want 3", len(cfg.MaskPatterns))
	}
	if cfg.MaskPatterns[0] != "*_KEY" {
		t.Errorf("MaskPatterns[0] = %q, want %q", cfg.MaskPatterns[0], "*_KEY")
	}
	if len(cfg.SensitiveVars) != 1 {
		t.Errorf("SensitiveVars length = %d, want 1", len(cfg.SensitiveVars))
	}
	if cfg.SensitiveVars[0].Name != "OPENAI_API_KEY" {
		t.Errorf("SensitiveVars[0].Name = %q, want expected", cfg.SensitiveVars[0].Name)
	}
}

func TestEnvSecurityConfig_ZeroValue(t *testing.T) {
	var cfg EnvSecurityConfig

	if cfg.MaskPatterns != nil {
		t.Error("zero MaskPatterns should be nil")
	}
	if cfg.SensitiveVars != nil {
		t.Error("zero SensitiveVars should be nil")
	}
}

// --- SecureEnvVar ---

func TestSecureEnvVar_AllFields(t *testing.T) {
	v := SecureEnvVar{
		Name:  "DB_PASSWORD",
		Value: "s3cret!",
	}

	if v.Name != "DB_PASSWORD" {
		t.Errorf("Name = %q, want expected", v.Name)
	}
	if v.Value != "s3cret!" {
		t.Errorf("Value = %q, want expected", v.Value)
	}
}

// --- SecurityPolicy composition ---

func TestSecurityPolicy_AllSubConfigs(t *testing.T) {
	policy := SecurityPolicy{
		PII: &PIIConfig{
			Enabled: true,
			Types:   []PIIType{PIIEmail, PIISSN},
			Action:  RedactionActionRedact,
			Model:   "presidio",
		},
		InjectionDefense: &InjectionDefenseConfig{
			Enabled:     true,
			Sensitivity: InjectionSensitivityHigh,
			Action:      InjectionActionBlock,
		},
		Transformations: []TransformationRule{
			{Match: `sk-\w+`, Replace: "[REDACTED]", Direction: TransformOut},
		},
		Network: &NetworkPolicy{
			AllowOut: []string{"*.openai.com"},
			DenyOut:  []string{"*"},
		},
		Audit: &AuditConfig{
			Enabled:             true,
			RedactSensitiveData: true,
		},
		EnvSecurity: &EnvSecurityConfig{
			MaskPatterns: []string{"*_KEY"},
		},
		Toxicity: &ToxicityConfig{
			Enabled:   true,
			Threshold: 0.9,
		},
		CodeSecurity: &CodeSecurityConfig{
			Enabled:                 true,
			DetectSuspiciousImports: true,
		},
		InvisibleText: &InvisibleTextConfig{
			Enabled:         true,
			DetectZeroWidth: true,
		},
	}

	if policy.PII == nil {
		t.Fatal("PII should not be nil")
	}
	if !policy.PII.Enabled {
		t.Error("PII should be enabled")
	}
	if policy.InjectionDefense == nil {
		t.Fatal("InjectionDefense should not be nil")
	}
	if policy.InjectionDefense.Sensitivity != InjectionSensitivityHigh {
		t.Errorf("Sensitivity = %q, want high", policy.InjectionDefense.Sensitivity)
	}
	if len(policy.Transformations) != 1 {
		t.Errorf("Transformations length = %d, want 1", len(policy.Transformations))
	}
	if policy.Network == nil {
		t.Fatal("Network should not be nil")
	}
	if policy.Audit == nil {
		t.Fatal("Audit should not be nil")
	}
	if policy.EnvSecurity == nil {
		t.Fatal("EnvSecurity should not be nil")
	}
	if policy.Toxicity == nil {
		t.Fatal("Toxicity should not be nil")
	}
	if policy.CodeSecurity == nil {
		t.Fatal("CodeSecurity should not be nil")
	}
	if policy.InvisibleText == nil {
		t.Fatal("InvisibleText should not be nil")
	}
}

func TestSecurityPolicy_ZeroValue(t *testing.T) {
	var policy SecurityPolicy

	if policy.PII != nil {
		t.Error("zero PII should be nil")
	}
	if policy.InjectionDefense != nil {
		t.Error("zero InjectionDefense should be nil")
	}
	if policy.Transformations != nil {
		t.Error("zero Transformations should be nil")
	}
	if policy.Network != nil {
		t.Error("zero Network should be nil")
	}
	if policy.Audit != nil {
		t.Error("zero Audit should be nil")
	}
	if policy.EnvSecurity != nil {
		t.Error("zero EnvSecurity should be nil")
	}
	if policy.Toxicity != nil {
		t.Error("zero Toxicity should be nil")
	}
	if policy.CodeSecurity != nil {
		t.Error("zero CodeSecurity should be nil")
	}
	if policy.InvisibleText != nil {
		t.Error("zero InvisibleText should be nil")
	}
}

func TestSecurityPolicy_PartialConfig(t *testing.T) {
	// Only PII and Audit set
	policy := SecurityPolicy{
		PII:   &PIIConfig{Enabled: true},
		Audit: &AuditConfig{Enabled: true},
	}

	if policy.PII == nil {
		t.Error("PII should not be nil")
	}
	if policy.InjectionDefense != nil {
		t.Error("InjectionDefense should be nil")
	}
	if policy.Transformations != nil {
		t.Error("Transformations should be nil")
	}
	if policy.Network != nil {
		t.Error("Network should be nil")
	}
	if policy.Audit == nil {
		t.Error("Audit should not be nil")
	}
}

// --- RequiresTLSInterception ---

func TestRequiresTLSInterception_NilPolicy(t *testing.T) {
	var policy *SecurityPolicy
	if policy.RequiresTLSInterception() {
		t.Error("nil policy should not require TLS interception")
	}
}

func TestRequiresTLSInterception_EmptyPolicy(t *testing.T) {
	policy := &SecurityPolicy{}
	if policy.RequiresTLSInterception() {
		t.Error("empty policy should not require TLS interception")
	}
}

func TestRequiresTLSInterception_PIIEnabled(t *testing.T) {
	policy := &SecurityPolicy{
		PII: &PIIConfig{Enabled: true},
	}
	if !policy.RequiresTLSInterception() {
		t.Error("PII enabled should require TLS interception")
	}
}

func TestRequiresTLSInterception_PIIDisabled(t *testing.T) {
	policy := &SecurityPolicy{
		PII: &PIIConfig{Enabled: false},
	}
	if policy.RequiresTLSInterception() {
		t.Error("PII disabled should not require TLS interception")
	}
}

func TestRequiresTLSInterception_InjectionEnabled(t *testing.T) {
	policy := &SecurityPolicy{
		InjectionDefense: &InjectionDefenseConfig{Enabled: true},
	}
	if !policy.RequiresTLSInterception() {
		t.Error("injection defense enabled should require TLS interception")
	}
}

func TestRequiresTLSInterception_InjectionDisabled(t *testing.T) {
	policy := &SecurityPolicy{
		InjectionDefense: &InjectionDefenseConfig{Enabled: false},
	}
	if policy.RequiresTLSInterception() {
		t.Error("injection defense disabled should not require TLS interception")
	}
}

func TestRequiresTLSInterception_TransformationsPresent(t *testing.T) {
	policy := &SecurityPolicy{
		Transformations: []TransformationRule{
			{Match: "x", Replace: "y"},
		},
	}
	if !policy.RequiresTLSInterception() {
		t.Error("transformations present should require TLS interception")
	}
}

func TestRequiresTLSInterception_TransformationsEmpty(t *testing.T) {
	policy := &SecurityPolicy{
		Transformations: []TransformationRule{},
	}
	if policy.RequiresTLSInterception() {
		t.Error("empty transformations should not require TLS interception")
	}
}

func TestRequiresTLSInterception_ToxicityEnabled(t *testing.T) {
	policy := &SecurityPolicy{
		Toxicity: &ToxicityConfig{Enabled: true, Threshold: 0.9},
	}
	if !policy.RequiresTLSInterception() {
		t.Error("toxicity enabled should require TLS interception")
	}
}

func TestRequiresTLSInterception_ToxicityDisabled(t *testing.T) {
	policy := &SecurityPolicy{
		Toxicity: &ToxicityConfig{Enabled: false},
	}
	if policy.RequiresTLSInterception() {
		t.Error("toxicity disabled should not require TLS interception")
	}
}

func TestRequiresTLSInterception_CodeSecurityEnabled(t *testing.T) {
	policy := &SecurityPolicy{
		CodeSecurity: &CodeSecurityConfig{Enabled: true},
	}
	if !policy.RequiresTLSInterception() {
		t.Error("code security enabled should require TLS interception")
	}
}

func TestRequiresTLSInterception_CodeSecurityDisabled(t *testing.T) {
	policy := &SecurityPolicy{
		CodeSecurity: &CodeSecurityConfig{Enabled: false},
	}
	if policy.RequiresTLSInterception() {
		t.Error("code security disabled should not require TLS interception")
	}
}

func TestRequiresTLSInterception_InvisibleTextEnabled(t *testing.T) {
	policy := &SecurityPolicy{
		InvisibleText: &InvisibleTextConfig{Enabled: true},
	}
	if !policy.RequiresTLSInterception() {
		t.Error("invisible text enabled should require TLS interception")
	}
}

func TestRequiresTLSInterception_InvisibleTextDisabled(t *testing.T) {
	policy := &SecurityPolicy{
		InvisibleText: &InvisibleTextConfig{Enabled: false},
	}
	if policy.RequiresTLSInterception() {
		t.Error("invisible text disabled should not require TLS interception")
	}
}

func TestRequiresTLSInterception_NetworkPolicyOnly(t *testing.T) {
	// NetworkPolicy alone does NOT require TLS interception
	// (network filtering is done at the IP/SNI level)
	policy := &SecurityPolicy{
		Network: &NetworkPolicy{
			AllowOut: []string{"*.openai.com"},
			DenyOut:  []string{"*"},
		},
	}
	if policy.RequiresTLSInterception() {
		t.Error("network policy alone should not require TLS interception")
	}
}

func TestRequiresTLSInterception_AuditOnly(t *testing.T) {
	// Audit alone does NOT require TLS interception
	policy := &SecurityPolicy{
		Audit: &AuditConfig{Enabled: true},
	}
	if policy.RequiresTLSInterception() {
		t.Error("audit alone should not require TLS interception")
	}
}

func TestRequiresTLSInterception_EnvSecurityOnly(t *testing.T) {
	// EnvSecurity alone does NOT require TLS interception
	policy := &SecurityPolicy{
		EnvSecurity: &EnvSecurityConfig{
			MaskPatterns: []string{"*_KEY"},
		},
	}
	if policy.RequiresTLSInterception() {
		t.Error("env security alone should not require TLS interception")
	}
}

func TestRequiresTLSInterception_MultipleActiveScanners(t *testing.T) {
	policy := &SecurityPolicy{
		PII:              &PIIConfig{Enabled: true},
		InjectionDefense: &InjectionDefenseConfig{Enabled: true},
		Toxicity:         &ToxicityConfig{Enabled: true},
		CodeSecurity:     &CodeSecurityConfig{Enabled: true},
		InvisibleText:    &InvisibleTextConfig{Enabled: true},
		Transformations: []TransformationRule{
			{Match: "test", Replace: "x"},
		},
	}
	if !policy.RequiresTLSInterception() {
		t.Error("multiple active scanners should require TLS interception")
	}
}

func TestRequiresTLSInterception_AllDisabledExceptTransformations(t *testing.T) {
	policy := &SecurityPolicy{
		PII:              &PIIConfig{Enabled: false},
		InjectionDefense: &InjectionDefenseConfig{Enabled: false},
		Toxicity:         &ToxicityConfig{Enabled: false},
		CodeSecurity:     &CodeSecurityConfig{Enabled: false},
		InvisibleText:    &InvisibleTextConfig{Enabled: false},
		Transformations: []TransformationRule{
			{Match: "secret", Replace: "[REDACTED]"},
		},
	}
	if !policy.RequiresTLSInterception() {
		t.Error("transformations should require TLS interception even with all scanners disabled")
	}
}

func TestRequiresTLSInterception_AllDisabledNoTransformations(t *testing.T) {
	policy := &SecurityPolicy{
		PII:              &PIIConfig{Enabled: false},
		InjectionDefense: &InjectionDefenseConfig{Enabled: false},
		Toxicity:         &ToxicityConfig{Enabled: false},
		CodeSecurity:     &CodeSecurityConfig{Enabled: false},
		InvisibleText:    &InvisibleTextConfig{Enabled: false},
		Transformations:  []TransformationRule{},
		Network:          &NetworkPolicy{AllowOut: []string{"*"}},
		Audit:            &AuditConfig{Enabled: true},
	}
	if policy.RequiresTLSInterception() {
		t.Error("all scanners disabled with empty transformations should not require TLS interception")
	}
}

// --- ToJSON (stub — will panic, but test structure is the specification) ---

func TestSecurityPolicy_ToJSON_ReturnsMap(t *testing.T) {


	policy := &SecurityPolicy{
		PII: &PIIConfig{
			Enabled: true,
			Types:   []PIIType{PIIEmail, PIISSN},
			Action:  RedactionActionRedact,
		},
		InjectionDefense: &InjectionDefenseConfig{
			Enabled:     true,
			Sensitivity: InjectionSensitivityHigh,
			Action:      InjectionActionBlock,
		},
	}

	result := policy.ToJSON()
	if result == nil {
		t.Fatal("ToJSON returned nil")
	}

	// Check snake_case keys
	if _, ok := result["pii"]; !ok {
		t.Error("expected 'pii' key in ToJSON output")
	}
	if _, ok := result["injection_defense"]; !ok {
		t.Error("expected 'injection_defense' key in ToJSON output")
	}
}

func TestSecurityPolicy_ToJSON_PIIFields(t *testing.T) {


	policy := &SecurityPolicy{
		PII: &PIIConfig{
			Enabled: true,
			Types:   []PIIType{PIIEmail, PIICreditCard},
			Action:  RedactionActionBlock,
			Model:   "custom-model",
		},
	}

	result := policy.ToJSON()
	pii, ok := result["pii"].(map[string]interface{})
	if !ok {
		t.Fatal("pii should be a map")
	}
	if pii["enabled"] != true {
		t.Error("pii.enabled should be true")
	}
	types, ok := pii["types"].([]interface{})
	if !ok {
		t.Fatal("pii.types should be a slice")
	}
	if len(types) != 2 {
		t.Errorf("pii.types length = %d, want 2", len(types))
	}
}

func TestSecurityPolicy_ToJSON_NilSubConfigs(t *testing.T) {


	policy := &SecurityPolicy{}
	result := policy.ToJSON()

	// nil sub-configs should either be absent or null in the map
	if result == nil {
		t.Fatal("ToJSON returned nil for empty policy")
	}
}

func TestSecurityPolicy_ToJSON_Transformations(t *testing.T) {


	policy := &SecurityPolicy{
		Transformations: []TransformationRule{
			{Match: `sk-\w+`, Replace: "[KEY]", Direction: TransformOut},
			{Match: `Bearer \w+`, Replace: "Bearer [R]", Direction: TransformBoth},
		},
	}

	result := policy.ToJSON()
	transforms, ok := result["transformations"].([]interface{})
	if !ok {
		t.Fatal("transformations should be a slice")
	}
	if len(transforms) != 2 {
		t.Errorf("transformations length = %d, want 2", len(transforms))
	}
}

func TestSecurityPolicy_ToJSON_AllScanners(t *testing.T) {


	policy := &SecurityPolicy{
		Toxicity:      &ToxicityConfig{Enabled: true, Threshold: 0.8},
		CodeSecurity:  &CodeSecurityConfig{Enabled: true, DetectSuspiciousImports: true},
		InvisibleText: &InvisibleTextConfig{Enabled: true, DetectZeroWidth: true},
	}

	result := policy.ToJSON()
	if _, ok := result["toxicity"]; !ok {
		t.Error("expected 'toxicity' key")
	}
	if _, ok := result["code_security"]; !ok {
		t.Error("expected 'code_security' key")
	}
	if _, ok := result["invisible_text"]; !ok {
		t.Error("expected 'invisible_text' key")
	}
}

// --- ParseSecurityPolicy (stub — will panic, but test structure is the specification) ---

func TestParseSecurityPolicy_BasicMap(t *testing.T) {


	data := map[string]interface{}{
		"pii": map[string]interface{}{
			"enabled": true,
			"types":   []interface{}{"email", "ssn"},
			"action":  "redact",
		},
	}

	policy := ParseSecurityPolicy(data)
	if policy == nil {
		t.Fatal("ParseSecurityPolicy returned nil")
	}
	if policy.PII == nil {
		t.Fatal("PII should not be nil")
	}
	if !policy.PII.Enabled {
		t.Error("PII.Enabled should be true")
	}
}

func TestParseSecurityPolicy_NilFields(t *testing.T) {


	data := map[string]interface{}{}
	policy := ParseSecurityPolicy(data)

	if policy == nil {
		t.Fatal("ParseSecurityPolicy returned nil for empty map")
	}
	if policy.PII != nil {
		t.Error("PII should be nil for empty map")
	}
	if policy.InjectionDefense != nil {
		t.Error("InjectionDefense should be nil for empty map")
	}
}

func TestParseSecurityPolicy_FullRoundTrip(t *testing.T) {


	original := &SecurityPolicy{
		PII: &PIIConfig{
			Enabled: true,
			Types:   []PIIType{PIIEmail, PIISSN, PIICreditCard},
			Action:  RedactionActionRedact,
			Model:   "presidio",
		},
		InjectionDefense: &InjectionDefenseConfig{
			Enabled:     true,
			Sensitivity: InjectionSensitivityHigh,
			Action:      InjectionActionBlock,
		},
		Transformations: []TransformationRule{
			{Match: `Bearer \w+`, Replace: "Bearer [R]", Direction: TransformOut},
		},
		Network: &NetworkPolicy{
			AllowOut: []string{"*.openai.com"},
			DenyOut:  []string{"*"},
		},
		Audit: &AuditConfig{
			Enabled:             true,
			RedactSensitiveData: true,
		},
		Toxicity: &ToxicityConfig{
			Enabled:   true,
			Threshold: 0.85,
		},
		CodeSecurity: &CodeSecurityConfig{
			Enabled:                 true,
			DetectSuspiciousImports: true,
		},
		InvisibleText: &InvisibleTextConfig{
			Enabled:         true,
			DetectZeroWidth: true,
		},
	}

	serialized := original.ToJSON()
	restored := ParseSecurityPolicy(serialized)

	if restored == nil {
		t.Fatal("round-trip returned nil")
	}
	if !restored.PII.Enabled {
		t.Error("PII.Enabled lost in round-trip")
	}
	if len(restored.PII.Types) != 3 {
		t.Errorf("PII.Types length = %d, want 3", len(restored.PII.Types))
	}
	if restored.InjectionDefense.Sensitivity != InjectionSensitivityHigh {
		t.Errorf("Sensitivity = %q, want high", restored.InjectionDefense.Sensitivity)
	}
	if len(restored.Transformations) != 1 {
		t.Errorf("Transformations length = %d, want 1", len(restored.Transformations))
	}
	if len(restored.Network.AllowOut) != 1 {
		t.Errorf("Network.AllowOut length = %d, want 1", len(restored.Network.AllowOut))
	}
	if !restored.Audit.Enabled {
		t.Error("Audit.Enabled lost in round-trip")
	}
	if !restored.Toxicity.Enabled {
		t.Error("Toxicity.Enabled lost in round-trip")
	}
	if restored.Toxicity.Threshold != 0.85 {
		t.Errorf("Toxicity.Threshold = %f, want 0.85", restored.Toxicity.Threshold)
	}
	if !restored.CodeSecurity.Enabled {
		t.Error("CodeSecurity.Enabled lost in round-trip")
	}
	if !restored.InvisibleText.Enabled {
		t.Error("InvisibleText.Enabled lost in round-trip")
	}
}

func TestParseSecurityPolicy_MissingOptionalFields(t *testing.T) {


	// Only PII present, everything else missing
	data := map[string]interface{}{
		"pii": map[string]interface{}{
			"enabled": false,
		},
	}

	policy := ParseSecurityPolicy(data)
	if policy.PII == nil {
		t.Fatal("PII should not be nil")
	}
	if policy.PII.Enabled {
		t.Error("PII.Enabled should be false")
	}
	if policy.InjectionDefense != nil {
		t.Error("InjectionDefense should be nil when not in map")
	}
	if policy.Toxicity != nil {
		t.Error("Toxicity should be nil when not in map")
	}
}

func TestParseSecurityPolicy_NullValues(t *testing.T) {


	data := map[string]interface{}{
		"pii":               nil,
		"injection_defense": nil,
		"transformations":   nil,
		"network":           nil,
		"audit":             nil,
		"toxicity":          nil,
		"code_security":     nil,
		"invisible_text":    nil,
	}

	policy := ParseSecurityPolicy(data)
	if policy == nil {
		t.Fatal("ParseSecurityPolicy returned nil")
	}
	// Null values should result in nil sub-configs
	if policy.PII != nil {
		t.Error("PII should be nil for null value")
	}
}
