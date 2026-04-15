package launcher

import (
	"context"
	"errors"
	"os"
	"strings"
	"wox/launcher/platform"
	"wox/util"
)

const nativeLauncherEnabledEnv = "WOX_NATIVE_LAUNCHER_ENABLED"

type RuntimeInstaller interface {
	UseLauncherRuntime(runtime Runtime)
}

type RuntimeFactory func(ctx context.Context) (Runtime, error)

func StartIfEnabled(ctx context.Context, installer RuntimeInstaller, factory RuntimeFactory) (Runtime, error) {
	if !IsEnabled() {
		return nil, nil
	}

	return Start(ctx, installer, factory)
}

func Start(ctx context.Context, installer RuntimeInstaller, factory RuntimeFactory) (Runtime, error) {
	if installer == nil {
		return nil, errors.New("launcher runtime installer is nil")
	}

	runtime, err := resolveRuntime(ctx, factory)
	if err != nil {
		return nil, err
	}

	if err := runtime.Start(ctx); err != nil {
		return nil, err
	}

	installer.UseLauncherRuntime(runtime)
	util.GetLogger().Info(ctx, "native launcher runtime installed")
	return runtime, nil
}

func IsEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(nativeLauncherEnabledEnv)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func resolveRuntime(ctx context.Context, factory RuntimeFactory) (Runtime, error) {
	if factory != nil {
		return factory(ctx)
	}

	return DefaultRuntimeFactory(ctx)
}

func DefaultRuntimeFactory(ctx context.Context) (Runtime, error) {
	return DefaultRuntimeFactoryWithOptions(ctx, WindowShellRuntimeOptions{})
}

func DefaultRuntimeFactoryWithOptions(ctx context.Context, options WindowShellRuntimeOptions) (Runtime, error) {
	_ = ctx
	bundle := platform.NewDefaultBundle()
	return NewWindowShellRuntimeWithBundleAndOptions(bundle, options), nil
}
