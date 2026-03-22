package pkg

import (
	"context"
	"errors"

	"github.com/gogrlx/snack"
)

// mockManager implements snack.Manager and all optional interfaces for testing.
type mockManager struct {
	installed             map[string]string // name -> version
	upgradeAvail          map[string]bool
	held                  map[string]bool
	repos                 []snack.Repository
	keys                  []string
	groupInstalled        map[string]bool
	upgrades              []snack.Package
	installCalled         bool
	removeCalled          bool
	purgeCalled           bool
	upgradeCalled         bool
	holdCalled            []string
	unholdCalled          []string
	groupInstallCalled    string
	addRepoCalled         *snack.Repository
	removeRepoCalled      string
	addKeyCalled          string
	removeKeyCalled       string
	upgradePackagesCalled bool

	// Error injection
	isInstalledErr      error
	versionErr          error
	installErr          error
	removeErr           error
	purgeErr            error
	upgradeErr          error
	cleanErr            error
	autoremoveErr       error
	holdErr             error
	unholdErr           error
	isHeldErr           error
	groupIsInstalledErr error
	groupInstallErr     error
	listReposErr        error
	addRepoErr          error
	removeRepoErr       error
	listKeysErr         error
	addKeyErr           error
	removeKeyErr        error
	upgradeAvailErr     error
	listUpgradesErr     error
	upgradePackagesErr  error
}

// Manager interface
func (m *mockManager) Install(_ context.Context, _ []snack.Target, _ ...snack.Option) (snack.InstallResult, error) {
	m.installCalled = true
	return snack.InstallResult{}, m.installErr
}

func (m *mockManager) Remove(_ context.Context, _ []snack.Target, _ ...snack.Option) (snack.RemoveResult, error) {
	m.removeCalled = true
	return snack.RemoveResult{}, m.removeErr
}

func (m *mockManager) Purge(_ context.Context, _ []snack.Target, _ ...snack.Option) error {
	m.purgeCalled = true
	return m.purgeErr
}

func (m *mockManager) Upgrade(_ context.Context, _ ...snack.Option) error {
	m.upgradeCalled = true
	return m.upgradeErr
}

func (m *mockManager) Update(_ context.Context) error { return nil }

func (m *mockManager) List(_ context.Context) ([]snack.Package, error) { return nil, nil }

func (m *mockManager) Search(_ context.Context, _ string) ([]snack.Package, error) {
	return nil, nil
}

func (m *mockManager) Info(_ context.Context, _ string) (*snack.Package, error) { return nil, nil }

func (m *mockManager) IsInstalled(_ context.Context, pkg string) (bool, error) {
	if m.isInstalledErr != nil {
		return false, m.isInstalledErr
	}
	_, ok := m.installed[pkg]
	return ok, nil
}

func (m *mockManager) Version(_ context.Context, pkg string) (string, error) {
	if m.versionErr != nil {
		return "", m.versionErr
	}
	return m.installed[pkg], nil
}

func (m *mockManager) Available() bool { return true }
func (m *mockManager) Name() string    { return "mock" }

// Cleaner interface
func (m *mockManager) Clean(_ context.Context) error {
	return m.cleanErr
}

func (m *mockManager) Autoremove(_ context.Context, _ ...snack.Option) error {
	return m.autoremoveErr
}

// Holder interface
func (m *mockManager) Hold(_ context.Context, pkgs []string) error {
	m.holdCalled = pkgs
	return m.holdErr
}

func (m *mockManager) Unhold(_ context.Context, pkgs []string) error {
	m.unholdCalled = pkgs
	return m.unholdErr
}

func (m *mockManager) ListHeld(_ context.Context) ([]snack.Package, error) { return nil, nil }

func (m *mockManager) IsHeld(_ context.Context, pkg string) (bool, error) {
	if m.isHeldErr != nil {
		return false, m.isHeldErr
	}
	return m.held[pkg], nil
}

