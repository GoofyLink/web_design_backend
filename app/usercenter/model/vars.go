// @Title vars.go
// @Description 用户中心数据访问扩展与公共约定；生成实现的阅读说明见同目录 doc.go 。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package model

import (
	"errors"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// TODO 表示查询没有匹配记录；与数据库连接失败等错误区分。
var ErrNotFound = sqlx.ErrNotFound

// TODO 表示预期更新没有影响记录，可用于判断版本冲突等情况。
var ErrNoRowsUpdate = errors.New("update db no rows change")

// TODO 标识手机号和密码登录，授权键使用手机号。
var UserAuthTypeSystem string = "system" // 平台内部登录类型。
// TODO 标识微信小程序授权，授权键使用 OpenID 。
var UserAuthTypeSmallWX string = "wxMini" // 微信小程序登录类型。
