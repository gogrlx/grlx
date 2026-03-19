package cook

import (
	"strings"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/props"
)

func TestRenderRecipeTemplateWithProps(t *testing.T) {
	const sproutID = "template-test-sprout"

	// Set props for this sprout.
	if err := props.SetProp(sproutID, "app_user", "deploy"); err != nil {
		t.Fatalf("SetProp app_user: %v", err)
	}
	if err := props.SetProp(sproutID, "app_group", "deploy"); err != nil {
		t.Fatalf("SetProp app_group: %v", err)
	}

	recipe := []byte(`steps:
  deploy config:
    file.managed:
      - name: /etc/app.conf
      - user: {{ props "app_user" }}
      - group: {{ props "app_group" }}
`)

	out, err := renderRecipeTemplate(sproutID, "test-recipe", recipe)
	if err != nil {
		t.Fatalf("renderRecipeTemplate: %v", err)
	}

	rendered := string(out)
	if !strings.Contains(rendered, "user: deploy") {
		t.Errorf("expected 'user: deploy' in rendered output, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "group: deploy") {
		t.Errorf("expected 'group: deploy' in rendered output, got:\n%s", rendered)
	}
}

func TestRenderRecipeTemplateConditionalBlock(t *testing.T) {
	const sproutID = "template-conditional-sprout"

	// With the prop set, the conditional block should be included.
	if err := props.SetProp(sproutID, "enable_debug", "true"); err != nil {
		t.Fatalf("SetProp: %v", err)
	}

	recipe := []byte(`steps:
  base:
    cmd.run:
      - name: echo hello
{{- if (props "enable_debug") }}
  debug step:
    cmd.run:
      - name: echo debug enabled
{{- end }}
`)

	out, err := renderRecipeTemplate(sproutID, "conditional-recipe", recipe)
	if err != nil {
		t.Fatalf("renderRecipeTemplate: %v", err)
	}

	rendered := string(out)
	if !strings.Contains(rendered, "debug step") {
		t.Errorf("expected 'debug step' in rendered output when prop is set, got:\n%s", rendered)
	}
}

func TestRenderRecipeTemplateConditionalBlockMissing(t *testing.T) {
	const sproutID = "template-conditional-missing-sprout"

	// No prop set — conditional block should be excluded.
	recipe := []byte(`steps:
  base:
    cmd.run:
      - name: echo hello
{{- if (props "enable_debug") }}
  debug step:
    cmd.run:
      - name: echo debug enabled
{{- end }}
`)

	out, err := renderRecipeTemplate(sproutID, "conditional-missing-recipe", recipe)
	if err != nil {
		t.Fatalf("renderRecipeTemplate: %v", err)
	}

	rendered := string(out)
	if strings.Contains(rendered, "debug step") {
		t.Errorf("expected 'debug step' to be excluded when prop is not set, got:\n%s", rendered)
	}
}

func TestRenderRecipeTemplateHostname(t *testing.T) {
	const sproutID = "template-hostname-sprout"

	recipe := []byte(`steps:
  set banner:
    file.content:
      - name: /etc/motd
      - text: "Managed by grlx - {{ hostname }}"
`)

	out, err := renderRecipeTemplate(sproutID, "hostname-recipe", recipe)
	if err != nil {
		t.Fatalf("renderRecipeTemplate: %v", err)
	}

	rendered := string(out)
	// hostname should not be empty — it should resolve to os.Hostname() or "localhost".
	if strings.Contains(rendered, "{{ hostname }}") {
		t.Errorf("hostname template function was not resolved, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Managed by grlx - ") {
		t.Errorf("expected 'Managed by grlx - ' prefix in rendered output, got:\n%s", rendered)
	}
}

func TestRenderRecipeTemplateMissingPropRendersEmpty(t *testing.T) {
	const sproutID = "template-missing-prop-sprout"

	recipe := []byte(`steps:
  deploy:
    file.managed:
      - name: /etc/app.conf
      - user: "{{ props "nonexistent_prop" }}"
`)

	out, err := renderRecipeTemplate(sproutID, "missing-prop-recipe", recipe)
	if err != nil {
		t.Fatalf("renderRecipeTemplate should not error on missing prop: %v", err)
	}

	rendered := string(out)
	// Missing prop should render as empty string.
	if !strings.Contains(rendered, `user: ""`) {
		t.Errorf("expected missing prop to render as empty string, got:\n%s", rendered)
	}
}

func TestPropsTemplatingEndToEnd(t *testing.T) {
	const sproutID = "e2e-props-sprout"

	// Set props.
	if err := props.SetProp(sproutID, "config_user", "appuser"); err != nil {
		t.Fatalf("SetProp config_user: %v", err)
	}
	if err := props.SetProp(sproutID, "config_path", "/opt/myapp/config.yaml"); err != nil {
		t.Fatalf("SetProp config_path: %v", err)
	}

	recipe := []byte(`steps:
  deploy app config:
    file.managed:
      - name: {{ props "config_path" }}
      - source: grlx://configs/app.yaml
      - user: {{ props "config_user" }}
      - mode: "644"
`)

	// Step 1: Render the template.
	rendered, err := renderRecipeTemplate(sproutID, "e2e-recipe", recipe)
	if err != nil {
		t.Fatalf("renderRecipeTemplate: %v", err)
	}

	// Step 2: Unmarshal the rendered YAML.
	recipeMap, err := unmarshalRecipe(rendered)
	if err != nil {
		t.Fatalf("unmarshalRecipe: %v", err)
	}

	// Step 3: Extract steps.
	stepsMap, err := stepsFromMap(recipeMap)
	if err != nil {
		t.Fatalf("stepsFromMap: %v", err)
	}
	if len(stepsMap) != 1 {
		t.Fatalf("expected 1 step, got %d", len(stepsMap))
	}

	// Step 4: Convert to Step structs.
	steps, err := makeRecipeSteps(stepsMap)
	if err != nil {
		t.Fatalf("makeRecipeSteps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}

	step := steps[0]
	if step.Ingredient != "file" {
		t.Errorf("expected ingredient 'file', got %q", step.Ingredient)
	}
	if step.Method != "managed" {
		t.Errorf("expected method 'managed', got %q", step.Method)
	}

	// Verify the props were resolved in the step properties.
	if name, ok := step.Properties["name"]; !ok || name != "/opt/myapp/config.yaml" {
		t.Errorf("expected name '/opt/myapp/config.yaml', got %v", step.Properties["name"])
	}
	if user, ok := step.Properties["user"]; !ok || user != "appuser" {
		t.Errorf("expected user 'appuser', got %v", step.Properties["user"])
	}
}

func TestPropsTemplatingConditionalEndToEnd(t *testing.T) {
	const sproutID = "e2e-conditional-sprout"

	// Set a prop that enables a conditional block.
	if err := props.SetProp(sproutID, "has_monitoring", "true"); err != nil {
		t.Fatalf("SetProp: %v", err)
	}

	recipe := []byte(`steps:
  base app:
    cmd.run:
      - name: echo base
{{- if (props "has_monitoring") }}
  monitoring agent:
    file.managed:
      - name: /etc/monitoring/agent.conf
      - source: grlx://monitoring/agent.conf
      - requisites:
        - require: base app
{{- end }}
`)

	rendered, err := renderRecipeTemplate(sproutID, "e2e-conditional", recipe)
	if err != nil {
		t.Fatalf("renderRecipeTemplate: %v", err)
	}

	recipeMap, err := unmarshalRecipe(rendered)
	if err != nil {
		t.Fatalf("unmarshalRecipe: %v", err)
	}

	stepsMap, err := stepsFromMap(recipeMap)
	if err != nil {
		t.Fatalf("stepsFromMap: %v", err)
	}

	// With the prop set, we should get 2 steps.
	if len(stepsMap) != 2 {
		t.Fatalf("expected 2 steps with prop set, got %d", len(stepsMap))
	}

	steps, err := makeRecipeSteps(stepsMap)
	if err != nil {
		t.Fatalf("makeRecipeSteps: %v", err)
	}

	// Find the monitoring step and verify its requisites.
	var monitoringStep *Step
	for _, s := range steps {
		if s.ID == "monitoring agent" {
			monitoringStep = s
			break
		}
	}
	if monitoringStep == nil {
		t.Fatal("monitoring agent step not found")
	}
	if len(monitoringStep.Requisites) != 1 {
		t.Fatalf("expected 1 requisite on monitoring step, got %d", len(monitoringStep.Requisites))
	}
	if monitoringStep.Requisites[0].Condition != Require {
		t.Errorf("expected requisite type 'require', got %q", monitoringStep.Requisites[0].Condition)
	}
}

func TestPropsTemplatingConditionalExcludedEndToEnd(t *testing.T) {
	const sproutID = "e2e-conditional-excluded-sprout"

	// No prop set — conditional block should be excluded.
	recipe := []byte(`steps:
  base app:
    cmd.run:
      - name: echo base
{{- if (props "has_monitoring") }}
  monitoring agent:
    file.managed:
      - name: /etc/monitoring/agent.conf
      - source: grlx://monitoring/agent.conf
{{- end }}
`)

	rendered, err := renderRecipeTemplate(sproutID, "e2e-conditional-excluded", recipe)
	if err != nil {
		t.Fatalf("renderRecipeTemplate: %v", err)
	}

	recipeMap, err := unmarshalRecipe(rendered)
	if err != nil {
		t.Fatalf("unmarshalRecipe: %v", err)
	}

	stepsMap, err := stepsFromMap(recipeMap)
	if err != nil {
		t.Fatalf("stepsFromMap: %v", err)
	}

	// Without the prop, we should get only 1 step.
	if len(stepsMap) != 1 {
		t.Fatalf("expected 1 step without prop set, got %d", len(stepsMap))
	}
}

func TestRenderRecipeTemplateInvalidSyntax(t *testing.T) {
	const sproutID = "template-invalid-sprout"

	recipe := []byte(`steps:
  bad step:
    cmd.run:
      - name: {{ props "unclosed
`)

	_, err := renderRecipeTemplate(sproutID, "invalid-recipe", recipe)
	if err == nil {
		t.Error("expected error for invalid template syntax, got nil")
	}
}

func TestRenderRecipeTemplateUndefinedFunction(t *testing.T) {
	const sproutID = "template-undef-func-sprout"

	recipe := []byte(`steps:
  bad step:
    cmd.run:
      - name: {{ nonexistent "arg" }}
`)

	_, err := renderRecipeTemplate(sproutID, "undef-func-recipe", recipe)
	if err == nil {
		t.Error("expected error for undefined template function, got nil")
	}
}

func TestTemplateFuncEnv(t *testing.T) {
	t.Setenv("GRLX_TEST_VAR", "hello_world")

	recipe := []byte(`value: {{ env "GRLX_TEST_VAR" }}`)
	out, err := renderRecipeTemplate("test-sprout", "env-test", recipe)
	if err != nil {
		t.Fatalf("renderRecipeTemplate error: %v", err)
	}
	if !strings.Contains(string(out), "hello_world") {
		t.Fatalf("expected 'hello_world' in output, got: %s", out)
	}
}

func TestTemplateFuncStringHelpers(t *testing.T) {
	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"join", `{{ join (split "a,b,c" ",") "-" }}`, "a-b-c"},
		{"upper", `{{ upper "hello" }}`, "HELLO"},
		{"lower", `{{ lower "HELLO" }}`, "hello"},
		{"trimSpace", `{{ trimSpace "  hi  " }}`, "hi"},
		{"hasPrefix", `{{ hasPrefix "hello" "hel" }}`, "true"},
		{"hasSuffix", `{{ hasSuffix "hello" "llo" }}`, "true"},
		{"contains", `{{ contains "hello world" "world" }}`, "true"},
		{"replace", `{{ replace "foo-bar" "-" "_" }}`, "foo_bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := renderRecipeTemplate("test-sprout", tt.name, []byte(tt.template))
			if err != nil {
				t.Fatalf("render error: %v", err)
			}
			if strings.TrimSpace(string(out)) != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, strings.TrimSpace(string(out)))
			}
		})
	}
}

