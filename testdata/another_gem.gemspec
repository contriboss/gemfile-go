# frozen_string_literal: true

Gem::Specification.new do |spec|
  spec.name = "another_gem"
  spec.version = "0.1.0"
  spec.authors = ["Another Dev"]
  spec.email = ["dev@example.com"]

  spec.summary = "Another test gem"
  spec.description = "Used for testing multiple gemspec scenarios"
  spec.homepage = "https://github.com/example/another_gem"
  spec.license = "Apache-2.0"

  spec.required_ruby_version = ">= 2.7.0"

  spec.files = Dir["lib/**/*.rb"]
  spec.require_paths = ["lib"]

  # Runtime dependencies
  spec.add_runtime_dependency "sinatra", "~> 3.0"

  # Development dependencies
  spec.add_development_dependency "minitest", "~> 5.0"
end
