# 🤝 贡献指南

我们非常欢迎您为 long-gate 做出贡献！无论是提交 Bug 报告、功能建议还是代码提交，都将帮助我们把项目做得更好。

## 报告 Bug

在提交 Bug 报告之前，请先搜索现有的 Issues，以确保该 Bug 尚未被报告。

提交 Bug 报告时，请包含以下信息：

1.  **环境信息:** Go 版本、操作系统。
2.  **重现步骤:** 清晰详细地描述如何重现问题。
3.  **预期结果:** 描述您期望的行为。
4.  **实际结果:** 描述实际发生的错误或行为。

## 提交 Pull Request (PR)

1.  **Fork** 本仓库到您的 GitHub 账户。
2.  **克隆** 您 Fork 的仓库到本地。
3.  从 `main` 分支创建一个新的特性分支 (`git checkout -b feature/your-feature-name`)。
4.  确保您的代码通过了所有测试，并且没有引入新的 Lint 警告。
5.  提交您的更改 (`git commit -m "feat: A brief description of your change"`)。请使用清晰的 Commit 消息。
6.  将您的分支推送到 Fork 的仓库 (`git push origin feature/your-feature-name`)。
7.  在 GitHub 上发起一个 **Pull Request** 到 `long-gate` 的 `main` 分支。

## 编码规范

* 请遵循 Go 语言的惯例，例如 `go fmt` 和 `go vet`。
* 所有公共函数和结构体都需要提供清晰的 **Go Doc** 文档。
* 关键功能变动需要包含相应的**单元测试**。

---