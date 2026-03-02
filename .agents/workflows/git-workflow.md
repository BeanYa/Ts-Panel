---
description: Git工作流规范，涵盖分支管理、命名规范及文档同步流程。
---

# Git 工作流规范

本工作流根据项目要求制定，用于指导日常开发中的 Git 操作。

## 1. 单纯的文档修改
// turbo
如果改动仅涉及文档（如文字修正、描述补充），可直接提交到 `main` 分支。

1. 确保在 `main` 分支：`git checkout main`
2. 进行文档修改
3. 提交并推送：
   ```bash
git add .
git commit -m "docs: sync documentation"
git push origin main
```

## 2. 功能开发、修复与版本发布
所有非文档类的代码改动必须另建分支开发。

### 2.1 创建分支
// turbo
根据改动类型选择相应的命名前缀（`xxx` 为功能名称或修复内容）：
- **功能开发**: `feature/xxx`
- **问题修复**: `fix/xxx`
- **发布版本**: `release/xxx`
- **紧急修复**: `hotfix/xxx`

```bash
git checkout main
git pull origin main
git checkout -b <type>/<description>
```

### 2.2 开发、测试与文档同步
// turbo
在进行功能开发和修复时，必须遵循 **TDD (测试驱动开发)** 规范：
1. **先写测试**：针对要修改或新增的功能，先在相关 `__tests__` 目录下编写单元测试或集成测试。
2. **运行预期失败**：执行测试，确保新加或修改的测试用例失败（红灯）。
3. **实现功能**：编写代码实现让测试通过（绿灯）。
4. **回归保障**：提交修改前，必须运行并确保所有历史全量测试均绿色通过，保障不破坏现有功能。

在提交修改前，**必须** 检查并更新以下文件（如有需要）：
- `README.md`
- `CONTRIBUTING.md`
确保目录树、模块说明等内容与项目当前状态完全同步。

### 2.3 提交并推送到远程
// turbo
完成开发 and 文档同步后，将改动提交并推送到远程仓库。

> **规则**: 完成需求审查通过后，非 main 分支的改动，直接提交到远程仓库。

```bash
git add .
git commit -m "<type>: <description>"
git push origin <your-branch-name>
```

### 2.4 代码审查与合并
在合并回 `main` 分支前，必须进行 **代码审查**。审查通过后即可合并。

### 2.5 需求确认完成后的合并与切换
// turbo
当需求确认完成后，将开发分支合并回 `main` 分支，并清理本地环境。

```bash
# 1. 切换回 main 分支
git checkout main
# 2. 拉取远程最新代码
git pull origin main
# 3. 合并开发分支（以 feature/xxx 为例）
git merge <your-branch-name>
# 4. 推送合并后的 main 到远程
git push origin main
# 5. (可选) 删除已合并的本地开发分支
git branch -d <your-branch-name>
```
