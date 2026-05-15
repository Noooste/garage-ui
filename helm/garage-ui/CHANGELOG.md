# Changelog

## [0.4.1](https://github.com/Noooste/garage-ui/compare/garage-ui-chart-v0.4.0...garage-ui-chart-v0.4.1) (2026-05-15)


### Bug Fixes

* Helm image tag ([#53](https://github.com/Noooste/garage-ui/issues/53)) ([ff46ff6](https://github.com/Noooste/garage-ui/commit/ff46ff623299461abade27478128f7e5ce409557))

## [0.4.0](https://github.com/Noooste/garage-ui/compare/garage-ui-chart-v0.3.0...garage-ui-chart-v0.4.0) (2026-05-15)


### Features

* add extraEnvs to helm chart to allow config override ([#45](https://github.com/Noooste/garage-ui/issues/45)) ([c8337de](https://github.com/Noooste/garage-ui/commit/c8337de3a885fc86a08a9be329a60c0846e52d62))
* add initial Helm chart for Garage UI deployment ([067e00b](https://github.com/Noooste/garage-ui/commit/067e00b474cb16ea81cc1b3b0584627e2e86fae6))
* add JWT key generator job and associated RBAC resources; create Kubernetes secret for JWT private key ([27cca27](https://github.com/Noooste/garage-ui/commit/27cca273d5eb3bba4a9973e684d27d4eb8748ec7))
* add namespace to metadata in Kubernetes resource files ([98842d7](https://github.com/Noooste/garage-ui/commit/98842d7c7e1fd2a4a89b05412964234e851328ba))
* add support for using Kubernetes secrets for admin token management ([d36f891](https://github.com/Noooste/garage-ui/commit/d36f8915acc0085c9e8c5fb7a28c895bb3ba687a))
* bump Helm chart version to 0.1.6 and update endpoint pattern in values schema ([7aa5aeb](https://github.com/Noooste/garage-ui/commit/7aa5aeb4f8355cce0d046a6fd384559fb6f3ae26))
* bump version to 0.1.10; update version badge in README ([bde726f](https://github.com/Noooste/garage-ui/commit/bde726fedff589f5aab034ce1ed35a53980f7f3e))
* bump version to 0.1.11 ([b8056c6](https://github.com/Noooste/garage-ui/commit/b8056c6d79754273dddaf0c0c5e4b1a10440d6a7))
* bump version to 0.1.4 and appVersion to 0.0.10; update authentication secrets in deployment and values ([62b7676](https://github.com/Noooste/garage-ui/commit/62b76769d0128faf8924cb05dfb2f8525f41b09d))
* bump version to 0.1.7 and appVersion to 0.0.11; update version badges in README ([7b31490](https://github.com/Noooste/garage-ui/commit/7b314904884b9031c68aa13bcfcddd9cf99ea2d2))
* bump version to 0.1.8 and appVersion to 0.1.0; update version badges in README ([f5deb03](https://github.com/Noooste/garage-ui/commit/f5deb0316b0d76255c06112d9eb9e6c94b1fcdd2))
* bump version to 0.1.9; update JWT key handling in deployment and secret management; enhance README with JWT key management instructions ([c1a1d64](https://github.com/Noooste/garage-ui/commit/c1a1d64928940294412db87ffdd6de817868b4d6))
* enhance admin role checks to support multiple roles configuration ([#43](https://github.com/Noooste/garage-ui/issues/43)) ([ff3977a](https://github.com/Noooste/garage-ui/commit/ff3977aeb532fcd284fbd65b3944e277721068f6))
* enhance authentication and upload features; add JWT support and upload progress component ([80553c6](https://github.com/Noooste/garage-ui/commit/80553c64500df66a13f6d8c9b9835fdfce679c84))
* enhance S3 service to use bucket-specific MinIO client for object retrieval and deletion ([b8b0b6b](https://github.com/Noooste/garage-ui/commit/b8b0b6b0fa961bc1d1392ab4421ee85931cd0dc0))
* implement admin and OIDC authentication methods; add login and user info endpoints ([61dae6c](https://github.com/Noooste/garage-ui/commit/61dae6c605bfc29a2a79bb2c5d485041118a3b49))
* refactor JWT key generation to create a Kubernetes secret directly; remove job and RBAC resources ([65447b9](https://github.com/Noooste/garage-ui/commit/65447b927a72fcb89d32daa2de2cc4215ba2e8cc))
* update Helm chart version to 0.1.5 and add values schema for configuration ([b4bdd13](https://github.com/Noooste/garage-ui/commit/b4bdd1374394780aa19d8bdf1905ec137cdbcf0e))
* update README version badge to 0.1.6 ([4864185](https://github.com/Noooste/garage-ui/commit/48641851838c910f50a0c772edf97f93ae848431))


### Bug Fixes

* **ci:** remove CHANGELOG.md seeds so release-please owns them ([#51](https://github.com/Noooste/garage-ui/issues/51)) ([cb3839e](https://github.com/Noooste/garage-ui/commit/cb3839e6189ec1d2a6609ff500ab7a79d0f5cbd9))
* update appVersion to v0.0.4 in Chart.yaml ([983e6e5](https://github.com/Noooste/garage-ui/commit/983e6e5fa92788deda70000e28b594f4d7f65052))
