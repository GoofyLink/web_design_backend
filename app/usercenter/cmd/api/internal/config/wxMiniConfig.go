// @Title wxMiniConfig.go
// @Description 声明 API 配置结构，字段与启动时读取的 YAML 对应。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package config

// WxMiniConf 保存微信小程序授权需要的应用配置。
type WxMiniConf struct {
	AppId  string `json:"AppId"`  // 微信小程序的应用标识。
	Secret string `json:"Secret"` // 微信小程序的应用密钥。
}
