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
