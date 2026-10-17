# 第三方声明

## MoviePilot autosignin

本插件为 [MoviePilot](https://github.com/jxxghp/MoviePilot) 内置插件 autosignin v3.0.3 的 DIAN115 移植版。

- 原作者：jxxghp 及 MoviePilot 社区贡献者
- 原始许可：MoviePilot 项目许可（见上游仓库）
- 移植方式：Go 语言重写调度与站点适配逻辑，保留站点签到 URL、判定文案与证据校验语义；部分功能（OCR、OpenAI 答题）未迁移

## 运行时与前端

- Go（标准库，无第三方依赖）
- Vue 3 / naive-ui / @lucide/vue（由 DIAN115 宿主共享提供）
- Vite + @originjs/vite-plugin-federation（联邦模块构建）
