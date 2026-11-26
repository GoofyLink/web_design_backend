# B4 编辑器保存协议 v1

本阶段只交付后端。目标引擎为 Fabric 6，实测序列化版本 6.7.1；不直接接受所有 yft 私有扩展对象。未知类型返回 400，前端不得静默丢弃不支持的对象后保存。

## Content

```json
{
  "width": 600,
  "height": 800,
  "engine": "fabric",
  "engineVersion": "6.7.1",
  "schemaVersion": 1,
  "coverAssetId": "123",
  "document": {
    "version": "6.7.1",
    "objects": [
      {"type": "Image", "assetId": "123", "src": "asset:123", "left": 10, "top": 20},
      {"type": "Textbox", "text": "你好", "fontAssetId": "456", "fontFamily": "asset-font-456", "fontSize": 32}
    ]
  }
}
```

coverAssetId 可省略；提供时必须是可访问的启用图片。模板 Content 还包含 title/categoryId/tags/cover；作品 Content 不包含这些字段，标题位于作品外层。单边 ≤8192、总像素 ≤4000 万、文档 ≤5 MiB、对象 ≤2000、嵌套深度 ≤32。空白作品允许 objects=[]。

允许 rect/circle/ellipse/triangle/line/path/polygon/polyline/text/textbox/i-text/itext/image/group，类型忽略大小写；允许 linear/radial 渐变、分组 layoutManager（fit-content/fixed/clip-path）。clipPath 中的资源同样验证并记录引用。暂不支持 Pattern、滤镜、视频及业务自定义对象。

图片必须有字符串 assetId，src 只允许省略、空值或 asset:ID。字体使用 fontAssetId 和 fontFamily=asset-font-ID；系统字体可用 sans-serif/serif/monospace/Arial/Times New Roman/Courier New，无需 fontAssetId。中文字体由管理员上传有合法授权的 TTF/OTF；系统字体存在与否不能由后端保证。

## 浏览器适配流程

1. 上传图片得到 assetId；字体通过 GET /api/v1/fonts 取得 ID 和 fontFamily。
2. 保存前用 Fabric 的自定义序列化字段保留 assetId/fontAssetId，把图片 src 转成 asset:ID。禁止保存 ticket URL、blob URL、data URL 或外链。只修改保存副本，保持当前画布可显示。
3. 打开作品后复制 document，递归收集图片/字体 ID（含 group、clipPath、背景图片）。调用对应 access-url 接口，替换工作副本中的图片 src；用 FontFace 加载字体，等待 document.fonts.ready，再 loadFromJSON。
4. 已经加载到画布的文件不因 ticket 到期消失；重载或失败重试时重新获取 access-url。ticket 60 秒有效，仅授权一个文件，退出/封禁/会话到期后失效。不要将其当登录 token 使用。
5. 作品保存携带最近读取的 version；成功更新本地 version。自动保存须串行，断网保留本地草稿；409 时提示刷新/另存副本，不能自动覆盖。
6. 模板预览接口返回草稿及发布内容，浏览器用同一适配器渲染；后端不生成预览图片。模板封面需先上传公共图片，再提交 coverAssetId 与对应 canonical cover 路径。

服务端不会下载 document 中的 URL。JSON 外链、source、href、URL 类字段均拒绝；text 字段允许显示普通链接文字。删除素材前查引用，存在作品（包括回收站）、模板草稿、发布快照或历史发布修订时拒绝删除/停用。

## 验证记录

- `internal/modules/canvas/testdata/fabric-6.7.1.json` 由真实 fabric@6.7.1 的 Rect/Circle/Group/Gradient.toObject() 生成，验证了分组布局字段、渐变及 JSON 往返。
- HTTP 集成测试覆盖 Image 的资源 ID、文字字体引用、封面、保存读回、授权、并发和模板套用。
- 尚未验证完整 yft 浏览器操作、中文字体实际排版、Canvas 导出与像素一致性。这些属于前端接入和 B5 验收，不能从后端测试推定已通过。
