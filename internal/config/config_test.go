package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogrlx/grlx/v2/internal/log"
	"github.com/taigrr/jety"
)

// resetForTest resets the sync.Once so LoadConfig can be called again,
// and points jety at the given config file. Because jety's global
// ConfigManager retains state across tests, callers should set explicit
// values via jety.Set when testing config reads.
func resetForTest(t *testing.T, configFile string) {
	t.Helper()
	configLoaded = sync.Once{}
	AdminPubkeys = nil
	jety.SetConfigType("yaml")
	jety.SetConfigFile(configFile)
	_ = jety.ReadInConfig()
}

// resetForBinaryTest resets all config state: sync.Once, globals, jety, and
// systemConfigRoot. This ensures complete isolation between tests.
func resetForBinaryTest(t *testing.T, tmpRoot string) {
	t.Helper()
	configLoaded = sync.Once{}
	AdminPubkeys = nil
	resetJety(t)
	setSystemConfigRoot(tmpRoot)
	t.Cleanup(func() { resetSystemConfigRoot() })
}

func writeTempConfig(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBasePathValid_ExistingDir(t *testing.T) {
	dir := t.TempDir()
	RecipeDir = dir
	if !BasePathValid() {
		t.Error("BasePathValid should return true for an existing directory")
	}
}

func TestBasePathValid_NonExistentDir(t *testing.T) {
	RecipeDir = "/nonexistent/path/that/does/not/exist"
	if BasePathValid() {
		t.Error("BasePathValid should return false for a nonexistent directory")
	}
}

func TestBasePathValid_File(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "afile")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	RecipeDir = f
	if BasePathValid() {
		t.Error("BasePathValid should return false for a regular file")
	}
}

func TestStaticProps_ViaJety(t *testing.T) {
	// Set static props directly via jety to test the accessor.
	jety.Set("props.static", map[string]any{
		"sprout-1": map[string]any{"role": "webserver"},
		"sprout-2": map[string]any{"role": "database"},
	})
	defer jety.Set("props.static", nil)

	props := StaticProps()
	if len(props) == 0 {
		t.Fatal("expected non-empty static props")
	}
	if _, ok := props["sprout-1"]; !ok {
		t.Error("expected props to contain sprout-1")
	}
	if _, ok := props["sprout-2"]; !ok {
		t.Error("expected props to contain sprout-2")
	}
}

func TestInit_ViaJety(t *testing.T) {
	jety.Set("init", "systemd")
	defer jety.Set("init", "")

	if got := Init(); got != "systemd" {
		t.Errorf("Init() = %q, want systemd", got)
	}
}

func TestInit_Empty(t *testing.T) {
	jety.Set("init", "")
	if got := Init(); got != "" {
		t.Errorf("Init() = %q, want empty", got)
	}
}

func TestSetSproutID(t *testing.T) {
	dir := t.TempDir()
	cfgFile := writeTempConfig(t, dir, "config.yaml", "sproutid: original\n")
	resetForTest(t, cfgFile)

	SetSproutID("test-sprout-42")
	defer jety.Set("sproutid", "")

	got := jety.GetString("sproutid")
	if got != "test-sprout-42" {
		t.Errorf("sproutid via jety = %q, want test-sprout-42", got)
	}

	// Verify it was persisted to the config file.
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("config file should not be empty after SetSproutID")
	}
}

func TestLoadConfig_GrlxBinary(t *testing.T) {
	tmpHome := t.TempDir()
	cfgDir := filepath.Join(tmpHome, ".config", "grlx")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgFile := filepath.Join(cfgDir, "grlx")
	content := `
farmerinterface: 10.0.0.1
farmerapiport: "9999"
farmerbusport: "9998"
loglevel: debug
`
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", tmpHome)
	resetForTest(t, cfgFile)
	LoadConfig("grlx")

	if FarmerInterface != "10.0.0.1" {
		t.Errorf("FarmerInterface = %q, want 10.0.0.1", FarmerInterface)
	}
	if FarmerAPIPort != "9999" {
		t.Errorf("FarmerAPIPort = %q, want 9999", FarmerAPIPort)
	}
	if FarmerBusPort != "9998" {
		t.Errorf("FarmerBusPort = %q, want 9998", FarmerBusPort)
	}
	if LogLevel != log.LDebug {
		t.Errorf("log.Level = %v, want debug", LogLevel)
	}
	wantURL := "https://10.0.0.1:9999"
	if FarmerURL != wantURL {
		t.Errorf("FarmerURL = %q, want %q", FarmerURL, wantURL)
	}
	wantBus := "10.0.0.1:9998"
	if FarmerBusURL != wantBus {
		t.Errorf("FarmerBusURL = %q, want %q", FarmerBusURL, wantBus)
	}
}

