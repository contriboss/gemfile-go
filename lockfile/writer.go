package lockfile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

const (
	defaultGemRemote = "https://rubygems.org/"
	indent2          = "  "
	indent4          = "    "
	indent6          = "      "
)

// LockfileWriter handles writing Gemfile.lock files.
type LockfileWriter struct {
	DefaultGemRemote string
}

// NewLockfileWriter creates a new LockfileWriter with default settings.
func NewLockfileWriter() *LockfileWriter {
	return &LockfileWriter{
		DefaultGemRemote: defaultGemRemote,
	}
}

// Write serializes a Lockfile to the given writer in Bundler's Gemfile.lock format.
func (w *LockfileWriter) Write(lf *Lockfile, writer io.Writer) error {
	buf := bufio.NewWriter(writer)
	defer buf.Flush()

	sections := []func(*Lockfile, *bufio.Writer) error{
		w.writeGemSection,
		w.writeGitSection,
		w.writePathSection,
		w.writePlatformsSection,
		w.writeDependenciesSection,
		w.writeBundledWithSection,
	}

	firstSection := true
	for _, writeSection := range sections {
		// Check if section has content before writing
		if err := writeSection(lf, buf); err != nil {
			return err
		}
		// Add blank line between sections (except before first)
		if !firstSection {
			if _, err := buf.WriteString("\n"); err != nil {
				return err
			}
		}
		firstSection = false
	}

	return buf.Flush()
}

// WriteFile writes a Lockfile to the specified file path.
func (w *LockfileWriter) WriteFile(lf *Lockfile, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create lockfile: %w", err)
	}
	defer file.Close()

	return w.Write(lf, file)
}

// writeGemSection writes the GEM section(s) with sorted specs.
// If gems have different SourceURLs, writes multiple GEM sections.
func (w *LockfileWriter) writeGemSection(lf *Lockfile, buf *bufio.Writer) error {
	if len(lf.GemSpecs) == 0 {
		return nil
	}

	// Group gems by source URL
	gemsBySource := make(map[string][]GemSpec)
	for _, spec := range lf.GemSpecs {
		source := spec.SourceURL
		if source == "" {
			source = w.DefaultGemRemote
		}
		gemsBySource[source] = append(gemsBySource[source], spec)
	}

	// Sort sources for consistent output
	var sources []string
	for source := range gemsBySource {
		sources = append(sources, source)
	}
	slices.Sort(sources)

	// Write a GEM section for each source
	for i, source := range sources {
		if i > 0 {
			// Add blank line between GEM sections
			if _, err := buf.WriteString("\n"); err != nil {
				return err
			}
		}

		if _, err := buf.WriteString("GEM\n"); err != nil {
			return err
		}
		if _, err := buf.WriteString(indent2 + "remote: " + source + "\n"); err != nil {
			return err
		}
		if _, err := buf.WriteString(indent2 + "specs:\n"); err != nil {
			return err
		}

		// Sort specs alphabetically by name
		specs := gemsBySource[source]
		slices.SortFunc(specs, func(a, b GemSpec) int {
			return strings.Compare(a.Name, b.Name)
		})

		for j := range specs {
			if err := w.writeGemSpec(buf, &specs[j]); err != nil {
				return err
			}
		}
	}

	return nil
}

// writeGemSpec writes a single gem spec with its dependencies.
func (w *LockfileWriter) writeGemSpec(buf *bufio.Writer, spec *GemSpec) error {
	version := spec.Version
	if spec.Platform != "" {
		version = fmt.Sprintf("%s-%s", version, spec.Platform)
	}
	if _, err := fmt.Fprintf(buf, "%s%s (%s)\n", indent4, spec.Name, version); err != nil {
		return err
	}

	// Write dependencies sorted by name
	deps := make([]Dependency, len(spec.Dependencies))
	copy(deps, spec.Dependencies)
	slices.SortFunc(deps, func(a, b Dependency) int {
		return strings.Compare(a.Name, b.Name)
	})

	for i := range deps {
		if err := w.writeDependency(buf, &deps[i], indent6); err != nil {
			return err
		}
	}

	return nil
}

