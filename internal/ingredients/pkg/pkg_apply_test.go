package pkg

import (
	"context"
	"testing"

	"github.com/gogrlx/snack"
)

// --- getManager error tests (covers all methods) ---

func TestApplyGetManagerError(t *testing.T) {
	restore := withManagerErr(errTest)
	defer restore()
	methods := []string{
		"cleaned", "installed", "latest", "removed", "purged",
		"uptodate", "held", "unheld", "group_installed",
		"repo_managed", "key_managed", "upgraded",
	}
	for _, method := range methods {
		p := Pkg{name: "nginx", method: method, properties: map[string]interface{}{"name": "nginx"}}
		result, err := p.Apply(context.Background())
		if err != errTest {
			t.Errorf("Apply(%q): expected errTest, got %v", method, err)
		}
		if result.Succeeded {
			t.Errorf("Apply(%q): should not succeed on manager error", method)
		}
	}
}

func TestTestGetManagerError(t *testing.T) {
	restore := withManagerErr(errTest)
	defer restore()
	methods := []string{
		"cleaned", "installed", "latest", "removed", "purged",
		"uptodate", "held", "unheld", "group_installed",
		"repo_managed", "key_managed", "upgraded",
	}
	for _, method := range methods {
		p := Pkg{name: "nginx", method: method, properties: map[string]interface{}{"name": "nginx"}}
		result, err := p.Test(context.Background())
		if err != errTest {
			t.Errorf("Test(%q): expected errTest, got %v", method, err)
		}
		if result.Succeeded {
			t.Errorf("Test(%q): should not succeed on manager error", method)
		}
	}
}

func TestApplyInvalidMethod(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "bogus", properties: map[string]interface{}{"name": "nginx"}}
	_, err := p.Apply(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid method")
	}
}

func TestTestInvalidMethod(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "bogus", properties: map[string]interface{}{"name": "nginx"}}
	_, err := p.Test(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid method")
	}
}

// --- installed ---

func TestInstalledAlreadyInstalled(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.24.0"
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "installed", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Succeeded || result.Changed {
		t.Error("expected succeeded=true, changed=false for already installed")
	}
	if mock.installCalled {
		t.Error("install should not have been called")
	}
}

func TestInstalledNeedsInstall(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "installed", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Succeeded || !result.Changed {
		t.Error("expected succeeded=true, changed=true")
	}
	if !mock.installCalled {
		t.Error("install should have been called")
	}
}

func TestInstalledVersionMismatch(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.22.0"
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "installed", properties: map[string]interface{}{
		"name":    "nginx",
		"version": "1.24.0",
	}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true for version mismatch")
	}
	if !mock.installCalled {
		t.Error("install should have been called")
	}
}

func TestInstalledReinstall(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.24.0"
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "installed", properties: map[string]interface{}{
		"name":      "nginx",
		"reinstall": true,
	}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true for reinstall")
	}
}

func TestInstalledTestMode(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "installed", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Succeeded || !result.Changed {
		t.Error("test mode should report changed=true for missing pkg")
	}
	if mock.installCalled {
		t.Error("install should NOT be called in test mode")
	}
}

func TestInstalledInstallError(t *testing.T) {
	mock := newMock()
	mock.installErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "installed", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
	if result.Succeeded {
		t.Error("should not succeed on install error")
	}
}

func TestInstalledIsInstalledError(t *testing.T) {
	mock := newMock()
	mock.isInstalledErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "installed", properties: map[string]interface{}{"name": "nginx"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

func TestInstalledVersionError(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.22.0"
	mock.versionErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "installed", properties: map[string]interface{}{
		"name":    "nginx",
		"version": "1.24.0",
	}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

func TestInstalledMultiplePkgs(t *testing.T) {
	mock := newMock()
	mock.installed["curl"] = "8.0"
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "installed", properties: map[string]interface{}{
		"name": "nginx",
		"pkgs": []interface{}{"nginx", "curl", "wget"},
	}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true (nginx and wget need install)")
	}
}

// --- removed ---

func TestRemovedPackageInstalled(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.24.0"
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "removed", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true")
	}
	if !mock.removeCalled {
		t.Error("remove should have been called")
	}
}

func TestRemovedPackageNotInstalled(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "removed", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false for already absent package")
	}
}

