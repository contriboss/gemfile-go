# frozen_string_literal: true

lib = File.expand_path("lib", __dir__)
$LOAD_PATH.unshift(lib) unless $LOAD_PATH.include?(lib)
require "test_gem/version"

Gem::Specification.new do |spec|
  spec.name = "test_gem"
  spec.version = "1.0.0"
  spec.authors = ["Test Author", "Another Author"]
  spec.email = ["test@example.com", "another@example.com"]

  spec.summary = "A test gem for gemspec parsing"
  spec.description = "This is a longer description of the test gem used for testing gemspec parsing functionality"
  spec.homepage = "https://github.com/example/test_gem"
  spec.license = "MIT"

  spec.metadata["homepage_uri"] = spec.homepage
  spec.metadata["source_code_uri"] = "https://github.com/example/test_gem"
  spec.metadata["changelog_uri"] = "https://github.com/example/test_gem/blob/master/CHANGELOG.md"

  spec.required_ruby_version = ">= 2.6.0"

  # Specify which files should be added to the gem when it is released.
  spec.files = Dir.chdir(File.expand_path(__dir__)) do
    `git ls-files -z`.split("\x0").reject { |f| f.match(%r{\A(?:test|spec|features)/}) }
  end
  spec.bindir = "exe"
  spec.executables = spec.files.grep(%r{\Aexe/}) { |f| File.basename(f) }
  spec.require_paths = ["lib"]

  # Runtime dependencies
  spec.add_runtime_dependency "rack", "~> 2.0"
  spec.add_runtime_dependency "thor", ">= 1.0", "< 2.0"
  spec.add_dependency "json", "~> 2.6"

  # Development dependencies
  spec.add_development_dependency "bundler", "~> 2.0"
  spec.add_development_dependency "rake", "~> 13.0"
  spec.add_development_dependency "rspec", "~> 3.10"
  spec.add_development_dependency "rubocop", "~> 1.25", ">= 1.25.0"
end