// Grouper interface
func (m *mockManager) GroupList(_ context.Context) ([]string, error) { return nil, nil }

func (m *mockManager) GroupInfo(_ context.Context, _ string) ([]snack.Package, error) {
	return nil, nil
}

func (m *mockManager) GroupInstall(_ context.Context, group string, _ ...snack.Option) error {
	m.groupInstallCalled = group
	return m.groupInstallErr
}

func (m *mockManager) GroupIsInstalled(_ context.Context, group string) (bool, error) {
	if m.groupIsInstalledErr != nil {
		return false, m.groupIsInstalledErr
	}
	return m.groupInstalled[group], nil
}

// RepoManager interface
func (m *mockManager) ListRepos(_ context.Context) ([]snack.Repository, error) {
	if m.listReposErr != nil {
		return nil, m.listReposErr
	}
	return m.repos, nil
}

func (m *mockManager) AddRepo(_ context.Context, repo snack.Repository) error {
	m.addRepoCalled = &repo
	return m.addRepoErr
}

func (m *mockManager) RemoveRepo(_ context.Context, id string) error {
	m.removeRepoCalled = id
	return m.removeRepoErr
}

// KeyManager interface
func (m *mockManager) ListKeys(_ context.Context) ([]string, error) {
	if m.listKeysErr != nil {
		return nil, m.listKeysErr
	}
	return m.keys, nil
}

func (m *mockManager) AddKey(_ context.Context, key string) error {
	m.addKeyCalled = key
	return m.addKeyErr
}

func (m *mockManager) RemoveKey(_ context.Context, keyID string) error {
	m.removeKeyCalled = keyID
	return m.removeKeyErr
}

// VersionQuerier interface
func (m *mockManager) LatestVersion(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockManager) ListUpgrades(_ context.Context) ([]snack.Package, error) {
	if m.listUpgradesErr != nil {
		return nil, m.listUpgradesErr
	}
	return m.upgrades, nil
}

func (m *mockManager) UpgradeAvailable(_ context.Context, pkg string) (bool, error) {
	if m.upgradeAvailErr != nil {
		return false, m.upgradeAvailErr
	}
	return m.upgradeAvail[pkg], nil
}

func (m *mockManager) VersionCmp(_ context.Context, _, _ string) (int, error) { return 0, nil }

// PackageUpgrader interface
func (m *mockManager) UpgradePackages(_ context.Context, _ []snack.Target, _ ...snack.Option) (snack.InstallResult, error) {
	m.upgradePackagesCalled = true
	return snack.InstallResult{}, m.upgradePackagesErr
}

// Compile-time interface checks
var (
	_ snack.Manager         = (*mockManager)(nil)
	_ snack.Cleaner         = (*mockManager)(nil)
	_ snack.Holder          = (*mockManager)(nil)
	_ snack.Grouper         = (*mockManager)(nil)
	_ snack.RepoManager     = (*mockManager)(nil)
	_ snack.KeyManager      = (*mockManager)(nil)
	_ snack.VersionQuerier  = (*mockManager)(nil)
	_ snack.PackageUpgrader = (*mockManager)(nil)
)

// newMock returns a fresh mockManager with empty maps initialized.
func newMock() *mockManager {
	return &mockManager{
		installed:      make(map[string]string),
		upgradeAvail:   make(map[string]bool),
		held:           make(map[string]bool),
		groupInstalled: make(map[string]bool),
	}
}

// withManager replaces getManager for a test and restores it when done.
func withManager(mgr snack.Manager) func() {
	orig := getManager
	getManager = func() (snack.Manager, error) { return mgr, nil }
	return func() { getManager = orig }
}

// withManagerErr replaces getManager to return an error.
func withManagerErr(err error) func() {
	orig := getManager
	getManager = func() (snack.Manager, error) { return nil, err }
	return func() { getManager = orig }
}

// errTest is a sentinel error for testing.
var errTest = errors.New("test error")