// writeGitSection writes the GIT section with grouped specs.
//
//nolint:gocyclo // Complexity from git source metadata handling
func (w *LockfileWriter) writeGitSection(lf *Lockfile, buf *bufio.Writer) error {
	if len(lf.GitSpecs) == 0 {
		return nil
	}

	// Group specs by source identity (remote + revision + branch + tag)
	type gitSource struct {
		remote   string
		revision string
		branch   string
		tag      string
		specs    []GitGemSpec
	}

	sourceMap := make(map[string]*gitSource)
	for i := range lf.GitSpecs {
		spec := &lf.GitSpecs[i]
		key := fmt.Sprintf("%s|%s|%s|%s", spec.Remote, spec.Revision, spec.Branch, spec.Tag)
		if sourceMap[key] == nil {
			sourceMap[key] = &gitSource{
				remote:   spec.Remote,
				revision: spec.Revision,
				branch:   spec.Branch,
				tag:      spec.Tag,
				specs:    []GitGemSpec{},
			}
		}
		sourceMap[key].specs = append(sourceMap[key].specs, *spec)
	}

	// Sort sources by remote
	var sources []*gitSource
	for _, src := range sourceMap {
		sources = append(sources, src)
	}
	slices.SortFunc(sources, func(a, b *gitSource) int {
		return strings.Compare(a.remote, b.remote)
	})

	// Write each git source block
	for _, src := range sources {
		if _, err := buf.WriteString("\nGIT\n"); err != nil {
			return err
		}
		if _, err := buf.WriteString(indent2 + "remote: " + src.remote + "\n"); err != nil {
			return err
		}
		if _, err := buf.WriteString(indent2 + "revision: " + src.revision + "\n"); err != nil {
			return err
		}
		if src.branch != "" {
			if _, err := buf.WriteString(indent2 + "branch: " + src.branch + "\n"); err != nil {
				return err
			}
		}
		if src.tag != "" {
			if _, err := buf.WriteString(indent2 + "tag: " + src.tag + "\n"); err != nil {
				return err
			}
		}
		if _, err := buf.WriteString(indent2 + "specs:\n"); err != nil {
			return err
		}

		// Sort specs within source
		slices.SortFunc(src.specs, func(a, b GitGemSpec) int {
			return strings.Compare(a.Name, b.Name)
		})

		for i := range src.specs {
			if err := w.writeGitGemSpec(buf, &src.specs[i]); err != nil {
				return err
			}
		}
	}

	return nil
}

// writeGitGemSpec writes a single git gem spec with its dependencies.
//
//nolint:dupl // Similar to writePathGemSpec but handles different type
func (w *LockfileWriter) writeGitGemSpec(buf *bufio.Writer, spec *GitGemSpec) error {
	if _, err := fmt.Fprintf(buf, "%s%s (%s)\n", indent4, spec.Name, spec.Version); err != nil {
		return err
	}

	// Write dependencies sorted by name
	deps := make([]Dependency, len(spec.Dependencies))
	copy(deps, spec.Dependencies)
	slices.SortFunc(deps, func(a, b Dependency) int {
		return strings.Compare(a.Name, b.Name)
	})

	for i := range deps {
		if err := w.writeDependency(buf, &deps[i], indent6); err != nil {
			return err
		}
	}

	return nil
}

// writePathSection writes the PATH section with grouped specs.
func (w *LockfileWriter) writePathSection(lf *Lockfile, buf *bufio.Writer) error {
	if len(lf.PathSpecs) == 0 {
		return nil
	}

	// Group specs by path
	type pathSource struct {
		remote string
		specs  []PathGemSpec
	}

	sourceMap := make(map[string]*pathSource)
	for i := range lf.PathSpecs {
		spec := &lf.PathSpecs[i]
		if sourceMap[spec.Remote] == nil {
			sourceMap[spec.Remote] = &pathSource{
				remote: spec.Remote,
				specs:  []PathGemSpec{},
			}
		}
		sourceMap[spec.Remote].specs = append(sourceMap[spec.Remote].specs, *spec)
	}

	// Sort sources by remote
	var sources []*pathSource
	for _, src := range sourceMap {
		sources = append(sources, src)
	}
	slices.SortFunc(sources, func(a, b *pathSource) int {
		return strings.Compare(a.remote, b.remote)
	})

	// Write each path source block
	for _, src := range sources {
		if _, err := buf.WriteString("\nPATH\n"); err != nil {
			return err
		}
		if _, err := buf.WriteString(indent2 + "remote: " + src.remote + "\n"); err != nil {
			return err
		}
		if _, err := buf.WriteString(indent2 + "specs:\n"); err != nil {
			return err
		}

		// Sort specs within source
		slices.SortFunc(src.specs, func(a, b PathGemSpec) int {
			return strings.Compare(a.Name, b.Name)
		})

		for i := range src.specs {
			if err := w.writePathGemSpec(buf, &src.specs[i]); err != nil {
				return err
			}
		}
	}

	return nil
}

