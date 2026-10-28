# 发布记录

## v1.2.0（2026-10-28）

- UI 迁移到官方独立页面模式（ui.mode=page，standalone HTML + connectPluginPage SDK），Module Federation 已废弃移除
- 兼容范围调整为 `>=5.0.46 <6.0.0`（页面模式最低宿主要求）
- 包内 index.html 纳入完整性清单与签名

SHA-256: `e4d35294e3f7e1cdc44412f62cd4dc591ec7d7c7c5d915af88c94ead57f94705`

## v1.1.1（2026-10-17）

- 修正运行时上报版本号（此前所有构建 UI 均显示 v1.0.2）

SHA-256: `1afec4a0797e777c3ab672bed62eece2e771a0daf31d9b020f00bf859fc5db14`

## v1.1.0（2026-10-17）

支持宿主托管凭据（credential_ref），PT Cookie 不再需要明文存进插件配置。

- manifest `permissions.network` 声明 22 个内置 PT 站点 origin，宿主据此为该安装实例启用托管凭据能力
- 配置页每个站点支持填入 `cred_xxx`（插件中心 → 本安装实例 → 托管凭据绑定创建后返回）；与直填 Cookie 二选一，优先托管凭据，Cookie 由宿主加密保管、请求时自动注入，不进插件存储与日志
- 运行时 Go 链路此前已内置支持，本次为 manifest 授权与配置页提示完善；自定义站点仍需直填 Cookie

SHA-256: `52129faf8b7f9bdabeb546fd0531ce89cc474d8e0dd84dac09e9a4b182e37e14`

## v1.0.3（2026-10-17）

修复宿主运行时阶段 `plugin_incompatible`（安装检查通过后运行时报错）。

- 根因：TinyGo 0.43 wasip1 默认 asyncify 调度器会让模块多导出 `malloc/free/calloc/realloc/fmin*/fmax*/asyncify_*`；宿主 5.x 只接受精确导出集合 `[memory, _initialize, dian115_alloc, dian115_handle]`（对照真机可用的社区插件 douban115 验证）
- 修复：构建后自动剥离多余导出（scripts/strip-exports.mjs），导出集合与契约完全一致；check-wasm 增加导出集合断言防止回归
- 模块 1.16 MiB，满足 2 MiB 上限；wazero（宿主同款引擎）探针验证 initialize/state/action/shutdown 全部正常
- 功能、权限、兼容范围（>=3.8.51 <6.0.0）与发布者签名身份不变

SHA-256: `fc7ab0e273c4d416c1b8dcbb27dc8b9e220fd16199cbd047efb4381706f32310`

## v1.0.2（2026-10-17）

按宿主构建要求重写 WASM 运行时。

- 运行时改用 TinyGo（-opt=z -no-debug）构建，模块 1.16 MiB，满足宿主 2 MiB 上限
- 移除 encoding/json，改为平台参考实现 pluginjson 做字段级读写（宿主点名拒绝链接 encoding/json 的模块）
- 功能、权限、兼容范围（>=3.8.51 <6.0.0）与发布者签名身份不变

SHA-256: `48a405510199274944932dbc6f163bee2536570172c381df0a1f9939cdbb0580`

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