# 发布记录

## v1.0.0（2026-10-17）

首个正式版本，适配自 MoviePilot autosignin 3.0.3。

- 内置 22 个站点预设，通用签到走 NexusPHP attendance.php 流程
- 三种调度模式（cron / 周期窗口 / 纯周期 / 随机）
- 签到证据严格校验，模拟登录保号，通知与历史记录
- Go WASM 运行时 + Vue Federation 配置页，宿主 Storage 持久化

SHA-256: `c55bd0b753c1a88a58a4f6ce3cd8d1fde0b81dfbca81b6a65e6fc27b9a54fb88`