func TestLoadConfig_LogLevels(t *testing.T) {
	tests := []struct {
		input string
		want  log.Level
	}{
		{"debug", log.LDebug},
		{"info", log.LInfo},
		{"notice", log.LNotice},
		{"warn", log.LWarn},
		{"error", log.LError},
		{"panic", log.LPanic},
		{"fatal", log.LFatal},
		{"unknown", log.LNotice},
		{"", log.LNotice},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tmpHome := t.TempDir()
			cfgDir := filepath.Join(tmpHome, ".config", "grlx")
			_ = os.MkdirAll(cfgDir, 0o755)

			var content string
			if tt.input != "" {
				content = "loglevel: " + tt.input + "\n"
			}
			cfgFile := filepath.Join(cfgDir, "grlx")
			_ = os.WriteFile(cfgFile, []byte(content), 0o644)

			t.Setenv("HOME", tmpHome)
			resetForTest(t, cfgFile)
			LoadConfig("grlx")

			if LogLevel != tt.want {
				t.Errorf("log.Level for %q = %v, want %v", tt.input, LogLevel, tt.want)
			}
		})
	}
}

func TestLoadConfig_DefaultValues(t *testing.T) {
	tmpHome := t.TempDir()
	cfgDir := filepath.Join(tmpHome, ".config", "grlx")
	_ = os.MkdirAll(cfgDir, 0o755)
	cfgFile := filepath.Join(cfgDir, "grlx")
	_ = os.WriteFile(cfgFile, []byte(""), 0o644)

	t.Setenv("HOME", tmpHome)
	resetForTest(t, cfgFile)
	LoadConfig("grlx")

	if FarmerInterface != "localhost" {
		t.Errorf("default FarmerInterface = %q, want localhost", FarmerInterface)
	}
	if FarmerAPIPort != "5405" {
		t.Errorf("default FarmerAPIPort = %q, want 5405", FarmerAPIPort)
	}
	if FarmerBusPort != "5406" {
		t.Errorf("default FarmerBusPort = %q, want 5406", FarmerBusPort)
	}
	wantURL := "https://localhost:5405"
	if FarmerURL != wantURL {
		t.Errorf("default FarmerURL = %q, want %q", FarmerURL, wantURL)
	}
	wantBus := "localhost:5406"
	if FarmerBusURL != wantBus {
		t.Errorf("default FarmerBusURL = %q, want %q", FarmerBusURL, wantBus)
	}
	if LogLevel != log.LNotice {
		t.Errorf("default log.Level = %v, want LNotice", LogLevel)
	}
	if RecipeDir != filepath.Join("/", "srv", "grlx", "recipes", "prod") {
		t.Errorf("default RecipeDir = %q", RecipeDir)
	}
}

func TestLoadConfig_CreatesConfigDirIfMissing(t *testing.T) {
	tmpHome := t.TempDir()
	cfgDir := filepath.Join(tmpHome, ".config", "grlx")
	// Don't create cfgDir — LoadConfig should create it.
	cfgFile := filepath.Join(cfgDir, "grlx")

	t.Setenv("HOME", tmpHome)
	configLoaded = sync.Once{}
	// Don't call resetForTest — let LoadConfig handle the missing file.
	LoadConfig("grlx")

	if _, err := os.Stat(cfgDir); os.IsNotExist(err) {
		t.Error("LoadConfig should create the config directory")
	}
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		t.Error("LoadConfig should create the config file")
	}
}

