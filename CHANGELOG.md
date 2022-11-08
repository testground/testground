# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.6.0] - unreleased
### Added
- This ̀`CHANGELOG.md` file. See [PR 1445]
- Support addition of containers to the control network See [PR 1481]
- Support custom environment variables for `local:docker` runner. See [PR 1507]

### Fixed
- Fix dependencies rewrites in the `exec:go` builder. See [PR 1469]

[PR 1445]: https://github.com/testground/testground/pull/1445
[PR 1469]: https://github.com/testground/testground/pull/1469
[PR 1481]: https://github.com/testground/testground/pull/1481
[PR 1507]: https://github.com/testground/testground/pull/1507