func TestRemovedTestMode(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.24.0"
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "removed", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("test mode should report changed=true")
	}
	if mock.removeCalled {
		t.Error("remove should NOT be called in test mode")
	}
}

func TestRemovedRemoveError(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.24.0"
	mock.removeErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "removed", properties: map[string]interface{}{"name": "nginx"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

// --- purged ---

func TestPurgedPackageInstalled(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.24.0"
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "purged", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true")
	}
	if !mock.purgeCalled {
		t.Error("purge should have been called")
	}
}

func TestPurgedPackageNotInstalled(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "purged", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false")
	}
}

func TestPurgedTestMode(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.24.0"
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "purged", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("test mode should report changed=true")
	}
	if mock.purgeCalled {
		t.Error("purge should NOT be called in test mode")
	}
}

func TestPurgedError(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.24.0"
	mock.purgeErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "purged", properties: map[string]interface{}{"name": "nginx"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

// --- cleaned ---

func TestCleanedApply(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "all", method: "cleaned", properties: map[string]interface{}{"name": "all"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Succeeded || !result.Changed {
		t.Error("expected succeeded=true, changed=true")
	}
}

func TestCleanedWithAutoremove(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "all", method: "cleaned", properties: map[string]interface{}{
		"name":       "all",
		"autoremove": true,
	}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Succeeded {
		t.Error("expected succeeded")
	}
	if len(result.Notes) != 2 {
		t.Errorf("expected 2 notes (clean + autoremove), got %d", len(result.Notes))
	}
}

func TestCleanedTestMode(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "all", method: "cleaned", properties: map[string]interface{}{
		"name":       "all",
		"autoremove": true,
	}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Succeeded || !result.Changed {
		t.Error("test mode should report succeeded=true, changed=true")
	}
	if len(result.Notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(result.Notes))
	}
}

func TestCleanedCleanError(t *testing.T) {
	mock := newMock()
	mock.cleanErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "all", method: "cleaned", properties: map[string]interface{}{"name": "all"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

func TestCleanedAutoremoveError(t *testing.T) {
	mock := newMock()
	mock.autoremoveErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "all", method: "cleaned", properties: map[string]interface{}{
		"name":       "all",
		"autoremove": true,
	}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

// --- latest ---

func TestLatestAlreadyLatest(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.24.0"
	mock.upgradeAvail["nginx"] = false
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "latest", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false for up-to-date package")
	}
}

func TestLatestNeedsInstall(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "latest", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true for not-installed package")
	}
	if !mock.installCalled {
		t.Error("install should have been called")
	}
}

func TestLatestNeedsUpgrade(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.22.0"
	mock.upgradeAvail["nginx"] = true
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "latest", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true for upgrade available")
	}
}

func TestLatestTestMode(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.22.0"
	mock.upgradeAvail["nginx"] = true
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "latest", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("test mode should report changed=true")
	}
	if mock.installCalled {
		t.Error("install should NOT be called in test mode")
	}
}

func TestLatestUpgradeAvailError(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.22.0"
	mock.upgradeAvailErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "latest", properties: map[string]interface{}{"name": "nginx"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

// --- uptodate ---

func TestUptodateApply(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "all", method: "uptodate", properties: map[string]interface{}{"name": "all"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true for system upgrade")
	}
	if !mock.upgradeCalled {
		t.Error("upgrade should have been called")
	}
}

func TestUptodateUpgradeError(t *testing.T) {
	mock := newMock()
	mock.upgradeErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "all", method: "uptodate", properties: map[string]interface{}{"name": "all"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

func TestUptodateTestModeNoUpgrades(t *testing.T) {
	mock := newMock()
	mock.upgrades = nil
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "all", method: "uptodate", properties: map[string]interface{}{"name": "all"}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false when no upgrades available")
	}
}

func TestUptodateTestModeWithUpgrades(t *testing.T) {
	mock := newMock()
	mock.upgrades = []snack.Package{{Name: "nginx"}, {Name: "curl"}}
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "all", method: "uptodate", properties: map[string]interface{}{"name": "all"}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true when upgrades available")
	}
}

func TestUptodateTestModeListUpgradesError(t *testing.T) {
	mock := newMock()
	mock.listUpgradesErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "all", method: "uptodate", properties: map[string]interface{}{"name": "all"}}
	_, err := p.Test(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

// --- held ---

func TestHeldAlreadyHeld(t *testing.T) {
	mock := newMock()
	mock.held["nginx"] = true
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "held", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false for already held")
	}
}

func TestHeldNeedsHold(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "held", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true")
	}
	if len(mock.holdCalled) == 0 {
		t.Error("hold should have been called")
	}
}

func TestHeldTestMode(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "held", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("test mode should report changed=true")
	}
	if len(mock.holdCalled) > 0 {
		t.Error("hold should NOT be called in test mode")
	}
}

func TestHeldHoldError(t *testing.T) {
	mock := newMock()
	mock.holdErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "held", properties: map[string]interface{}{"name": "nginx"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

func TestHeldIsHeldError(t *testing.T) {
	mock := newMock()
	mock.isHeldErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "held", properties: map[string]interface{}{"name": "nginx"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

// --- unheld ---

func TestUnheldCurrentlyHeld(t *testing.T) {
	mock := newMock()
	mock.held["nginx"] = true
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "unheld", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true")
	}
	if len(mock.unholdCalled) == 0 {
		t.Error("unhold should have been called")
	}
}

func TestUnheldNotHeld(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "unheld", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false")
	}
}

func TestUnheldTestMode(t *testing.T) {
	mock := newMock()
	mock.held["nginx"] = true
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "unheld", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("test mode should report changed=true")
	}
	if len(mock.unholdCalled) > 0 {
		t.Error("unhold should NOT be called in test mode")
	}
}

func TestUnheldError(t *testing.T) {
	mock := newMock()
	mock.held["nginx"] = true
	mock.unholdErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "unheld", properties: map[string]interface{}{"name": "nginx"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

// --- group_installed ---

func TestGroupInstalledAlreadyInstalled(t *testing.T) {
	mock := newMock()
	mock.groupInstalled["dev-tools"] = true
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "dev-tools", method: "group_installed", properties: map[string]interface{}{"name": "dev-tools"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false for already installed group")
	}
}

func TestGroupInstalledNeedsInstall(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "dev-tools", method: "group_installed", properties: map[string]interface{}{"name": "dev-tools"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true")
	}
	if mock.groupInstallCalled != "dev-tools" {
		t.Errorf("expected GroupInstall called with 'dev-tools', got %q", mock.groupInstallCalled)
	}
}

func TestGroupInstalledTestMode(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "dev-tools", method: "group_installed", properties: map[string]interface{}{"name": "dev-tools"}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("test mode should report changed=true")
	}
	if mock.groupInstallCalled != "" {
		t.Error("group install should NOT be called in test mode")
	}
}

func TestGroupInstalledError(t *testing.T) {
	mock := newMock()
	mock.groupInstallErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "dev-tools", method: "group_installed", properties: map[string]interface{}{"name": "dev-tools"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

func TestGroupInstalledCheckError(t *testing.T) {
	mock := newMock()
	mock.groupIsInstalledErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "dev-tools", method: "group_installed", properties: map[string]interface{}{"name": "dev-tools"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

// --- repo_managed ---

func TestRepoManagedAddNew(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "my-repo", method: "repo_managed", properties: map[string]interface{}{
		"name": "my-repo",
		"url":  "https://repo.example.com",
	}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true for new repo")
	}
	if mock.addRepoCalled == nil {
		t.Fatal("addRepo should have been called")
	}
	if mock.addRepoCalled.URL != "https://repo.example.com" {
		t.Errorf("expected URL 'https://repo.example.com', got %q", mock.addRepoCalled.URL)
	}
}

func TestRepoManagedAlreadyExists(t *testing.T) {
	mock := newMock()
	mock.repos = []snack.Repository{{ID: "my-repo", Name: "my-repo"}}
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "my-repo", method: "repo_managed", properties: map[string]interface{}{"name": "my-repo"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false for existing repo")
	}
}

func TestRepoManagedRemove(t *testing.T) {
	mock := newMock()
	mock.repos = []snack.Repository{{ID: "my-repo", Name: "my-repo"}}
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "my-repo", method: "repo_managed", properties: map[string]interface{}{
		"name":   "my-repo",
		"absent": true,
	}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true for removing repo")
	}
	if mock.removeRepoCalled != "my-repo" {
		t.Errorf("expected removeRepo called with 'my-repo', got %q", mock.removeRepoCalled)
	}
}

func TestRepoManagedRemoveAbsent(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "my-repo", method: "repo_managed", properties: map[string]interface{}{
		"name":   "my-repo",
		"absent": true,
	}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false for already absent repo")
	}
}

func TestRepoManagedTestMode(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "my-repo", method: "repo_managed", properties: map[string]interface{}{"name": "my-repo"}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("test mode should report changed=true")
	}
	if mock.addRepoCalled != nil {
		t.Error("addRepo should NOT be called in test mode")
	}
}

func TestRepoManagedRemoveTestMode(t *testing.T) {
	mock := newMock()
	mock.repos = []snack.Repository{{ID: "my-repo", Name: "my-repo"}}
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "my-repo", method: "repo_managed", properties: map[string]interface{}{
		"name":   "my-repo",
		"absent": true,
	}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("test mode should report changed=true")
	}
}

func TestRepoManagedListError(t *testing.T) {
	mock := newMock()
	mock.listReposErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "my-repo", method: "repo_managed", properties: map[string]interface{}{"name": "my-repo"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

func TestRepoManagedAddError(t *testing.T) {
	mock := newMock()
	mock.addRepoErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "my-repo", method: "repo_managed", properties: map[string]interface{}{"name": "my-repo"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

func TestRepoManagedRemoveError(t *testing.T) {
	mock := newMock()
	mock.repos = []snack.Repository{{ID: "my-repo", Name: "my-repo"}}
	mock.removeRepoErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "my-repo", method: "repo_managed", properties: map[string]interface{}{
		"name":   "my-repo",
		"absent": true,
	}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

// --- key_managed ---

func TestKeyManagedAddNew(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "ABCD1234", method: "key_managed", properties: map[string]interface{}{"name": "ABCD1234"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true for new key")
	}
	if mock.addKeyCalled != "ABCD1234" {
		t.Errorf("expected addKey called with 'ABCD1234', got %q", mock.addKeyCalled)
	}
}

func TestKeyManagedAlreadyPresent(t *testing.T) {
	mock := newMock()
	mock.keys = []string{"ABCD1234"}
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "ABCD1234", method: "key_managed", properties: map[string]interface{}{"name": "ABCD1234"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false for existing key")
	}
}

func TestKeyManagedRemove(t *testing.T) {
	mock := newMock()
	mock.keys = []string{"ABCD1234"}
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "ABCD1234", method: "key_managed", properties: map[string]interface{}{
		"name":   "ABCD1234",
		"absent": true,
	}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true for removing key")
	}
	if mock.removeKeyCalled != "ABCD1234" {
		t.Errorf("expected removeKey called with 'ABCD1234', got %q", mock.removeKeyCalled)
	}
}

func TestKeyManagedRemoveAbsent(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "ABCD1234", method: "key_managed", properties: map[string]interface{}{
		"name":   "ABCD1234",
		"absent": true,
	}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false for already absent key")
	}
}

func TestKeyManagedTestModeAdd(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "ABCD1234", method: "key_managed", properties: map[string]interface{}{"name": "ABCD1234"}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("test mode should report changed=true")
	}
	if mock.addKeyCalled != "" {
		t.Error("addKey should NOT be called in test mode")
	}
}

func TestKeyManagedTestModeRemove(t *testing.T) {
	mock := newMock()
	mock.keys = []string{"ABCD1234"}
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "ABCD1234", method: "key_managed", properties: map[string]interface{}{
		"name":   "ABCD1234",
		"absent": true,
	}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("test mode should report changed=true")
	}
}

func TestKeyManagedListError(t *testing.T) {
	mock := newMock()
	mock.listKeysErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "ABCD1234", method: "key_managed", properties: map[string]interface{}{"name": "ABCD1234"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

func TestKeyManagedAddError(t *testing.T) {
	mock := newMock()
	mock.addKeyErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "ABCD1234", method: "key_managed", properties: map[string]interface{}{"name": "ABCD1234"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

func TestKeyManagedRemoveError(t *testing.T) {
	mock := newMock()
	mock.keys = []string{"ABCD1234"}
	mock.removeKeyErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "ABCD1234", method: "key_managed", properties: map[string]interface{}{
		"name":   "ABCD1234",
		"absent": true,
	}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

// --- upgraded ---

func TestUpgradedNeedsUpgrade(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.22.0"
	mock.upgradeAvail["nginx"] = true
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "upgraded", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected changed=true")
	}
	if !mock.upgradePackagesCalled {
		t.Error("upgradePackages should have been called")
	}
}

func TestUpgradedAlreadyLatest(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.24.0"
	mock.upgradeAvail["nginx"] = false
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "upgraded", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected changed=false for already latest")
	}
}

func TestUpgradedTestMode(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.22.0"
	mock.upgradeAvail["nginx"] = true
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "upgraded", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("test mode should report changed=true")
	}
	if mock.upgradePackagesCalled {
		t.Error("upgradePackages should NOT be called in test mode")
	}
}

func TestUpgradedError(t *testing.T) {
	mock := newMock()
	mock.installed["nginx"] = "1.22.0"
	mock.upgradeAvail["nginx"] = true
	mock.upgradePackagesErr = errTest
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "upgraded", properties: map[string]interface{}{"name": "nginx"}}
	_, err := p.Apply(context.Background())
	if err != errTest {
		t.Errorf("expected errTest, got %v", err)
	}
}

func TestUpgradedNotInstalled(t *testing.T) {
	mock := newMock()
	restore := withManager(mock)
	defer restore()

	p := Pkg{name: "nginx", method: "upgraded", properties: map[string]interface{}{"name": "nginx"}}
	result, err := p.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Not installed = nothing to upgrade
	if result.Changed {
		t.Error("expected changed=false for not-installed package")
	}
}

// --- parsePkgsList edge cases ---

func TestParsePkgsListMapInterfaceInterface(t *testing.T) {
	// Test the map[interface{}]interface{} branch
	targets := parsePkgsList([]interface{}{
		map[interface{}]interface{}{
			"redis": ">=7.0",
		},
	})
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Name != "redis" || targets[0].Version != ">=7.0" {
		t.Errorf("unexpected target: %+v", targets[0])
	}
}

func TestParsePkgsListMapNonStringKey(t *testing.T) {
	targets := parsePkgsList([]interface{}{
		map[interface{}]interface{}{
			123: "1.0", // non-string key should be skipped
		},
	})
	if len(targets) != 0 {
		t.Errorf("expected 0 targets for non-string key, got %d", len(targets))
	}
}

func TestParsePkgsListMapNonStringVersion(t *testing.T) {
	targets := parsePkgsList([]interface{}{
		map[string]interface{}{
			"redis": 7, // non-string version
		},
	})
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Version != "" {
		t.Errorf("expected empty version for non-string, got %q", targets[0].Version)
	}
}

func TestParseTargetNamesSimple(t *testing.T) {
	p := Pkg{
		name: "nginx",
		properties: map[string]interface{}{
			"name": "nginx",
			"pkgs": []interface{}{"nginx", "curl"},
		},
	}
	names := p.parseTargetNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if names[0] != "nginx" || names[1] != "curl" {
		t.Errorf("unexpected names: %v", names)
	}
}

// --- note helper ---

func TestNoteString(t *testing.T) {
	n := note("installed %d packages: %s", 3, "a, b, c")
	expected := "installed 3 packages: a, b, c"
	if n.String() != expected {
		t.Errorf("expected %q, got %q", expected, n.String())
	}
}

// --- Parse edge cases ---

func TestParseEmptyName(t *testing.T) {
	p := Pkg{}
	_, err := p.Parse("test-id", "installed", map[string]interface{}{"name": ""})
	if err == nil {
		t.Error("expected error for empty name string")
	}
}

func TestParseNonStringName(t *testing.T) {
	p := Pkg{}
	_, err := p.Parse("test-id", "installed", map[string]interface{}{"name": 123})
	if err == nil {
		t.Error("expected error for non-string name")
	}
}