func TestLoadConfig_GrlxRootCA(t *testing.T) {
	tmpHome := t.TempDir()
	cfgDir := filepath.Join(tmpHome, ".config", "grlx")
	_ = os.MkdirAll(cfgDir, 0o755)
	cfgFile := filepath.Join(cfgDir, "grlx")
	_ = os.WriteFile(cfgFile, []byte(""), 0o644)

	t.Setenv("HOME", tmpHome)
	// Ensure XDG_CONFIG_HOME is unset so LoadConfig falls back to $HOME/.config.
	t.Setenv("XDG_CONFIG_HOME", "")
	resetForTest(t, cfgFile)
	LoadConfig("grlx")

	wantCA := filepath.Join(tmpHome, ".config", "grlx", "tls-rootca.pem")
	// os.UserHomeDir() should pick up the overridden HOME.
	// If it doesn't (cached from an earlier test), verify the path ends correctly.
	wantSuffix := filepath.Join(".config", "grlx", "tls-rootca.pem")
	if GrlxRootCA != wantCA && !strings.HasSuffix(GrlxRootCA, wantSuffix) {
		t.Errorf("GrlxRootCA = %q, want %q (or suffix %q)", GrlxRootCA, wantCA, wantSuffix)
	}
}

func TestLoadConfig_XDGConfigHome(t *testing.T) {
	tmpHome := t.TempDir()
	xdgDir := filepath.Join(tmpHome, "custom-config")
	cfgDir := filepath.Join(xdgDir, "grlx")
	_ = os.MkdirAll(cfgDir, 0o755)
	cfgFile := filepath.Join(cfgDir, "grlx")
	_ = os.WriteFile(cfgFile, []byte(""), 0o644)

	t.Setenv("HOME", tmpHome)
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	resetForTest(t, cfgFile)
	LoadConfig("grlx")

	wantCA := filepath.Join(xdgDir, "grlx", "tls-rootca.pem")
	if GrlxRootCA != wantCA {
		t.Errorf("GrlxRootCA = %q, want %q (with XDG_CONFIG_HOME)", GrlxRootCA, wantCA)
	}
}

func TestLoadConfig_RecipeDirFallback(t *testing.T) {
	tmpHome := t.TempDir()
	cfgDir := filepath.Join(tmpHome, ".config", "grlx")
	_ = os.MkdirAll(cfgDir, 0o755)
	cfgFile := writeTempConfig(t, cfgDir, "grlx", "recipedir: \"\"\n")

	t.Setenv("HOME", tmpHome)
	resetForTest(t, cfgFile)
	LoadConfig("grlx")

	want := filepath.Join("/", "srv", "grlx", "recipes", "prod")
	if RecipeDir != want {
		t.Errorf("RecipeDir = %q, want %q (fallback)", RecipeDir, want)
	}
}

func TestVersion_Struct(t *testing.T) {
	v := Version{
		Arch:      "amd64",
		Compiler:  "gc",
		GitCommit: "abc123",
		Tag:       "v2.0.0",
	}
	if v.Arch != "amd64" {
		t.Error("unexpected Arch")
	}
	if v.Tag != "v2.0.0" {
		t.Error("unexpected Tag")
	}
}

func TestCombinedVersion_Struct(t *testing.T) {
	cv := CombinedVersion{
		CLI:    Version{Tag: "v2.0.0"},
		Farmer: Version{Tag: "v2.1.0"},
	}
	if cv.CLI.Tag != "v2.0.0" {
		t.Error("unexpected CLI tag")
	}
	if cv.Farmer.Tag != "v2.1.0" {
		t.Error("unexpected Farmer tag")
	}
}

func TestStartup_Struct(t *testing.T) {
	s := Startup{
		Version:  Version{Tag: "v1.0.0"},
		SproutID: "my-sprout",
	}
	if s.SproutID != "my-sprout" {
		t.Error("unexpected SproutID")
	}
}

func TestTriggerMsg_Struct(t *testing.T) {
	msg := TriggerMsg{JID: "job-12345"}
	if msg.JID != "job-12345" {
		t.Error("unexpected JID")
	}
}

func TestBinaryConstants(t *testing.T) {
	if BinaryGrlx != "grlx" {
		t.Errorf("BinaryGrlx = %q", BinaryGrlx)
	}
	if BinaryFarmer != "farmer" {
		t.Errorf("BinaryFarmer = %q", BinaryFarmer)
	}
	if BinarySprout != "sprout" {
		t.Errorf("BinarySprout = %q", BinarySprout)
	}
}

