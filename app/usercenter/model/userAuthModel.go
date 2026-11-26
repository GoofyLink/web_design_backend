// @Title userAuthModel.go
// @Description 用户中心数据访问扩展与公共约定；生成实现的阅读说明见同目录 doc.go 。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// TODO 编译期检查扩展类型是否实现接口；该声明不会创建数据库连接。
var _ UserAuthModel = (*customUserAuthModel)(nil)

type (
	// UserAuthModel 授权表的数据访问接口，负责平台标识与本地用户的映射。
	UserAuthModel interface {
		userAuthModel // 嵌入生成接口；新增自定义方法应声明在本扩展接口中。
	}

	// customUserAuthModel 授权 Model 的扩展实现，供业务追加查询方法。
	customUserAuthModel struct {
		*defaultUserAuthModel // 嵌入生成的授权表实现，复用 SQL 与缓存操作。
	}
)

// NewUserAuthModel 用共享数据库连接和缓存配置构造 Model ，返回接口供 RPC logic 调用。
// @title NewUserAuthModel
// @description 用共享数据库连接和缓存配置构造 Model ，返回接口供 RPC logic 调用。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param conn sqlx.SqlConn "供 Model 复用的数据库连接。"
// @param c cache.CacheConf "Model 缓存配置。"
// @return result UserAuthModel "初始化后的依赖对象或逻辑对象。"
func NewUserAuthModel(conn sqlx.SqlConn, c cache.CacheConf) UserAuthModel {
	// TODO 包装生成的默认实现；实际 SQL 、缓存失效和事务转发在同名 _gen.go 文件中。
	return &customUserAuthModel{
		defaultUserAuthModel: newUserAuthModel(conn, c),
	}
}
