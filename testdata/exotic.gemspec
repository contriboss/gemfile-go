# frozen_string_literal: true

# Exotic gemspec with non-orthodox patterns to test parser robustness

VERSION = "3.14.159"

Gem::Specification.new do |s|
  # Dynamic version loading (tree-sitter should fail here)
  s.name = %q{exotic_gem}
  s.version = VERSION  # Constant reference

  # Using heredoc for description
  s.description = <<~DESC
    This is a multi-line description
    that spans several lines and includes
    special characters like "quotes" and 'apostrophes'
  DESC

  # Using %Q for summary with interpolation
  s.summary = %Q{An exotic gem with #{VERSION} features}

  # Dynamic author list
  authors = ["Alice", "Bob"]
  authors << "Charlie" if ENV['INCLUDE_CHARLIE']
  s.authors = authors

  # Conditional dependencies
  if RUBY_VERSION >= "3.0"
    s.add_runtime_dependency "modern_gem", "~> 2.0"
  else
    s.add_runtime_dependency "legacy_gem", "~> 1.0"
  end

  # Using tap pattern for metadata
  s.metadata = {}.tap do |m|
    m["homepage_uri"] = "https://example.com"
    m["source_code_uri"] = "https://github.com/example/exotic"
    m["changelog_uri"] = "https://github.com/example/exotic/CHANGELOG.md"
  end

  # Metaprogramming for dependencies
  %w[thor rake].each do |gem_name|
    s.add_development_dependency gem_name
  end

  # Complex version constraints
  s.add_runtime_dependency "complex_gem", ">= 1.0", "< 3.0", "!= 2.5.0"

  # Using send to dynamically set properties
  [:homepage, :license].each do |prop|
    value = case prop
    when :homepage then "https://exotic.example.com"
    when :license then "MIT"
    end
    s.send("#{prop}=", value)
  end

  # Files using Dir with complex glob
  s.files = Dir.glob("{lib,exe}/**/*", File::FNM_DOTMATCH).reject { |f|
    f.match?(/\.gem$/) || File.directory?(f)
  }

  # Platform-specific dependency
  s.add_runtime_dependency "win32-api" if Gem.win_platform?

  # Using a lambda for required ruby version
  s.required_ruby_version = Gem::Requirement.new(">= 2.7.0")

  # Inline conditional for email
  s.email = ENV['CI'] ? "ci@example.com" : ["dev@example.com", "support@example.com"]
end