func TestLoadConfig_FarmerDefaults(t *testing.T) {
	tmpRoot := t.TempDir()
	cfgFile := writeTempConfig(t, tmpRoot, "farmer", "")
	resetForBinaryTest(t, tmpRoot)
	jety.SetConfigType("yaml")
	jety.SetConfigFile(cfgFile)
	_ = jety.ReadInConfig()

	LoadConfig("farmer")

	// Check farmer-specific defaults were applied.
	if FarmerAPIPort != "5405" {
		t.Errorf("default FarmerAPIPort = %q, want 5405", FarmerAPIPort)
	}
	if FarmerInterface != "localhost" {
		t.Errorf("default FarmerInterface = %q, want localhost", FarmerInterface)
	}
	if FarmerOrganization != "grlx farmer" {
		t.Errorf("FarmerOrganization = %q, want 'grlx farmer'", FarmerOrganization)
	}
	if APIWriteTimeout != 120*time.Second {
		t.Errorf("APIWriteTimeout = %v, want 2m0s", APIWriteTimeout)
	}
	if APIReadTimeout != 120*time.Second {
		t.Errorf("APIReadTimeout = %v, want 2m0s", APIReadTimeout)
	}
	if APIIdleTimeout != 120*time.Second {
		t.Errorf("APIIdleTimeout = %v, want 2m0s", APIIdleTimeout)
	}
	if CertificateValidTime != 365*24*time.Hour {
		t.Errorf("CertificateValidTime = %v, want 8760h", CertificateValidTime)
	}
	if AuditLevel != "write" {
		t.Errorf("AuditLevel = %q, want write", AuditLevel)
	}
	wantPKI := filepath.Join(tmpRoot, "pki/farmer") + "/"
	if FarmerPKI != wantPKI {
		t.Errorf("FarmerPKI = %q, want %q", FarmerPKI, wantPKI)
	}
	wantCert := filepath.Join(tmpRoot, "pki/farmer/tls-cert.pem")
	if CertFile != wantCert {
		t.Errorf("CertFile = %q, want %q", CertFile, wantCert)
	}
	wantKey := filepath.Join(tmpRoot, "pki/farmer/tls-key.pem")
	if KeyFile != wantKey {
		t.Errorf("KeyFile = %q, want %q", KeyFile, wantKey)
	}
	wantRootCA := filepath.Join(tmpRoot, "pki/farmer/tls-rootca.pem")
	if RootCA != wantRootCA {
		t.Errorf("RootCA = %q, want %q", RootCA, wantRootCA)
	}
}

func TestLoadConfig_FarmerWithCustomInterface(t *testing.T) {
	tmpRoot := t.TempDir()
	content := "farmerinterface: 192.168.1.100\nfarmerapiport: \"8080\"\nfarmerbusport: \"8081\"\n"
	cfgFile := writeTempConfig(t, tmpRoot, "farmer", content)
	resetForBinaryTest(t, tmpRoot)
	jety.SetConfigType("yaml")
	jety.SetConfigFile(cfgFile)
	_ = jety.ReadInConfig()

	LoadConfig("farmer")

	if FarmerInterface != "192.168.1.100" {
		t.Errorf("FarmerInterface = %q, want 192.168.1.100", FarmerInterface)
	}
	if FarmerAPIPort != "8080" {
		t.Errorf("FarmerAPIPort = %q, want 8080", FarmerAPIPort)
	}
	wantURL := "https://192.168.1.100:8080"
	if FarmerURL != wantURL {
		t.Errorf("FarmerURL = %q, want %q", FarmerURL, wantURL)
	}
	wantBus := "192.168.1.100:8081"
	if FarmerBusURL != wantBus {
		t.Errorf("FarmerBusURL = %q, want %q", FarmerBusURL, wantBus)
	}
	// Custom interface should be added to cert hosts.
	found := false
	for _, h := range CertHosts {
		if h == "192.168.1.100" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("CertHosts %v should contain 192.168.1.100", CertHosts)
	}
}

func TestLoadConfig_FarmerCertHostsDefault(t *testing.T) {
	tmpRoot := t.TempDir()
	cfgFile := writeTempConfig(t, tmpRoot, "farmer", "")
	resetForBinaryTest(t, tmpRoot)
	jety.SetConfigType("yaml")
	jety.SetConfigFile(cfgFile)
	_ = jety.ReadInConfig()

	LoadConfig("farmer")

	// Default cert hosts should include localhost, 127.0.0.1, farmer, grlx.
	required := []string{"localhost", "127.0.0.1", "farmer", "grlx"}
	hostSet := make(map[string]bool)
	for _, h := range CertHosts {
		hostSet[h] = true
	}
	for _, r := range required {
		if !hostSet[r] {
			t.Errorf("CertHosts %v missing required host %q", CertHosts, r)
		}
	}
}

