package lockfile

import (
	"bufio"
	"bytes"
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
		w.writeGitSection,
		w.writePathSection,
		w.writePluginSection,
		w.writeGemSection,
		w.writePlatformsSection,
		w.writeDependenciesSection,
		w.writeChecksumsSection,
		w.writeRubyVersionSection,
		w.writeBundledWithSection,
	}

	wroteAny := false
	for _, writeSection := range sections {
		var sectionBuf bytes.Buffer
		sectionWriter := bufio.NewWriter(&sectionBuf)
		if err := writeSection(lf, sectionWriter); err != nil {
			return err
		}
		if err := sectionWriter.Flush(); err != nil {
			return err
		}
		section := sectionBuf.String()
		if section == "" {
			continue
		}
		section = strings.TrimLeft(section, "\n")
		if section == "" {
			continue
		}
		if wroteAny {
			if _, err := buf.WriteString("\n"); err != nil {
				return err
			}
		}
		if _, err := buf.WriteString(section); err != nil {
			return err
		}
		wroteAny = true
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
//
//nolint:gocyclo // Section writing mirrors Bundler's nested layout rules.
func (w *LockfileWriter) writeGemSection(lf *Lockfile, buf *bufio.Writer) error {
	if len(lf.GemSpecs) == 0 {
		return nil
	}

	// Group gems by source URL
	gemsBySource := make(map[string][]GemSpec)
	for i := range lf.GemSpecs {
		spec := lf.GemSpecs[i]
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
			if a.Name != b.Name {
				return strings.Compare(a.Name, b.Name)
			}
			aVer := a.Version
			if a.Platform != "" {
				aVer = aVer + "-" + a.Platform
			}
			bVer := b.Version
			if b.Platform != "" {
				bVer = bVer + "-" + b.Platform
			}
			if aVer != bVer {
				return strings.Compare(aVer, bVer)
			}
			return strings.Compare(a.Platform, b.Platform)
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

	// Group specs by source identity (remote + revision + branch + tag + ref + submodules + glob)
	type gitSource struct {
		remote     string
		revision   string
		branch     string
		tag        string
		ref        string
		submodules bool
		glob       string
		specs      []GitGemSpec
	}

	sourceMap := make(map[string]*gitSource)
	for i := range lf.GitSpecs {
		spec := &lf.GitSpecs[i]
		key := fmt.Sprintf("%s|%s|%s|%s|%s|%v|%s", spec.Remote, spec.Revision, spec.Branch, spec.Tag, spec.Ref, spec.Submodules, spec.Glob)
		if sourceMap[key] == nil {
			sourceMap[key] = &gitSource{
				remote:     spec.Remote,
				revision:   spec.Revision,
				branch:     spec.Branch,
				tag:        spec.Tag,
				ref:        spec.Ref,
				submodules: spec.Submodules,
				glob:       spec.Glob,
				specs:      []GitGemSpec{},
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
		if src.ref != "" {
			if _, err := buf.WriteString(indent2 + "ref: " + src.ref + "\n"); err != nil {
				return err
			}
		}
		if src.submodules {
			if _, err := buf.WriteString(indent2 + "submodules: true\n"); err != nil {
				return err
			}
		}
		if src.glob != "" {
			if _, err := buf.WriteString(indent2 + "glob: " + src.glob + "\n"); err != nil {
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

	// Group specs by path + glob
	type pathSource struct {
		remote string
		glob   string
		specs  []PathGemSpec
	}

	sourceMap := make(map[string]*pathSource)
	for i := range lf.PathSpecs {
		spec := &lf.PathSpecs[i]
		normalizedRemote := normalizePathRemoteForLockfileOutput(spec.Remote)
		key := normalizedRemote + "|" + spec.Glob
		if sourceMap[key] == nil {
			sourceMap[key] = &pathSource{
				remote: normalizedRemote,
				glob:   spec.Glob,
				specs:  []PathGemSpec{},
			}
		}
		sourceMap[key].specs = append(sourceMap[key].specs, *spec)
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
		if src.glob != "" {
			if _, err := buf.WriteString(indent2 + "glob: " + src.glob + "\n"); err != nil {
				return err
			}
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
		normalized := normalizePlatformForLockfileOutput(p)
		if normalized == "" {
			continue
		}
		platformSet[normalized] = true
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

	gitPathSet := make(map[string]bool)
	for i := range lf.GitSpecs {
		spec := &lf.GitSpecs[i]
		if spec.Name != "" {
			gitPathSet[spec.Name] = true
		}
	}
	for i := range lf.PathSpecs {
		spec := &lf.PathSpecs[i]
		if spec.Name != "" {
			gitPathSet[spec.Name] = true
		}
	}

	for i := range deps {
		dep := deps[i]
		if !strings.HasSuffix(dep.Name, "!") && gitPathSet[dep.Name] {
			dep.Name += "!"
		}
		if err := w.writeDependency(buf, &dep, indent2); err != nil {
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
	if _, err := buf.WriteString("  " + lf.BundledWith + "\n"); err != nil {
		return err
	}

	return nil
}

// writeDependency writes a single dependency line.
func (w *LockfileWriter) writeDependency(buf *bufio.Writer, dep *Dependency, indent string) error {
	constraints := normalizeConstraintsForLockfile(dep.Constraints)
	if len(constraints) == 0 {
		if _, err := buf.WriteString(indent + dep.Name + "\n"); err != nil {
			return err
		}
		return nil
	}

	constraintsStr := strings.Join(constraints, ", ")
	if _, err := fmt.Fprintf(buf, "%s%s (%s)\n", indent, dep.Name, constraintsStr); err != nil {
		return err
	}
	return nil
}

// writeChecksumsSection writes the CHECKSUMS section.
//
//nolint:gocyclo // Requires ordering + fallback logic to match Bundler output.
func (w *LockfileWriter) writeChecksumsSection(lf *Lockfile, buf *bufio.Writer) error {
	if len(lf.Checksums) == 0 {
		return nil
	}

	if _, err := buf.WriteString("\nCHECKSUMS\n"); err != nil {
		return err
	}

	// Sort lock names for consistent output
	var lockNames []string
	for name := range lf.Checksums {
		lockNames = append(lockNames, name)
	}
	slices.Sort(lockNames)

	type lockEntry struct {
		name            string
		versionPlatform string
	}

	lockEntries := make(map[string]lockEntry)
	addLockEntry := func(name, version, platform string) {
		if name == "" || version == "" {
			return
		}
		versionPlatform := version
		lockName := name + "-" + version
		if platform != "" {
			versionPlatform = version + "-" + platform
			lockName = name + "-" + version + "-" + platform
		}
		lockEntries[lockName] = lockEntry{
			name:            name,
			versionPlatform: versionPlatform,
		}
	}

	for i := range lf.GemSpecs {
		spec := &lf.GemSpecs[i]
		addLockEntry(spec.Name, spec.Version, spec.Platform)
	}
	for i := range lf.GitSpecs {
		spec := &lf.GitSpecs[i]
		addLockEntry(spec.Name, spec.Version, "")
	}
	for i := range lf.PathSpecs {
		spec := &lf.PathSpecs[i]
		addLockEntry(spec.Name, spec.Version, "")
	}

	for _, lockName := range lockNames {
		checksums := lf.Checksums[lockName]
		entry, ok := lockEntries[lockName]
		if !ok {
			// Parse lock_name back to name (version[-platform]) as fallback.
			// Lock name format: gem_name-version or gem_name-version-platform
			parts := strings.SplitN(lockName, "-", 2)
			if len(parts) < 2 {
				continue
			}
			entry = lockEntry{name: parts[0], versionPlatform: parts[1]}
		}

		if len(checksums) > 0 {
			checksumStr := ChecksumsToLock(checksums)
			if _, err := fmt.Fprintf(buf, "%s%s (%s) %s\n", indent2, entry.name, entry.versionPlatform, checksumStr); err != nil {
				return err
			}
		} else {
			// Entry exists but no checksum
			if _, err := fmt.Fprintf(buf, "%s%s (%s)\n", indent2, entry.name, entry.versionPlatform); err != nil {
				return err
			}
		}
	}

	return nil
}

// writeRubyVersionSection writes the RUBY VERSION section.
func (w *LockfileWriter) writeRubyVersionSection(lf *Lockfile, buf *bufio.Writer) error {
	if lf.RubyVersion == "" {
		return nil
	}

	if _, err := buf.WriteString("\nRUBY VERSION\n"); err != nil {
		return err
	}
	if _, err := buf.WriteString(indent2 + lf.RubyVersion + "\n"); err != nil {
		return err
	}

	return nil
}

// writePluginSection writes the PLUGIN SOURCE section.
//
//nolint:gocyclo // Complexity from plugin source metadata handling
func (w *LockfileWriter) writePluginSection(lf *Lockfile, buf *bufio.Writer) error {
	if len(lf.PluginSpecs) == 0 {
		return nil
	}

	// Group specs by source identity (remote + type)
	type pluginSource struct {
		remote  string
		ptype   string
		options map[string]string
		specs   []PluginSpec
	}

	sourceMap := make(map[string]*pluginSource)
	for i := range lf.PluginSpecs {
		spec := &lf.PluginSpecs[i]
		key := fmt.Sprintf("%s|%s", spec.Remote, spec.Type)
		if sourceMap[key] == nil {
			sourceMap[key] = &pluginSource{
				remote:  spec.Remote,
				ptype:   spec.Type,
				options: spec.Options,
				specs:   []PluginSpec{},
			}
		}
		sourceMap[key].specs = append(sourceMap[key].specs, *spec)
	}

	// Sort sources by remote
	var sources []*pluginSource
	for _, src := range sourceMap {
		sources = append(sources, src)
	}
	slices.SortFunc(sources, func(a, b *pluginSource) int {
		return strings.Compare(a.remote, b.remote)
	})

	// Write each plugin source block
	for _, src := range sources {
		if _, err := buf.WriteString("\nPLUGIN SOURCE\n"); err != nil {
			return err
		}
		if _, err := buf.WriteString(indent2 + "remote: " + src.remote + "\n"); err != nil {
			return err
		}
		if src.ptype != "" {
			if _, err := buf.WriteString(indent2 + "type: " + src.ptype + "\n"); err != nil {
				return err
			}
		}
		// Write additional options
		if len(src.options) > 0 {
			var optKeys []string
			for k := range src.options {
				optKeys = append(optKeys, k)
			}
			slices.Sort(optKeys)
			for _, k := range optKeys {
				if _, err := fmt.Fprintf(buf, "%s%s: %s\n", indent2, k, src.options[k]); err != nil {
					return err
				}
			}
		}
		if _, err := buf.WriteString(indent2 + "specs:\n"); err != nil {
			return err
		}

		// Sort specs within source
		slices.SortFunc(src.specs, func(a, b PluginSpec) int {
			return strings.Compare(a.Name, b.Name)
		})

		for i := range src.specs {
			if err := w.writePluginSpec(buf, &src.specs[i]); err != nil {
				return err
			}
		}
	}

	return nil
}

// writePluginSpec writes a single plugin spec with its dependencies.
//
//nolint:dupl // Similar to writeGitGemSpec/writePathGemSpec but handles different type
func (w *LockfileWriter) writePluginSpec(buf *bufio.Writer, spec *PluginSpec) error {
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
