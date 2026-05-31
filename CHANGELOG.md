# Changelog

## [0.8.0](https://github.com/Noooste/garage-ui/compare/v0.7.0...v0.8.0) (2026-05-31)


### ⚠ BREAKING CHANGES

* **backend,helm:** bind to IPv6 wildcard by default for dual-stack support

### Features

* **backend,helm:** bind to IPv6 wildcard by default for dual-stack support ([5427758](https://github.com/Noooste/garage-ui/commit/5427758eaadc4fa1327402b958b7e7e1f43aecdd))
* **docs:** add documentation generation command to Makefile ([186af18](https://github.com/Noooste/garage-ui/commit/186af18d54f739bd9b467cbf3ab9ccc2e92ddf62))

## [0.7.0](https://github.com/Noooste/garage-ui/compare/v0.6.2...v0.7.0) (2026-05-23)


### Features

* **backend,frontend:** enable quotas support in bucket settings ([#64](https://github.com/Noooste/garage-ui/issues/64)) ([1f16edd](https://github.com/Noooste/garage-ui/commit/1f16edd39cfa2f3a51cb576871642c9d23545781))
* **backend:** Support _FILE suffix on sensitive env variables ([#63](https://github.com/Noooste/garage-ui/issues/63)) ([36ec8e8](https://github.com/Noooste/garage-ui/commit/36ec8e800ea0ff5306e4d0f9f4aea4f72a5a109b))


### Bug Fixes

* **auth:** remove auto-enable token auth logic ([1b645b0](https://github.com/Noooste/garage-ui/commit/1b645b0c2c6e05dea98a7fc5d7595ca015913770))

## [0.6.2](https://github.com/Noooste/garage-ui/compare/v0.6.1...v0.6.2) (2026-05-15)


### Bug Fixes

* **helm:** update appVersion format and improve image tag handling ([709b9f2](https://github.com/Noooste/garage-ui/commit/709b9f2ad33fb3852bc23be1d647bb1d9388169b))

## [0.6.1](https://github.com/Noooste/garage-ui/compare/v0.6.0...v0.6.1) (2026-05-15)


### Bug Fixes

* Helm image tag ([#53](https://github.com/Noooste/garage-ui/issues/53)) ([ff46ff6](https://github.com/Noooste/garage-ui/commit/ff46ff623299461abade27478128f7e5ce409557))

## [0.6.0](https://github.com/Noooste/garage-ui/compare/v0.5.0...v0.6.0) (2026-05-15)


### Features

* add extraEnvs to helm chart to allow config override ([#45](https://github.com/Noooste/garage-ui/issues/45)) ([c8337de](https://github.com/Noooste/garage-ui/commit/c8337de3a885fc86a08a9be329a60c0846e52d62))
* enhance bucket credential retrieval to support read/write operations and improve caching logic ([#46](https://github.com/Noooste/garage-ui/issues/46)) ([c8cb3c4](https://github.com/Noooste/garage-ui/commit/c8cb3c49239bcf21b0abacba86622d55a202c4bd))


### Bug Fixes

* **ci:** correct appVersion tracking and remove changelog seeds ([#49](https://github.com/Noooste/garage-ui/issues/49)) ([f04c381](https://github.com/Noooste/garage-ui/commit/f04c38168d026c9746c5fb9f8182cb9fadc06f53))
* **ci:** remove CHANGELOG.md seeds so release-please owns them ([#51](https://github.com/Noooste/garage-ui/issues/51)) ([cb3839e](https://github.com/Noooste/garage-ui/commit/cb3839e6189ec1d2a6609ff500ab7a79d0f5cbd9))
