# 发布记录

## v1.0.1（2026-10-17）

修复 DIAN115 5.x 安装兼容性。

- DIAN115 兼容范围：`>=3.8.51 <6.0.0`（支持 5.0.21）
- 功能、权限和发布者签名身份不变

SHA-256: `b79f41d9470cd78e719ed8f116a11f900ef94f0eaf6aacc1c6a9149f34bae9d9`

## v1.0.0（2026-10-17）

首个正式版本，适配自 MoviePilot autosignin 3.0.3。

- 内置 22 个站点预设，通用签到走 NexusPHP attendance.php 流程
- 三种调度模式（cron / 周期窗口 / 纯周期 / 随机）
- 签到证据严格校验，模拟登录保号，通知与历史记录
- Go WASM 运行时 + Vue Federation 配置页，宿主 Storage 持久化

SHA-256: `c55bd0b753c1a88a58a4f6ce3cd8d1fde0b81dfbca81b6a65e6fc27b9a54fb88`