func TestTemplateFuncPathHelpers(t *testing.T) {
	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"base", `{{ base "/etc/app/config.yml" }}`, "config.yml"},
		{"dir", `{{ dir "/etc/app/config.yml" }}`, "/etc/app"},
		{"ext", `{{ ext "/etc/app/config.yml" }}`, ".yml"},
		{"cleanPath", `{{ cleanPath "/etc/../etc/app/./config.yml" }}`, "/etc/app/config.yml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := renderRecipeTemplate("test-sprout", tt.name, []byte(tt.template))
			if err != nil {
				t.Fatalf("render error: %v", err)
			}
			if strings.TrimSpace(string(out)) != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, strings.TrimSpace(string(out)))
			}
		})
	}
}

func TestTemplateFuncDefault(t *testing.T) {
	t.Setenv("GRLX_EMPTY", "")
	t.Setenv("GRLX_SET", "custom")

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"empty uses default", `{{ default "fallback" (env "GRLX_EMPTY") }}`, "fallback"},
		{"set uses value", `{{ default "fallback" (env "GRLX_SET") }}`, "custom"},
		{"unset uses default", `{{ default "fallback" (env "GRLX_UNSET_VAR_XYZ") }}`, "fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := renderRecipeTemplate("test-sprout", tt.name, []byte(tt.template))
			if err != nil {
				t.Fatalf("render error: %v", err)
			}
			if strings.TrimSpace(string(out)) != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, strings.TrimSpace(string(out)))
			}
		})
	}
}

