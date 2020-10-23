package compute

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fastly/cli/pkg/common"
	"github.com/fastly/cli/pkg/errors"
	"github.com/fastly/cli/pkg/text"
)

// AssemblyScript implements Toolchain for the AssemblyScript language.
type AssemblyScript struct{}

// Name implements the Toolchain interface and returns the name of the toolchain.
func (a AssemblyScript) Name() string { return "assemblyscript" }

// DisplayName implements the Toolchain interface and returns the name of the
// toolchain suitable for displaying or printing to output.
func (a AssemblyScript) DisplayName() string { return "AssemblyScript (beta)" }

// StarterKits implements the Toolchain interface and returns the list of
// starter kits that can be used to initialize a new package for the toolchain.
func (a AssemblyScript) StarterKits() []StarterKit {
	return []StarterKit{
		{
			Name: "Default",
			Path: "https://github.com/fastly/compute-starter-kit-assemblyscript-default",
			Tag:  "v0.1.0",
		},
	}
}

// SourceDirectory implements the Toolchain interface and returns the source
// directory for AssemblyScript packages.
func (a AssemblyScript) SourceDirectory() string { return "src" }

// IncludeFiles implements the Toolchain interface and returns a list of
// additional files to include in the package archive for AssemblyScript packages.
func (a AssemblyScript) IncludeFiles() []string {
	return []string{"package.json"}
}

// Verify implements the Toolchain interface and verifies whether the
// AssemblyScript language toolchain is correctly configured on the host.
func (a AssemblyScript) Verify(out io.Writer) error {
	// 1) Check `npm` is on $PATH
	//
	// npm is Node/AssemblyScript's toolchain installer and manager, it is
	// needed to assert that the correct versions of the asc compiler and
	// @fastly/as-compute package are installed. We only check whether the
	// binary exists on the users $PATH and error with installation help text.
	fmt.Fprintf(out, "Checking if npm is installed...\n")

	p, err := exec.LookPath("npm")
	if err != nil {
		return errors.RemediationError{
			Inner:       fmt.Errorf("`npm` not found in $PATH"),
			Remediation: fmt.Sprintf("To fix this error, install Node.js and npm by visiting:\n\n\t$ %s", text.Bold("https://nodejs.org/")),
		}
	}

	fmt.Fprintf(out, "Found npm at %s\n", p)

	// 2) Check package.json file exists in $PWD
	//
	// A valid npm package is needed for compilation and to assert whether the
	// required dependencies are installed locally. Therefore, we first assert
	// whether one exists in the current $PWD.
	fpath, err := filepath.Abs("package.json")
	if err != nil {
		return fmt.Errorf("getting package.json path: %w", err)
	}

	if !common.FileExists(fpath) {
		return errors.RemediationError{
			Inner:       fmt.Errorf("package.json not found"),
			Remediation: fmt.Sprintf("To fix this error, run the following command:\n\n\t$ %s", text.Bold("npm init")),
		}
	}

	fmt.Fprintf(out, "Found package.json at %s\n", fpath)

	// 3) Check if `asc` is installed.
	//
	// asc is the AssemblyScript compiler. We first check if it exists in the
	// package.json and then whether the binary exists in the npm bin directory.
	fmt.Fprintf(out, "Checking if AssemblyScript is installed...\n")
	if !checkPackageDependencyExists("assemblyscript") {
		return errors.RemediationError{
			Inner:       fmt.Errorf("`assemblyscript` not found in package.json"),
			Remediation: fmt.Sprintf("To fix this error, run the following command:\n\n\t$ %s", text.Bold("npm install --save-dev assemblyscript")),
		}
	}

	p, err = getNpmBinPath()
	if err != nil {
		return errors.RemediationError{
			Inner:       fmt.Errorf("could not determine npm bin path"),
			Remediation: fmt.Sprintf("To fix this error, run the following command:\n\n\t$ %s", text.Bold("npm install --global npm@latest")),
		}
	}

	path, err := exec.LookPath(filepath.Join(p, "asc"))
	if err != nil {
		return fmt.Errorf("getting asc path: %w", err)
	}
	if !common.FileExists(path) {
		return errors.RemediationError{
			Inner:       fmt.Errorf("`asc` binary not found in %s", p),
			Remediation: fmt.Sprintf("To fix this error, run the following command:\n\n\t$ %s", text.Bold("npm install --save-dev assemblyscript")),
		}
	}

	fmt.Fprintf(out, "Found asc at %s\n", path)

	return nil
}

