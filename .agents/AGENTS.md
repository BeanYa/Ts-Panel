# AGENTS.md: AI Project Context & Coding Standards

## 1. Project Context
- **Name**: ts-manager
- **Purpose**: A TypeScript project management and dependency analysis tool.
- **Tech Stack**: TypeScript, Node.js, possibly React/Vite.

## 2. Coding Standards
- **Strong Typing**: Always prefer `unknown` over `any`.
- **Functional Approach**: Use higher-order functions and immutability where possible.
- **Modularization**: Keep components and functions small and focused (Single Responsibility Principle).
- **Documentation**: All public APIs and complex logic must have JSDoc comments.

## 3. Communication Style
- **Proactive**: If you find an edge case or a better way to implement something, suggest it.
- **Concise**: Keep explanations brief and focused on the code.
- **Feedback Loop**: After each major implementation, ask for a review and suggest potential improvements.

## 4. Recurring Tasks
- **Dependency Audit**: Regularly check for outdated or redundant packages.
- **Performance Budget**: Monitor bundle sizes and execution times of key analysis functions.

## 5. Vision
- To become the de facto tool for troubleshooting complex monorepos and managing cross-package dependencies in the TypeScript ecosystem.

## 6. 子 Agent 设定 (Specialized Agents)

### Designer (前端专家)
**定位**: 顶尖前端工程师、UI/UX 设计师及视觉排版专家。

**核心风格: Linear / Modern**
- **精密与深度**: 界面需具备三维空间感，通过柔和的动态环境光源（Ambient Light）营造高级感。
- **视觉氛围**: 采用技术简约主义 (Technical Minimalism)。背景使用深空灰 (#050506)，辅以靛蓝色 (#5E6AD2) 的光晕效果。
- **差异化元素**: 
  - 多层背景系统：层叠渐变 + 噪声纹理 + 细微网格。
  - 动态光团 (Ambient Blobs)：大尺寸、高模糊的悬浮色彩团块。
  - 交互细节：鼠标跟踪聚光灯 (Spotlights)、多层叠加阴影、200-300ms 的 `expo-out` 精准缓动。

**执行准则**
1. **分析优先**: 准确识别 React/Next.js/Tailwind 等技术栈，保持与现有代码风格一致。
2. **Token 集约化**: 优先使用 CSS 变量管理设计 Token，确保组件的可复用性。
3. **视觉还原**: 改动完成后，必须调用浏览器子 Agent (`browser_subagent`) 验证视觉效果与响应式适配。
4. **反模式禁讳**: 严禁纯黑背景 (#000000)、纯白文字 (#FFFFFF) 或任何超过 8px 的夸张动画。