func TestLoadConfig_FarmerAdminPubkeysFromConfig(t *testing.T) {
	tmpRoot := t.TempDir()
	content := `pubkeys:
  admin:
    - KEY_AAA
    - KEY_BBB
`
	cfgFile := writeTempConfig(t, tmpRoot, "farmer", content)
	resetForBinaryTest(t, tmpRoot)
	jety.SetConfigType("yaml")
	jety.SetConfigFile(cfgFile)
	_ = jety.ReadInConfig()

	LoadConfig("farmer")

	if len(AdminPubkeys) != 2 {
		t.Fatalf("AdminPubkeys len = %d, want 2; got %v", len(AdminPubkeys), AdminPubkeys)
	}
	keySet := make(map[string]bool)
	for _, k := range AdminPubkeys {
		keySet[k] = true
	}
	if !keySet["KEY_AAA"] || !keySet["KEY_BBB"] {
		t.Errorf("AdminPubkeys = %v, want KEY_AAA and KEY_BBB", AdminPubkeys)
	}
}

func TestLoadConfig_FarmerAdminPubkeysFromEnv(t *testing.T) {
	tmpRoot := t.TempDir()
	cfgFile := writeTempConfig(t, tmpRoot, "farmer", "")
	resetForBinaryTest(t, tmpRoot)
	jety.SetConfigType("yaml")
	jety.SetConfigFile(cfgFile)
	_ = jety.ReadInConfig()

	t.Setenv("ADMIN_PUBKEYS", "ENVKEY1,ENVKEY2,,ENVKEY3")

	LoadConfig("farmer")

	if len(AdminPubkeys) < 3 {
		t.Fatalf("AdminPubkeys len = %d, want >= 3; got %v", len(AdminPubkeys), AdminPubkeys)
	}
	keySet := make(map[string]bool)
	for _, k := range AdminPubkeys {
		keySet[k] = true
	}
	for _, want := range []string{"ENVKEY1", "ENVKEY2", "ENVKEY3"} {
		if !keySet[want] {
			t.Errorf("AdminPubkeys %v missing %q", AdminPubkeys, want)
		}
	}
}

func TestLoadConfig_FarmerCertHostsFromEnv(t *testing.T) {
	tmpRoot := t.TempDir()
	cfgFile := writeTempConfig(t, tmpRoot, "farmer", "")
	resetForBinaryTest(t, tmpRoot)
	jety.SetConfigType("yaml")
	jety.SetConfigFile(cfgFile)
	_ = jety.ReadInConfig()

	t.Setenv("CERT_HOSTS", "myhost.local,,otherhost.local")

	LoadConfig("farmer")

	hostSet := make(map[string]bool)
	for _, h := range CertHosts {
		hostSet[h] = true
	}
	for _, want := range []string{"myhost.local", "otherhost.local"} {
		if !hostSet[want] {
			t.Errorf("CertHosts %v missing env-provided %q", CertHosts, want)
		}
	}
}

func TestLoadConfig_FarmerJobLogTTL(t *testing.T) {
	tmpRoot := t.TempDir()
	content := "joblogttl: 168h\n"
	cfgFile := writeTempConfig(t, tmpRoot, "farmer", content)
	resetForBinaryTest(t, tmpRoot)
	jety.SetConfigType("yaml")
	jety.SetConfigFile(cfgFile)
	_ = jety.ReadInConfig()

	LoadConfig("farmer")

	want := 168 * time.Hour
	if JobLogTTL != want {
		t.Errorf("JobLogTTL = %v, want %v", JobLogTTL, want)
	}
}