func TestTemplateFuncSproutID(t *testing.T) {
	out, err := renderRecipeTemplate("my-sprout-123", "sprout-test", []byte(`id: {{ sproutID }}`))
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(string(out), "my-sprout-123") {
		t.Fatalf("expected sproutID in output, got: %s", out)
	}
}

func TestTemplateFuncTernary(t *testing.T) {
	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"true", `{{ ternary "yes" "no" true }}`, "yes"},
		{"false", `{{ ternary "yes" "no" false }}`, "no"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := renderRecipeTemplate("test-sprout", tt.name, []byte(tt.template))
			if err != nil {
				t.Fatalf("render error: %v", err)
			}
			if strings.TrimSpace(string(out)) != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, strings.TrimSpace(string(out)))
			}
		})
	}
}

// --- Static props (farmer-side) integration tests ---

func TestStaticPropsInTemplate(t *testing.T) {
	// Reset props state.
	props.ClearStaticProps()

	// Load static props as a farmer would from config.
	cfg := map[string]interface{}{
		"static-template-sprout": map[string]interface{}{
			"db_host": "db.internal.example.com",
			"db_port": "5432",
		},
	}
	props.LoadStaticProps(cfg)
	t.Cleanup(props.ClearStaticProps)

	recipe := []byte(`steps:
  configure db:
    file.managed:
      - name: /etc/app/db.conf
      - source: grlx://configs/db.conf
      - context:
          host: {{ props "db_host" }}
          port: {{ props "db_port" }}
`)

	out, err := renderRecipeTemplate("static-template-sprout", "static-recipe", recipe)
	if err != nil {
		t.Fatalf("renderRecipeTemplate: %v", err)
	}

	rendered := string(out)
	if !strings.Contains(rendered, "host: db.internal.example.com") {
		t.Errorf("expected static prop db_host resolved, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "port: 5432") {
		t.Errorf("expected static prop db_port resolved, got:\n%s", rendered)
	}
}

func TestMixedStaticAndDynamicPropsInTemplate(t *testing.T) {
	props.ClearStaticProps()

	// Static prop from farmer config.
	cfg := map[string]interface{}{
		"mixed-sprout": map[string]interface{}{
			"region": "us-east-1",
		},
	}
	props.LoadStaticProps(cfg)
	t.Cleanup(props.ClearStaticProps)

	// Dynamic prop set at runtime (e.g. from sprout facts).
	if err := props.SetProp("mixed-sprout", "instance_id", "i-abc123"); err != nil {
		t.Fatalf("SetProp: %v", err)
	}

	recipe := []byte(`steps:
  tag instance:
    cmd.run:
      - name: "cloud tag --region={{ props "region" }} --id={{ props "instance_id" }}"
`)

	out, err := renderRecipeTemplate("mixed-sprout", "mixed-recipe", recipe)
	if err != nil {
		t.Fatalf("renderRecipeTemplate: %v", err)
	}

	rendered := string(out)
	if !strings.Contains(rendered, "--region=us-east-1") {
		t.Errorf("expected static prop 'region' resolved, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "--id=i-abc123") {
		t.Errorf("expected dynamic prop 'instance_id' resolved, got:\n%s", rendered)
	}
}

func TestDynamicPropOverridesStaticInTemplate(t *testing.T) {
	props.ClearStaticProps()

	// Static prop.
	cfg := map[string]interface{}{
		"override-sprout": map[string]interface{}{
			"log_level": "info",
		},
	}
	props.LoadStaticProps(cfg)
	t.Cleanup(props.ClearStaticProps)

	// Dynamic prop with the same key should override.
	if err := props.SetProp("override-sprout", "log_level", "debug"); err != nil {
		t.Fatalf("SetProp: %v", err)
	}

	recipe := []byte(`level: {{ props "log_level" }}`)
	out, err := renderRecipeTemplate("override-sprout", "override-recipe", recipe)
	if err != nil {
		t.Fatalf("renderRecipeTemplate: %v", err)
	}

	rendered := strings.TrimSpace(string(out))
	if rendered != "level: debug" {
		t.Errorf("expected dynamic prop to override static, got: %s", rendered)
	}
}

func TestMultiSproutIsolation(t *testing.T) {
	props.ClearStaticProps()

	cfg := map[string]interface{}{
		"sprout-a": map[string]interface{}{
			"role": "webserver",
		},
		"sprout-b": map[string]interface{}{
			"role": "database",
		},
	}
	props.LoadStaticProps(cfg)
	t.Cleanup(props.ClearStaticProps)

	recipe := []byte(`role: {{ props "role" }}`)

	outA, err := renderRecipeTemplate("sprout-a", "iso-a", recipe)
	if err != nil {
		t.Fatalf("render sprout-a: %v", err)
	}
	outB, err := renderRecipeTemplate("sprout-b", "iso-b", recipe)
	if err != nil {
		t.Fatalf("render sprout-b: %v", err)
	}

	if strings.TrimSpace(string(outA)) != "role: webserver" {
		t.Errorf("sprout-a expected 'role: webserver', got %q", strings.TrimSpace(string(outA)))
	}
	if strings.TrimSpace(string(outB)) != "role: database" {
		t.Errorf("sprout-b expected 'role: database', got %q", strings.TrimSpace(string(outB)))
	}
}

func TestStaticPropsEndToEndPipeline(t *testing.T) {
	props.ClearStaticProps()

	// Simulate farmer config with static props for a sprout.
	cfg := map[string]interface{}{
		"e2e-static-sprout": map[string]interface{}{
			"app_port":    "8080",
			"app_user":    "webapp",
			"config_path": "/etc/myapp/config.yml",
		},
	}
	props.LoadStaticProps(cfg)
	t.Cleanup(props.ClearStaticProps)

	recipe := []byte(`steps:
  deploy config:
    file.managed:
      - name: {{ props "config_path" }}
      - source: grlx://app/config.yml
      - user: {{ props "app_user" }}
      - mode: "644"
  start service:
    cmd.run:
      - name: "systemctl start myapp"
      - requisites:
        - require: deploy config
`)

	// Step 1: Render.
	rendered, err := renderRecipeTemplate("e2e-static-sprout", "e2e-static", recipe)
	if err != nil {
		t.Fatalf("renderRecipeTemplate: %v", err)
	}

	// Step 2: Unmarshal.
	recipeMap, err := unmarshalRecipe(rendered)
	if err != nil {
		t.Fatalf("unmarshalRecipe: %v", err)
	}

	// Step 3: Extract steps.
	stepsMap, err := stepsFromMap(recipeMap)
	if err != nil {
		t.Fatalf("stepsFromMap: %v", err)
	}
	if len(stepsMap) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(stepsMap))
	}

	// Step 4: Convert to Step structs.
	steps, err := makeRecipeSteps(stepsMap)
	if err != nil {
		t.Fatalf("makeRecipeSteps: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}

	// Verify static props resolved in step properties.
	var deployStep *Step
	var serviceStep *Step
	for _, s := range steps {
		switch s.ID {
		case "deploy config":
			deployStep = s
		case "start service":
			serviceStep = s
		}
	}

	if deployStep == nil {
		t.Fatal("deploy config step not found")
	}
	if name, ok := deployStep.Properties["name"]; !ok || name != "/etc/myapp/config.yml" {
		t.Errorf("expected name '/etc/myapp/config.yml', got %v", deployStep.Properties["name"])
	}
	if user, ok := deployStep.Properties["user"]; !ok || user != "webapp" {
		t.Errorf("expected user 'webapp', got %v", deployStep.Properties["user"])
	}
	if deployStep.Ingredient != "file" {
		t.Errorf("expected ingredient 'file', got %q", deployStep.Ingredient)
	}
	if deployStep.Method != "managed" {
		t.Errorf("expected method 'managed', got %q", deployStep.Method)
	}

	if serviceStep == nil {
		t.Fatal("start service step not found")
	}
	if len(serviceStep.Requisites) != 1 {
		t.Fatalf("expected 1 requisite on service step, got %d", len(serviceStep.Requisites))
	}
	if serviceStep.Requisites[0].Condition != Require {
		t.Errorf("expected requisite type 'require', got %q", serviceStep.Requisites[0].Condition)
	}
}

func TestStaticPropsClearedBetweenReloads(t *testing.T) {
	props.ClearStaticProps()

	// First load.
	cfg1 := map[string]interface{}{
		"reload-sprout": map[string]interface{}{
			"version": "1.0",
		},
	}
	props.LoadStaticProps(cfg1)

	recipe := []byte(`v: {{ props "version" }}`)
	out, err := renderRecipeTemplate("reload-sprout", "reload-1", recipe)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.TrimSpace(string(out)) != "v: 1.0" {
		t.Fatalf("expected 'v: 1.0', got %q", strings.TrimSpace(string(out)))
	}

	// Simulate config reload: clear then load new.
	props.ClearStaticProps()
	cfg2 := map[string]interface{}{
		"reload-sprout": map[string]interface{}{
			"version": "2.0",
		},
	}
	props.LoadStaticProps(cfg2)
	t.Cleanup(props.ClearStaticProps)

	out, err = renderRecipeTemplate("reload-sprout", "reload-2", recipe)
	if err != nil {
		t.Fatalf("render after reload: %v", err)
	}
	if strings.TrimSpace(string(out)) != "v: 2.0" {
		t.Errorf("expected 'v: 2.0' after reload, got %q", strings.TrimSpace(string(out)))
	}
}

func TestStaticPropsNestedTemplateExpressions(t *testing.T) {
	props.ClearStaticProps()

	cfg := map[string]interface{}{
		"nested-sprout": map[string]interface{}{
			"app_name": "myapp",
			"app_env":  "production",
		},
	}
	props.LoadStaticProps(cfg)
	t.Cleanup(props.ClearStaticProps)

	// Use static props combined with template functions.
	recipe := []byte(`steps:
  deploy:
    cmd.run:
      - name: "deploy {{ upper (props "app_name") }} to {{ props "app_env" }}"
`)

	out, err := renderRecipeTemplate("nested-sprout", "nested-recipe", recipe)
	if err != nil {
		t.Fatalf("renderRecipeTemplate: %v", err)
	}

	rendered := string(out)
	if !strings.Contains(rendered, "deploy MYAPP to production") {
		t.Errorf("expected composed template expression, got:\n%s", rendered)
	}
}

func TestStaticPropsWithDefaultFallback(t *testing.T) {
	props.ClearStaticProps()

	cfg := map[string]interface{}{
		"default-sprout": map[string]interface{}{
			"port": "9090",
		},
	}
	props.LoadStaticProps(cfg)
	t.Cleanup(props.ClearStaticProps)

	recipe := []byte(`port: {{ default "8080" (props "port") }}
host: {{ default "localhost" (props "missing_host") }}`)

	out, err := renderRecipeTemplate("default-sprout", "default-recipe", recipe)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(out)
	if !strings.Contains(rendered, "port: 9090") {
		t.Errorf("expected static prop to take precedence over default, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "host: localhost") {
		t.Errorf("expected default fallback for missing prop, got:\n%s", rendered)
	}
}