// writePathGemSpec writes a single path gem spec with its dependencies.
//
//nolint:dupl // Similar to writeGitGemSpec but handles different type
func (w *LockfileWriter) writePathGemSpec(buf *bufio.Writer, spec *PathGemSpec) error {
	if _, err := fmt.Fprintf(buf, "%s%s (%s)\n", indent4, spec.Name, spec.Version); err != nil {
		return err
	}

	// Write dependencies sorted by name
	deps := make([]Dependency, len(spec.Dependencies))
	copy(deps, spec.Dependencies)
	slices.SortFunc(deps, func(a, b Dependency) int {
		return strings.Compare(a.Name, b.Name)
	})

	for i := range deps {
		if err := w.writeDependency(buf, &deps[i], indent6); err != nil {
			return err
		}
	}

	return nil
}

// writePlatformsSection writes the PLATFORMS section.
func (w *LockfileWriter) writePlatformsSection(lf *Lockfile, buf *bufio.Writer) error {
	if len(lf.Platforms) == 0 {
		return nil
	}

	if _, err := buf.WriteString("\nPLATFORMS\n"); err != nil {
		return err
	}

	// Deduplicate and sort platforms
	platformSet := make(map[string]bool)
	for _, p := range lf.Platforms {
		platformSet[p] = true
	}

	platforms := make([]string, 0, len(platformSet))
	for p := range platformSet {
		platforms = append(platforms, p)
	}
	slices.Sort(platforms)

	for _, platform := range platforms {
		if _, err := buf.WriteString(indent2 + platform + "\n"); err != nil {
			return err
		}
	}

	return nil
}

// writeDependenciesSection writes the DEPENDENCIES section.
func (w *LockfileWriter) writeDependenciesSection(lf *Lockfile, buf *bufio.Writer) error {
	if len(lf.Dependencies) == 0 {
		return nil
	}

	if _, err := buf.WriteString("\nDEPENDENCIES\n"); err != nil {
		return err
	}

	// Sort dependencies by name
	deps := make([]Dependency, len(lf.Dependencies))
	copy(deps, lf.Dependencies)
	slices.SortFunc(deps, func(a, b Dependency) int {
		return strings.Compare(a.Name, b.Name)
	})

	for i := range deps {
		if err := w.writeDependency(buf, &deps[i], indent2); err != nil {
			return err
		}
	}

	return nil
}

// writeBundledWithSection writes the BUNDLED WITH section.
func (w *LockfileWriter) writeBundledWithSection(lf *Lockfile, buf *bufio.Writer) error {
	if lf.BundledWith == "" {
		return nil
	}

	if _, err := buf.WriteString("\nBUNDLED WITH\n"); err != nil {
		return err
	}
	if _, err := buf.WriteString("   " + lf.BundledWith + "\n"); err != nil {
		return err
	}

	return nil
}

// writeDependency writes a single dependency line.
func (w *LockfileWriter) writeDependency(buf *bufio.Writer, dep *Dependency, indent string) error {
	if len(dep.Constraints) == 0 {
		if _, err := buf.WriteString(indent + dep.Name + "\n"); err != nil {
			return err
		}
		return nil
	}

	constraints := strings.Join(dep.Constraints, ", ")
	if _, err := fmt.Fprintf(buf, "%s%s (%s)\n", indent, dep.Name, constraints); err != nil {
		return err
	}
	return nil
}

// Write is a convenience function to write a lockfile to a writer.
func Write(lf *Lockfile, writer io.Writer) error {
	w := NewLockfileWriter()
	return w.Write(lf, writer)
}

// WriteFile is a convenience function to write a lockfile to a file.
func WriteFile(lf *Lockfile, path string) error {
	w := NewLockfileWriter()
	return w.WriteFile(lf, path)
}
