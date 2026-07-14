# Changelog

## [0.9.0](https://github.com/Noooste/garage-ui/compare/v0.8.5...v0.9.0) (2026-07-11)


### Features

* **backend,frontend:** implement prefix and recursive substring search for bucket objects ([#89](https://github.com/Noooste/garage-ui/issues/89)) ([3098e47](https://github.com/Noooste/garage-ui/commit/3098e474f202f36b527c8636cd1d4e8c08e287d2))
* **oidc:** add fine grained access control ([#91](https://github.com/Noooste/garage-ui/issues/91)) ([0d804fb](https://github.com/Noooste/garage-ui/commit/0d804fbd39ed656511b671d237ad14fc8c21786d))

## [0.8.5](https://github.com/Noooste/garage-ui/compare/v0.8.4...v0.8.5) (2026-07-03)


### Bug Fixes

* **frontend:** add downloadObject function for downloading files from a bucket ([430d1a5](https://github.com/Noooste/garage-ui/commit/430d1a5d68e4f9915a2951d5b31c1b5ea56cb24c))

## [0.8.4](https://github.com/Noooste/garage-ui/compare/v0.8.3...v0.8.4) (2026-06-22)


### Bug Fixes

* **helm:** track appVersion in release-please and fix badges ([1ffdfe8](https://github.com/Noooste/garage-ui/commit/1ffdfe83c884dc5e39d071343d5269164746c536))

## [0.8.3](https://github.com/Noooste/garage-ui/compare/v0.8.2...v0.8.3) (2026-06-21)


### Bug Fixes

* **backend:** improve API version detection with retry logic for health probes ([46aa375](https://github.com/Noooste/garage-ui/commit/46aa3752c81788787388d1c67e29cef786bdabff))
* **helm:** update version badges in README for Garage UI ([28c186f](https://github.com/Noooste/garage-ui/commit/28c186f3eba1a1c111100712f1eaec22a5d18eb2))

## [0.8.2](https://github.com/Noooste/garage-ui/compare/v0.8.1...v0.8.2) (2026-06-07)


### Bug Fixes

* **backend:** prevent OIDC login loop from empty cookie name ([#76](https://github.com/Noooste/garage-ui/issues/76)) ([22be89b](https://github.com/Noooste/garage-ui/commit/22be89b2ff86465abb90dab0344ef9366ab181b3))

## [0.8.1](https://github.com/Noooste/garage-ui/compare/v0.8.0...v0.8.1) (2026-05-31)


### Bug Fixes

* **frontend:** align three-dot menu item icon spacing and text alignment ([#72](https://github.com/Noooste/garage-ui/issues/72)) ([45f8770](https://github.com/Noooste/garage-ui/commit/45f87707996e92d0f8f75e79c8f60a13556eaf6e))

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
