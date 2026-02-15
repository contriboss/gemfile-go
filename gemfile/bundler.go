package gemfile

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// InstallOptions represents options for the install command
type InstallOptions struct {
	Gemfile     string
	InstallPath string
}

// Install runs 'bundle install' with appropriate environment variables
func Install(opts *InstallOptions) error {
	if os.Getenv("SKIP_BUNDLE_INSTALL") == "true" {
		return nil
	}
	cmd := exec.CommandContext(context.Background(), "bundle", "install")

	// Add environment variables
	cmd.Env = os.Environ()

	if opts.Gemfile != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("BUNDLE_GEMFILE=%s", opts.Gemfile))
	}

	if opts.InstallPath != "" {
		// support for the --path option has been removed from bundler v4.0.6,
		// so we'll need to set BUNDLE_PATH on the CLI
		cmd.Env = append(cmd.Env, fmt.Sprintf("BUNDLE_PATH=%s", opts.InstallPath))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run 'bundle install': %w", err)
	}

	return nil
}