// Initialize implements the Toolchain interface and initializes a newly cloned
// package by installing required dependencies.
func (a AssemblyScript) Initialize(out io.Writer) error {
	// 1) Check `npm` is on $PATH
	//
	// npm is Node/AssemblyScript's toolchain package manager, it is needed to
	// install the package dependencies on initialization. We only check whether
	// the binary exists on the users $PATH and error with installation help text.
	fmt.Fprintf(out, "Checking if npm is installed...\n")

	p, err := exec.LookPath("npm")
	if err != nil {
		return errors.RemediationError{
			Inner:       fmt.Errorf("`npm` not found in $PATH"),
			Remediation: fmt.Sprintf("To fix this error, install Node.js and npm by visiting:\n\n\t$ %s", text.Bold("https://nodejs.org/")),
		}
	}

	fmt.Fprintf(out, "Found npm at %s\n", p)

	// 2) Check package.json file exists in $PWD
	//
	// A valid npm package manifest file is needed for the install command to
	// work. Therefore, we first assert whether one exists in the current $PWD.
	fpath, err := filepath.Abs("package.json")
	if err != nil {
		return fmt.Errorf("getting package.json path: %w", err)
	}

	if !common.FileExists(fpath) {
		return errors.RemediationError{
			Inner:       fmt.Errorf("package.json not found"),
			Remediation: fmt.Sprintf("To fix this error, run the following command:\n\n\t$ %s", text.Bold("npm init")),
		}
	}

	fmt.Fprintf(out, "Found package.json at %s\n", fpath)

	// Call npm install.
	cmd := common.NewStreamingExec("npm", []string{"install"}, []string{}, false, out)
	if err := cmd.Exec(); err != nil {
		return err
	}

	return nil
}

// Build implements the Toolchain interface and attempts to compile the package
// AssemblyScript source to a Wasm binary.
func (a AssemblyScript) Build(out io.Writer, verbose bool) error {
	// Check if bin directory exists and create if not.
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current working directory: %w", err)
	}
	binDir := filepath.Join(pwd, "bin")
	if err := common.MakeDirectoryIfNotExists(binDir); err != nil {
		return fmt.Errorf("error making bin directory: %w", err)
	}

	npmdir, err := getNpmBinPath()
	if err != nil {
		return err
	}

	args := []string{
		"src/index.ts",
		"--binaryFile",
		filepath.Join(binDir, "main.wasm"),
		"--optimize",
		"--noAssert",
	}
	if verbose {
		args = append(args, "--verbose")
	}

	// Call asc with the build arguments.
	cmd := common.NewStreamingExec(filepath.Join(npmdir, "asc"), args, []string{}, verbose, out)
	if err := cmd.Exec(); err != nil {
		return err
	}

	return nil
}

func getNpmBinPath() (string, error) {
	path, err := exec.Command("npm", "bin").Output()
	if err != nil {
		return "", fmt.Errorf("error getting npm bin path: %w", err)
	}
	return strings.TrimSpace(string(path)), nil
}

func checkPackageDependencyExists(name string) bool {
	// gosec flagged this:
	// G204 (CWE-78): Subprocess launched with variable
	// Disabling as the variables come from trusted sources.
	/* #nosec */
	cmd := exec.Command("npm", "link", "--json", "--depth", "0", name)
	if err := cmd.Start(); err != nil {
		return false
	}
	if err := cmd.Wait(); err != nil {
		return false
	}
	return true
}
