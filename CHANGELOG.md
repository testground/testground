# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.6.0] - unreleased
### Added
- This ̀`CHANGELOG.md` file. See [PR 1445]
- Support addition of containers to the control network. See [PR 1481]
- Add `pick`, `toml`, `withEnv`, and `atoi` to templates. See [PR 1516]
- Add `[[runs]]` field to compositions. See [PR 1516]
- Add `--run-ids` option during `testground run`. See [PR 1516]
- Add `--result-file ./output.csv` option during `testground run`. See [PR 1516]
- Move default `TESTGROUND_HOME` from `~/testgraound` to xdg directory specification. See [PR 1544]

### Fixed
- Fix dependencies rewrites in the `exec:go` builder. See [PR 1469]


### Changed
- Port shell code used for integration testing to a go package. See [PR 1537]

[PR 1445]: https://github.com/testground/testground/pull/1445
[PR 1469]: https://github.com/testground/testground/pull/1469
[PR 1481]: https://github.com/testground/testground/pull/1481
[PR 1537]: https://github.com/testground/testground/pull/1537
[PR 1544]: https://github.com/testground/testground/pull/1544
