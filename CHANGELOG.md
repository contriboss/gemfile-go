# Changelog

## Unreleased

### Features

* Add back gemfile parsing


## [0.14.0](https://github.com/contriboss/gemfile-go/compare/v0.13.0...v0.14.0) (2026-02-15)


### Features

* ✨ Soft & Hard APIs ([16a8c56](https://github.com/contriboss/gemfile-go/commit/16a8c56cf414837a0d352ce29191c8082ebbb84b))

### Bug Fixes

* Raise errors when lockfiles are missing or invalid instead of invoking bundler
* Never install gems - remain read-only

## [0.13.0](https://github.com/contriboss/gemfile-go/compare/v0.12.0...v0.13.0) (2026-02-15)


### Features

* Fallback to bundle install when lockfile is missing ([0232609](https://github.com/contriboss/gemfile-go/commit/023260960b8958ec083b505ae0d4b5b99671f348))


### Miscellaneous Chores

* release 0.13.0 ([0232609](https://github.com/contriboss/gemfile-go/commit/023260960b8958ec083b505ae0d4b5b99671f348))

## [0.12.0](https://github.com/contriboss/gemfile-go/compare/v0.11.0...v0.12.0) (2026-02-14)


### Features

* Enhance Ruby logic support in Gemfiles ([1808d5e](https://github.com/contriboss/gemfile-go/commit/1808d5e7689fc6eb340d74538151bd3444a7c481))
* improve if/unless detection to avoid false positives from comments/strings ([415474f](https://github.com/contriboss/gemfile-go/commit/415474f59ebbf74238e5dc5a3f0c523fd3521662))
* make filePath parameter optional for backwards compatibility ([fd0fd76](https://github.com/contriboss/gemfile-go/commit/fd0fd76bfd0efc3bea657a4e1995a84c8ac78cbd))
* normalize relative path sources in tree-sitter parser ([aa71e54](https://github.com/contriboss/gemfile-go/commit/aa71e5474363e05aa9983ff10685445a36b4c777))
* raise error on RUBY_VERSION/RUBY_ENGINE conditions instead of logging ([8961a14](https://github.com/contriboss/gemfile-go/commit/8961a14ddb9e35b9140317a212ad917ccbd71cc6))


### Bug Fixes

* correct indentation in test function ([14a4030](https://github.com/contriboss/gemfile-go/commit/14a403071a763580a3545f97a6991b063b4cbc60))
* relative path resolution in eval_gemfile to match Bundler behavior ([4b3be71](https://github.com/contriboss/gemfile-go/commit/4b3be71cf7038ddfbf3730affdaf850e45108a10))
* ensure `gemspec` and `path:` sources are resolved relative to the Gemfile they are defined in
* Remove ENV value logging to prevent secret leakage in CI logs ([ea3e573](https://github.com/contriboss/gemfile-go/commit/ea3e57340535b874504240fbca2b823f488b44f2))

## [0.11.0](https://github.com/contriboss/gemfile-go/compare/v0.10.0...v0.11.0) (2026-02-11)

### Features

* ✨ support `eval_gemfile` macro for modular Gemfiles ([a489eb9](https://github.com/contriboss/gemfile-go/commit/a489eb9e1f0376d0348a4fc7bde0173a25169e7d))

## [0.10.0](https://github.com/contriboss/gemfile-go/compare/v0.9.0...v0.10.0) (2026-02-08)

### Features

* support single group and platform values (e.g., `group: :test` or `platform: :mri`) ([c4ab169](https://github.com/contriboss/gemfile-go/commit/c4ab1694953ad0e24b2affe21557993bf13a8c59))

### Bug Fixes

* `release` workflow: Use proper `/v2/` in path to `golangci-lint` in `magefile.go`

## [0.9.0](https://github.com/contriboss/gemfile-go/compare/v0.8.0...v0.9.0) (2026-02-08)


### Features

* ✨ support hash 🚀 syntax in gemfiles and gemspecs ([d8f48ee](https://github.com/contriboss/gemfile-go/commit/d8f48eef4ae5bfd3da4113346856133b61a0f1e9))

### Bug Fixes

* support hash rocket syntax for gem and gemspec options
* improve group parsing to handle comments and trailing tokens correctly
* improve version constraint extraction to correctly handle hash rocket and symbolized hash options

## [0.8.0](https://github.com/contriboss/gemfile-go/compare/v0.7.3...v0.8.0) (2026-01-22)


### Features

* support Appraisal flows ([#16](https://github.com/contriboss/gemfile-go/issues/16)) ([25eb8c3](https://github.com/contriboss/gemfile-go/commit/25eb8c31a58863a5211973acd0f46c726354e143))


### Bug Fixes

* update golangci-lint to v2 ([#14](https://github.com/contriboss/gemfile-go/issues/14)) ([2db2e36](https://github.com/contriboss/gemfile-go/commit/2db2e3665f88feeba88b8f7c9b46097b7de3bcdb))

## [0.7.3](https://github.com/contriboss/gemfile-go/compare/v0.7.2...v0.7.3) (2026-01-19)


### Bug Fixes

* handle Env with the parser ([d9c30aa](https://github.com/contriboss/gemfile-go/commit/d9c30aa7c279b3ffb55c266b934891c1953a0c91))

## [0.7.2](https://github.com/contriboss/gemfile-go/compare/v0.7.1...v0.7.2) (2026-01-13)


### Bug Fixes

* dedup should not remove ruby ([e087caa](https://github.com/contriboss/gemfile-go/commit/e087caa5ff21d1b967be5928f76168fa37c52198))

## [0.7.1](https://github.com/contriboss/gemfile-go/compare/v0.7.0...v0.7.1) (2026-01-13)


### Bug Fixes

* normalize gnu/musl ([b7d138c](https://github.com/contriboss/gemfile-go/commit/b7d138c8d577eefb161f94e24f8153a3f3ce4a3d))

## [0.7.0](https://github.com/contriboss/gemfile-go/compare/v0.6.0...v0.7.0) (2026-01-13)


### Features

* normalize Bundler output with bundler 4 ([#9](https://github.com/contriboss/gemfile-go/issues/9)) ([965b818](https://github.com/contriboss/gemfile-go/commit/965b81879414ddacd7063189ef57d9149ef7db06))

## [0.6.0](https://github.com/contriboss/gemfile-go/compare/v0.5.1...v0.6.0) (2026-01-04)


### Features

* support new gemlock stucture ([#8](https://github.com/contriboss/gemfile-go/issues/8)) ([e81c160](https://github.com/contriboss/gemfile-go/commit/e81c160d2b70d6ebcfdb669bb794b96aae35b9e1))
