Gem::Specification.new do |spec|
  spec.name = "test_relative"
  spec.version = "1.0.0"
  spec.authors = ["Test Author"]
  spec.email = ["test@example.com"]
  spec.summary = "Test gem for relative path resolution"
  spec.homepage = "https://github.com/example/test_relative"
  spec.license = "MIT"
  
  spec.add_runtime_dependency "rack", "~> 2.0"
  spec.add_runtime_dependency "json", ">= 2.0"
  
  spec.add_development_dependency "rspec", "~> 3.0"
end