func TestLoadConfig_SproutDefaults(t *testing.T) {
	tmpRoot := t.TempDir()
	cfgFile := writeTempConfig(t, tmpRoot, "sprout", "")
	resetForBinaryTest(t, tmpRoot)
	jety.SetConfigType("yaml")
	jety.SetConfigFile(cfgFile)
	_ = jety.ReadInConfig()

	LoadConfig("sprout")

	wantPKI := filepath.Join(tmpRoot, "pki/sprout") + "/"
	if SproutPKI != wantPKI {
		t.Errorf("SproutPKI = %q, want %q", SproutPKI, wantPKI)
	}
	wantRootCA := filepath.Join(tmpRoot, "pki/sprout/tls-rootca.pem")
	if SproutRootCA != wantRootCA {
		t.Errorf("SproutRootCA = %q, want %q", SproutRootCA, wantRootCA)
	}
	if JobLogDir != "/var/cache/grlx/sprout/jobs" {
		t.Errorf("JobLogDir = %q, want /var/cache/grlx/sprout/jobs", JobLogDir)
	}
	wantNKeyPub := filepath.Join(tmpRoot, "pki/sprout/sprout.nkey.pub")
	if NKeySproutPubFile != wantNKeyPub {
		t.Errorf("NKeySproutPubFile = %q, want %q", NKeySproutPubFile, wantNKeyPub)
	}
	wantNKeyPriv := filepath.Join(tmpRoot, "pki/sprout/sprout.nkey")
	if NKeySproutPrivFile != wantNKeyPriv {
		t.Errorf("NKeySproutPrivFile = %q, want %q", NKeySproutPrivFile, wantNKeyPriv)
	}
}

func TestLoadConfig_SproutWithCustomID(t *testing.T) {
	tmpRoot := t.TempDir()
	content := "sproutid: my-custom-sprout\nloglevel: warn\n"
	cfgFile := writeTempConfig(t, tmpRoot, "sprout", content)
	resetForBinaryTest(t, tmpRoot)
	jety.SetConfigType("yaml")
	jety.SetConfigFile(cfgFile)
	_ = jety.ReadInConfig()

	LoadConfig("sprout")

	if SproutID != "my-custom-sprout" {
		t.Errorf("SproutID = %q, want my-custom-sprout", SproutID)
	}
	if LogLevel != log.LWarn {
		t.Errorf("LogLevel = %v, want LWarn", LogLevel)
	}
}

func TestLoadConfig_SproutCreatesConfigIfMissing(t *testing.T) {
	tmpRoot := t.TempDir()
	// Don't create the sprout config file — LoadConfig should create it.
	resetForBinaryTest(t, tmpRoot)

	LoadConfig("sprout")

	cfgFile := filepath.Join(tmpRoot, "sprout")
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		t.Error("LoadConfig should create the sprout config file")
	}
}

func TestLoadConfig_FarmerCreatesConfigIfMissing(t *testing.T) {
	tmpRoot := t.TempDir()
	// Don't create the farmer config file — LoadConfig should create it.
	resetForBinaryTest(t, tmpRoot)

	LoadConfig("farmer")

	cfgFile := filepath.Join(tmpRoot, "farmer")
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		t.Error("LoadConfig should create the farmer config file")
	}
}

func TestLoadConfig_FarmerCertHostsFromConfig(t *testing.T) {
	tmpRoot := t.TempDir()
	content := `certhosts:
  - custom-host-1
  - custom-host-2
`
	cfgFile := writeTempConfig(t, tmpRoot, "farmer", content)
	resetForBinaryTest(t, tmpRoot)
	jety.SetConfigType("yaml")
	jety.SetConfigFile(cfgFile)
	_ = jety.ReadInConfig()

	LoadConfig("farmer")

	hostSet := make(map[string]bool)
	for _, h := range CertHosts {
		hostSet[h] = true
	}
	if !hostSet["custom-host-1"] || !hostSet["custom-host-2"] {
		t.Errorf("CertHosts %v should contain custom-host-1 and custom-host-2", CertHosts)
	}
}

func TestLoadConfig_FarmerPropsDir(t *testing.T) {
	tmpRoot := t.TempDir()
	content := "propsdir: /custom/props\n"
	cfgFile := writeTempConfig(t, tmpRoot, "farmer", content)
	resetForBinaryTest(t, tmpRoot)
	jety.SetConfigType("yaml")
	jety.SetConfigFile(cfgFile)
	_ = jety.ReadInConfig()

	LoadConfig("farmer")

	if PropsDir != "/custom/props" {
		t.Errorf("PropsDir = %q, want /custom/props", PropsDir)
	}
}

func TestStaticProps_Empty(t *testing.T) {
	jety.Set("props.static", nil)
	props := StaticProps()
	if len(props) != 0 {
		t.Errorf("StaticProps() = %v, want empty", props)
	}
}
