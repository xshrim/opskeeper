# 图标系统设计

## 目标

项目使用一套统一的图标值和渲染入口，同时满足通用界面图标、引入图标和用户上传图片三类需求：

1. Lucide 负责保存、编辑、删除、展开、状态等通用界面图标。
2. simple-icons 负责已有的基础设施和厂商图标。
3. 当 Lucide 和 simple-icons 都没有对应图标时，按需使用 theSVG 单色图标补充。
4. 本地上传图片继续作为图标来源，并保持现有 data URL 兼容。

不使用 theSVG Color。品牌图标优先保持单色和主题可控，避免彩色 Logo 在深色主题下产生对比度问题。

## 命名空间

命名空间是图标值的来源前缀，不是资源 scope、权限域或数据库表。它让渲染器可以判断应该使用哪一种图标数据和渲染方式。

| 来源 | 值格式 | 示例 |
| --- | --- | --- |
| Lucide | `lucide:<name>` | `lucide:Cpu` |
| simple-icons | `iconify:simple-icons:<name>` | `iconify:simple-icons:github` |
| theSVG | `iconify:thesvg:<name>` | `iconify:thesvg:openai-chatgpt` |
| 本地图片 | 现有 `data:image/...` | `data:image/png;base64,...` |

所有默认值和持久化图标值都直接使用命名空间形式，不保留无前缀或 `brand:` 解析分支。资源目录中历史上的 `application`、`metrics` 等语义图标键也会在迁移时转换为对应的 `lucide:` 或 `iconify:` 值。命名空间只作用于保存到图标字段的值，不改变 `provider_type`、资源 `kind` 或显示名称。

## 注册与按需引入

`frontend/src/lib/iconifyIcons.ts` 是引入图标的唯一注册表。每个实际使用的图标只增加一个静态导入和一条注册项：

- simple-icons 使用命名导入，并保存品牌的 `path`。
- theSVG 使用单图标模块，并保存 Iconify 的 `body`、尺寸等数据。
- `IconValue.svelte` 根据注册项来源渲染 simple-icons path 或 theSVG SVG body。
- `BrandIcon.svelte`、`ResourceBrandIcon.svelte`、`ProviderBrandIcon.svelte` 和 `IconPicker.svelte` 共享这张注册表。

不要导入完整的图标 JSON，不要根据用户输入动态加载任意图标，也不要让浏览器运行时访问 Iconify CDN。新增图标属于构建时变更，需要重新构建前端后才会出现在图标选择器中。

## 图标选择器

`IconPicker.svelte` 提供三个分类：

- 通用图标：Lucide 图标，选择后保存为 `lucide:<name>`。
- 引入图标：已注册的 simple-icons 和 theSVG 图标，选择后保存为对应的 `iconify:` 值。
- 本地图片：通过文件选择器读取为现有 `data:image/...` 值。

搜索支持 Lucide 名称、图标名称、别名和注册值。只有已注册的引入图标会显示在引入图标列表中。

## 主题与渲染

simple-icons 和 theSVG 单色图标统一使用 `currentColor` 或外层 `color`，因此可以跟随项目的浅色、深色主题和交互状态变化。品牌图标的路径仍然是品牌资产，不得在组件中复制或手工重绘。

如果未来确实需要彩色品牌 Logo，应单独评估主题对比度、明暗变体和许可证，再扩展图标来源；本版本不引入 theSVG Color。

## 依赖与许可证

- `lucide-svelte`：通用 UI 图标。
- `simple-icons`：已使用的品牌图标，按需命名导入。
- `@iconify/svelte` 与 `@iconify-icons/thesvg`：theSVG 单色补充图标，按需单图标导入。

各依赖的许可证和品牌商标声明随 npm 包保留。新增品牌前应确认图标存在于目标集合，并避免用相近品牌替代缺失图标；确实缺少时使用明确的文字字标或通用 fallback。
